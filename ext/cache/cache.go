// Package cache provides cache/v1 from one store per process. Each app gets its own provider, a view that keeps its
// buckets apart from every other app's (keys live under "<app id>/<bucket>"), so on a multisite host every instance
// shares one store and its disk tier. In memory every key holds its own value (bytes or a Go value); on disk the same
// bytes under any number of keys, from any number of instances, are one file.
//
// Each bucket has its own budget (entries, memory, disk), from its code and pocketone.yaml's `cache` section
// (`defaults`, `buckets`), with built-in defaults so none is unbounded. The store's own keys are ceilings over every
// bucket together, and turn on the disk tier.
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"gopkg.in/yaml.v3"

	clearv1 "github.com/pockethost/pocketone/ext/contracts/cache/clear/v1"
	cachev1 "github.com/pockethost/pocketone/ext/contracts/cache/v1"
	"github.com/pockethost/pocketone/ext/extkit"
)

// Name is the extension's key: its pocketone.yaml section and an instance's `extensions.<name>`.
const Name = "cache"

// Built-in limits, for what the config doesn't set.
const (
	DefaultMaxMemory       = 128 << 20 // the store: 128 MiB for every app's buckets together
	DefaultMaxDisk         = 1 << 30   // the store: 1 GiB disk tier, when there is one
	DefaultBucketMaxMemory = 32 << 20  // one bucket
	DefaultBucketMaxDisk   = 256 << 20 // one bucket
)

// BucketConfig caps one app's bucket. Unset (zero) fields keep what the code asked for.
type BucketConfig struct {
	Eviction  cachev1.Eviction `yaml:"eviction,omitempty" json:"eviction,omitempty" doc:"Which of the bucket's entries go first when it's full: lru, mru, fifo or lifo."`
	MaxItems  int              `yaml:"max_items,omitempty" json:"max_items,omitempty" doc:"Most entries the bucket holds."`
	MaxMemory Size             `yaml:"max_memory,omitempty" json:"max_memory,omitempty" doc:"Most bytes of keys and values the bucket holds in memory, such as 32MiB (the default)."`
	MaxDisk   Size             `yaml:"max_disk,omitempty" json:"max_disk,omitempty" doc:"Most bytes of disk files the bucket's keys point at, each counted in full, such as 256MiB (the default)."`
	TTL       Duration         `yaml:"ttl,omitempty" json:"ttl,omitempty" doc:"How long an entry lives unless the code says otherwise, such as 10m."`
}

// Config is the `cache` section of pocketone.yaml. On a multisite host an instance's `extensions.cache` may set
// `enabled`, `defaults` and `buckets` for that instance; the store's own keys are the host's only.
type Config struct {
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty" doc:"Turns the cache off when false; extensions then run uncached."`

	// The store, shared by every app in the process: ceilings over every bucket together.
	Eviction  cachev1.Eviction `yaml:"eviction,omitempty" json:"eviction,omitempty" doc:"Which values leave memory (or disk) first when the store reaches its ceiling: lru, mru, fifo or lifo."`
	MaxMemory Size             `yaml:"max_memory,omitempty" json:"max_memory,omitempty" doc:"Memory for every bucket together, such as 512MiB. Default 128MiB."`
	Disk      *bool            `yaml:"disk,omitempty" json:"disk,omitempty" doc:"Turns on a disk tier in pb_cache/cache, next to pb_data (on a multisite host, the host's own pb_cache)."`
	DiskPath  string           `yaml:"disk_path,omitempty" json:"disk_path,omitempty" doc:"Puts the disk tier in a cache folder there instead of pb_cache, and turns it on. Relative: to the app's folder (the one holding pb_data) when the app has the process to itself, the working directory when several apps share it (Pocker)."`
	MaxDisk   Size             `yaml:"max_disk,omitempty" json:"max_disk,omitempty" doc:"Size of the disk tier for every bucket together, such as 10GiB. Default 1GiB."`

	// Each app's buckets.
	Defaults BucketConfig            `yaml:"defaults,omitempty" json:"defaults,omitempty" doc:"Caps for any bucket whose code leaves them unset."`
	Buckets  map[string]BucketConfig `yaml:"buckets,omitempty" json:"buckets,omitempty" doc:"Caps per bucket name, over what its code asks for."`
}

