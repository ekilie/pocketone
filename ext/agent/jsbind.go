package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	agentv1 "pocketone.dev/ext/contracts/agent/v1"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/js"
)

// BindJS exposes agent/v1 to the app's JS hooks as $agent (and, when a provider of agent.chat/v1 runs, the chat
// methods: chat.go). The Go side answers in JSON text and a small JS layer (jsPrelude) builds $agent over it, so
// hooks get plain JS data, and so each blocking call has two forms: the plain one fails fast while the model loads,
// and its .async twin (run off the app's VM by the JS engine) waits for it.
func (*Extension) BindJS(app core.App, rt js.Runtime) {
	get := func() (agentv1.Agent, error) {
		a, ok := agentv1.Contract.Of(app)
		if !ok {
			return nil, agentv1.ErrNotConfigured
		}
		return a, nil
	}
	cfg, _ := Section.Get()
	defaultWait := cfg.withDefaults().timeout().String()
	n := rt.NewObject()
	// complete(opts, wait): wait=true is .async, which waits for the model up to opts.wait (default agent.timeout)
	_ = n.Set("complete", func(opts string, wait bool) (string, error) {
		in, err := completeInput(opts)
		if err != nil {
			return "", errors.New("$agent.complete: " + err.Error())
		}
		if wait && in.Wait == "" {
			in.Wait = defaultWait
		}
		a, err := get()
		if err != nil {
			return "", err
		}
		return jsonOf(a.Complete(context.Background(), extkit.Internal, in))
	})
	// stream(opts, onText, {onWait}): on the calling VM, so onText can write to the response as the answer comes
	_ = n.Set("stream", js.NativeFunc(func(c js.Call) (any, error) { return streamJS(c, get) }))
	_ = n.Set("status", func() (string, error) {
		a, err := get()
		if err != nil {
			return jsonOf(agentv1.Status{Message: err.Error()}, nil)
		}
		return jsonOf(a.Status(context.Background(), extkit.Internal))
	})
	// warm(opts, wait): returns at once; wait=true (.async) waits for the warm, up to opts.wait (default agent.timeout)
	_ = n.Set("warm", func(opts string, wait bool) (string, error) {
		var in agentv1.WarmInput
		if err := json.Unmarshal([]byte(opts), &in); err != nil {
			return "", errors.New("$agent.warm: " + err.Error())
		}
		if wait && in.Wait == "" {
			in.Wait = defaultWait
		}
		if !wait {
			in.Wait = ""
		}
		a, err := get()
		if err != nil {
			return jsonOf(agentv1.WarmResult{Message: err.Error()}, nil) // never throws at boot
		}
		return jsonOf(a.Warm(context.Background(), extkit.Internal, in))
	})
	_ = n.Set("forget", func(id string) error {
		a, err := get()
		if err != nil {
			return nil
		}
		_, err = a.Forget(context.Background(), extkit.Internal, agentv1.ForgetInput{ID: id})
		return err
	})
	_ = n.Set("count", func(opts string) (string, error) {
		var in agentv1.CountInput
		if err := json.Unmarshal([]byte(opts), &in); err != nil {
			return "", errors.New("$agent.count: " + err.Error())
		}
		a, err := get()
		if err != nil {
			return jsonOf(countOf(nil, in), nil)
		}
		return jsonOf(a.Count(context.Background(), extkit.Internal, in))
	})
	_ = n.Set("threads", func() (string, error) {
		a, err := get()
		if err != nil {
			return jsonOf(agentv1.ThreadList{Threads: []agentv1.ThreadInfo{}}, nil)
		}
		return jsonOf(a.Threads(context.Background(), extkit.Internal))
	})
	_ = n.Set("forgetThread", func(id string) error {
		a, err := get()
		if err != nil {
			return nil
		}
		_, err = a.ForgetThread(context.Background(), extkit.Internal, agentv1.ForgetInput{ID: id})
		return err
	})
	bindChatJS(app, rt, n)
	if err := rt.Set(nativeGlobal, n); err != nil {
		panic(fmt.Errorf("agent: bind JS: %w", err))
	}
	if _, err := rt.Eval(jsPrelude, "ext/agent"); err != nil {
		panic(fmt.Errorf("agent: bind JS: %w", err))
	}
}

const nativeGlobal = "__pocketone_agent"

