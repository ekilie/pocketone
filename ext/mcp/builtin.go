package mcp

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strings"
	"time"
)

// BuiltinGroups are the groups of pb_* tools, one tool per stock REST endpoint.
var BuiltinGroups = []string{"health", "records", "auth", "files", "collections", "settings", "logs", "crons", "sql", "backups"}

// superuserGroups map endpoints only superusers can use; their tools are hidden from other callers in tools/list.
var superuserGroups = []string{"collections", "settings", "logs", "crons", "sql", "backups"}

// builtinTools are the pb_* tools: each maps one stock PocketBase REST endpoint with the same public API extensions use
// ([Endpoint]), so they double as examples. The endpoints decide who may do what.
func builtinTools(groups []string, readOnly bool, maxFile int) []Endpoint {
	if len(groups) == 0 {
		groups = BuiltinGroups
	}
	out := []Endpoint{}
	for _, b := range builtins() {
		t := b.tool
		if t.Annotations == nil {
			t.Annotations = methodAnnotations(t.Method)
		}
		if !slices.Contains(groups, b.group) || (readOnly && !t.Annotations.ReadOnly()) {
			continue
		}
		t.SuperuserOnly = slices.Contains(superuserGroups, b.group)
		if t.Response != nil { // the file tools read up to the file cap
			t.MaxResponse = maxFile
		}
		out = append(out, t)
	}
	return out
}

type builtin struct {
	group string
	tool  Endpoint
}

// ---------------------------------------------------------------- schema helpers

type props map[string]any

