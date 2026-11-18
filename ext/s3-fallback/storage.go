// Package storage is the operator's fall-through storage: named S3-compatible backends, and mounts that send each
// filesystem kind (record uploads, backup zips, PITR history, any extension's own kind) to one of them. To an app
// nothing is configured: Settings show S3 off, uploads and the Backups screen work as before, and the backends'
// credentials never reach its settings, env or JS. Downloads stream through the server, or, with ext/s3redirect,
// become signed redirects (to s3redirect a mount is just another S3 filesystem).
//
//	set.Add(s3fallback.New(s3fallback.Config{})) // on the standard set every app gets
//
//	storage:
//	  backends:
//	    r2-eu:   { endpoint: …, bucket: pocketbase-eu, region: auto, access_key: …, secret: …, prefix: "" }
//	  mounts:              # kind → backend
//	    storage:    r2-eu  # record uploads (app.NewFilesystem)
//	    backups:    r2-eu  # backup zips (app.NewBackupsFilesystem)
//	    litestream: r2-eu  # PITR history (ext/pitr asks for kind "litestream")
//
// Each app gets <backend prefix><root><kind>/ per mounted kind and can't reach anything outside it. The root is the
// caller's choice and opaque to the extension: for an isolate (Pocker), Register uses "instances/<id>/";
// an app with the process to itself passes "" (or any name). Several deployments sharing one bucket keep apart with
// their own backend prefix or roots. When an app turns on its own S3 for uploads or backups, PocketBase's normal code
// takes over with the app's credentials. Other kinds are served whenever they're mounted: the extension asking for
// them (e.g. ext/pitr) checks its own app setting first. Without mounts the extension does nothing. Files already on
// an app's local disk are not read or moved.
//
// The old shape (`bucket:` + `uploads:`/`backups:`) still works: it becomes one backend named "default" mounted for
// storage and backups, with a warning.
package s3fallback

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/filesystem"
	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/s3redirect/sigv4"
)

// Name is the extension's name and config key.
const Name = "storage"

// Config is the process configuration.
type Config struct {
	// Enabled is the default for every instance: on unless false. An instance's `extensions.storage.enabled`
	// overrides it.
	Enabled *bool `yaml:"enabled" doc:"On for every instance unless false; an instance's extensions.storage.enabled overrides it."`

	// Backends are the named S3-compatible destinations.
	Backends map[string]*Bucket `yaml:"backends" doc:"Named S3-compatible destinations: endpoint, bucket, region, access_key, secret, prefix, force_path_style."`

	// Mounts maps a filesystem kind ("storage", "backups", "litestream", …) to a backend name.
	Mounts map[string]string `yaml:"mounts" doc:"Filesystem kind (storage, backups, litestream, ...) to backend name."`

	// Deprecated: the old single-bucket shape. Bucket becomes backend "default", mounted for storage (unless
	// Uploads is false) and backups (unless Backups is false).
	Bucket  *Bucket `yaml:"bucket" doc:"Deprecated: one bucket, becomes backend default. Use backends and mounts."`
	Uploads *bool   `yaml:"uploads" doc:"Deprecated: with bucket, false keeps uploads off it."`
	Backups *bool   `yaml:"backups" doc:"Deprecated: with bucket, false keeps backups off it."`

	Logger *slog.Logger `yaml:"-"`
}

// Bucket is an S3-compatible destination.
type Bucket struct {
	Endpoint       string `yaml:"endpoint" doc:"S3 endpoint URL."`
	Bucket         string `yaml:"bucket" doc:"Bucket name."`
	Region         string `yaml:"region" doc:"Bucket region (auto for R2)."`
	AccessKey      string `yaml:"access_key" doc:"Access key id."`
	Secret         string `yaml:"secret" doc:"Secret key; use a $VAR from the .env."`
	Prefix         string `yaml:"prefix" doc:"Prepended to every key, to share a bucket."` // prepended to every key
	ForcePathStyle bool   `yaml:"force_path_style" doc:"Path-style addressing, for MinIO and similar servers."`
}

// Extension is the extension: an app extension, registered on each instance (or a standalone app).
type Extension struct {
	cfg  Config
	log  *slog.Logger
	once sync.Once // the `storage` section is read on first use
}

var _ extkit.Extension = (*Extension)(nil)

// New returns the extension for cfg.
func New(cfg Config) *Extension {
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	return &Extension{cfg: cfg, log: log.With("extension", Name)}
}

// Name implements extkit.Extension.
func (p *Extension) Name() string { return Name }

// Config returns the effective configuration.
func (p *Extension) Config() Config {
	p.configure()
	return p.cfg
}

// configure reads pocketone.yaml's `storage` section once; fields set in Go are kept.
func (p *Extension) configure() { p.once.Do(p.readSection) }

func (p *Extension) readSection() {
	c, found := Section.Get()
	if !found {
		return
	}
	if p.cfg.Enabled == nil {
		p.cfg.Enabled = c.Enabled
	}
	if p.cfg.Backends == nil {
		p.cfg.Backends = c.Backends
	}
	if p.cfg.Mounts == nil {
		p.cfg.Mounts = c.Mounts
	}
	if p.cfg.Uploads == nil {
		p.cfg.Uploads = c.Uploads
	}
	if p.cfg.Backups == nil {
		p.cfg.Backups = c.Backups
	}
	if p.cfg.Bucket == nil {
		p.cfg.Bucket = c.Bucket
	}
}

