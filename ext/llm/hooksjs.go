package llm

import (
	"encoding/json"
	"errors"
	"fmt"

	"ganigeorgiev.com/pocketbase/pocketbase/core"

	llmc "pocketone.dev/ext/contracts/llm"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/js"
)

// $llm.onCall and $llm.onCallDone: page code's handlers on the OnCall and OnCallDone hooks. A handler runs on the app's
// JS VM (js.Func.Call schedules it there from the goroutine the model call runs on, and awaits an async handler). It
// gets the event as JSON and hands its changes back as JSON; the prelude turns both into a plain JS object with
// next() and refuse(). Handlers bound here have the default priority, so they run before the built-in limits.

// jsCallEvent is what an onCall handler sees.
type jsCallEvent struct {
	Kind         string          `json:"kind"`
	Op           string          `json:"op"`
	Model        string          `json:"model"`
	Subject      string          `json:"subject"`
	User         *jsUser         `json:"user"`  // the signed-in user the call is for, or null
	Guest        bool            `json:"guest"` // the call is for a guest
	Input        json.RawMessage `json:"input"`
	GuestsCapped bool            `json:"guestsCapped"`
}

type jsUser struct {
	Collection string `json:"collection"`
	ID         string `json:"id"`
}

// jsCallChanges is what an onCall handler hands back.
type jsCallChanges struct {
	Model        string          `json:"model"`
	Subject      string          `json:"subject"`
	Input        json.RawMessage `json:"input"`
	GuestsCapped bool            `json:"guestsCapped"`
	Next         bool            `json:"next"`
}

type jsDoneEvent struct {
	Kind    string     `json:"kind"`
	Op      string     `json:"op"`
	Model   string     `json:"model"`
	Subject string     `json:"subject"`
	User    *jsUser    `json:"user"`
	Guest   bool       `json:"guest"`
	Usage   llmc.Usage `json:"usage"`
	Job     string     `json:"job,omitempty"`
}

func userOf(subject string) *jsUser {
	if p := llmc.ParseSubject(subject); p.Kind == "user" {
		return &jsUser{Collection: p.Collection, ID: p.ID}
	}
	return nil
}

// bindHooksJS adds onCall(fn) and onCallDone(fn) to the native object; the prelude wraps them (hooksPrelude).
func bindHooksJS(app core.App, n js.Object) {
	_ = n.Set("onCall", js.NativeFunc(func(c js.Call) (any, error) {
		f := c.Arg(0).Func()
		if f == nil {
			return nil, errors.New("$llm.onCall: give it a function")
		}
		llmc.OnCall(app).BindFunc(func(e *llmc.CallEvent) error {
			input, _ := json.Marshal(e.Input)
			ev, _ := json.Marshal(jsCallEvent{Kind: e.Kind, Op: e.Op, Model: e.Model, Subject: e.Subject,
				User: userOf(e.Subject), Guest: llmc.IsGuest(e.Subject), Input: input, GuestsCapped: e.GuestsCapped})
			out, err := f.Call(string(ev))
			if err != nil {
				return thrownError(err)
			}
			s, _ := out.(string)
			var ch jsCallChanges
			if err := json.Unmarshal([]byte(s), &ch); err != nil {
				return fmt.Errorf("$llm.onCall: %w", err)
			}
			e.Model, e.Subject, e.GuestsCapped = ch.Model, ch.Subject, ch.GuestsCapped
			if len(ch.Input) > 0 && string(ch.Input) != "null" && e.Input != nil {
				if err := json.Unmarshal(ch.Input, e.Input); err != nil {
					return badInput("$llm.onCall: e.input: " + err.Error())
				}
			}
			if !ch.Next {
				return nil // the chain stops here, which refuses the call (see begin)
			}
			return e.Next()
		})
		return nil, nil
	}))
	_ = n.Set("onCallDone", js.NativeFunc(func(c js.Call) (any, error) {
		f := c.Arg(0).Func()
		if f == nil {
			return nil, errors.New("$llm.onCallDone: give it a function")
		}
		llmc.OnCallDone(app).BindFunc(func(e *llmc.CallDoneEvent) error {
			ev, _ := json.Marshal(jsDoneEvent{Kind: e.Kind, Op: e.Op, Model: e.Model, Subject: e.Subject,
				User: userOf(e.Subject), Guest: llmc.IsGuest(e.Subject), Usage: e.Usage, Job: e.Job})
			if _, err := f.Call(string(ev)); err != nil {
				app.Logger().Warn("llm: an onCallDone handler failed", "model", e.Model, "error", js.Message(err))
			}
			return e.Next()
		})
		return nil, nil
	}))
}

// thrownError turns what an onCall handler threw into the call's error: e.refuse(message) (or anything thrown with a
// code) becomes that code's error; anything else is a handler bug, reported as one.
func thrownError(err error) error {
	if v, ok := js.Thrown(err); ok {
		if m, isObj := v.(map[string]any); isObj && m["code"] != nil {
			status := 400
			switch s := m["status"].(type) {
			case int64:
				status = int(s)
			case float64:
				status = int(s)
			}
			return &extkit.Error{Code: fmt.Sprint(m["code"]), Status: status, Message: js.Message(err)}
		}
	}
	return &extkit.Error{Code: "js_error", Status: 500, Message: "$llm.onCall: " + js.Message(err)}
}

// hooksPrelude is the JS side: plain event objects with next() and refuse(); handlers may be async.
const hooksPrelude = `
  const hooks = {
    onCall: (fn) => n.onCall(async (raw) => {
      const e = JSON.parse(raw);
      let next = false;
      e.next = () => { next = true; };
      e.refuse = (message) => { const err = new Error(message); err.code = "limit"; err.status = 429; return err; };
      await fn(e);
      return JSON.stringify({ model: e.model, subject: e.subject ?? "", input: e.input, guestsCapped: !!e.guestsCapped, next });
    }),
    onCallDone: (fn) => n.onCallDone(async (raw) => {
      const e = JSON.parse(raw);
      e.next = () => {};
      await fn(e);
    }),
  };
`
