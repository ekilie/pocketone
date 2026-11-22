package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/filesystem"

	llmc "pocketone.dev/ext/contracts/llm"
	alignv1 "pocketone.dev/ext/contracts/llm/align/v1"
	embedv1 "pocketone.dev/ext/contracts/llm/embed/v1"
	imagev1 "pocketone.dev/ext/contracts/llm/image/v1"
	musicv1 "pocketone.dev/ext/contracts/llm/music/v1"
	speechv1 "pocketone.dev/ext/contracts/llm/speech/v1"
	textv1 "pocketone.dev/ext/contracts/llm/text/v1"
	transcribev1 "pocketone.dev/ext/contracts/llm/transcribe/v1"
	videov1 "pocketone.dev/ext/contracts/llm/video/v1"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/js"
)

// BindJS exposes the llm.* contracts to the app's page code as $llm.<kind>.<operation>Async. Every call that reaches a
// provider is async only and says so in its name (await $llm.text.completeAsync({...})): the app has one JS VM, and a
// model call that held it for seconds would make every other request wait. Only local calls (count, file, the chat
// records) are plain. The Go side takes and answers JSON text; a small JS layer (jsPrelude) builds $llm over it and
// runs each provider call with the engine's .async, off the VM.
func (*Extension) BindJS(app core.App, rt js.Runtime) {
	invokers := map[string]func(op string, raw json.RawMessage) (any, error){
		llmc.KindText:       invoker(app, textv1.Contract, textv1.W),
		llmc.KindImage:      invoker(app, imagev1.Contract, imagev1.W),
		llmc.KindSpeech:     invoker(app, speechv1.Contract, speechv1.W),
		llmc.KindTranscribe: invoker(app, transcribev1.Contract, transcribev1.W),
		llmc.KindAlign:      invoker(app, alignv1.Contract, alignv1.W),
		llmc.KindMusic:      invoker(app, musicv1.Contract, musicv1.W),
		llmc.KindVideo:      invoker(app, videov1.Contract, videov1.W),
		llmc.KindEmbed:      invoker(app, embedv1.Contract, embedv1.W),
	}
	n := rt.NewObject()
	// call(kind, op, opts): any operation that answers JSON; the prelude calls it with .async
	_ = n.Set("call", func(kind, op, opts string) (string, error) {
		inv, ok := invokers[kind]
		if !ok {
			return "", fmt.Errorf("$llm: no kind %q", kind)
		}
		res, err := inv(op, json.RawMessage(opts))
		if err != nil {
			return "", err
		}
		b, err := json.Marshal(res)
		return string(b), err
	})
	_ = n.Set("streamStart", func(kind, opts string) (string, error) { return streamStart(app, kind, opts) })
	_ = n.Set("streamNext", streamNext)
	_ = n.Set("streamStop", streamStop)
	// file(f, download): a generated file as a PocketBase file, for record.set("field", ...). Plain $llm.file takes only
	// a file with its bytes; fileAsync may download one the provider only gave a URL for.
	_ = n.Set("file", func(f string, download bool) (*filesystem.File, error) {
		var v llmc.File
		if err := json.Unmarshal([]byte(f), &v); err != nil {
			return nil, errors.New("$llm.file: " + err.Error())
		}
		if len(v.Data) > 0 {
			return filesystem.NewFileFromBytes(v.Data, orDefault(v.Name, "file"))
		}
		if v.URL == "" {
			return nil, errors.New("$llm.file: the file has no data")
		}
		if !download {
			return nil, errors.New("$llm.file: the file has only a URL; use await $llm.fileAsync(file) to download it")
		}
		return filesystem.NewFileFromURL(context.Background(), v.URL)
	})
	// subject(e): whom a request's calls are for ($llm.for(e)); sets the guest cookie when the request has none
	_ = n.Set("subject", func(e *core.RequestEvent) string { return SubjectOf(app, e) })
	bindChatJS(app, rt, n)
	if err := rt.Set(nativeGlobal, n); err != nil {
		panic(fmt.Errorf("llm: bind JS: %w", err))
	}
	if _, err := rt.Eval(jsPrelude, "ext/llm"); err != nil {
		panic(fmt.Errorf("llm: bind JS: %w", err))
	}
}

