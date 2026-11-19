package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	apitokensv1 "pocketone.dev/ext/contracts/api-tokens/v1"
	mcpv1 "pocketone.dev/ext/contracts/mcp/v1"
	"pocketone.dev/ext/extkit"
	"slices"
	"strings"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// JSON-RPC error codes.
const (
	codeParse          = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternal       = -32603
)

// maxBody fits a 10 MB file as base64 plus its JSON-RPC envelope.
const maxBody = 16 << 20

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func rawID(id any) json.RawMessage {
	switch v := id.(type) {
	case json.RawMessage:
		if len(v) > 0 {
			return v
		}
	case nil:
	default:
		if b, err := json.Marshal(v); err == nil {
			return b
		}
	}
	return json.RawMessage("null")
}

func rpcError(id any, code int, message string) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": rawID(id), "error": map[string]any{"code": code, "message": message}}
}

func rpcResult(id any, result any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": rawID(id), "result": result}
}

func (s *Server) originAllowed(e *core.RequestEvent) bool {
	origin := e.Request.Header.Get("Origin")
	if origin == "" {
		return true // not a browser
	}
	if slices.Contains(s.cfg.AllowedOrigins, strings.TrimRight(origin, "/")) {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if strings.EqualFold(u.Host, e.Request.Host) {
		return true
	}
	if pub, err := url.Parse(s.publicOrigin(e)); err == nil && strings.EqualFold(u.Host, pub.Host) {
		return true
	}
	return false
}

func (s *Server) handlePost(e *core.RequestEvent) error {
	if !s.originAllowed(e) {
		return e.JSON(http.StatusForbidden, rpcError(nil, codeInvalidRequest, "Origin not allowed."))
	}
	if v := e.Request.Header.Get("MCP-Protocol-Version"); v != "" && !slices.Contains(SupportedVersions, v) {
		return e.JSON(http.StatusBadRequest, rpcError(nil, codeInvalidRequest,
			"Unsupported MCP-Protocol-Version "+v+"; supported: "+strings.Join(SupportedVersions, ", ")))
	}

	grant, err := s.cfg.Auth.Authenticate(e, s.ResourceURL(e))
	if err != nil || grant == nil {
		return s.challenge(e, err)
	}

	body, err := io.ReadAll(http.MaxBytesReader(e.Response, e.Request.Body, maxBody))
	if err != nil {
		return e.JSON(http.StatusRequestEntityTooLarge, rpcError(nil, codeInvalidRequest, "Request too large."))
	}
	body = bytes.TrimSpace(body)
	if len(body) > 0 && body[0] == '[' {
		return e.JSON(http.StatusBadRequest, rpcError(nil, codeInvalidRequest, "JSON-RPC batches are not supported."))
	}
	var msg rpcRequest
	if err := json.Unmarshal(body, &msg); err != nil {
		return e.JSON(http.StatusBadRequest, rpcError(nil, codeParse, "Parse error: "+err.Error()))
	}
	if msg.JSONRPC != "2.0" {
		return e.JSON(http.StatusBadRequest, rpcError(msg.ID, codeInvalidRequest, "Invalid Request: expected a single JSON-RPC 2.0 message."))
	}
	// Notifications (no id) and the client's responses (no method) need no answer.
	if msg.Method == "" || len(msg.ID) == 0 || string(msg.ID) == "null" {
		return e.NoContent(http.StatusAccepted)
	}

	return s.dispatch(e, grant, &msg)
}

func (s *Server) dispatch(e *core.RequestEvent, grant *Grant, msg *rpcRequest) error {
	id := msg.ID
	switch msg.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(msg.Params, &p)
		version := SupportedVersions[0]
		if slices.Contains(SupportedVersions, p.ProtocolVersion) {
			version = p.ProtocolVersion
		}
		return e.JSON(http.StatusOK, rpcResult(id, map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      s.serverInfo(e),
			"instructions":    s.instructionsFor(grant),
		}))

	case "ping":
		return e.JSON(http.StatusOK, rpcResult(id, map[string]any{}))

	case "tools/list":
		list := []map[string]any{}
		super := grant.Auth != nil && grant.Auth.IsSuperuser()
		for _, t := range s.Tools() {
			if t.SuperuserOnly && !super {
				continue
			}
			if !s.reaches(grant, t) {
				continue
			}
			list = append(list, toolDef(t))
		}
		for _, t := range s.contractTools() {
			if t.SuperuserOnly && !super {
				continue
			}
			if !scopeAllows(grant, t.Scope) {
				continue
			}
			list = append(list, contractToolDef(t))
		}
		return e.JSON(http.StatusOK, rpcResult(id, map[string]any{"tools": list}))

	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return e.JSON(http.StatusOK, rpcError(id, codeInvalidParams, "Invalid params: "+err.Error()))
		}
		if p.Arguments == nil {
			p.Arguments = map[string]any{}
		}
		t := s.tool(p.Name)
		if t == nil {
			if ct, ok := s.contractTool(p.Name); ok {
				return e.JSON(http.StatusOK, rpcResult(id, s.runContractTool(e, grant, ct, p.Arguments)))
			}
			return e.JSON(http.StatusOK, rpcError(id, codeInvalidParams, "Unknown tool: "+p.Name))
		}
		c := &Call{App: s.app, Event: e, Grant: grant, Tool: t, Args: p.Arguments, s: s}
		return e.JSON(http.StatusOK, rpcResult(id, s.run(c)))

	default:
		return e.JSON(http.StatusOK, rpcError(id, codeMethodNotFound, "Method not found: "+msg.Method))
	}
}

