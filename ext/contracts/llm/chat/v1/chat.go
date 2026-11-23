// Package chatv1 is the llm.chat/v1 contract: stored conversations with the app's language model. A chat is an id
// the app keeps (on its own records, or handed to its frontend), with an optional title and the app's own meta; the
// provider stores its messages, sends the recent ones with each question (and a summary of the older ones), and
// saves both turns. It calls llm.text/v1 for the answers. Chats aren't tied to users: an app that has them puts the user
// in meta and decides who sees what.
//
// A wire contract at /api/_c/llm.chat/v1. In pb_hooks it's $llm.chat.new, $llm.chat.open(id) and $llm.chat.list.
package chatv1

//go:generate go run pocketone.dev/ext/contracts/cmd/contractgen

import (
	"context"
	"encoding/json"
	"time"

	"pocketone.dev/ext/contracts/llm"
	textv1 "pocketone.dev/ext/contracts/llm/text/v1"
	"pocketone.dev/ext/extkit"
)

// Chats stores an app's conversations with its language model.
//
//contract:id llm.chat/v1
//contract:kind wire
//contract:scope chat
//contract:js hooks
type Chats interface {
	// New starts a chat and returns it; its id is how the app finds it again.
	//
	//op: POST new
	New(ctx context.Context, a extkit.Auth, in NewInput) (Chat, error)
	// Send asks the chat a question: the provider sends the system text, the chat's summary, its recent messages and
	// the question (with Context after it) to llm.text/v1, saves both turns, and folds older messages into the summary
	// when it's due.
	//
	//op: POST send
	Send(ctx context.Context, a extkit.Auth, in SendInput) (SendResult, error)
	// SendStream is Send with the answer as it's written: llm.text/v1 chunks, the last one with done and the result.
	//
	//op: POST send-stream
	SendStream(ctx context.Context, a extkit.Auth, in SendInput) (extkit.Stream, error)
	// Get returns a chat.
	//
	//op: POST get scope=chat:read
	Get(ctx context.Context, a extkit.Auth, in Ref) (Chat, error)
	// History returns a chat's messages, oldest first: the last Limit before the message Before (all when 0 and "").
	//
	//op: POST history scope=chat:read
	History(ctx context.Context, a extkit.Auth, in HistoryInput) (History, error)
	// Update sets a chat's title or meta (fields left out stay).
	//
	//op: POST update
	Update(ctx context.Context, a extkit.Auth, in UpdateInput) (Chat, error)
	// List returns chats, most recently updated first; Meta keeps those whose meta has each of its top-level keys
	// with that value.
	//
	//op: POST list scope=chat:read
	List(ctx context.Context, a extkit.Auth, in ListInput) (ChatList, error)
	// Forget deletes a chat, its messages and the model's state for it.
	//
	//op: POST forget
	Forget(ctx context.Context, a extkit.Auth, in Ref) (Forgotten, error)
	// ForgetWhere deletes every chat whose meta matches (as in List), e.g. all of a user the app deleted.
	//
	//op: POST forget-where
	ForgetWhere(ctx context.Context, a extkit.Auth, in ListInput) (Forgotten, error)
}

// NewInput is a new chat's optional title and meta (the app's own JSON object).
type NewInput struct {
	Title string          `json:"title,omitempty"`
	Meta  json.RawMessage `json:"meta,omitempty"`
}

// Ref names a chat.
type Ref struct {
	ID string `json:"id"`
}

// Chat is a stored conversation.
type Chat struct {
	ID       string          `json:"id"`
	Title    string          `json:"title"`
	Summary  string          `json:"summary"`
	Meta     json.RawMessage `json:"meta"`
	Messages int             `json:"messages"`
	Created  time.Time       `json:"created"`
	Updated  time.Time       `json:"updated"`
}

// SendInput is a question for a chat and how to ask it.
type SendInput struct {
	ID   string `json:"id"`
	Text string `json:"text"`
	// System is the preamble, sent first on every turn (keep it the same for the prompt cache).
	System string `json:"system,omitempty"`
	// Window is how many recent messages to send (default 12); older ones reach the model through the summary.
	Window int `json:"window,omitempty"`
	// SummarizeEvery folds the messages that left the window into the summary once that many have (default 6).
	SummarizeEvery int `json:"summarizeEvery,omitempty"`
	// Context is text sent after the question (the passages that answer it, say). The chat keeps the question as sent, to send it the same way next turn; history shows the question alone.
	Context     string          `json:"context,omitempty"`
	Schema      json.RawMessage `json:"schema,omitempty"`
	MaxTokens   int             `json:"maxTokens,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	// Model is the text model to answer with ("provider/model", required).
	Model string `json:"model"`
	// Reasoning is how much a reasoning model thinks first: "", "off", "low", "medium" or "high".
	Reasoning string `json:"reasoning,omitempty"` // Enrich adds the best passages from the app's search buckets after the question, beside Context (search/v1);
	// the answer's Passages lists them.
	Enrich *llm.Enrich `json:"enrich,omitempty"`
}

// SendResult is the answer, and whether this turn folded older messages into the summary.
type SendResult struct {
	Answer     textv1.CompleteResult `json:"answer"`
	Summarized bool                  `json:"summarized,omitempty"`
}

// HistoryInput picks a page of a chat's messages.
type HistoryInput struct {
	ID     string `json:"id"`
	Limit  int    `json:"limit,omitempty"`
	Before string `json:"before,omitempty"` // a message id
}

// History is a page of messages, oldest first.
type History struct {
	Messages []Message `json:"messages"`
}

// Message is one saved turn.
type Message struct {
	ID      string    `json:"id"`
	Role    string    `json:"role"` // user or assistant
	Content string    `json:"content"`
	Created time.Time `json:"created"`
}

// UpdateInput changes a chat's title or meta.
type UpdateInput struct {
	ID    string          `json:"id"`
	Title *string         `json:"title,omitempty"`
	Meta  json.RawMessage `json:"meta,omitempty"`
}

// ListInput filters and pages chats.
type ListInput struct {
	Meta   map[string]any `json:"meta,omitempty"`
	Limit  int            `json:"limit,omitempty"` // default 50
	Offset int            `json:"offset,omitempty"`
}

// ChatList is a page of chats.
type ChatList struct {
	Chats []Chat `json:"chats"`
}

// Forgotten says how many chats were deleted.
type Forgotten struct {
	Count int `json:"count"`
}

// ErrNoChat: no chat has that id.
var ErrNoChat = &extkit.Error{Code: "not_found", Status: 404, Message: "no chat with that id"}
