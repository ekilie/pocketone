package wasm

import (
	"context"
	"os"
	"testing"
)

// For the tests only (this file isn't part of the package outside go test): modules are otherwise called only from
// pb_hooks.

// Apps in the tests that don't set CacheDir share one compilation cache for the whole run, in a temporary folder
// removed at the end, so the suite never touches the user's cache folder and compiles each module once.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ext-wasm-cache-")
	if err != nil {
		panic(err)
	}
	defaultCacheDir = func() (string, error) { return dir, nil }
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// CacheMaxAge is how old a cache file gets before the cache prunes it.
const CacheMaxAge = cacheMaxAge

// PruneCache deletes the files in dir older than maxAge.
var PruneCache = pruneCache

// Call runs a module function directly.
func (h *Host) Call(ctx context.Context, name, function string, input []byte) ([]byte, error) {
	return h.run(ctx, name, function, input)
}

// Config is the settings the Host runs with.
func (h *Host) Config() Config { return h.cfg }

// ProducersLanguage is the language a "producers" custom section names.
var ProducersLanguage = producersLanguage

// Truncate cuts a message to at most n bytes, never inside a UTF-8 character.
var Truncate = truncate
