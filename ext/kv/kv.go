// Package kv is a PocketOne extension: the app's key-value store. It provides kv/v1 (named JSON values in
// PocketBase's _params, served to superusers at /api/_c/kv/v1) and gives page code $kv. Other extensions use the
// contract's helpers (kvv1.Get, kvv1.Set, ...), never this package.
package kv

import (
	"context"
	"encoding/json"
	"fmt"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	kvv1 "pocketone.dev/ext/contracts/kv/v1"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/js"
)

// Name is the extension's name.
const Name = "kv"

// Extension provides kv/v1 and $kv.
type Extension struct{}

var (
	_ extkit.Extension = (*Extension)(nil)
	_ extkit.JSBinder  = (*Extension)(nil)
	_ js.Typer         = (*Extension)(nil)
)

// New returns the extension.
func New() *Extension { return &Extension{} }

// Name implements [extkit.Extension].
func (*Extension) Name() string { return Name }

// Contracts implements [extkit.Declarer].
func (*Extension) Contracts() extkit.Contracts {
	return extkit.Contracts{Implements: extkit.IDs("kv/v1"), Defaults: extkit.IDs("kv/v1")}
}

// Register implements [extkit.Extension].
func (*Extension) Register(app core.App, _ extkit.Config) error {
	kvv1.Contract.ProvideDefault(app, store{app})
	return nil
}

// store is kv/v1 over the contract's helpers.
type store struct{ app core.App }

// public refuses a private name: only the app's Go code reaches those.
func public(name string) error {
	if kvv1.IsPrivate(name) {
		return kvv1.ErrPrivate
	}
	return nil
}

// publicList is kvv1.List without the private names.
func publicList(app core.App, prefix string) ([]kvv1.Entry, error) {
	if err := public(prefix); err != nil {
		return nil, err
	}
	entries, err := kvv1.List(app, prefix)
	if err != nil {
		return nil, err
	}
	out := entries[:0]
	for _, e := range entries {
		if !kvv1.IsPrivate(e.Name) {
			out = append(out, e)
		}
	}
	return out, nil
}

func (s store) List(_ context.Context, _ extkit.Auth, in kvv1.ListInput) (kvv1.Listing, error) {
	entries, err := publicList(s.app, in.Prefix)
	if err != nil {
		return kvv1.Listing{}, err
	}
	return kvv1.Listing{Entries: entries}, nil
}

func (s store) Get(_ context.Context, _ extkit.Auth, in kvv1.Key) (kvv1.Entry, error) {
	if err := public(in.Name); err != nil {
		return kvv1.Entry{}, err
	}
	e, found, err := kvv1.Lookup(s.app, in.Name)
	if err == nil && !found {
		err = kvv1.ErrNotFound
	}
	return e, err
}

func (s store) Set(_ context.Context, _ extkit.Auth, in kvv1.SetInput) (kvv1.Entry, error) {
	if err := public(in.Name); err != nil {
		return kvv1.Entry{}, err
	}
	return kvv1.SetRaw(s.app, in.Name, in.Value)
}

func (s store) Delete(_ context.Context, _ extkit.Auth, in kvv1.Key) error {
	if err := public(in.Name); err != nil {
		return err
	}
	return kvv1.Delete(s.app, in.Name)
}

// BindJS implements [extkit.JSBinder]: $kv in the app's page code and pb_hooks. Every call is local and sync (one
// SQLite read or write), so none needs an Async form. Like the wire API, it never reaches private names.
func (*Extension) BindJS(app core.App, rt js.Runtime) {
	n := rt.NewObject()
	// get(name): the value's JSON, or "" when there is none
	_ = n.Set("get", func(name string) (string, error) {
		if err := public(name); err != nil {
			return "", jsErr("get", err)
		}
		e, found, err := kvv1.Lookup(app, name)
		if err != nil || !found {
			return "", jsErr("get", err)
		}
		return string(e.Value), nil
	})
	_ = n.Set("set", func(name, value string) error {
		if err := public(name); err != nil {
			return jsErr("set", err)
		}
		_, err := kvv1.SetRaw(app, name, json.RawMessage(value))
		return jsErr("set", err)
	})
	_ = n.Set("delete", func(name string) error {
		if err := public(name); err != nil {
			return jsErr("delete", err)
		}
		return jsErr("delete", kvv1.Delete(app, name))
	})
	_ = n.Set("list", func(prefix string) (string, error) {
		entries, err := publicList(app, prefix)
		if err != nil {
			return "", jsErr("list", err)
		}
		b, err := json.Marshal(entries)
		return string(b), err
	})
	if err := rt.Set(nativeGlobal, n); err != nil {
		panic(fmt.Errorf("kv: bind JS: %w", err))
	}
	if _, err := rt.Eval(jsPrelude, "ext/kv"); err != nil {
		panic(fmt.Errorf("kv: bind JS: %w", err))
	}
}

func jsErr(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("$kv.%s: %w", op, err)
}

const nativeGlobal = "__pocketone_kv"

// jsPrelude builds $kv over the Go functions, which it takes off the global scope: values cross as JSON text.
const jsPrelude = `(() => {
  const n = globalThis.__pocketone_kv;
  delete globalThis.__pocketone_kv;
  const name = (op, s) => {
    if (typeof s !== "string") throw new TypeError("$kv." + op + ": the name must be a string");
    return s;
  };
  globalThis.$kv = Object.freeze({
    get: (k) => {
      const raw = n.get(name("get", k));
      return raw === "" ? null : JSON.parse(raw);
    },
    set: (k, value) => {
      const raw = JSON.stringify(value);
      if (raw === undefined) throw new TypeError("$kv.set: the value can't be stored as JSON");
      n.set(name("set", k), raw);
    },
    delete: (k) => { n.delete(name("delete", k)); },
    list: (prefix) => JSON.parse(n.list(prefix ?? "")),
  });
})()`

// TypeScript implements jsvm's Typer: $kv's declarations, added to the app's types.d.ts.
func (*Extension) TypeScript() string { return typeScript }

const typeScript = `/** One value in the app's key-value store. created and updated are ISO 8601 times. */
type KvEntry = { name: string; value: any; created: string; updated: string }

/**
 * The app's key-value store (ext/kv, the kv/v1 contract): named JSON values kept without a collection or a migration,
 * in PocketBase's _params, so they ride along in backups. Names are 1 to 128 characters: letters, digits and
 * _ - . : / , starting with a letter or digit; start yours with a prefix of your own ("myapp/settings"). Values are any
 * JSON up to 1 MiB. Every call is local and sync.
 */
declare var $kv: {
  /** The value stored under name, parsed, or null when there is none. */
  get(name: string): any
  /** Stores value (as JSON.stringify) under name, replacing any value it had. */
  set(name: string, value: any): void
  /** Removes name's value; no error when there was none. */
  delete(name: string): void
  /** The values whose names start with prefix (all of them when it's omitted), sorted by name. */
  list(prefix?: string): KvEntry[]
}
`
