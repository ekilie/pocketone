// Package cache provides cache/v1 from one store per process. Each app gets its own provider, a view that keeps its
// buckets apart from every other app's (keys live under "<app id>/<bucket>"), so on a multisite host every instance
// shares one store, its limits and its disk tier. Underneath, values are stored by content: the same bytes under any
// number of keys, from any number of instances, are kept once and count once.
//
// The store's limits (memory, items, disk) are the operator's, from pocketone.yaml's `cache` section. A bucket's own
// limits, from its code and the section's `defaults` and `buckets`, cap what one app's bucket may hold inside it.
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"gopkg.in/yaml.v3"

	cachev1 "github.com/pockethost/pocketone/ext/contracts/cache/v1"
	"github.com/pockethost/pocketone/ext/extkit"
)

// Name is the extension's key: its pocketone.yaml section and an instance's `extensions.<name>`.
const Name = "cache"

// The store's built-in limits, for what the config doesn't set.
const (
	DefaultMaxMemory = 128 << 20 // 128 MiB for every app's buckets together
	DefaultMaxDisk   = 1 << 30   // 1 GiB disk tier, when there is one
)

// BucketConfig caps one app's bucket. Unset (zero) fields keep what the code asked for.
type BucketConfig struct {
	Eviction  cachev1.Eviction `yaml:"eviction,omitempty" json:"eviction,omitempty" doc:"Which of the bucket's entries go first when it's full: lru, mru, fifo or lifo."`
	MaxItems  int              `yaml:"max_items,omitempty" json:"max_items,omitempty" doc:"Most entries the bucket holds."`
	MaxMemory Size             `yaml:"max_memory,omitempty" json:"max_memory,omitempty" doc:"Most bytes of keys and values the bucket holds, such as 32MiB (shared values count in full)."`
	TTL       Duration         `yaml:"ttl,omitempty" json:"ttl,omitempty" doc:"How long an entry lives unless the code says otherwise, such as 10m."`
}

// Config is the `cache` section of pocketone.yaml. On a multisite host an instance's `extensions.cache` may set
// `enabled`, `defaults` and `buckets` for that instance; the store's own keys are the host's only.
type Config struct {
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty" doc:"Turns the cache off when false; extensions then run uncached."`

	// The store, shared by every app in the process.
	Eviction  cachev1.Eviction `yaml:"eviction,omitempty" json:"eviction,omitempty" doc:"Which values leave memory first when the store is full: lru, mru, fifo or lifo."`
	MaxItems  int              `yaml:"max_items,omitempty" json:"max_items,omitempty" doc:"Most distinct values kept in memory (identical bytes count once)."`
	MaxMemory Size             `yaml:"max_memory,omitempty" json:"max_memory,omitempty" doc:"Memory for every bucket together, such as 512MiB. Default 128MiB."`
	DiskPath  string           `yaml:"disk_path,omitempty" json:"disk_path,omitempty" doc:"Turns on a disk tier there for what memory evicts (in a p1cache folder, emptied when the process starts). Relative: under pb_data on a single app, the working directory on a multisite host."`
	MaxDisk   Size             `yaml:"max_disk,omitempty" json:"max_disk,omitempty" doc:"Size of the disk tier, such as 10GiB. Default 1GiB."`

	// Each app's buckets.
	Defaults BucketConfig            `yaml:"defaults,omitempty" json:"defaults,omitempty" doc:"Caps for any bucket whose code leaves them unset."`
	Buckets  map[string]BucketConfig `yaml:"buckets,omitempty" json:"buckets,omitempty" doc:"Caps per bucket name, over what its code asks for."`
}

// Section is the key of the `cache` section (pocketone.yaml).
var Section = extkit.NewSectionFunc(Name, func() Config { return Config{} },
	"The cache (cache/v1): one store for every app in the process (memory, disk tier), and caps per bucket.")

// Extension provides cache/v1 as the default, so another provider (Redis, say) replaces it. One Extension value is
// one store: every app it's registered on shares it.
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
		x.store.flush()
	}
	return nil
}

// Contracts implements [extkit.Declarer].
func (x *Extension) Contracts() extkit.Contracts {
	return extkit.Contracts{Implements: extkit.IDs("cache/v1"), Defaults: extkit.IDs("cache/v1")}
}

// Register implements [extkit.Extension]: it provides cache/v1 for app, a view of the process's store.
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
	cachev1.Contract.ProvideDefault(app, &provider{store: st, app: extkit.AppID(app), cfg: c, buckets: map[string]*bucket{}})
	return nil
}

// storeFor opens the store once, on the first app registered.
func (x *Extension) storeFor(app core.App, c Config) (*store, error) {
	x.once.Do(func() {
		if err := c.validate(); err != nil {
			x.storeErr = err
			return
		}
		lim := poolLimits{eviction: c.Eviction, maxItems: c.MaxItems, maxMem: int64(c.MaxMemory), maxDisk: int64(c.MaxDisk)}
		if lim.eviction == "" {
			lim.eviction = cachev1.LRU
		}
		if lim.maxMem <= 0 {
			lim.maxMem = DefaultMaxMemory
		}
		if lim.maxDisk <= 0 {
			lim.maxDisk = DefaultMaxDisk
		}
		if d := c.DiskPath; d != "" {
			if !filepath.IsAbs(d) && extkit.AppID(app) == "" {
				d = filepath.Join(app.DataDir(), d) // a single app: under its pb_data
			}
			if d, x.storeErr = filepath.Abs(d); x.storeErr != nil {
				return
			}
			lim.dir = d
		}
		x.store, x.storeErr = newStore(lim, slog.Default().With("extension", Name)) // the process's, not one app's: it outlives them
	})
	return x.store, x.storeErr
}

// merge lays over's set fields onto base.
func merge(base, over Config) Config {
	if over.Enabled != nil {
		base.Enabled = over.Enabled
	}
	if over.Eviction != "" {
		base.Eviction = over.Eviction
	}
	if over.MaxItems != 0 {
		base.MaxItems = over.MaxItems
	}
	if over.MaxMemory != 0 {
		base.MaxMemory = over.MaxMemory
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
	if o.TTL != 0 {
		b.TTL = o.TTL
	}
	return b
}

// provider is one app's cache/v1: a view of the store under the app's id.
type provider struct {
	store *store
	app   string // extkit.AppID: "" on a single app
	cfg   Config

	mu      sync.Mutex
	buckets map[string]*bucket
}

var _ cachev1.Cache = (*provider)(nil)

// Bucket implements cachev1.Cache. Caps, weakest first: the config's defaults, the code's options, the config's
// entry for this bucket.
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
	bc := p.cfg.Defaults.over(code).over(p.cfg.Buckets[name])
	lim := spaceLimits{eviction: bc.Eviction, maxItems: bc.MaxItems, maxMem: int64(bc.MaxMemory), ttl: time.Duration(bc.TTL)}
	if lim.eviction == "" {
		lim.eviction = cachev1.LRU
	}
	b := &bucket{store: p.store, sp: p.store.open(p.app+"/"+name, lim)}
	p.buckets[name] = b
	return b, nil
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
