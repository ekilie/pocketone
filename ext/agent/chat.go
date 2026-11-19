package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	chatv1 "github.com/pockethost/pocketone/ext/contracts/agent/chat/v1"
	agentv1 "github.com/pockethost/pocketone/ext/contracts/agent/v1"
	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/extkit/js"
)

// errNoChat: no extension provides agent.chat/v1 on the app.
var errNoChat = errors.New("$agent chats aren't available: add the agent-chat extension (agent.chat/v1)")

// bindChatJS adds the natives under $agent.newChat, $agent.chat(id) and $agent.chats. They look agent.chat/v1 up on
// each call, so they work with whichever extension provides it, and say what's missing when none does. ext/agent
// imports only the contract, never ext/agent-chat.
func bindChatJS(app core.App, rt js.Runtime, n js.Object) {
	get := func() (chatv1.Chats, error) {
		c, ok := chatv1.Contract.Of(app)
		if !ok {
			return nil, errNoChat
		}
		return c, nil
	}
	cfg, _ := Section.Get()
	defaultWait := cfg.withDefaults().timeout().String()
	ctx := context.Background()
	call := func(fn func(c chatv1.Chats, raw []byte) (any, error)) func(string) (string, error) {
		return func(opts string) (string, error) {
			c, err := get()
			if err != nil {
				return "", err
			}
			v, err := fn(c, []byte(opts))
			if err != nil {
				return "", err
			}
			return jsonOf(v, nil)
		}
	}
	_ = n.Set("chatNew", call(func(c chatv1.Chats, raw []byte) (any, error) {
		var in chatv1.NewInput
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, fmt.Errorf("$agent.newChat: %w", err)
		}
		return c.New(ctx, extkit.Internal, in)
	}))
	_ = n.Set("chatSend", func(opts string, wait bool) (string, error) {
		c, err := get()
		if err != nil {
			return "", err
		}
		var in chatv1.SendInput
		if err := json.Unmarshal([]byte(opts), &in); err != nil {
			return "", fmt.Errorf("chat.send: %w", err)
		}
		if wait && in.Wait == "" {
			in.Wait = defaultWait
		}
		return jsonOf(c.Send(ctx, extkit.Internal, in))
	})
	_ = n.Set("chatStream", js.NativeFunc(func(cl js.Call) (any, error) {
		c, err := get()
		if err != nil {
			return nil, err
		}
		var in chatv1.SendInput
		if err := json.Unmarshal([]byte(cl.Arg(0).String()), &in); err != nil {
			return nil, fmt.Errorf("chat.send: %w", err)
		}
		var onWait js.Func
		if obj := cl.Arg(2).Object(); obj != nil {
			onWait = obj.Get("onWait").Func()
		}
		if a, ok := agentv1.Contract.Of(app); ok {
			if err := waitWithStatus(a, in.Wait, onWait); err != nil {
				return nil, err
			}
			in.Wait = ""
		}
		s, err := c.SendStream(ctx, extkit.Internal, in)
		if err != nil {
			return nil, err
		}
		if s.Close != nil {
			defer s.Close()
		}
		res, err := runStream(s, cl.Arg(1).Func())
		if err != nil {
			return nil, err
		}
		return jsonOf(chatv1.SendResult{Answer: *res}, nil)
	}))
	_ = n.Set("chatGet", func(id string) (string, error) {
		c, err := get()
		if err != nil {
			return "", err
		}
		return jsonOf(c.Get(ctx, extkit.Internal, chatv1.Ref{ID: id}))
	})
	_ = n.Set("chatHistory", call(func(c chatv1.Chats, raw []byte) (any, error) {
		var in chatv1.HistoryInput
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, fmt.Errorf("chat.history: %w", err)
		}
		return c.History(ctx, extkit.Internal, in)
	}))
	_ = n.Set("chatUpdate", call(func(c chatv1.Chats, raw []byte) (any, error) {
		var in chatv1.UpdateInput
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, fmt.Errorf("chat: %w", err)
		}
		return c.Update(ctx, extkit.Internal, in)
	}))
	_ = n.Set("chatForget", func(id string) error {
		c, err := get()
		if err != nil {
			return err
		}
		_, err = c.Forget(ctx, extkit.Internal, chatv1.Ref{ID: id})
		return err
	})
	_ = n.Set("chatsList", call(func(c chatv1.Chats, raw []byte) (any, error) {
		var in chatv1.ListInput
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, fmt.Errorf("$agent.chats.list: %w", err)
		}
		return c.List(ctx, extkit.Internal, in)
	}))
	_ = n.Set("chatsForget", call(func(c chatv1.Chats, raw []byte) (any, error) {
		var in chatv1.ListInput
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, fmt.Errorf("$agent.chats.forget: %w", err)
		}
		if len(in.Meta) == 0 {
			return nil, errors.New("$agent.chats.forget: give the meta to match")
		}
		return c.ForgetWhere(ctx, extkit.Internal, in)
	}))
	if _, err := rt.Eval(chatPrelude, "ext/agent chat"); err != nil {
		panic(fmt.Errorf("agent: bind JS: %w", err))
	}
}

// chatPrelude defines the chat part of $agent, which jsPrelude adds. A chat's context option may be a function of
// the question; it runs in the hook's VM before the call (also for send.async).
const chatPrelude = `globalThis.__pocketone_agent_chat = (n, J, S, twin) => {
  const contextOf = (c, q) => typeof c === "function" ? String(c(q) ?? "") : (c ?? "");
  const input = (id, o, text, s) => ({ id, text, system: o.system, window: o.window, summarizeEvery: o.summarizeEvery,
    maxTokens: s.maxTokens ?? o.maxTokens, temperature: s.temperature ?? o.temperature, wait: s.wait ?? o.wait,
    schema: s.schema, context: contextOf(s.context ?? o.context, text) });
  const chat = (id, o = {}) => {
    if (!id) throw new Error("$agent.chat: give the chat's id ($agent.newChat() returns one)");
    const send = twin((text, s = {}) => {
      const i = S(input(id, o, text, s));
      return (s.onText || s.onWait ? J(n.chatStream(i, s.onText, { onWait: s.onWait })) : J(n.chatSend(i, false))).answer;
    }, async (text, s = {}) => J(await n.chatSend.async(S(input(id, o, text, s)), true)).answer);
    return Object.freeze({
      id,
      send,
      get: () => J(n.chatGet(id)),
      history: (h = {}) => J(n.chatHistory(S({ id, limit: h.limit, before: h.before }))).messages,
      summary: () => J(n.chatGet(id)).summary,
      title: (t) => t === undefined ? J(n.chatGet(id)).title : J(n.chatUpdate(S({ id, title: String(t) }))).title,
      meta: (m) => m === undefined ? J(n.chatGet(id)).meta : J(n.chatUpdate(S({ id, meta: m }))).meta,
      forget: () => { n.chatForget(id) },
    });
  };
  return {
    newChat: (o = {}) => J(n.chatNew(S(o))).id,
    chat,
    chats: Object.freeze({
      list: (o = {}) => J(n.chatsList(S(o))).chats,
      forget: (o = {}) => J(n.chatsForget(S(o))).count,
    }),
  };
}`