func on(b *bool) bool { return b == nil || *b }

// mounts returns kind → backend, with the deprecated single-bucket shape folded in and every mount checked.
func (p *Extension) mounts() (map[string]*Bucket, error) {
	p.configure()
	backends := map[string]*Bucket{}
	for name, b := range p.cfg.Backends {
		if b == nil {
			return nil, fmt.Errorf("storage: backend %q is empty", name)
		}
		backends[name] = b
	}
	mounts := map[string]string{}
	for kind, name := range p.cfg.Mounts {
		mounts[kind] = name
	}
	if p.cfg.Bucket != nil {
		if _, taken := backends["default"]; taken {
			return nil, fmt.Errorf("storage: `bucket:` (deprecated) and a backend named \"default\" can't both be set")
		}
		backends["default"] = p.cfg.Bucket
		if _, set := mounts[core.FilesystemKindStorage]; !set && on(p.cfg.Uploads) {
			mounts[core.FilesystemKindStorage] = "default"
		}
		if _, set := mounts[core.FilesystemKindBackups]; !set && on(p.cfg.Backups) {
			mounts[core.FilesystemKindBackups] = "default"
		}
		p.log.Warn("storage: `bucket:`/`uploads:`/`backups:` are deprecated; use `backends:` and `mounts:`")
	}
	out := map[string]*Bucket{}
	kinds := make([]string, 0, len(mounts))
	for kind := range mounts {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	for _, kind := range kinds {
		b, ok := backends[mounts[kind]]
		if !ok {
			return nil, fmt.Errorf("storage: mount %q names unknown backend %q", kind, mounts[kind])
		}
		out[kind] = b
	}
	return out, nil
}

// FS returns the filesystem for kind under root, fenced to <backend prefix><root><kind>/. ok is false when the kind
// isn't mounted. Close the result when done.
func (p *Extension) FS(root, kind string) (fsys *filesystem.System, ok bool, err error) {
	mounts, err := p.mounts()
	if err != nil {
		return nil, false, err
	}
	b, ok := mounts[kind]
	if !ok {
		return nil, false, nil
	}
	fsys, err = open(b, keyPrefix(b, root, kind))
	return fsys, err == nil, err
}

// keyPrefix is <backend prefix><root><kind>/, with root ending in "/" when set.
func keyPrefix(b *Bucket, root, kind string) string {
	if root != "" && !strings.HasSuffix(root, "/") {
		root += "/"
	}
	return b.Prefix + root + kind + "/"
}

// Bind serves app's filesystems from the mounts under root (opaque, the caller's choice: "" for an app with the
// process to itself, "instances/<id>/" for an isolate, as under Pocker). Uploads and backups step aside when the
// app turned on its own S3 for them; any other mounted kind is served whenever it's asked for.
func (p *Extension) Bind(app core.App, root string) error {
	mounts, err := p.mounts()
	if err != nil || len(mounts) == 0 {
		return err
	}
	app.OnFilesystemCreate().BindFunc(func(e *core.FilesystemCreateEvent) error {
		b, mounted := mounts[e.Kind]
		if !mounted {
			return e.Next()
		}
		s := e.App.Settings()
		switch e.Kind {
		case core.FilesystemKindStorage:
			if s.S3.Enabled {
				return e.Next()
			}
		case core.FilesystemKindBackups:
			if s.Backups.S3.Enabled {
				return e.Next()
			}
		}
		fsys, err := open(b, keyPrefix(b, root, e.Kind))
		if err != nil {
			return err
		}
		e.Filesystem = fsys
		return e.Next()
	})
	return nil
}

// settings is the shape of an app's settings layers (an instance's `extensions.storage`). Only `enabled` is read per
// app; backends and mounts are the operator's, from the `storage` section or the Config given to [New].
type settings struct {
	Enabled *bool `json:"enabled"`
}

// Register implements [extkit.Extension]: binds app to the mounts under "instances/<id>/" (its [extkit.AppID])
// when apps share a process (Pocker), or at the backends' roots when the app has the process to itself, unless the
// app has the extension off.
func (p *Extension) Register(app core.App, layers extkit.Config) error {
	p.configure()
	var s settings
	if err := layers.Decode(&s); err != nil {
		return err
	}
	enabled := on(p.cfg.Enabled)
	if s.Enabled != nil {
		enabled = *s.Enabled
	}
	if !enabled {
		return nil
	}
	root := ""
	if id := extkit.AppID(app); id != "" {
		root = "instances/" + id + "/"
	}
	return p.Bind(app, root)
}

// open returns b fenced to prefix.
func open(b *Bucket, prefix string) (*filesystem.System, error) {
	s3, err := filesystem.NewS3(b.Bucket, b.Region, b.Endpoint, b.AccessKey, b.Secret, b.ForcePathStyle)
	if err != nil {
		return nil, err
	}
	sign := sigv4.Config{Endpoint: b.Endpoint, Bucket: b.Bucket, Region: b.Region, AccessKey: b.AccessKey,
		SecretKey: b.Secret, PathStyle: b.ForcePathStyle}
	return filesystem.New(prefixDriver{Driver: s3.Driver(), prefix: prefix, sign: sign}), nil
}

// Section is the key of the `storage` section (pocketone.yaml): storage backends and mounts for a multisite host.
var Section = extkit.NewSection(Name, Config{}, "S3 storage backends and mounts for a multisite host's instances: uploads, backups, PITR history.")
