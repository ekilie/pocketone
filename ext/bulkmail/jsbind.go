package bulkmail

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/extkit/js"
)

// BindJS exposes the Go service to an app's JS hooks as the $bulkmail global. Add it to the jsvm OnInit (next to
// the sandbox's, under pocker):
//
//	jsvm.MustRegister(app, jsvm.Config{OnInit: func(rt js.Runtime) { bulkmail.BindJS(app, rt) }})
//
//	$bulkmail.start(id, [limit]), .pause(id), .resume(id), .cancel(id) -> campaign record
//	$bulkmail.test(id, [to]), .preview(id, [email])           -> preview: {subject, html, text, to}
//	$bulkmail.count({collection, filter, email_field} | {sql}) -> {count, suppressed, sample}
//	$bulkmail.suppress(email, [reason]), .unsuppress(email), .isSuppressed(email)
//	$bulkmail.unsubscribe(email, [campaignId])                 -> suppresses and fires OnUnsubscribe
//	$bulkmail.unsubscribeURL(email, [campaignId])
//	$bulkmail.limits()                                         -> {rate, daily, sentToday}
//
// To react to bulk mail activity from JS, use the regular record hooks on its collections, for example
// onRecordAfterCreateSuccess(e => …, "_bulkmail_suppressions") or onRecordAfterUpdateSuccess(…, "_bulkmail_sends").
//
// The SQL audience (and SQL vars) runs read-only, but it can read any table of the app; a sandboxed multitenant
// host only exposes $bulkmail to tenants it trusts with their own data, which is every tenant: it's their app.
func BindJS(app core.App, rt js.Runtime) {
	get := func() (*BulkMail, error) {
		if bm := For(app); bm != nil {
			return bm, nil
		}
		return nil, errors.New("bulkmail isn't registered on this app")
	}
	action := func(fn func(*BulkMail, string) (*core.Record, error)) func(string) (*core.Record, error) {
		return func(id string) (*core.Record, error) {
			bm, err := get()
			if err != nil {
				return nil, err
			}
			return fn(bm, id)
		}
	}
	ns := map[string]any{
		"start": func(id string, limit int) (*core.Record, error) {
			bm, err := get()
			if err != nil {
				return nil, err
			}
			return bm.StartLimit(id, max(limit, 0))
		},
		"pause":  action((*BulkMail).Pause),
		"resume": action((*BulkMail).Resume),
		"cancel": action((*BulkMail).Cancel),
		"test": func(id, to string) error {
			bm, err := get()
			if err != nil {
				return err
			}
			return bm.Test(id, to)
		},
		"preview": func(id, email string) (map[string]any, error) {
			bm, err := get()
			if err != nil {
				return nil, err
			}
			msg, err := bm.Preview(id, email)
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"to": msg.To[0].Address, "from": msg.From.String(), "subject": msg.Subject,
				"html": msg.HTML, "text": msg.Text, "headers": msg.Headers,
			}, nil
		},
		"count": func(def any) (map[string]any, error) {
			bm, err := get()
			if err != nil {
				return nil, err
			}
			raw, err := json.Marshal(def)
			if err != nil {
				return nil, err
			}
			a, err := ParseAudience(raw)
			if err != nil {
				return nil, err
			}
			n, sup, sample, err := bm.CountAudience(a, 5)
			if err != nil {
				return nil, err
			}
			return map[string]any{"count": n, "suppressed": sup, "sample": sample}, nil
		},
		"suppress": func(email, reason string) (*core.Record, error) {
			bm, err := get()
			if err != nil {
				return nil, err
			}
			if reason == "" {
				reason = ReasonManual
			}
			return bm.Suppress(email, reason, "js", "")
		},
		"unsuppress": func(email string) error {
			bm, err := get()
			if err != nil {
				return err
			}
			return bm.Unsuppress(email)
		},
		"isSuppressed": func(email string) (bool, error) {
			bm, err := get()
			if err != nil {
				return false, err
			}
			return bm.IsSuppressed(email)
		},
		"unsubscribe": func(email, campaignID string) error {
			bm, err := get()
			if err != nil {
				return err
			}
			return bm.Unsubscribe(email, campaignID, "js")
		},
		"unsubscribeURL": func(email, campaignID string) (string, error) {
			bm, err := get()
			if err != nil {
				return "", err
			}
			return bm.UnsubscribeURL(email, campaignID), nil
		},
		"limits": func() (map[string]any, error) {
			bm, err := get()
			if err != nil {
				return nil, err
			}
			rate, daily, sent, err := bm.Limits()
			if err != nil {
				return nil, err
			}
			return map[string]any{"rate": rate, "daily": daily, "sentToday": sent}, nil
		},
	}
	if err := rt.Set("$bulkmail", ns); err != nil {
		panic(fmt.Errorf("bulkmail: bind JS: %w", err))
	}
}

// LegacyJSMigration returns a PocketBase JS migration that creates the bulk mail collections exactly as this
// extension defines them (same ids), skipping any that exist. The legacy PocketHost mothership (stock PocketBase,
// no Go) runs it so its data is already in this shape when the extension takes over.
func LegacyJSMigration() (string, error) {
	cols, err := json.MarshalIndent(Collections(), "", "  ")
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("/// <reference path=\"../pb_data/types.d.ts\" />\n")
	b.WriteString("// Generated by ext/bulkmail LegacyJSMigration(). Do not edit: change the extension's schema.go and regenerate.\n")
	b.WriteString("migrate((app) => {\n  const collections = ")
	b.WriteString(strings.ReplaceAll(string(cols), "\n", "\n  "))
	b.WriteString("\n  for (const c of collections) {\n")
	b.WriteString("    try {\n      app.findCollectionByNameOrId(c.id)\n      continue\n    } catch (_) {}\n")
	b.WriteString("    app.save(new Collection(c))\n  }\n}, (app) => {\n")
	b.WriteString("  // never drops: the ledger and suppressions are the record of who may be mailed\n})\n")
	return b.String(), nil
}
