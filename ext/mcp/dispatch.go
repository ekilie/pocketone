package mcp

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	tokensv1 "github.com/pockethost/pocketone/ext/contracts/tokens/v1"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"regexp"
	"strings"
)

// handler is the app's own HTTP handler, built from the router captured on serve.
func (s *Server) handler() (http.Handler, error) {
	s.muxOnce.Do(func() {
		if s.router == nil {
			s.muxErr = errors.New("mcp: the app isn't serving")
			return
		}
		s.mux, s.muxErr = s.router.BuildMux()
	})
	return s.mux, s.muxErr
}

// PathValue escapes one path parameter. Empty, "." and ".." are refused: the router would clean them into another
// route.
func PathValue(v string) (string, error) {
	if v == "" || v == "." || v == ".." {
		return "", fmt.Errorf("invalid path value %q", v)
	}
	return url.PathEscape(v), nil
}

var pathParam = regexp.MustCompile(`\{([A-Za-z0-9_]+)\}`)

// Path fills a path template's `{name}` segments from args (escaped with [PathValue]) and reports which args it used.
func (c *Call) Path(template string) (string, map[string]bool, error) {
	used := map[string]bool{}
	var err error
	out := pathParam.ReplaceAllStringFunc(template, func(m string) string {
		name := m[1 : len(m)-1]
		used[name] = true
		v, e := PathValue(argStr(c.Args, name))
		if e != nil && err == nil {
			err = fmt.Errorf("%q: %w", name, e)
		}
		return v
	})
	return out, used, err
}

// defaultRequest maps the arguments onto the endpoint: path params by name, files as multipart, the rest as query
// (GET/HEAD/DELETE) or JSON body.
func (c *Call) defaultRequest() (*Request, error) {
	t := c.Tool
	path, used, err := c.Path(t.Path)
	if err != nil {
		return nil, err
	}
	req := &Request{Method: t.Method, Path: path}
	rest := map[string]any{}
	for k, v := range c.Args {
		if used[k] {
			continue
		}
		files, ok, err := asFiles(v, c.s.cfg.MaxFile)
		if err != nil {
			return nil, fmt.Errorf("%q: %w", k, err)
		}
		if ok {
			if req.Files == nil {
				req.Files = map[string][]File{}
			}
			req.Files[k] = files
			continue
		}
		rest[k] = v
	}
	switch t.Method {
	case http.MethodGet, http.MethodHead, http.MethodDelete:
		if len(req.Files) > 0 {
			return nil, errors.New("files can't be sent with " + t.Method)
		}
		req.Query = url.Values{}
		for k, v := range rest {
			req.Query.Set(k, queryValue(v))
		}
	default:
		if len(rest) > 0 || len(req.Files) == 0 {
			req.Body = rest
		}
	}
	return req, nil
}

func queryValue(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return ""
	case float64, bool, int, int64:
		return fmt.Sprint(x)
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// asFiles reports whether v is a file argument ({name, base64, type?}, or a list of them) and decodes it.
func asFiles(v any, max int) ([]File, bool, error) {
	one := func(m map[string]any) (File, bool, error) {
		b64, ok := m["base64"].(string)
		if !ok {
			return File{}, false, nil
		}
		name, _ := m["name"].(string)
		if name == "" {
			return File{}, true, errors.New("a file needs a name")
		}
		if base64.StdEncoding.DecodedLen(len(b64)) > max+3 {
			return File{}, true, fmt.Errorf("file %s is over the %d MB limit", name, max>>20)
		}
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return File{}, true, fmt.Errorf("file %s: bad base64: %w", name, err)
		}
		if len(data) > max {
			return File{}, true, fmt.Errorf("file %s is over the %d MB limit", name, max>>20)
		}
		typ, _ := m["type"].(string)
		return File{Name: name, Type: typ, Data: data}, true, nil
	}
	switch x := v.(type) {
	case map[string]any:
		f, ok, err := one(x)
		if !ok || err != nil {
			return nil, ok, err
		}
		return []File{f}, true, nil
	case []any:
		if len(x) == 0 {
			return nil, false, nil
		}
		out := []File{}
		for _, item := range x {
			m, isMap := item.(map[string]any)
			if !isMap {
				return nil, false, nil
			}
			f, ok, err := one(m)
			if !ok {
				return nil, false, nil
			}
			if err != nil {
				return nil, true, err
			}
			out = append(out, f)
		}
		return out, true, nil
	}
	return nil, false, nil
}