// Section is the key of the `cache` section (pocketone.yaml).
var Section = extkit.NewSectionFunc(Name, func() Config { return Config{} },
	"The cache (cache/v1): one store for every app in the process (memory, disk tier), and caps per bucket.")

// Extension provides cache/v1 and cache.clear/v1 as defaults, so another provider (Redis, say) replaces them. One
// Extension value is one store: every app it's registered on shares it.
type Extension struct {
	cfg Config

	once     sync.Once
	store    *store
	storeErr error
}

var (
	_ extkit.Extension = (*Extension)(nil)
	_ extkit.Closer    = (*Extension)(nil)
)

// New returns the extension. cfg sits under pocketone.yaml's section (and, for bucket caps, an instance's settings).
func New(cfg Config) *Extension { return &Extension{cfg: cfg} }

// Name implements [extkit.Extension].
func (x *Extension) Name() string { return Name }

// Close implements [extkit.Closer]: it waits for the disk tier's queued writes and removals, once every app has
// stopped.
func (x *Extension) Close(context.Context) error {
	if x.store != nil {
		x.store.close()
	}
	return nil
}

// Clear empties the whole store: every bucket of every app in the process, in memory, on disk and in the
// On a multisite host it's the host's to call (an operator command), never an instance's.
func (x *Extension) Clear(context.Context) error {
	if x.store != nil {
		x.store.clearPrefix("")
	}
	return nil
}

// Contracts implements [extkit.Declarer].
func (x *Extension) Contracts() extkit.Contracts {
	return extkit.Contracts{Implements: extkit.IDs("cache/v1", "cache.clear/v1"), Defaults: extkit.IDs("cache/v1", "cache.clear/v1")}
}

// Register implements [extkit.Extension]: it provides cache/v1 and cache.clear/v1 for app, a view of the process's
// store.
func (x *Extension) Register(app core.App, layers extkit.Config) error {
	c := x.cfg
	if s, found := Section.Get(); found {
		c = merge(c, s)
	}
	st, err := x.storeFor(app, c)
	if err != nil {
		return err
	}
	var l Config
	if err := layers.Decode(&l); err != nil {
		return fmt.Errorf("cache: %w", err)
	}
	c = merge(c, Config{Defaults: l.Defaults, Buckets: l.Buckets}) // an instance sets its own caps, never the store's
	if err := c.validate(); err != nil {
		return err
	}
	p := &provider{store: st, app: extkit.AppID(app), cfg: c, buckets: map[string]*bucket{}}
	cachev1.Contract.ProvideDefault(app, p)
	clearv1.Contract.ProvideDefault(app, clearer{p})
	return nil
}

// storeFor opens the store once, on the first app registered.
func (x *Extension) storeFor(app core.App, c Config) (*store, error) {
	x.once.Do(func() {
		if err := c.validate(); err != nil {
			x.storeErr = err
			return
		}
		lim := poolLimits{eviction: c.Eviction, maxMem: int64(c.MaxMemory), maxDisk: int64(c.MaxDisk)}
		if lim.eviction == "" {
			lim.eviction = cachev1.LRU
		}
		if lim.maxMem <= 0 {
			lim.maxMem = DefaultMaxMemory
		}
		if lim.maxDisk <= 0 {
			lim.maxDisk = DefaultMaxDisk
		}
		lim.dir, x.storeErr = diskDir(app, c)
		if x.storeErr != nil {
			return
		}
		x.store, x.storeErr = newStore(lim, slog.Default().With("extension", Name)) // the process's, not one app's: it outlives them
	})
	return x.store, x.storeErr
}

