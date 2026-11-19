// Package sourcetest is an instance.source/v1 provider for tests: a fixed list of instances that a test can change
// while it runs, the way an edit to a host's instances file or a control plane's update would.
//
//	src, scope := sourcetest.Scope(sourcev1.Instance{ID: "a", Hosts: []string{"a.test"}})
//	m := pocker.NewManager(scope, pocker.ManagerOptions{...}) // reads instance.source/v1 from scope
//	src.Set(sourcev1.Instance{ID: "a", Hosts: []string{"a.test"}, Dev: true}) // an upsert, seen by Watch
package sourcetest

import (
	"context"
	"sync"

	"pocketone.dev/ext/contracts/instance/source/v1"
	"pocketone.dev/ext/extkit"
)

// Source is a sourcev1.Source whose instances a test sets. Changes made with Set and Delete reach every Watch.
type Source struct {
	mu       sync.Mutex
	list     []sourcev1.Instance
	watchers []chan sourcev1.Change
}

var _ sourcev1.Source = (*Source)(nil)

// New returns a source holding list.
func New(list ...sourcev1.Instance) *Source {
	return &Source{list: append([]sourcev1.Instance(nil), list...)}
}

// Scope returns a source holding list and a sealed host scope ("multisite") that provides it as instance.source/v1.
func Scope(list ...sourcev1.Instance) (*Source, *extkit.Scope) {
	src := New(list...)
	s := extkit.NewScope("multisite")
	prev := s.Registering("sourcetest")
	sourcev1.Contract.ProvideIn(s, src)
	s.Registering(prev)
	if err := s.Seal(nil); err != nil {
		panic(err)
	}
	return src, s
}

// ScopeOf is [Scope] for a test that never changes the list.
func ScopeOf(list ...sourcev1.Instance) *extkit.Scope {
	_, s := Scope(list...)
	return s
}

// Snapshot implements sourcev1.Source.
func (s *Source) Snapshot(context.Context) ([]sourcev1.Instance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]sourcev1.Instance(nil), s.list...), nil
}

// Watch implements sourcev1.Source: the changes made after it, until ctx is done.
func (s *Source) Watch(ctx context.Context) (<-chan sourcev1.Change, error) {
	ch := make(chan sourcev1.Change, 64)
	s.mu.Lock()
	s.watchers = append(s.watchers, ch)
	s.mu.Unlock()
	go func() {
		<-ctx.Done()
		s.mu.Lock()
		defer s.mu.Unlock()
		for i, w := range s.watchers {
			if w == ch {
				s.watchers = append(s.watchers[:i], s.watchers[i+1:]...)
				break
			}
		}
		close(ch)
	}()
	return ch, nil
}

// Set adds inst, or replaces the instance with its ID, and tells every watcher.
func (s *Source) Set(inst sourcev1.Instance) {
	s.mu.Lock()
	defer s.mu.Unlock()
	replaced := false
	for i := range s.list {
		if s.list[i].ID == inst.ID {
			s.list[i], replaced = inst, true
		}
	}
	if !replaced {
		s.list = append(s.list, inst)
	}
	s.send(sourcev1.Change{Kind: sourcev1.Upsert, Instance: inst})
}

// Delete removes the instance with id and tells every watcher.
func (s *Source) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.list {
		if s.list[i].ID == id {
			s.list = append(s.list[:i], s.list[i+1:]...)
			break
		}
	}
	s.send(sourcev1.Change{Kind: sourcev1.Delete, Instance: sourcev1.Instance{ID: id}})
}

func (s *Source) send(c sourcev1.Change) {
	for _, w := range s.watchers {
		w <- c
	}
}