func (s *Server) serverInfo(e *core.RequestEvent) map[string]any {
	meta := s.app.Settings().Meta
	title := meta.AppName
	if title == "" {
		title = "PocketBase"
	}
	name := s.cfg.Name
	if name == "" {
		name = slug(meta.AppName)
	}
	info := map[string]any{"name": name, "title": title, "version": s.cfg.Version, "websiteUrl": s.publicOrigin(e)}
	if s.cfg.Description != "" {
		info["description"] = s.cfg.Description
	}
	if len(s.cfg.Icons) > 0 {
		info["icons"] = s.cfg.Icons
	}
	return info
}

func slug(v string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(v) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
	}
	if out := strings.Trim(b.String(), "-"); out != "" {
		return out
	}
	return "pocketbase"
}

var noArgs = map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}

func toolDef(t *Endpoint) map[string]any {
	def := map[string]any{"name": t.Name, "description": t.Description}
	if t.InputSchema != nil {
		def["inputSchema"] = t.InputSchema
	} else {
		def["inputSchema"] = noArgs
	}
	if t.Title != "" {
		def["title"] = t.Title
	}
	if t.OutputSchema != nil {
		def["outputSchema"] = t.OutputSchema
	}
	if t.Annotations != nil {
		def["annotations"] = t.Annotations
	}
	return def
}

// contractToolDef is an mcp/v1 tool as tools/list describes it.
func contractToolDef(t mcpv1.Tool) map[string]any {
	def := map[string]any{"name": t.Name, "description": t.Description, "inputSchema": noArgs}
	if t.InputSchema != nil {
		def["inputSchema"] = t.InputSchema
	}
	if t.Title != "" {
		def["title"] = t.Title
	}
	switch {
	case t.ReadOnly:
		def["annotations"] = Read
	case t.Destructive:
		def["annotations"] = Destroy
	default:
		def["annotations"] = Write
	}
	return def
}

// scopeAllows: a connection limited to scopes sees a contract tool only when it holds the tool's scope.
func scopeAllows(g *Grant, scope string) bool {
	if g == nil || len(g.Scopes) == 0 || scope == "" || scope == extkit.ScopePublic || scope == extkit.ScopeAny {
		return true
	}
	return slices.Contains(g.Scopes, scope)
}

// grantAuth is who an MCP connection acts as, the way contracts see a caller.
func grantAuth(g *Grant) extkit.Auth {
	a := extkit.Auth{Via: "mcp"}
	if g == nil || g.Auth == nil {
		return a
	}
	a.Kind, a.ID = "record", g.Auth.Id
	a.Collection, a.RecordID, a.Email = g.Auth.Collection().Name, g.Auth.Id, g.Auth.Email()
	if g.Auth.IsSuperuser() {
		a.Kind = "superuser"
	}
	if len(g.Scopes) > 0 {
		a.Scopes = g.Scopes
	}
	return a
}

// runContractTool calls an mcp/v1 tool: a contract operation (scope check and hooks included) or an extension's tool.
func (s *Server) runContractTool(e *core.RequestEvent, g *Grant, t mcpv1.Tool, args map[string]any) (result map[string]any) {
	defer func() {
		if r := recover(); r != nil {
			result = errorResult(fmt.Errorf("%v", r))
		}
	}()
	if err := checkArgs(t.InputSchema, args); err != nil {
		return errorResult(err)
	}
	raw, err := json.Marshal(args)
	if err != nil {
		return errorResult(err)
	}
	v, err := t.Call(e.Request.Context(), grantAuth(g), raw)
	if err != nil {
		return errorResult(err)
	}
	return toolResult(v)
}

// reaches: the tool can work on this connection. A scoped connection doesn't list tools none of its scopes reaches
// (method + path, "{name}" segments matching anything); the check on each real request is what enforces.
func (s *Server) reaches(g *Grant, t *Endpoint) bool {
	if g == nil || len(g.Scopes) == 0 || t.Path == "" {
		return true
	}
	tk, ok := apitokensv1.Contract.Of(s.app)
	if !ok {
		return true
	}
	method := t.Method
	if method == "" {
		method = http.MethodGet
	}
	return tk.Reaches(g.Scopes, method, t.Path)
}

// instructionsFor adds, for a scoped connection, which scopes it's limited to.
func (s *Server) instructionsFor(g *Grant) string {
	text := s.instructionsText()
	if g == nil || len(g.Scopes) == 0 {
		return text
	}
	return text + "\n\nThis connection is limited to these scopes: " + strings.Join(g.Scopes, ", ") + ". Tools " +
		"outside them aren't listed, and requests they don't allow are refused; the user can reconnect with more access."
}
