package agent

import (
	"context"
	"sync"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	agentv1 "pocketone.dev/ext/contracts/agent/v1"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/multisite"
)

// Linking this package makes the shared model part of every host built from config: the host provides agent/v1 for its
// instances when pocketone.yaml's agent section names a model.
func init() { multisite.Link(HostExtension{}) }

// HostExtension runs one model for every instance of a multisite host and provides agent/v1 for them. Each instance
// gets PerInstance's share: a few calls at once, each answer at most max_tokens long. An instance whose admins set
// their own API (or off) in Settings → LLM is answered that way instead, within the same endpoint.
//
// Instances that run in this process (Pocker isolates) call it through $agent and their own /api/_c/agent/v1; the
// in-app extension steps aside for it.
type HostExtension struct{}

// Name implements multisite.Extension.
func (HostExtension) Name() string { return Name }

// Contracts implements extkit.Declarer.
func (HostExtension) Contracts() extkit.Contracts {
	return extkit.Contracts{Implements: extkit.IDs("agent/v1")}
}

// RegisterHost implements multisite.Extension: nothing unless the agent section names a model.
func (HostExtension) RegisterHost(h *multisite.Host) error {
	cfg, _ := Section.Get()
	if cfg.Model == "" {
		return nil
	}
	r, err := sharedRunner(cfg, h.Logger)
	if err != nil {
		return err
	}
	hs := newHostShare(r, cfg.withDefaults())
	hostShares.Store(hs, true)
	agentv1.Contract.ProvideForInstances(h.Scope, func(id string) agentv1.Agent { return &hostAgent{share: hs, id: id} })
	return nil
}

// hostShares are the hosts' shares in this process, to release an instance's warms when it stops.
var hostShares sync.Map // *hostShare → true

// hostShare divides a host's model between its instances.
type hostShare struct {
	r   *runner
	cfg Config

	mu       sync.Mutex
	inFlight map[string]int             // calls waiting or running, per instance
	warms    map[string]map[string]bool // warm ids per instance, released when it stops
}

func (s *hostShare) trackWarm(inst, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.warms[inst] == nil {
		s.warms[inst] = map[string]bool{}
	}
	s.warms[inst][id] = true
}

func (s *hostShare) untrackWarm(inst, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.warms[inst], id)
}

// releaseInstance drops an instance's warms when it stops: their pins go, so the memory is the others' again.
func (s *hostShare) releaseInstance(inst string) {
	s.mu.Lock()
	ids := s.warms[inst]
	delete(s.warms, inst)
	s.mu.Unlock()
	for id := range ids {
		s.r.unwarm(pinOf(inst, id))
	}
}

func newHostShare(r *runner, cfg Config) *hostShare {
	return &hostShare{r: r, cfg: cfg, inFlight: map[string]int{}, warms: map[string]map[string]bool{}}
}

// admit takes one of the instance's call slots; release gives it back.
func (s *hostShare) admit(id string) (release func(), err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inFlight[id] >= s.cfg.PerInstance.Concurrency {
		return nil, &extkit.Error{Code: "busy", Status: 429, Message: "too many calls to the shared model at once from this app; wait for one to finish"}
	}
	s.inFlight[id]++
	return func() {
		s.mu.Lock()
		s.inFlight[id]--
		if s.inFlight[id] <= 0 {
			delete(s.inFlight, id)
		}
		s.mu.Unlock()
	}, nil
}

// maxTokens is the longest answer a call may get on the shared model.
func (s *hostShare) maxTokens() int {
	if m := s.cfg.PerInstance.MaxTokens; m > 0 {
		return m
	}
	return s.cfg.MaxTokens
}

// hostAgent is agent/v1 for one instance of the host.
type hostAgent struct {
	share *hostShare
	id    string
}

func (a *hostAgent) app() (core.App, bool) {
	v, ok := instanceApps.Load(a.id)
	if !ok {
		return nil, false
	}
	return v.(core.App), true
}

func (a *hostAgent) Complete(ctx context.Context, _ extkit.Auth, in agentv1.CompleteInput) (agentv1.CompleteResult, error) {
	return a.answer(ctx, in, nil)
}

// Stream implements agent/v1 for the instance: the answer as it's written, through the same share.
func (a *hostAgent) Stream(_ context.Context, _ extkit.Auth, in agentv1.CompleteInput) (extkit.Stream, error) {
	if err := checkInput(in); err != nil {
		return extkit.Stream{}, err
	}
	return streamOf(func(ctx context.Context, onText func(string) bool) (agentv1.CompleteResult, error) {
		return a.answer(ctx, in, onText)
	}), nil
}