// credential is the Authorization value the inner request carries: the caller's own PocketBase token when that's
// what they sent, else a token for the same record made for this call (as ext/pages' ctx.pb() does).
func (c *Call) credential() (string, error) {
	if c.Grant == nil {
		return "", nil
	}
	if c.Grant.Credential != "" {
		return c.Grant.Credential, nil
	}
	if c.Grant.Auth == nil {
		return "", nil
	}
	tok, err := c.Grant.Auth.NewAuthToken()
	if err != nil {
		return "", err
	}
	c.Grant.Credential = "Bearer " + tok // once per MCP request
	return c.Grant.Credential, nil
}

// send runs req through the app's router as the caller and captures up to max bytes of the response.
func (s *Server) send(c *Call, req *Request, max int) (*Response, error) {
	h, err := s.handler()
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(req.Path, "/") || strings.Contains(req.Path, "?") || strings.Contains(req.Path, "#") {
		return nil, fmt.Errorf("invalid request path %q", req.Path)
	}
	if req.Path == DefaultPath || strings.HasPrefix(req.Path, DefaultPath+"/") {
		return nil, errors.New("a tool can't call the MCP endpoint")
	}
	method := req.Method
	if method == "" {
		method = http.MethodGet
	}

	var body io.Reader
	contentType := ""
	switch {
	case len(req.Files) > 0:
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		if req.Body != nil {
			j, err := json.Marshal(req.Body)
			if err != nil {
				return nil, err
			}
			if err := mw.WriteField("@jsonPayload", string(j)); err != nil {
				return nil, err
			}
		}
		for field, files := range req.Files {
			for _, f := range files {
				hdr := make(textproto.MIMEHeader)
				hdr.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, field, f.Name))
				typ := f.Type
				if typ == "" {
					typ = http.DetectContentType(f.Data)
				}
				hdr.Set("Content-Type", typ)
				part, err := mw.CreatePart(hdr)
				if err != nil {
					return nil, err
				}
				if _, err := part.Write(f.Data); err != nil {
					return nil, err
				}
			}
		}
		if err := mw.Close(); err != nil {
			return nil, err
		}
		body, contentType = &buf, mw.FormDataContentType()
	case req.Body != nil:
		j, err := json.Marshal(req.Body)
		if err != nil {
			return nil, err
		}
		body, contentType = bytes.NewReader(j), "application/json"
	}

	target := req.Path
	if len(req.Query) > 0 {
		target += "?" + req.Query.Encode()
	}
	orig := c.Event.Request
	ctx := orig.Context()
	if c.Grant != nil && len(c.Grant.Scopes) > 0 {
		// a scoped connection: ext/tokens checks this request like a scoped token's
		ctx = tokensv1.WithScopes(ctx, c.Grant.Scopes)
	}
	r := httptest.NewRequestWithContext(ctx, method, target, body)
	r.Host = orig.Host
	r.RemoteAddr = orig.RemoteAddr
	for _, k := range []string{"X-Forwarded-For", "X-Real-IP", "CF-Connecting-IP", "X-Forwarded-Proto", "User-Agent", "Accept-Language"} {
		if v := orig.Header.Get(k); v != "" {
			r.Header.Set(k, v)
		}
	}
	for k, vs := range req.Header {
		if strings.EqualFold(k, "Authorization") {
			continue // the caller's identity isn't the tool's to choose
		}
		for _, v := range vs {
			r.Header.Add(k, v)
		}
	}
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	cred, err := c.credential()
	if err != nil {
		return nil, err
	}
	if cred != "" {
		r.Header.Set("Authorization", cred)
	}
	r.Header.Set("X-MCP-Tool", c.Tool.Name)

	w := &capWriter{header: http.Header{}, max: max}
	h.ServeHTTP(w, r)
	status := w.status
	if status == 0 {
		status = http.StatusOK
	}
	return &Response{Status: status, Header: w.header, Body: w.buf.Bytes(), Truncated: w.cut}, nil
}

