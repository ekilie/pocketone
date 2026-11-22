package llm

import (
	"context"
	"strings"
)

// A subject is whom a call is for, so per-user and per-guest limits know whose bucket it counts toward:
//
//   - "user:<collection>/<id>": a signed-in auth record
//   - "guest:<id>": an anonymous visitor, by a guest cookie the provider sets
//   - "guest:ip:<address>": an anonymous visitor without that cookie yet
//   - "": nobody in particular (crons, background work); counts only toward app-wide limits
//
// App code names it from the request it serves ($llm.for(e) in page code). It travels in the call's context, so
// contract inputs don't carry it and an extension calling another (llm-chat calling llm.text/v1) passes it on by
// passing its context.

type subjectKey struct{}

// WithSubject returns ctx carrying subject s.
func WithSubject(ctx context.Context, s string) context.Context {
	return context.WithValue(ctx, subjectKey{}, s)
}

// SubjectOf is the subject ctx carries, or "".
func SubjectOf(ctx context.Context) string {
	s, _ := ctx.Value(subjectKey{}).(string)
	return s
}

// UserSubject is the subject for an auth record.
func UserSubject(collection, id string) string { return "user:" + collection + "/" + id }

// IsGuest reports whether s is an anonymous visitor's subject (by cookie or by IP).
func IsGuest(s string) bool { return strings.HasPrefix(s, "guest:") }

// IsUser reports whether s is a signed-in auth record's subject.
func IsUser(s string) bool { return strings.HasPrefix(s, "user:") }