// answer is Complete, and Stream when onText is set.
func (a *hostAgent) answer(ctx context.Context, in agentv1.CompleteInput, onText func(string) bool) (agentv1.CompleteResult, error) {
	if err := checkInput(in); err != nil {
		return agentv1.CompleteResult{}, err
	}
	switch c := a.choice(); c.Mode {
	case ModeOff:
		return agentv1.CompleteResult{}, errOff(c)
	case ModeAPI:
		return c.API.run(ctx, in, onText)
	}
	release, err := a.share.admit(a.id)
	if err != nil {
		return agentv1.CompleteResult{}, err
	}
	defer release()
	limit := a.share.maxTokens()
	maxTokens := in.MaxTokens
	if maxTokens <= 0 || maxTokens > limit {
		maxTokens = limit
	}
	res, err := a.share.r.complete(ctx, in, maxTokens, "host", onText, a.scope()) // each instance's caches are its own
	return res, err
}

// scope is the instance's: its id for the prompt cache, its own pb_cache for thread state (so it counts against the
// instance's disk), within per_instance.thread_cache_gb.
func (a *hostAgent) scope() scopeOpts {
	sc := scopeOpts{Scope: a.id}
	if app, ok := a.app(); ok && a.share.cfg.PerInstance.ThreadCache >= 0 {
		sc.ThreadDir, sc.ThreadDisk = threadDir(app), gb(a.share.cfg.PerInstance.ThreadCache)
	}
	return sc
}

// Warm implements agent/v1 for the instance, within its pinned memory (per_instance.pinned_mb).
func (a *hostAgent) Warm(ctx context.Context, _ extkit.Auth, in agentv1.WarmInput) (agentv1.WarmResult, error) {
	id := warmID(a.id, in)
	switch c := a.choice(); c.Mode {
	case ModeOff:
		return agentv1.WarmResult{ID: id, Message: errOff(c).Error()}, nil
	case ModeAPI:
		return agentv1.WarmResult{ID: id, Message: "answered by the app's own API, which caches prompts itself"}, nil
	}
	if limit := int64(a.share.cfg.PerInstance.PinnedMB) << 20; limit > 0 && a.share.r.pinnedBytes(a.id) >= limit {
		return agentv1.WarmResult{ID: id, Message: "this app's warmed prompts already fill its share of the shared model's memory; forget one first"}, nil
	}
	a.share.trackWarm(a.id, id)
	return warmOn(ctx, a.share.r, a.id, id, in), nil
}

// Forget implements agent/v1 for the instance.
func (a *hostAgent) Forget(_ context.Context, _ extkit.Auth, in agentv1.ForgetInput) (agentv1.Done, error) {
	a.share.untrackWarm(a.id, in.ID)
	a.share.r.unwarm(pinOf(a.id, in.ID))
	return agentv1.Done{OK: true}, nil
}

// Count implements agent/v1 for the instance.
func (a *hostAgent) Count(_ context.Context, _ extkit.Auth, in agentv1.CountInput) (agentv1.CountResult, error) {
	if a.choice().Mode == ModeAPI {
		return countOf(nil, in), nil
	}
	return countOf(a.share.r, in), nil
}

// Threads implements agent/v1 for the instance: only its own.
func (a *hostAgent) Threads(context.Context, extkit.Auth) (agentv1.ThreadList, error) {
	return threadsOf(a.share.r, a.scope()), nil
}

// ForgetThread implements agent/v1 for the instance.
func (a *hostAgent) ForgetThread(_ context.Context, _ extkit.Auth, in agentv1.ForgetInput) (agentv1.Done, error) {
	if a.share.r.phase.Load().(string) == "ready" {
		sc := a.scope()
		a.share.r.model.forgetThread(a.id, in.ID, sc.ThreadDir)
	}
	return agentv1.Done{OK: true}, nil
}

func (a *hostAgent) Status(context.Context, extkit.Auth) (agentv1.Status, error) {
	switch c := a.choice(); c.Mode {
	case ModeOff:
		return agentv1.Status{Message: errOff(c).Error()}, nil
	case ModeAPI:
		return agentv1.Status{Backend: "app", Model: c.API.Model, Ready: true}, nil
	}
	return a.share.r.status("host"), nil
}

// choice is what the instance's admins chose in Settings → LLM: off, their own API, or (by default) the host's model.
func (a *hostAgent) choice() choice {
	app, ok := a.app()
	if !ok {
		return choice{Mode: ModeHost, Source: "host"}
	}
	return choose(app, a.share.cfg, true)
}