// capWriter keeps the first max bytes of a response and drops the rest.
type capWriter struct {
	header http.Header
	status int
	buf    bytes.Buffer
	max    int
	cut    bool
}

func (w *capWriter) Header() http.Header { return w.header }

func (w *capWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
}

func (w *capWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if room := w.max - w.buf.Len(); room > 0 {
		if len(p) > room {
			w.buf.Write(p[:room])
			w.cut = true
		} else {
			w.buf.Write(p)
		}
	} else if len(p) > 0 {
		w.cut = true
	}
	return len(p), nil
}

func (w *capWriter) Flush() {}

// run makes the tool's request and turns the response into an MCP result.
func (s *Server) run(c *Call) (result map[string]any) {
	defer func() {
		if r := recover(); r != nil {
			result = errorResult(fmt.Errorf("%v", r))
		}
	}()
	if err := checkArgs(c.Tool.InputSchema, c.Args); err != nil {
		return errorResult(err)
	}
	var req *Request
	var err error
	if c.Tool.Request != nil {
		req, err = c.Tool.Request(c)
	} else {
		req, err = c.defaultRequest()
	}
	if err != nil {
		return errorResult(err)
	}
	if req == nil {
		return errorResult(errors.New("the tool made no request"))
	}
	max := c.Tool.MaxResponse
	if max <= 0 {
		max = s.cfg.MaxResponse
	}
	res, err := s.send(c, req, max)
	if err != nil {
		return errorResult(err)
	}
	if c.Tool.Response != nil {
		v, err := c.Tool.Response(c, res)
		if err != nil {
			return errorResult(err)
		}
		return toolResult(v)
	}
	return defaultResult(res, max)
}

// defaultResult: 2xx JSON is the result (structuredContent when it's an object), other text as text; 3xx says where
// it points; 4xx/5xx are tool errors carrying PocketBase's {status, message, data} so the model can fix its call.
func defaultResult(res *Response, max int) map[string]any {
	if res.Status >= 300 && res.Status < 400 {
		return textResult(fmt.Sprintf("HTTP %d, Location: %s", res.Status, res.Header.Get("Location")), false)
	}
	if res.Status >= 400 {
		msg := strings.TrimSpace(string(res.Body))
		if msg == "" {
			msg = http.StatusText(res.Status)
		}
		return textResult(fmt.Sprintf("HTTP %d: %s", res.Status, msg), true)
	}
	if res.Truncated {
		return textResult(string(res.Body)+fmt.Sprintf("\n\n[cut at %d KB; ask for less, e.g. a smaller perPage or fewer fields]", max>>10), false)
	}
	if len(bytes.TrimSpace(res.Body)) == 0 {
		return textResult(fmt.Sprintf("HTTP %d (no content)", res.Status), false)
	}
	if strings.Contains(res.Header.Get("Content-Type"), "json") {
		var v any
		if json.Unmarshal(res.Body, &v) == nil {
			return toolResult(v)
		}
	}
	return textResult(string(res.Body), false)
}

func textResult(text string, isError bool) map[string]any {
	out := map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}
	if isError {
		out["isError"] = true
	}
	return out
}

func errorResult(err error) map[string]any { return textResult(err.Error(), true) }

func toolResult(v any) map[string]any {
	switch x := v.(type) {
	case Content:
		return map[string]any{"content": []map[string]any(x)}
	case string:
		return textResult(x, false)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return errorResult(fmt.Errorf("result is not JSON: %w", err))
	}
	var pretty bytes.Buffer
	if json.Indent(&pretty, raw, "", "  ") != nil {
		pretty.Write(raw)
	}
	out := map[string]any{"content": []map[string]any{{"type": "text", "text": pretty.String()}}}
	if len(raw) > 0 && raw[0] == '{' {
		out["structuredContent"] = json.RawMessage(raw)
	}
	return out
}