// diskDir is the disk tier's folder, which the store owns and empties when it opens: "" for none, pb_cache/cache by
// default (the host's, for a store every instance shares), or a cache folder in disk_path.
func diskDir(app core.App, c Config) (string, error) {
	on := c.DiskPath != "" || (c.Disk != nil && *c.Disk)
	if c.Disk != nil && !*c.Disk {
		on = false
	}
	if !on {
		return "", nil
	}
	if c.DiskPath == "" {
		return extkit.HostCacheDir(app, Name) // the store is the process's, so the host's pb_cache
	}
	d := c.DiskPath
	if !filepath.IsAbs(d) && extkit.AppID(app) == "" {
		d = filepath.Join(filepath.Dir(filepath.Clean(app.DataDir())), d) // the app has the process to itself
	}
	d, err := filepath.Abs(filepath.Join(d, Name)) // a folder of its own: the store empties it
	if err != nil {
		return "", err
	}
	return d, os.MkdirAll(d, 0o755)
}

// merge lays over's set fields onto base.
func merge(base, over Config) Config {
	if over.Enabled != nil {
		base.Enabled = over.Enabled
	}
	if over.Eviction != "" {
		base.Eviction = over.Eviction
	}
	if over.MaxMemory != 0 {
		base.MaxMemory = over.MaxMemory
	}
	if over.Disk != nil {
		base.Disk = over.Disk
	}
	if over.DiskPath != "" {
		base.DiskPath = over.DiskPath
	}
	if over.MaxDisk != 0 {
		base.MaxDisk = over.MaxDisk
	}
	base.Defaults = base.Defaults.over(over.Defaults)
	if len(over.Buckets) > 0 {
		m := make(map[string]BucketConfig, len(base.Buckets)+len(over.Buckets))
		for k, v := range base.Buckets {
			m[k] = v
		}
		for k, v := range over.Buckets {
			m[k] = m[k].over(v)
		}
		base.Buckets = m
	}
	return base
}

func (c Config) validate() error {
	bad := func(where string, e cachev1.Eviction) error {
		return fmt.Errorf("cache: %s: unknown eviction %q (lru, mru, fifo, lifo)", where, e)
	}
	if !c.Eviction.Valid() {
		return bad("store", c.Eviction)
	}
	if !c.Defaults.Eviction.Valid() {
		return bad("defaults", c.Defaults.Eviction)
	}
	for n, b := range c.Buckets {
		if err := cachev1.ValidName(n); err != nil {
			return err
		}
		if !b.Eviction.Valid() {
			return bad("bucket "+n, b.Eviction)
		}
	}
	return nil
}

// over returns b with o's set fields.
func (b BucketConfig) over(o BucketConfig) BucketConfig {
	if o.Eviction != "" {
		b.Eviction = o.Eviction
	}
	if o.MaxItems != 0 {
		b.MaxItems = o.MaxItems
	}
	if o.MaxMemory != 0 {
		b.MaxMemory = o.MaxMemory
	}
	if o.MaxDisk != 0 {
		b.MaxDisk = o.MaxDisk
	}
	if o.TTL != 0 {
		b.TTL = o.TTL
	}
	return b
}

// provider is one app's cache/v1: a view of the store under the app's id.
type provider struct {
	store *store
	app   string // extkit.AppID: "" when the app has the process to itself
	cfg   Config

	mu      sync.Mutex
	buckets map[string]*bucket
}

var _ cachev1.Cache = (*provider)(nil)

