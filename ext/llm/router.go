package llm

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"

	llmc "pocketone.dev/ext/contracts/llm"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/llm/providers/oai"
)

// router sends each call to the provider its model names, with the app's settings read on every call (so a save in
// Settings → LLM applies at once).
type router struct {
	app core.App

	mu    sync.Mutex
	lists map[string]cachedList // provider fingerprint → its model list
}

type cachedList struct {
	at     time.Time
	models []llmc.ModelInfo
}

// listTTL is how long a provider's model list is kept before it's listed again.
const listTTL = 30 * time.Minute

func (r *router) settings() (Settings, error) {
	return LoadSettings(r.app)
}

// forget drops the cached model lists.
func (r *router) forget() {
	r.mu.Lock()
	r.lists = nil
	r.mu.Unlock()
}

// target is a call's resolved model: its provider, the provider's driver, and the provider's own model id.
type target struct {
	p     Provider
	d     driver
	model string // the provider's id
}

// ref is the full "provider/model".
func (t target) ref() string { return t.p.Name + "/" + t.model }

// resolve finds what answers a call naming ref ("provider/model").
func (r *router) resolve(kind, ref string) (target, error) {
	s, err := r.settings()
	if err != nil {
		return target{}, err
	}
	if len(s.Providers) == 0 {
		return target{}, llmc.ErrNotConfigured
	}
	if strings.TrimSpace(ref) == "" {
		return target{}, &extkit.Error{Code: llmc.ErrUnknownModel.Code, Status: 400,
			Message: fmt.Sprintf("name the %s model as provider/model, like vultr/glm-5.2", kind)}
	}
	name, model, ok := llmc.SplitModel(ref)
	if !ok {
		return target{}, &extkit.Error{Code: llmc.ErrUnknownModel.Code, Status: 400,
			Message: fmt.Sprintf("model %q: name it as provider/model, like vultr/glm-5.2", ref)}
	}
	p, ok := s.provider(name)
	if !ok {
		return target{}, &extkit.Error{Code: llmc.ErrUnknownModel.Code, Status: 400,
			Message: fmt.Sprintf("model %q: no provider named %q in Settings → LLM", ref, name)}
	}
	d, err := newDriver(p)
	if err != nil {
		return target{}, err
	}
	return target{p: p, d: d, model: model}, nil
}

// fingerprint keys a provider's cached list: a change to anything that reaches it lists again.
func fingerprint(p Provider) string {
	return p.Name + "\x00" + p.Type + "\x00" + p.endpoint() + "\x00" + p.Account + "\x00" + p.Key
}

// catalog lists every model the provider has of a kind PocketOne supports, cached for listTTL unless fresh.
func (r *router) catalog(ctx context.Context, p Provider, fresh bool) ([]llmc.ModelInfo, error) {
	key := fingerprint(p)
	r.mu.Lock()
	c, ok := r.lists[key]
	r.mu.Unlock()
	if ok && !fresh && time.Since(c.at) < listTTL {
		return c.models, nil
	}
	d, err := newDriver(p)
	if err != nil {
		return nil, err
	}
	ms, err := d.models(ctx)
	if err != nil {
		return nil, providerError(p, err)
	}
	for i := range ms {
		ms[i].Provider, ms[i].Type = p.Name, p.Type
		sort.Strings(ms[i].Kinds)
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].ID < ms[j].ID })
	r.mu.Lock()
	if r.lists == nil {
		r.lists = map[string]cachedList{}
	}
	r.lists[key] = cachedList{at: time.Now(), models: ms}
	r.mu.Unlock()
	return ms, nil
}

// models is a kind's models operation: the models of that kind across providers (or one), ids made full.
// A provider whose list fails is left out; when they all fail, the first error is the answer.
func (r *router) models(ctx context.Context, kind string, in llmc.ModelsInput) (llmc.Models, error) {
	s, err := r.settings()
	if err != nil {
		return llmc.Models{}, err
	}
	out := llmc.Models{Models: []llmc.ModelInfo{}}
	var firstErr error
	for _, p := range s.Providers {
		if in.Provider != "" && p.Name != in.Provider {
			continue
		}
		ms, err := r.catalog(ctx, p, false)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, m := range ms {
			if !slices.Contains(m.Kinds, kind) {
				continue
			}
			m.ID = p.Name + "/" + m.ID
			out.Models = append(out.Models, m)
		}
	}
	if len(out.Models) == 0 && firstErr != nil {
		return out, firstErr
	}
	return out, nil
}

// providerError turns a driver's error into the contract's: a cloud's own error keeps its message (with the
// provider's name) under the code "provider"; the contract's errors pass through.
func providerError(p Provider, err error) error {
	if err == nil {
		return nil
	}
	var xe *extkit.Error
	if errors.As(err, &xe) {
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &extkit.Error{Code: "timeout", Status: 504, Message: p.Name + ": the provider didn't answer in time"}
	}
	if errors.Is(err, context.Canceled) {
		return err
	}
	status := llmc.ErrProvider.Status
	var oe *oai.Error
	if errors.As(err, &oe) && (oe.Status == 400 || oe.Status == 404 || oe.Status == 413 || oe.Status == 422) {
		status = 400 // the request was wrong, not the provider
	}
	return &extkit.Error{Code: llmc.ErrProvider.Code, Status: status, Message: p.Name + ": " + err.Error()}
}

// Jobs: a job's id is "<provider>:<the provider's id>", so get and content find the provider again.

func jobID(p Provider, id string) string { return p.Name + ":" + id }

// job resolves a job id to its provider's driver and the provider's id.
func (r *router) job(id string) (target, string, error) {
	name, pid, ok := strings.Cut(id, ":")
	if !ok || name == "" || pid == "" {
		return target{}, "", &extkit.Error{Code: "bad_request", Status: 400, Message: fmt.Sprintf("job %q: not a job id", id)}
	}
	s, err := r.settings()
	if err != nil {
		return target{}, "", err
	}
	p, ok := s.provider(name)
	if !ok {
		return target{}, "", &extkit.Error{Code: "not_found", Status: 404, Message: fmt.Sprintf("job %q: no provider named %q", id, name)}
	}
	d, err := newDriver(p)
	if err != nil {
		return target{}, "", err
	}
	return target{p: p, d: d}, pid, nil
}

// waitJob polls a job until it stops or wait runs out; a job that's done comes back with its files downloaded.
func (r *router) waitJob(ctx context.Context, t target, kind string, j llmc.Job, wait string) (llmc.Job, error) {
	d, err := time.ParseDuration(wait)
	if err != nil || d <= 0 {
		return j, nil
	}
	ctx, cancel := context.WithTimeout(ctx, d)
	defer cancel()
	pid := strings.TrimPrefix(j.ID, t.p.Name+":")
	interval := t.d.pollInterval()
	for j.Status == llmc.JobQueued || j.Status == llmc.JobRunning {
		select {
		case <-ctx.Done():
			return j, nil // still running: the caller follows it with get
		case <-time.After(interval):
		}
		next, err := t.d.getJob(ctx, kind, pid)
		if err != nil {
			if ctx.Err() != nil {
				return j, nil
			}
			return j, providerError(t.p, err)
		}
		next.ID, next.Model = j.ID, j.Model
		j = next
	}
	if j.Status == llmc.JobDone && len(j.Files) == 0 {
		n := max(j.Outputs, 1)
		for i := range n {
			f, err := t.d.jobContent(ctx, kind, pid, i)
			if err != nil {
				return j, providerError(t.p, err)
			}
			j.Files = append(j.Files, f)
		}
		j.Outputs = len(j.Files)
	}
	return j, nil
}
