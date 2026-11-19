package docker

import (
	"fmt"
	"testing"

	remotev1 "pocketone.dev/ext/contracts/remote/v1"
)

func TestLogRing(t *testing.T) {
	var l logRing
	for i := range 5 {
		l.add(remotev1.LogEntry{Message: fmt.Sprint(i)})
	}
	got, next := l.last(2)
	if len(got) != 2 || got[1].(remotev1.LogEntry).Message != "4" || next != 5 {
		t.Fatalf("last: %v %d", got, next)
	}
	for i := 5; i < logRingSize+10; i++ {
		l.add(remotev1.LogEntry{Message: fmt.Sprint(i)})
	}
	got, next = l.since(next) // lines 5..1009, of which the ring kept the last 1000
	if len(got) != logRingSize || got[0].(remotev1.LogEntry).Message != "10" || next != logRingSize+10 {
		t.Fatalf("since: %d entries, first %v, next %d", len(got), got[0], next)
	}
	if got, _ := l.since(next); len(got) != 0 {
		t.Fatalf("nothing new: %v", got)
	}
}

func TestJWTID(t *testing.T) {
	// header.{"id":"abc"}.sig
	if id := jwtID("eyJhbGciOiJIUzI1NiJ9.eyJpZCI6ImFiYyJ9.x"); id != "abc" {
		t.Fatalf("id %q", id)
	}
	if id := jwtID("ph_pat_x"); id != "" {
		t.Fatalf("not a jwt: %q", id)
	}
}
