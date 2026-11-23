package main

import (
	"strings"
	"testing"
)

const pbFixture = `/**
 * The running app.
 *
 * @group PocketBase
 */
declare var $app: PocketBase

interface PocketBase extends excludeHooks<core.App>{}

/** @group PocketBase */declare function onRecordCreate(handler: (e: core.RecordEvent) => void, ...tags: string[]): void

/**
 * Registers a cron job.
 */
declare function cronAdd(
  jobId: string,
  handler: () => void,
): void;

declare namespace $dbx {
  let _in: dbx._in
  export { _in as in }
  export let and: dbx.and
}

interface Collection extends core.Collection{
  type: "base"
} // merge
/** Collection model class. */
declare class Collection implements core.Collection {
  constructor(data?: Partial<Collection>)
}

declare namespace core {
 /**
  * Record is a row. See [App.FindRecordById].
  */
 interface Record extends _t001 {
  [key:string]: any;
  /**
   * GetString returns a value as a string.
   */
  getString(key: string): string
 }
 type _t001 = BaseModel
 interface Record {
  /**
   * Collection returns the record's collection.
   */
  collection(): (Collection)
 }
 interface BaseModel {
  id: string
 }
 interface RecordEvent {
  record?: Record
 }
 interface App {
  /**
   * OnRecordCreate is triggered before a record is created.
   */
  onRecordCreate(...tags: string[]): void
 }
}

declare namespace dbx {
 interface Expression {
 }
 interface and {
  /**
   * And joins expressions with AND.
   */
  (...exps: Expression[]): Expression
 }
 interface _in {
  /**
   * In checks membership.
   */
  (col: string, ...values: any[]): Expression
 }
}
`

func TestParsePBTypes(t *testing.T) {
	api := parsePBTypes(pbFixture)

	if len(api.Vars) != 1 || api.Vars[0].Name != "$app" || api.Vars[0].Type != "PocketBase" {
		t.Fatalf("vars = %+v", api.Vars)
	}
	if len(api.Hooks) != 1 || api.Hooks[0].Sig.Name != "onRecordCreate" {
		t.Fatalf("hooks = %+v", api.Hooks)
	}
	if len(api.Funcs) != 1 || api.Funcs[0].Sig.Name != "cronAdd" || len(api.Funcs[0].Sig.Params) != 2 {
		t.Fatalf("funcs = %+v", api.Funcs)
	}
	if len(api.Classes) != 1 || api.Classes[0].Implements != "core.Collection" || len(api.Classes[0].Ctors) != 1 {
		t.Fatalf("classes = %+v", api.Classes)
	}
	for _, tt := range api.Top {
		if tt.Name == "Collection" {
			t.Error("a class's merge interface listed as a type")
		}
	}

	// $dbx members resolve to the Go functions they are, renames applied
	got := map[string]apiEntry{}
	for _, e := range api.Members["$dbx"] {
		got[e.Path] = e
	}
	if e := got["$dbx.and"]; !e.Sig.Method || e.Doc.Summary != "And joins expressions with AND." || api.EntryNS["$dbx.and"] != "dbx" {
		t.Errorf("$dbx.and = %+v", e)
	}
	if e := got["$dbx.in"]; !e.Sig.Method || len(e.Sig.Params) != 2 {
		t.Errorf("$dbx.in = %+v", e)
	}

	// interface blocks of one type merge; embeds come through the alias
	rec := api.NS["core"]["Record"]
	if rec == nil || len(rec.Fields) != 2 || rec.Doc.Summary == "" || len(rec.Extends) != 1 {
		t.Fatalf("core.Record = %+v", rec)
	}
	if api.NS["dbx"]["and"].Call == nil {
		t.Error("dbx.and not read as a function")
	}

	if h := api.typeHTML("core", "Array<Record>|null"); !strings.Contains(h, `<a href="/docs/reference/javascript/pocketbase/core#corerecord">Record</a>`) ||
		strings.Contains(h, "|") || !strings.Contains(h, "&lt;") {
		t.Errorf("typeHTML = %s", h)
	}
	if d := api.docLinks("core", "See [App.FindRecordById] and [link](x) and `a[i]`."); !strings.Contains(d, "[`App.findRecordById`](/docs/reference/javascript/pocketbase/core#coreapp)") ||
		!strings.Contains(d, "[link](x)") || !strings.Contains(d, "`a[i]`") {
		t.Errorf("docLinks = %s", d)
	}

	page := pbGlobalsPage(api, nil)
	for _, want := range []string{"### $app", "### $dbx.and()", "### cronAdd()", "## Hooks", "`onRecordCreate(handler, ...tags)`",
		"OnRecordCreate is triggered before a record is created.", "### Collection"} {
		if !strings.Contains(page, want) {
			t.Errorf("globals page lacks %q", want)
		}
	}
	ns := pbNSPage(api, "core", nil)
	for _, want := range []string{"## core.Record", "Has everything", "corebasemodel", "`getString()`", "`collection()`"} {
		if !strings.Contains(ns, want) {
			t.Errorf("core page lacks %q", want)
		}
	}
}