// invoker calls a contract's operation by name with JSON input, through the provider's guarded side (scope checks
// and action hooks run as for any call).
func invoker[T any](app core.App, c *extkit.Contract[T], w *extkit.Wire[T]) func(op string, raw json.RawMessage) (any, error) {
	return func(op string, raw json.RawMessage) (any, error) {
		impl, ok := c.Of(app)
		if !ok {
			return nil, llmc.ErrNotConfigured
		}
		for _, o := range w.Ops {
			if o.Name != op || o.Out != extkit.OutJSON {
				continue
			}
			req := extkit.OpRequest{JSON: raw}
			if o.Method == "GET" {
				req.JSON, req.Query = nil, queryOf(raw)
			}
			return o.Invoke(subjectCtx(raw), impl, extkit.Internal, req)
		}
		return nil, fmt.Errorf("$llm: %s has no operation %q", c.ID(), op)
	}
}

// subjectCtx is a context carrying the subject $llm.for(e) put in a call's input as "__subject" (contract inputs
// ignore the key). Only page code calls $llm, and page code is the app's own, so the subject is trusted as given.
func subjectCtx[B ~string | ~[]byte](raw B) context.Context {
	var v struct {
		Subject string `json:"__subject"`
	}
	_ = json.Unmarshal([]byte(raw), &v)
	if v.Subject == "" {
		return context.Background()
	}
	return llmc.WithSubject(context.Background(), v.Subject)
}

// queryOf is a GET operation's input (a JSON object) as query values.
func queryOf(raw json.RawMessage) url.Values {
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	q := url.Values{}
	for k, v := range m {
		if v != nil && k != "__subject" {
			q.Set(k, fmt.Sprint(v))
		}
	}
	return q
}

const nativeGlobal = "__pocketone_llm"

// jsPrelude builds $llm over the Go functions, which it takes off the global scope.
var jsPrelude = `(() => {
  const n = globalThis.__pocketone_llm;
  delete globalThis.__pocketone_llm;
  const J = (s) => JSON.parse(s);
  // build(sub) is $llm with every call made for subject sub ("" for nobody in particular): $llm.for(e) names it
  const build = (sub) => {
  const S = (o) => JSON.stringify(sub ? { ...(o ?? {}), __subject: sub } : (o ?? {}));
  // a provider call, off the VM
  const callAsync = async (kind, op, o) => J(await n.call.async(kind, op, S(o)));
  const text = (o) => (o && typeof o.schema === "string" ? { ...o, schema: JSON.parse(o.schema) } : o);
  const count = (x) => typeof x === "string" ? { text: x } : Array.isArray(x) ? { messages: x } : (x ?? {});
  const modelsAsync = (kind) => (o) => callAsync(kind, "models", o);
  const jobs = (kind) => Object.freeze({
    createAsync: (o) => callAsync(kind, "create", o),
    getAsync: (id) => callAsync(kind, "get", { id }),
    contentAsync: (id, index) => callAsync(kind, "content", { id, index: index ?? 0 }),
    modelsAsync: modelsAsync(kind),
  });` + streamPrelude + chatPrelude + `
  return Object.freeze({
    text: Object.freeze({
      completeAsync: (o) => callAsync("text", "complete", text(o)),
      streamAsync: async (o, onText, onReasoning) => readStream(n.streamStart("text", S(text(o))), onText, onReasoning),
      count: (x) => J(n.call("text", "count", S(count(x)))).tokens,
      modelsAsync: modelsAsync("text"),
    }),
    image: Object.freeze({
      generateAsync: (o) => callAsync("image", "generate", o),
      editAsync: (o) => callAsync("image", "edit", o),
      modelsAsync: modelsAsync("image"),
    }),
    speech: Object.freeze({
      speakAsync: (o) => callAsync("speech", "speak", o),
      voicesAsync: async (model) => (await callAsync("speech", "voices", { model })).voices,
      modelsAsync: modelsAsync("speech"),
    }),
    transcribe: Object.freeze({
      transcribeAsync: (o) => callAsync("transcribe", "transcribe", o),
      modelsAsync: modelsAsync("transcribe"),
    }),
    align: Object.freeze({ alignAsync: (o) => callAsync("align", "align", o), modelsAsync: modelsAsync("align") }),
    music: jobs("music"),
    video: jobs("video"),
    embed: Object.freeze({ embedAsync: (o) => callAsync("embed", "embed", o), modelsAsync: modelsAsync("embed") }),
    file: (f) => n.file(S(f), false),
    fileAsync: (f) => n.file.async(S(f), true),
    chat,
  });
  };
  const base = build("");
  // for(e): e is a request event (a pb_hooks route's e) or a page's ctx (its .event)
  globalThis.$llm = Object.freeze({ ...base, for: (e) => build(n.subject(e && e.event ? e.event : e)) });
})()`
