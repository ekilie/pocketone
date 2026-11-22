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

// BindJS exposes the llm.* contracts to the app's JS hooks as $llm.<kind>.<operation>. The Go side takes and answers
// JSON text, and a small JS layer (jsPrelude) builds $llm over it, so hooks get plain JS data. Each call has an .async
// twin, run off the app's VM by the JS engine, for slow calls (music and video with wait).
func (*Extension) BindJS(app core.App, rt js.Runtime) {
	invokers := map[string]func(op string, raw json.RawMessage) (any, error){
		llmc.KindText:       invoker(app, textv1.Contract, textv1.W),
		llmc.KindImage:      invoker(app, imagev1.Contract, imagev1.W),
		llmc.KindSpeech:     invoker(app, speechv1.Contract, speechv1.W),
		llmc.KindTranscribe: invoker(app, transcribev1.Contract, transcribev1.W),
		llmc.KindMusic:      invoker(app, musicv1.Contract, musicv1.W),
		llmc.KindVideo:      invoker(app, videov1.Contract, videov1.W),
		llmc.KindEmbed:      invoker(app, embedv1.Contract, embedv1.W),
	}
	n := rt.NewObject()
	// call(kind, op, opts): any operation that answers JSON
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
	// stream(opts, onText, onReasoning): on the calling VM, so onText can write to the response as the answer comes
	_ = n.Set("stream", js.NativeFunc(func(c js.Call) (any, error) { return streamJS(app, c) }))
	// file(f): a generated file as a PocketBase file, for record.set("field", ...)
	_ = n.Set("file", func(f string) (*filesystem.File, error) {
		var v llmc.File
		if err := json.Unmarshal([]byte(f), &v); err != nil {
			return nil, errors.New("$llm.file: " + err.Error())
		}
		if len(v.Data) == 0 {
			if v.URL == "" {
				return nil, errors.New("$llm.file: the file has no data")
			}
			return filesystem.NewFileFromURL(context.Background(), v.URL)
		}
		return filesystem.NewFileFromBytes(v.Data, orDefault(v.Name, "file"))
	})
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
			return o.Invoke(context.Background(), impl, extkit.Internal, req)
		}
		return nil, fmt.Errorf("$llm: %s has no operation %q", c.ID(), op)
	}
}

// queryOf is a GET operation's input (a JSON object) as query values.
func queryOf(raw json.RawMessage) url.Values {
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	q := url.Values{}
	for k, v := range m {
		if v != nil {
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
  const S = (o) => JSON.stringify(o ?? {});
  const twin = (sync, async) => Object.defineProperty(sync, "async", { value: async });
  const op = (kind, name, prep) => twin(
    (o) => J(n.call(kind, name, S(prep ? prep(o) : o))),
    async (o) => J(await n.call.async(kind, name, S(prep ? prep(o) : o))),
  );
  const text = (o) => (o && typeof o.schema === "string" ? { ...o, schema: JSON.parse(o.schema) } : o);
  const count = (x) => typeof x === "string" ? { text: x } : Array.isArray(x) ? { messages: x } : (x ?? {});
  const models = (kind) => (o) => J(n.call(kind, "models", S(o)));
  const jobs = (kind) => Object.freeze({
    create: op(kind, "create"),
    get: (id) => J(n.call(kind, "get", S({ id }))),
    content: (id, index) => J(n.call(kind, "content", S({ id, index: index ?? 0 }))),
    models: models(kind),
  });` + chatPrelude + `
  globalThis.$llm = Object.freeze({
    chat,
    text: Object.freeze({
      complete: op("text", "complete", text),
      stream: (o, onText, onReasoning) => J(n.stream(text(o), onText, onReasoning)),
      count: (x) => J(n.call("text", "count", S(count(x)))).tokens,
      models: models("text"),
    }),
    image: Object.freeze({ generate: op("image", "generate"), edit: op("image", "edit"), models: models("image") }),
    speech: Object.freeze({
      speak: op("speech", "speak"),
      voices: (model) => J(n.call("speech", "voices", S({ model }))).voices,
      models: models("speech"),
    }),
    transcribe: Object.freeze({ transcribe: op("transcribe", "transcribe"), models: models("transcribe") }),
    music: jobs("music"),
    video: jobs("video"),
    embed: Object.freeze({ embed: op("embed", "embed"), models: models("embed") }),
    file: (f) => n.file(S(f)),
  });
})()`

// streamJS is $llm.text.stream(opts, onText, onReasoning): onText(piece) as the answer is written (false stops it),
// onReasoning(piece) for the model's thinking when the provider streams it.
func streamJS(app core.App, c js.Call) (any, error) {
	opts := c.Arg(0).Export()
	b, err := json.Marshal(opts)
	if err != nil {
		return nil, errors.New("$llm.text.stream: " + err.Error())
	}
	var in textv1.CompleteInput
	if err := json.Unmarshal(b, &in); err != nil {
		return nil, errors.New("$llm.text.stream: " + err.Error())
	}
	t, ok := textv1.Contract.Of(app)
	if !ok {
		return nil, llmc.ErrNotConfigured
	}
	s, err := t.Stream(context.Background(), extkit.Internal, in)
	if err != nil {
		return nil, err
	}
	if s.Close != nil {
		defer s.Close()
	}
	res, err := runStream(s, c.Arg(1).Func(), c.Arg(2).Func())
	if err != nil {
		return nil, err
	}
	out, err := json.Marshal(res)
	return string(out), err
}

// runStream runs a llm.text/v1 stream on the calling VM and returns the result.
func runStream(s extkit.Stream, onText, onReasoning js.Func) (*textv1.CompleteResult, error) {
	var res *textv1.CompleteResult
	var cbErr error
	call := func(f js.Func, piece string) error {
		if f == nil || piece == "" {
			return nil
		}
		v, err := f.Call(piece)
		if err != nil {
			cbErr = errors.New(js.Message(err))
			return cbErr
		}
		if v == false {
			return errStopped
		}
		return nil
	}
	err := s.Run(context.Background(), func(e any) error {
		ch, _ := e.(textv1.Chunk)
		if ch.Done {
			res = ch.Result
			return nil
		}
		if err := call(onReasoning, ch.Reasoning); err != nil {
			return err
		}
		return call(onText, ch.Text)
	})
	if cbErr != nil {
		return nil, cbErr
	}
	if err != nil && !errors.Is(err, errStopped) {
		return nil, err
	}
	if res == nil {
		return &textv1.CompleteResult{StopReason: "cancelled"}, nil
	}
	return res, nil
}
