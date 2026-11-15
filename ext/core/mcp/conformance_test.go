package mcp_test

import (
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/core/mcp"
	"github.com/pockethost/pocketone/ext/core/pluginkit"
	"github.com/pockethost/pocketone/ext/core/pluginkit/conformance"
)

// notes is a plugin that adds an MCP tool, the way a contributed plugin would.
type notes struct{ toolName string }

func (n *notes) Name() string { return "notes" }
func (n *notes) Register(app core.App, _ pluginkit.Config) error {
	if s := mcp.For(app); s != nil {
		return s.AddEndpoint(mcp.Endpoint{
			Name: n.toolName, Description: "Lists the app's notes.", Method: "GET", Path: "/api/notes",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		})
	}
	return nil
}

func withMCP(app core.App) error {
	_, err := mcp.Register(app, mcp.Config{})
	return err
}

// The conformance test sees tools a plugin adds to ext/core/mcp's server (and not the pb_* built-ins).
func TestConformanceChecksPluginTools(t *testing.T) {
	conformance.Run(t, func() pluginkit.Plugin { return &notes{toolName: "notes_list_notes"} }, conformance.Options{Setup: withMCP})
}

func TestConformanceReportsBadToolName(t *testing.T) {
	ft := &failRecorder{T: t}
	func() {
		defer func() { _ = recover() }()
		conformance.CheckMCPTools(ft, &notes{toolName: "listNotes"}, conformance.Options{Setup: withMCP})
	}()
	if len(ft.errs) != 1 {
		t.Fatalf("want one problem for listNotes, got %v", ft.errs)
	}
}

type failRecorder struct {
	*testing.T
	errs []string
}

func (r *failRecorder) Errorf(f string, a ...any) { r.errs = append(r.errs, f) }
func (r *failRecorder) Failed() bool              { return len(r.errs) > 0 }
