package modetest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Trap is the dev mail trap (ext/mailtrap) of the process that runs an instance. On a host every dev instance shares
// it, so a test with several instances tells their mail apart by recipient.
type Trap struct {
	t    *testing.T
	base string
	addr string
}

// TrapMessage is one caught message.
type TrapMessage struct {
	ID      int      `json:"id"`
	RcptTo  []string `json:"rcptTo"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
	Text    string   `json:"text"`
}

// Trap is the mail trap that catches this instance's mail.
func (s *Site) Trap() *Trap {
	return &Trap{t: s.t, base: s.Proc.TrapURL, addr: strings.TrimPrefix(s.Proc.TrapURL, "http://")}
}

// List is every message in the inbox.
func (tr *Trap) List() []TrapMessage {
	tr.t.Helper()
	code, b := request(tr.addr, tr.base+"/api/messages", http.MethodGet, "", nil)
	if code != 200 {
		tr.t.Fatalf("mail trap inbox: %d %s", code, b)
	}
	var out []TrapMessage
	if err := json.Unmarshal(b, &out); err != nil {
		tr.t.Fatalf("mail trap inbox: %v %s", err, b)
	}
	return out
}

// Wait waits for exactly n messages (15s) and returns them.
func (tr *Trap) Wait(n int) []TrapMessage {
	tr.t.Helper()
	var msgs []TrapMessage
	WaitFor(tr.t, fmt.Sprintf("%d messages in the mail trap", n), 15*time.Second, func() (bool, string) {
		msgs = tr.List()
		return len(msgs) >= n, fmt.Sprintf("%d messages", len(msgs))
	})
	if len(msgs) != n {
		tr.t.Fatalf("mail trap: %d messages, want %d: %+v", len(msgs), n, msgs)
	}
	return msgs
}

// Clear empties the inbox.
func (tr *Trap) Clear() {
	tr.t.Helper()
	if code, b := request(tr.addr, tr.base+"/api/messages", http.MethodDelete, "", nil); code != 204 {
		tr.t.Fatalf("clear mail trap: %d %s", code, b)
	}
}

// Message is one message with its bodies.
func (tr *Trap) Message(id int) TrapMessage {
	tr.t.Helper()
	code, b := request(tr.addr, fmt.Sprintf("%s/api/messages/%d", tr.base, id), http.MethodGet, "", nil)
	var m TrapMessage
	if code != 200 || json.Unmarshal(b, &m) != nil {
		tr.t.Fatalf("trap message %d: %d %s", id, code, b)
	}
	return m
}

// Raw is one message as sent (headers and body).
func (tr *Trap) Raw(id int) string {
	tr.t.Helper()
	code, b := request(tr.addr, fmt.Sprintf("%s/api/messages/%d/raw", tr.base, id), http.MethodGet, "", nil)
	if code != 200 {
		tr.t.Fatalf("trap raw %d: %d %s", id, code, b)
	}
	return string(b)
}

// To is the id of the message sent to this address; it fails when there is none.
func (tr *Trap) To(msgs []TrapMessage, to string) int {
	tr.t.Helper()
	for _, m := range msgs {
		for _, r := range m.RcptTo {
			if r == to {
				return m.ID
			}
		}
	}
	tr.t.Fatalf("no message to %s in %+v", to, msgs)
	return 0
}