func obj(p props, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": map[string]any(p), "additionalProperties": false}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func str(desc string) map[string]any  { return map[string]any{"type": "string", "description": desc} }
func num(desc string) map[string]any  { return map[string]any{"type": "integer", "description": desc} }
func flag(desc string) map[string]any { return map[string]any{"type": "boolean", "description": desc} }
func object(desc string) map[string]any {
	return map[string]any{"type": "object", "description": desc}
}

const fileHelp = ` File fields take {"name": "a.png", "base64": "…", "type": "image/png"} (or a list of them); ` +
	`"field+" appends to a multi-file field and "field-" with filenames removes files.`

var (
	pCollection = str("Collection name or id.")
	pID         = str("Record id.")
	pFilter     = str(`PocketBase filter, e.g. "status = 'active' && created > '2026-01-01'".`)
	pSort       = str(`Sort, e.g. "-created,title".`)
	pPage       = num("Page number (default 1).")
	pPerPage    = num("Items per page (default 20, max 500).")
	pExpand     = str(`Relations to expand, e.g. "author,comments_via_post".`)
	pFields     = str(`Fields to return, e.g. "id,title,expand.author.name".`)
	pConfirm    = str("Type the name again to confirm.")
)

// listQuery is the default mapping with perPage 20 unless given (models do better with small pages).
func listQuery(c *Call) (*Request, error) {
	req, err := c.defaultRequest()
	if err != nil {
		return nil, err
	}
	if req.Query.Get("perPage") == "" {
		req.Query.Set("perPage", "20")
	}
	return req, nil
}

// dataRequest sends the "data" argument as the body (multipart when it holds files) and expand/fields as the query.
func dataRequest(c *Call) (*Request, error) {
	p, _, err := c.Path(c.Tool.Path)
	if err != nil {
		return nil, err
	}
	req := &Request{Method: c.Tool.Method, Path: p, Query: url.Values{}}
	for _, k := range []string{"expand", "fields"} {
		if v := argStr(c.Args, k); v != "" {
			req.Query.Set(k, v)
		}
	}
	data, _ := c.Args["data"].(map[string]any)
	body := map[string]any{}
	for k, v := range data {
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
		body[k] = v
	}
	req.Body = body
	return req, nil
}

// confirmed wraps a mapping so it only runs when "confirm" repeats the named argument.
func confirmed(arg string, next func(c *Call) (*Request, error)) func(c *Call) (*Request, error) {
	return func(c *Call) (*Request, error) {
		want := argStr(c.Args, arg)
		if want == "" || argStr(c.Args, "confirm") != want {
			return nil, fmt.Errorf(`set "confirm" to %q to go ahead`, want)
		}
		args := map[string]any{}
		for k, v := range c.Args {
			if k != "confirm" {
				args[k] = v
			}
		}
		cc := *c
		cc.Args = args
		if next == nil {
			return cc.defaultRequest()
		}
		return next(&cc)
	}
}

// ---------------------------------------------------------------- files

// fileToken asks the API for a file token as the caller ("" for guests), for protected files and backup downloads.
func fileToken(c *Call) (string, error) {
	if c.Auth() == nil {
		return "", nil
	}
	res, err := c.Do(&Request{Method: http.MethodPost, Path: "/api/files/token"}, 0)
	if err != nil {
		return "", err
	}
	if res.Status != http.StatusOK {
		return "", fmt.Errorf("file token: HTTP %d: %s", res.Status, res.Body)
	}
	var v struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(res.Body, &v); err != nil {
		return "", err
	}
	return v.Token, nil
}

func withFileToken(c *Call) (*Request, error) {
	req, err := c.defaultRequest()
	if err != nil {
		return nil, err
	}
	tok, err := fileToken(c)
	if err != nil {
		return nil, err
	}
	if tok != "" {
		req.Query.Set("token", tok)
	}
	return req, nil
}

// storageClient follows the one redirect a download may get (ext/s3redirect sends files straight from storage).
var storageClient = &http.Client{
	Timeout:       60 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

// fileResult turns a download into MCP content: images as image, text as text, anything else as an embedded file.
func fileResult(name string) func(c *Call, res *Response) (any, error) {
	return func(c *Call, res *Response) (any, error) {
		max := c.s.cfg.MaxFile
		if res.Status >= 300 && res.Status < 400 {
			loc := res.Header.Get("Location")
			if u, err := url.Parse(loc); err != nil || (u.Scheme != "https" && u.Scheme != "http") {
				return nil, fmt.Errorf("download redirected to %q", loc)
			}
			r, err := storageClient.Get(loc)
			if err != nil {
				return nil, err
			}
			defer r.Body.Close()
			data, err := io.ReadAll(io.LimitReader(r.Body, int64(max)+1))
			if err != nil {
				return nil, err
			}
			res = &Response{Status: r.StatusCode, Header: r.Header, Body: data, Truncated: len(data) > max}
			if res.Truncated {
				res.Body = data[:max]
			}
		}
		if res.Status >= 400 {
			return nil, fmt.Errorf("HTTP %d: %s", res.Status, strings.TrimSpace(string(res.Body)))
		}
		filename := argStr(c.Args, name)
		typ := res.Header.Get("Content-Type")
		if mt, _, err := mime.ParseMediaType(typ); err == nil {
			typ = mt
		}
		if typ == "" || typ == "application/octet-stream" {
			if t := mime.TypeByExtension(path.Ext(filename)); t != "" {
				typ = t
			} else {
				typ = http.DetectContentType(res.Body)
			}
		}
		if res.Truncated {
			return fmt.Sprintf("%s (%s) is over the %d MB limit for tools; download it another way.", filename, typ, max>>20), nil
		}
		switch {
		case strings.HasPrefix(typ, "image/"):
			return Content{{"type": "image", "data": base64.StdEncoding.EncodeToString(res.Body), "mimeType": typ}}, nil
		case isText(typ):
			return Content{{"type": "resource", "resource": map[string]any{"uri": "file:///" + filename, "mimeType": typ, "text": string(res.Body)}}}, nil
		}
		return Content{{"type": "resource", "resource": map[string]any{"uri": "file:///" + filename, "mimeType": typ,
			"blob": base64.StdEncoding.EncodeToString(res.Body)}}}, nil
	}
}

func isText(typ string) bool {
	return strings.HasPrefix(typ, "text/") || slices.Contains([]string{"application/json", "application/xml",
		"application/javascript", "application/x-yaml", "application/yaml", "image/svg+xml"}, typ) ||
		strings.HasSuffix(typ, "+json") || strings.HasSuffix(typ, "+xml")
}

// ---------------------------------------------------------------- the tools

func builtins() []builtin {
	rec := "/api/collections/{collection}/records"
	auth := "/api/collections/{collection}"
	return []builtin{
		{"health", Endpoint{Name: "pb_check_health", Title: "Health", Method: "GET", Path: "/api/health",
			Description: "Check the app is up (GET /api/health); superusers also see whether backups can run."}},

		// records
		{"records", Endpoint{Name: "pb_list_records", Title: "List records", Method: "GET", Path: rec, Request: listQuery,
			Description: "List a collection's records (GET /api/collections/{collection}/records), filtered, sorted and paged. The collection's list rule decides what you see.",
			InputSchema: obj(props{"collection": pCollection, "filter": pFilter, "sort": pSort, "page": pPage, "perPage": pPerPage,
				"expand": pExpand, "fields": pFields, "skipTotal": flag("Skip counting the total (faster).")}, "collection")}},
		{"records", Endpoint{Name: "pb_get_record", Title: "Get record", Method: "GET", Path: rec + "/{id}",
			Description: "Get one record (GET /api/collections/{collection}/records/{id}).",
			InputSchema: obj(props{"collection": pCollection, "id": pID, "expand": pExpand, "fields": pFields}, "collection", "id")}},
		{"records", Endpoint{Name: "pb_create_record", Title: "Create record", Method: "POST", Path: rec, Request: dataRequest,
			Description: "Create a record (POST /api/collections/{collection}/records) from data." + fileHelp,
			InputSchema: obj(props{"collection": pCollection, "data": object("Field values."), "expand": pExpand, "fields": pFields}, "collection", "data")}},
		{"records", Endpoint{Name: "pb_update_record", Title: "Update record", Method: "PATCH", Path: rec + "/{id}", Request: dataRequest,
			Description: "Update a record (PATCH /api/collections/{collection}/records/{id}) with the fields in data; others stay." + fileHelp +
				` Numbers take "field+"/"field-" to add or subtract.`,
			InputSchema: obj(props{"collection": pCollection, "id": pID, "data": object("Fields to change."), "expand": pExpand, "fields": pFields}, "collection", "id", "data")}},
		{"records", Endpoint{Name: "pb_delete_record", Title: "Delete record", Method: "DELETE", Path: rec + "/{id}",
			Description: "Delete a record (DELETE /api/collections/{collection}/records/{id}).",
			InputSchema: obj(props{"collection": pCollection, "id": pID}, "collection", "id")}},
		{"records", Endpoint{Name: "pb_run_batch", Title: "Batch", Method: "POST", Path: "/api/batch",
			Description: `Run several record creates/updates/upserts/deletes in one transaction (POST /api/batch; off unless enabled in settings). Each request: {"method": "POST", "url": "/api/collections/posts/records", "body": {...}}.`,
			InputSchema: obj(props{"requests": map[string]any{"type": "array", "items": map[string]any{"type": "object"}}}, "requests")}},

		// auth (per auth collection)
		{"auth", Endpoint{Name: "pb_list_auth_methods", Title: "Auth methods", Method: "GET", Path: auth + "/auth-methods",
			Description: "The sign-in methods an auth collection has enabled (GET /api/collections/{collection}/auth-methods).",
			InputSchema: obj(props{"collection": pCollection}, "collection")}},
		{"auth", Endpoint{Name: "pb_request_otp", Title: "Request OTP", Method: "POST", Path: auth + "/request-otp",
			Description: "Email a one-time sign-in code (POST …/request-otp).",
			InputSchema: obj(props{"collection": pCollection, "email": str("Account email.")}, "collection", "email")}},
		{"auth", Endpoint{Name: "pb_request_verification", Title: "Request verification", Method: "POST", Path: auth + "/request-verification",
			Description: "Email a verification link (POST …/request-verification).",
			InputSchema: obj(props{"collection": pCollection, "email": str("Account email.")}, "collection", "email")}},
		{"auth", Endpoint{Name: "pb_confirm_verification", Title: "Confirm verification", Method: "POST", Path: auth + "/confirm-verification",
			Description: "Confirm an email with the token from the verification email (POST …/confirm-verification).",
			InputSchema: obj(props{"collection": pCollection, "token": str("Token from the email.")}, "collection", "token")}},
		{"auth", Endpoint{Name: "pb_request_password_reset", Title: "Request password reset", Method: "POST", Path: auth + "/request-password-reset",
			Description: "Email a password reset link (POST …/request-password-reset).",
			InputSchema: obj(props{"collection": pCollection, "email": str("Account email.")}, "collection", "email")}},
		{"auth", Endpoint{Name: "pb_confirm_password_reset", Title: "Confirm password reset", Method: "POST", Path: auth + "/confirm-password-reset",
			Description: "Set a new password with the token from the reset email (POST …/confirm-password-reset).",
			InputSchema: obj(props{"collection": pCollection, "token": str("Token from the email."), "password": str("New password."),
				"passwordConfirm": str("New password again.")}, "collection", "token", "password", "passwordConfirm")}},
		{"auth", Endpoint{Name: "pb_request_email_change", Title: "Request email change", Method: "POST", Path: auth + "/request-email-change",
			Description: "Email a confirmation link to a new address for your account (POST …/request-email-change).",
			InputSchema: obj(props{"collection": pCollection, "newEmail": str("New email.")}, "collection", "newEmail")}},
		{"auth", Endpoint{Name: "pb_confirm_email_change", Title: "Confirm email change", Method: "POST", Path: auth + "/confirm-email-change",
			Description: "Confirm an email change with the token from the email and the account password (POST …/confirm-email-change).",
			InputSchema: obj(props{"collection": pCollection, "token": str("Token from the email."), "password": str("Account password.")}, "collection", "token", "password")}},

		// files
		{"files", Endpoint{Name: "pb_download_file", Title: "Download file", Method: "GET", Path: "/api/files/{collection}/{recordId}/{filename}",
			Request: withFileToken, Response: fileResult("filename"),
			Description: "Read a file attached to a record (GET /api/files/…; protected files use a file token made as you). Images come back as images, text as text, other files as embedded data. Up to 10 MB. Get filenames from the record's file fields.",
			InputSchema: obj(props{"collection": pCollection, "recordId": pID, "filename": str("Stored filename (from the record)."),
				"thumb": str(`Image thumb size, e.g. "100x100" (must be listed on the field).`)}, "collection", "recordId", "filename")}},

		// collections (superusers)
		{"collections", Endpoint{Name: "pb_list_collections", Title: "List collections", Method: "GET", Path: "/api/collections", Request: listQuery,
			Description: "List collections with their fields and rules (GET /api/collections). Start here to learn the data model.",
			InputSchema: obj(props{"filter": pFilter, "sort": pSort, "page": pPage, "perPage": pPerPage, "fields": pFields})}},
		{"collections", Endpoint{Name: "pb_get_collection", Title: "Get collection", Method: "GET", Path: "/api/collections/{collection}",
			Description: "One collection's fields, rules and options (GET /api/collections/{collection}).",
			InputSchema: obj(props{"collection": pCollection}, "collection")}},
		{"collections", Endpoint{Name: "pb_create_collection", Title: "Create collection", Method: "POST", Path: "/api/collections", Request: dataRequest,
			Description: `Create a collection (POST /api/collections). data is the collection JSON, e.g. {"name": "posts", "type": "base", "fields": [{"name": "title", "type": "text", "required": true}], "listRule": ""}. pb_get_collection_scaffolds shows the defaults.`,
			InputSchema: obj(props{"data": object("Collection JSON.")}, "data")}},
		{"collections", Endpoint{Name: "pb_update_collection", Title: "Update collection", Method: "PATCH", Path: "/api/collections/{collection}", Request: dataRequest,
			Description: "Change a collection (PATCH /api/collections/{collection}): name, rules, options. fields replaces the whole field list, so send every field you keep (with its id).",
			InputSchema: obj(props{"collection": pCollection, "data": object("Properties to change.")}, "collection", "data")}},
		{"collections", Endpoint{Name: "pb_delete_collection", Title: "Delete collection", Method: "DELETE", Path: "/api/collections/{collection}",
			Request:     confirmed("collection", nil),
			Description: "Delete a collection and all its records (DELETE /api/collections/{collection}). Set confirm to the collection name.",
			InputSchema: obj(props{"collection": pCollection, "confirm": pConfirm}, "collection", "confirm")}},
		{"collections", Endpoint{Name: "pb_truncate_collection", Title: "Truncate collection", Method: "DELETE", Path: "/api/collections/{collection}/truncate",
			Request:     confirmed("collection", nil),
			Description: "Delete every record of a collection (DELETE /api/collections/{collection}/truncate). Set confirm to the collection name.",
			InputSchema: obj(props{"collection": pCollection, "confirm": pConfirm}, "collection", "confirm")}},
		{"collections", Endpoint{Name: "pb_import_collections", Title: "Import collections", Method: "PUT", Path: "/api/collections/import",
			Description: "Create or update many collections at once (PUT /api/collections/import). deleteMissing also deletes collections not in the list.",
			InputSchema: obj(props{"collections": map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
				"deleteMissing": flag("Delete collections (and their records) that aren't in the list.")}, "collections")}},
		{"collections", Endpoint{Name: "pb_get_collection_scaffolds", Title: "Collection scaffolds", Method: "GET", Path: "/api/collections/meta/scaffolds",
			Description: "Default JSON for a new base, auth and view collection (GET /api/collections/meta/scaffolds)."}},
		{"collections", Endpoint{Name: "pb_list_oauth2_providers", Title: "OAuth2 providers", Method: "GET", Path: "/api/collections/meta/oauth2-providers",
			Description: "The OAuth2 providers PocketBase supports, with their names and logos (GET /api/collections/meta/oauth2-providers)."}},
		{"collections", Endpoint{Name: "pb_dry_run_view", Title: "Dry-run view", Method: "POST", Path: "/api/collections/meta/dry-run-view",
			Annotations: Read,
			Description: "Check a view collection's SELECT and see the fields it would have (POST /api/collections/meta/dry-run-view).",
			InputSchema: obj(props{"query": str("SELECT statement.")}, "query")}},

		// settings
		{"settings", Endpoint{Name: "pb_get_settings", Title: "Get settings", Method: "GET", Path: "/api/settings",
			Description: "App settings (GET /api/settings); secrets come back masked.",
			InputSchema: obj(props{"fields": pFields})}},
		{"settings", Endpoint{Name: "pb_update_settings", Title: "Update settings", Method: "PATCH", Path: "/api/settings", Request: dataRequest,
			Description: `Change settings (PATCH /api/settings) with the sections in data, e.g. {"meta": {"appName": "Acme"}}.`,
			InputSchema: obj(props{"data": object("Settings to change.")}, "data")}},
		{"settings", Endpoint{Name: "pb_test_s3", Title: "Test S3", Method: "POST", Path: "/api/settings/test/s3", Annotations: Read,
			Description: "Check the S3 connection (POST /api/settings/test/s3).",
			InputSchema: obj(props{"filesystem": map[string]any{"type": "string", "enum": []any{"storage", "backups"}}}, "filesystem")}},
		{"settings", Endpoint{Name: "pb_test_email", Title: "Test email", Method: "POST", Path: "/api/settings/test/email",
			Description: "Send a test email (POST /api/settings/test/email).",
			InputSchema: obj(props{"email": str("Recipient."), "template": map[string]any{"type": "string",
				"enum": []any{"verification", "password-reset", "email-change", "otp", "login-alert"}}, "collection": str("Auth collection (default _superusers).")}, "email", "template")}},
		{"settings", Endpoint{Name: "pb_generate_apple_client_secret", Title: "Apple client secret", Method: "POST", Path: "/api/settings/apple/generate-client-secret",
			Annotations: Read,
			Description: "Make a Sign in with Apple client secret (POST /api/settings/apple/generate-client-secret).",
			InputSchema: obj(props{"clientId": str(""), "teamId": str(""), "keyId": str(""), "privateKey": str("The .p8 key."),
				"duration": num("Seconds, up to 15777000.")}, "clientId", "teamId", "keyId", "privateKey", "duration")}},

		// logs
		{"logs", Endpoint{Name: "pb_list_logs", Title: "List logs", Method: "GET", Path: "/api/logs", Request: listQuery,
			Description: `Request and app logs (GET /api/logs), newest first with sort "-created". Filter e.g. "level >= 8" for errors, "data.status >= 400".`,
			InputSchema: obj(props{"filter": pFilter, "sort": pSort, "page": pPage, "perPage": pPerPage})}},
		{"logs", Endpoint{Name: "pb_get_log", Title: "Get log", Method: "GET", Path: "/api/logs/{id}",
			Description: "One log entry (GET /api/logs/{id}).", InputSchema: obj(props{"id": str("Log id.")}, "id")}},
		{"logs", Endpoint{Name: "pb_get_log_stats", Title: "Log stats", Method: "GET", Path: "/api/logs/stats",
			Description: "Log counts per hour (GET /api/logs/stats).", InputSchema: obj(props{"filter": pFilter})}},
		{"logs", Endpoint{Name: "pb_truncate_logs", Title: "Truncate logs", Method: "DELETE", Path: "/api/logs",
			Description: "Delete all logs (DELETE /api/logs)."}},

		// crons
		{"crons", Endpoint{Name: "pb_list_crons", Title: "List crons", Method: "GET", Path: "/api/crons",
			Description: "Registered cron jobs and their schedules (GET /api/crons)."}},
		{"crons", Endpoint{Name: "pb_run_cron", Title: "Run cron", Method: "POST", Path: "/api/crons/{id}",
			Description: "Run a cron job now (POST /api/crons/{id}).", InputSchema: obj(props{"id": str("Cron id.")}, "id")}},

		// sql
		{"sql", Endpoint{Name: "pb_run_sql", Title: "Run SQL", Method: "POST", Path: "/api/sql", Annotations: Destroy,
			Description: "Run a SQL statement on the app's database (POST /api/sql). Writes bypass API rules and hooks; prefer the record tools for data changes.",
			InputSchema: obj(props{"query": str("SQL statement.")}, "query")}},

		// backups
		{"backups", Endpoint{Name: "pb_list_backups", Title: "List backups", Method: "GET", Path: "/api/backups",
			Description: "Backups with size and date (GET /api/backups)."}},
		{"backups", Endpoint{Name: "pb_create_backup", Title: "Create backup", Method: "POST", Path: "/api/backups",
			Description: "Start a backup (POST /api/backups); name is optional (*.zip).", InputSchema: obj(props{"name": str("File name ending in .zip.")})}},
		{"backups", Endpoint{Name: "pb_upload_backup", Title: "Upload backup", Method: "POST", Path: "/api/backups/upload",
			Description: `Upload a backup zip (POST /api/backups/upload): file is {"name": "x.zip", "base64": "…"}, up to 10 MB.`,
			InputSchema: obj(props{"file": object("The zip.")}, "file")}},
		{"backups", Endpoint{Name: "pb_download_backup", Title: "Download backup", Method: "GET", Path: "/api/backups/{key}",
			Request: withFileToken, Response: fileResult("key"),
			Description: "Download a backup zip as embedded data (GET /api/backups/{key}), up to 10 MB.",
			InputSchema: obj(props{"key": str("Backup file name.")}, "key")}},
		{"backups", Endpoint{Name: "pb_delete_backup", Title: "Delete backup", Method: "DELETE", Path: "/api/backups/{key}",
			Request:     confirmed("key", nil),
			Description: "Delete a backup (DELETE /api/backups/{key}). Set confirm to the backup name.",
			InputSchema: obj(props{"key": str("Backup file name."), "confirm": pConfirm}, "key", "confirm")}},
		{"backups", Endpoint{Name: "pb_restore_backup", Title: "Restore backup", Method: "POST", Path: "/api/backups/{key}/restore",
			Annotations: Destroy, Request: confirmed("key", nil),
			Description: "Replace the app's data with a backup and restart (POST /api/backups/{key}/restore). Set confirm to the backup name. The app restarts, so this call may get no answer; check pb_check_health after.",
			InputSchema: obj(props{"key": str("Backup file name."), "confirm": pConfirm}, "key", "confirm")}},
	}
}
