//go:build race

package pitr

// PocketBase's notify-dir watcher goroutine reads IsBootstrapped() without synchronization while
// ResetBootstrapState() clears the DB handles (core/notify_watcher.go vs core/base.go ClearBootstrap). Any app
// stop can trip it; it is upstream's, not this extension's.
const raceEnabled = true
