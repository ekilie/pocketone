// Package cache provides cache/v1 in the app's own process: each bucket keeps its entries in memory, evicting by
// LRU, MRU, FIFO or LIFO within its item and memory caps, with an optional disk tier that evicted entries move to and
// that survives restarts. Operators override any bucket's limits in pocketone.yaml's `cache` section.
package cache

import (
	"encoding/json"
	"fmt"
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

// Built-in limits for what neither code nor config sets.
const (
	DefaultMaxMemory = 32 << 20  // 32 MiB per bucket
	DefaultMaxDisk   = 256 << 20 // 256 MiB per bucket's disk tier
)

// BucketConfig overrides a bucket's limits. Unset (zero) fields keep what the code asked for.
type BucketConfig struct {
	Eviction  cachev1.Eviction `yaml:"eviction,omitempty" json:"eviction,omitempty" doc:"What a full bucket drops first: lru, mru, fifo or lifo."`
	MaxItems  int              `yaml:"max_items,omitempty" json:"max_items,omitempty" doc:"Most entries kept in memory."`
	MaxMemory Size             `yaml:"max_memory,omitempty" json:"max_memory,omitempty" doc:"Most bytes kept in memory, such as 64MiB."`
	TTL       Duration         `yaml:"ttl,omitempty" json:"ttl,omitempty" doc:"How long an entry lives unless the code says otherwise, such as 10m."`
	DiskPath  string           `yaml:"disk_path,omitempty" json:"disk_path,omitempty" doc:"Turns on the disk tier there; relative to pb_data."`
	MaxDisk   Size             `yaml:"max_disk,omitempty" json:"max_disk,omitempty" doc:"Most bytes the disk tier holds, such as 1GiB."`
}

// Config is the `cache` section of pocketone.yaml, and the shape of an instance's `extensions.cache` on a multisite
// host (whose keys replace the section's).
type Config struct {
	// Defaults fill what a bucket's code leaves unset.
	Defaults BucketConfig `yaml:"defaults,omitempty" json:"defaults,omitempty" doc:"Limits for any bucket whose code leaves them unset."`
	// Buckets override named buckets' limits, over what their code asks for.
	Buckets map[string]BucketConfig `yaml:"buckets,omitempty" json:"buckets,omitempty" doc:"Limits per bucket name, over what its code asks for."`
}

// Section is the key of the `cache` section (pocketone.yaml).
var Section = extkit.NewSectionFunc(Name, func() Config { return Config{} },
	"Cache buckets (cache/v1): default limits, and limits per bucket over what its code asks for.")

// Extension provides cache/v1 as the default, so another provider (Redis, say) replaces it.
type Extension struct{ cfg Config }

var _ extkit.Extension = (*Extension)(nil)

// New returns the extension. cfg sits under pocketone.yaml's section and an instance's settings.
func New(cfg Config) *Extension { return &Extension{cfg: cfg} }

// Name implements [extkit.Extension].
func (x *Extension) Name() string { return Name }

// Contracts implements [extkit.Declarer].
func (x *Extension) Contracts() extkit.Contracts {
	return extkit.Contracts{Implements: extkit.IDs("cache/v1"), Defaults: extkit.IDs("cache/v1")}
}

// Register implements [extkit.Extension]: it provides cache/v1 for app.
func (x *Extension) Register(app core.App, layers extkit.Config) error {
	c := x.cfg
	if s, found := Section.Get(); found {
		c = merge(c, s)
	}
	var l Config
	if err := layers.Decode(&l); err != nil {
		return fmt.Errorf("cache: %w", err)
	}
	c = merge(c, l)
	if err := c.validate(); err != nil {
		return err
	}
	cachev1.Contract.ProvideDefault(app, &provider{app: app, cfg: c, buckets: map[string]*bucket{}})
	return nil
}

// merge lays over's set fields onto base.
func merge(base, over Config) Config {
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
	if !c.Defaults.Eviction.Valid() {
		return fmt.Errorf("cache: defaults: unknown eviction %q (lru, mru, fifo, lifo)", c.Defaults.Eviction)
	}
	for n, b := range c.Buckets {
		if err := cachev1.ValidName(n); err != nil {
			return err
		}
		if !b.Eviction.Valid() {
			return fmt.Errorf("cache: bucket %s: unknown eviction %q (lru, mru, fifo, lifo)", n, b.Eviction)
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
	if o.DiskPath != "" {
		b.DiskPath = o.DiskPath
	}
	if o.MaxDisk != 0 {
		b.MaxDisk = o.MaxDisk
	}
	return b
}

func fromOptions(o cachev1.Options) BucketConfig {
	return BucketConfig{Eviction: o.Eviction, MaxItems: o.MaxItems, MaxMemory: Size(o.MaxMemory), TTL: Duration(o.TTL),
		DiskPath: o.DiskPath, MaxDisk: Size(o.MaxDisk)}
}

// provider is one app's cache/v1.
type provider struct {
	app core.App
	cfg Config

	mu      sync.Mutex
	buckets map[string]*bucket
}

var _ cachev1.Cache = (*provider)(nil)

// Bucket implements cachev1.Cache. Limits, weakest first: built-in defaults, the config's defaults, the code's
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
	bc := p.cfg.Defaults.over(fromOptions(opts)).over(p.cfg.Buckets[name])
	lim := limits{eviction: bc.Eviction, maxItems: bc.MaxItems, maxMem: int64(bc.MaxMemory), ttl: time.Duration(bc.TTL),
		maxDisk: int64(bc.MaxDisk)}
	if lim.eviction == "" {
		lim.eviction = cachev1.LRU
	}
	if lim.maxMem <= 0 {
		lim.maxMem = DefaultMaxMemory
	}
	if lim.maxDisk <= 0 {
		lim.maxDisk = DefaultMaxDisk
	}
	if bc.DiskPath != "" {
		lim.dir = p.diskDir(bc.DiskPath, name)
	}
	b, err := newBucket(name, lim, p.app.Logger())
	if err != nil {
		return nil, err
	}
	p.buckets[name] = b
	return b, nil
}

// diskDir is a bucket's disk folder: <path>/<bucket>, under pb_data when path is relative. An absolute path on a
// multisite host gets the instance's id in it, so instances never share files.
func (p *provider) diskDir(path, name string) string {
	if !filepath.IsAbs(path) {
		return filepath.Join(p.app.DataDir(), path, name)
	}
	if id := extkit.AppID(p.app); id != "" {
		return filepath.Join(path, id, name)
	}
	return filepath.Join(path, name)
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
