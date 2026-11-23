package main

import (
	"strings"
	"testing"
)

// TestRESTCoverage: core's routes are read from apis/ and every one is on a REST page.
func TestRESTCoverage(t *testing.T) {
	routes, err := coreRoutes("../..")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(routes, "\n")
	for _, want := range []string{"GET /api/collections/{collection}/records", "POST /api/collections/{collection}/auth-with-password",
		"GET /api/health", "POST /api/batch", "POST /api/backups/{key}/restore", "GET /api/oauth2-redirect"} {
		if !strings.Contains(joined, want) {
			t.Errorf("route %q not read; have:\n%s", want, joined)
		}
	}
	if err := checkRESTCoverage("../.."); err != nil {
		t.Error(err)
	}
}