// jsPrelude builds $agent over the Go functions, which it takes off the global scope.
const jsPrelude = `(() => {
  const n = globalThis.__pocketone_agent;
  delete globalThis.__pocketone_agent;
  const J = (s) => JSON.parse(s);
  const S = (o) => JSON.stringify(o ?? {});
  const twin = (sync, async) => Object.defineProperty(sync, "async", { value: async });
  const handle = (r) => Object.assign(r, { forget: () => n.forget(r.id) });
  const countInput = (x) => typeof x === "string" ? { text: x } : Array.isArray(x) ? { messages: x } : (x ?? {});
  const agent = {
    complete: twin((o) => J(n.complete(S(o), false)), async (o) => J(await n.complete.async(S(o), true))),
    stream: (o, onText, w) => J(n.stream(o, onText, w)),
    status: () => J(n.status()),
    warm: twin((o) => handle(J(n.warm(S(o), false))), async (o) => handle(J(await n.warm.async(S(o), true)))),
    forget: (id) => n.forget(id),
    count: (x) => J(n.count(S(countInput(x)))).tokens,
    threads: Object.freeze({ list: () => J(n.threads()).threads, forget: (id) => n.forgetThread(id) }),
  };
  if (n.chatNew) Object.assign(agent, globalThis.__pocketone_agent_chat(n, J, S, twin));
  delete globalThis.__pocketone_agent_chat;
  globalThis.$agent = Object.freeze(agent);
})()`

// completeInput reads $agent.complete's options (JSON text) into the contract's input; schema may be an object or a
// JSON string.
func completeInput(opts string) (agentv1.CompleteInput, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(opts), &m); err != nil {
		return agentv1.CompleteInput{}, err
	}
	return inputFromJS(m)
}

func jsonOf(v any, err error) (string, error) {
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(v)
	return string(b), err
}

// streamJS is $agent.stream(opts, onText, {onWait}): onText(piece) as the answer is written (false stops it). With
// opts.wait, a model that's still loading is waited for first (waitWithStatus); without it, a loading model fails at
// once with not_ready.
func streamJS(c js.Call, get func() (agentv1.Agent, error)) (any, error) {
	opts, _ := c.Arg(0).Export().(map[string]any)
	if opts == nil {
		return nil, errors.New("$agent.stream: the first argument must be an options object")
	}
	in, err := inputFromJS(opts)
	if err != nil {
		return nil, errors.New("$agent.stream: " + err.Error())
	}
	a, err := get()
	if err != nil {
		return nil, err
	}
	var onWait js.Func
	if obj := c.Arg(2).Object(); obj != nil {
		onWait = obj.Get("onWait").Func()
	}
	if err := waitWithStatus(a, in.Wait, onWait); err != nil {
		return nil, err
	}
	in.Wait = ""
	s, err := a.Stream(context.Background(), extkit.Internal, in)
	if err != nil {
		return nil, err
	}
	if s.Close != nil {
		defer s.Close()
	}
	res, err := runStream(s, c.Arg(1).Func())
	if err != nil {
		return nil, err
	}
	return jsonOf(res, nil)
}

// waitWithStatus waits up to wait (a duration) for a's model to be ready, calling onWait(status) about twice a second
// while it isn't. It returns at once with no wait, a ready model, or no model configured (the call then says why).
func waitWithStatus(a agentv1.Agent, wait string, onWait js.Func) error {
	d, _ := time.ParseDuration(wait)
	if d <= 0 {
		return nil
	}
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		st, err := a.Status(context.Background(), extkit.Internal)
		if err != nil || st.Ready || st.Backend == "" {
			return nil
		}
		if onWait != nil {
			v, _ := toJS(st)
			if _, err := onWait.Call(v); err != nil {
				return errors.New(js.Message(err))
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}

// runStream runs an agent/v1 stream on the calling VM, calling onText(piece) for each piece (false stops it), and
// returns the result.
func runStream(s extkit.Stream, onText js.Func) (*agentv1.CompleteResult, error) {
	var res *agentv1.CompleteResult
	var cbErr error
	err := s.Run(context.Background(), func(e any) error {
		ch, _ := e.(agentv1.Chunk)
		if ch.Done {
			res = ch.Result
			return nil
		}
		if onText == nil || ch.Text == "" {
			return nil
		}
		v, err := onText.Call(ch.Text)
		if err != nil {
			cbErr = errors.New(js.Message(err))
			return cbErr
		}
		if v == false {
			return errStopped
		}
		return nil
	})
	if cbErr != nil {
		return nil, cbErr
	}
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, errors.New("the answer ended without a result")
	}
	return res, nil
}
