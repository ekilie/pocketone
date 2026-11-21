package deploy

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"pocketone.dev/ext/extkit"
)

// The base: each file's hash as of the last time the project and the remote agreed on it, in .pocketone/sync.json.
// `dev` keeps it as it syncs; `deploy` and `pull` record it too, so a deploy can tell that the remote changed since
// (an edit through the live file drive, the dashboard's automigrate, another developer's deploy) and stop instead of
// overwriting it. Without a base (a fresh checkout, CI) a deploy overwrites as before.

// ServerChange is a file the remote changed since the base that a deploy would overwrite or delete.
type ServerChange struct {
	Path string
	// What happened on the remote: "changed", "added" or "deleted".
	Kind string
}

// ServerChangesError stops a deploy that would overwrite changes made on the remote since the base.
type ServerChangesError struct {
	Remote  string
	Changes []ServerChange
}

func (e *ServerChangesError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d file(s) changed on %s since your last deploy, pull or dev, and this deploy would overwrite them:", len(e.Changes), e.Remote)
	for _, c := range e.Changes {
		fmt.Fprintf(&b, "\n   %s (%s on the server)", c.Path, c.Kind)
	}
	fmt.Fprintf(&b, "\nBring them down with '%s pull' or '%s dev', or deploy with --force to overwrite them.", extkit.Command, extkit.Command)
	return b.String()
}

// serverChanges compares the remote's code set with the base and with what this deploy sends (manifest): a file the
// remote changed since the base, and that the deploy would leave different from the remote's copy, is one the deploy
// would overwrite. ok is false when there's no base for this remote, or the remote can't list its files.
func serverChanges(ctx context.Context, c *Client, opts CollectOptions, manifest map[string]string) (changes []ServerChange, ok bool, err error) {
	s, err := NewSyncer(c, SyncOptions{CollectOptions: opts})
	if err != nil {
		return nil, false, err
	}
	if s.firstRun {
		return nil, false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.fetchRemote(ctx); err != nil {
		if errors.Is(err, ErrNoSyncAPI) {
			return nil, false, nil
		}
		return nil, false, err
	}
	base := s.state.Files
	seen := map[string]bool{}
	for _, m := range []map[string]string{s.remote, base} {
		for p := range m {
			seen[p] = true
		}
	}
	for p := range seen {
		if !s.synced(p) {
			continue
		}
		r, rok := s.remote[p]
		b, bok := base[p]
		if rok == bok && r == b {
			continue // unchanged on the remote
		}
		if l, lok := manifest[p]; lok == rok && l == r {
			continue // the deploy leaves it as the remote has it
		}
		kind := "changed"
		switch {
		case !bok:
			kind = "added"
		case !rok:
			kind = "deleted"
		}
		changes = append(changes, ServerChange{Path: p, Kind: kind})
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return changes, true, nil
}

// recordBase saves the base after a deploy or pull: for each of paths (nil: every file in the code set), the
// remote's hash, where the local copy now matches it. Other files keep their base. A remote that can't list its
// files records nothing.
func recordBase(ctx context.Context, c *Client, opts CollectOptions, paths []string) error {
	s, err := NewSyncer(c, SyncOptions{CollectOptions: opts})
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.fetchRemote(ctx); err != nil {
		if errors.Is(err, ErrNoSyncAPI) {
			return nil
		}
		return err
	}
	local, err := s.local()
	if err != nil {
		return err
	}
	if paths == nil {
		s.state.Files = map[string]string{} // a whole-set deploy: the base is exactly what both sides agree on now
		for p := range s.remote {
			paths = append(paths, p)
		}
		for p := range local {
			paths = append(paths, p)
		}
	}
	for _, p := range paths {
		r, rok := s.remote[p]
		l, lok := local[p]
		switch {
		case rok && lok && l.Hash == r:
			s.state.Files[p] = r
		case !rok && !lok:
			delete(s.state.Files, p)
		}
	}
	return s.saveState()
}
