package agent

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	streamv1 "github.com/pockethost/pocketone/ext/contracts/agent/stream/v1"
	agentv1 "github.com/pockethost/pocketone/ext/contracts/agent/v1"
	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/multisite"
)

// Linking this package makes the shared model part of every host built from config: the host provides agent/v1 for its
// instances when pocketone.yaml's agent section names a model.
func init() { multisite.Link(HostExtension{}) }

// HostExtension runs one model for every instance of a multisite host and provides agent/v1 for them. Each instance
// gets PerInstance's share: a few calls at once and a daily allowance of answer tokens. An instance whose admins set
// their own API in Settings → Agent is answered by that API instead, within the same endpoint.
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
	agentv1.Contract.ProvideForInstances(h.Scope, func(id string) agentv1.Agent { return &hostAgent{share: hs, id: id} })
	currentShare.Store(hs)
	return nil
}

// currentShare is the share of the host this process runs, for instances' agent.stream/v1 (see hostedStreamer).
var currentShare atomic.Pointer[hostShare]

// hostedStreamer is agent.stream/v1 in an instance whose host provides agent/v1: the host's model through the same
// share (quota, concurrency, the instance's own API), streamed in this process. A host whose agent/v1 isn't this
// package's gets the whole answer in one piece instead.
type hostedStreamer struct {
	app core.App
	id  string
}

func (s *hostedStreamer) Stream(ctx context.Context, in streamv1.Input) (agentv1.CompleteResult, error) {
	if hs := currentShare.Load(); hs != nil {
		return (&hostAgent{share: hs, id: s.id}).answer(ctx, in.CompleteInput, in.OnText)
	}
	res, err := agentv1.Contract.Must(s.app).Complete(ctx, extkit.Internal, in.CompleteInput)
	if err == nil && in.OnText != nil && res.Text != "" {
		in.OnText(res.Text)
	}
	return res, err
}

// hostShare divides a host's model between its instances.
type hostShare struct {
	r   *runner
	cfg Config
	now func() time.Time

	mu       sync.Mutex
	inFlight map[string]int
	day      string
	used     map[string]int // answer tokens today, per instance
}

func newHostShare(r *runner, cfg Config) *hostShare {
	return &hostShare{r: r, cfg: cfg, now: time.Now, inFlight: map[string]int{}, used: map[string]int{}}
}

// admit takes one of the instance's call slots and checks its allowance; release gives the slot back.
func (s *hostShare) admit(id string) (release func(), err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rollDay()
	if lim := s.cfg.PerInstance.TokensPerDay; lim > 0 && s.used[id] >= lim {
		return nil, agentv1.ErrQuota
	}
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

func (s *hostShare) charge(id string, tokens int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rollDay()
	s.used[id] += tokens
}

// rollDay starts a new day's allowances (UTC). Caller holds mu.
func (s *hostShare) rollDay() {
	if d := s.now().UTC().Format("2006-01-02"); d != s.day {
		s.day, s.used = d, map[string]int{}
	}
}

func (s *hostShare) quota(id string) *agentv1.Quota {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rollDay()
	if s.cfg.PerInstance.TokensPerDay <= 0 {
		return nil
	}
	return &agentv1.Quota{TokensPerDay: s.cfg.PerInstance.TokensPerDay, UsedToday: s.used[id], MaxTokens: s.maxTokens()}
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

// answer is Complete, and agent.stream/v1 for the instance when onText is set.
func (a *hostAgent) answer(ctx context.Context, in agentv1.CompleteInput, onText func(string) bool) (agentv1.CompleteResult, error) {
	if err := checkInput(in); err != nil {
		return agentv1.CompleteResult{}, err
	}
	if app, ok := a.app(); ok {
		if api, ok := ejected(app); ok {
			api.Timeout = a.share.cfg.timeout()
			return api.run(ctx, in, onText)
		}
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
	res, err := a.share.r.complete(ctx, in, maxTokens, "host", onText)
	a.share.charge(a.id, res.Usage.OutputTokens)
	return res, err
}

func (a *hostAgent) Status(context.Context, extkit.Auth) (agentv1.Status, error) {
	if app, ok := a.app(); ok {
		if api, ok := ejected(app); ok {
			return agentv1.Status{Backend: "app", Model: api.Model, Ready: true}, nil
		}
	}
	st := a.share.r.status("host")
	st.Quota = a.share.quota(a.id)
	return st, nil
}