// Bucket implements cachev1.Cache. Limits, weakest first: the built-in defaults, the config's defaults, the code's
// options, the config's entry for this bucket.
func (p *provider) Bucket(name string, opts cachev1.Options) (cachev1.Bucket, error) {
	if err := cachev1.ValidName(name); err != nil {
		return nil, err
	}
	if !opts.Eviction.Valid() {
		return nil, fmt.Errorf("cache: bucket %s: unknown eviction %q (lru, mru, fifo, lifo)", name, opts.Eviction)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if b, ok := p.buckets[name]; ok {
		return b, nil
	}
	code := BucketConfig{Eviction: opts.Eviction, MaxItems: opts.MaxItems, MaxMemory: Size(opts.MaxMemory), TTL: Duration(opts.TTL)}
	builtin := BucketConfig{MaxMemory: DefaultBucketMaxMemory, MaxDisk: DefaultBucketMaxDisk}
	bc := builtin.over(p.cfg.Defaults).over(code).over(p.cfg.Buckets[name])
	lim := spaceLimits{eviction: bc.Eviction, maxItems: bc.MaxItems, maxMem: int64(bc.MaxMemory), maxDisk: int64(bc.MaxDisk), ttl: time.Duration(bc.TTL)}
	if lim.eviction == "" {
		lim.eviction = cachev1.LRU
	}
	b := &bucket{store: p.store, sp: p.store.open(p.app+"/"+name, lim)}
	p.buckets[name] = b
	return b, nil
}

// Clear implements cachev1.Cache: every bucket of this app, opened since it started or not.
func (p *provider) Clear(context.Context) error {
	p.store.clearPrefix(p.app + "/")
	return nil
}

// clearer is an app's cache.clear/v1.
type clearer struct{ p *provider }

var _ clearv1.Clearer = clearer{}

// Clear implements clearv1.Clearer: one of this app's buckets, or all of them.
func (c clearer) Clear(_ context.Context, _ extkit.Auth, in clearv1.ClearInput) (clearv1.ClearResult, error) {
	if in.Bucket == "" {
		return clearv1.ClearResult{Keys: c.p.store.clearPrefix(c.p.app + "/")}, nil
	}
	if err := cachev1.ValidName(in.Bucket); err != nil {
		return clearv1.ClearResult{}, err
	}
	return clearv1.ClearResult{Keys: c.p.store.clearID(c.p.app + "/" + in.Bucket)}, nil
}

// Size is a byte count that reads "64MiB", "10MB", "512KiB" or a plain number from YAML and JSON.
type Size int64

// ParseSize parses a plain byte count or a number with a B, KB, KiB, MB, MiB, GB or GiB suffix.
func ParseSize(s string) (Size, error) {
	t := strings.TrimSpace(s)
	units := []struct {
		suffix string
		mult   float64
	}{{"KIB", 1 << 10}, {"MIB", 1 << 20}, {"GIB", 1 << 30}, {"KB", 1e3}, {"MB", 1e6}, {"GB", 1e9}, {"B", 1}}
	mult := 1.0
	for _, u := range units {
		if strings.HasSuffix(strings.ToUpper(t), u.suffix) {
			t, mult = strings.TrimSpace(t[:len(t)-len(u.suffix)]), u.mult
			break
		}
	}
	n, err := strconv.ParseFloat(t, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid size %q (such as 64MiB)", s)
	}
	return Size(n * mult), nil
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (s *Size) UnmarshalYAML(n *yaml.Node) error {
	v, err := ParseSize(n.Value)
	*s = v
	return err
}

// UnmarshalJSON implements json.Unmarshaler.
func (s *Size) UnmarshalJSON(b []byte) error {
	var str string
	if json.Unmarshal(b, &str) != nil {
		str = string(b)
	}
	v, err := ParseSize(str)
	*s = v
	return err
}

// Duration is a time.Duration that reads "10m" from YAML and JSON (and a number of nanoseconds from JSON).
type Duration time.Duration

// UnmarshalYAML implements yaml.Unmarshaler.
func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	v, err := time.ParseDuration(n.Value)
	if err != nil {
		return fmt.Errorf("invalid duration %q (such as 10m)", n.Value)
	}
	*d = Duration(v)
	return nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var e extkit.Duration
	if err := e.UnmarshalJSON(b); err != nil {
		return err
	}
	*d = Duration(e)
	return nil
}
