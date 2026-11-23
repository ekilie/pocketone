package main

import (
	"strings"
	"testing"
)

// TestFieldTypes reads core's field types: every type the dashboard offers, with what it stores and its options.
func TestFieldTypes(t *testing.T) {
	fts, err := fieldTypes("../..")
	if err != nil {
		t.Fatal(err)
	}
	byType := map[string]fieldType{}
	for _, f := range fts {
		byType[f.Type] = f
	}
	for _, typ := range []string{"text", "number", "bool", "email", "url", "editor", "date", "autodate", "select", "file", "relation", "json", "password"} {
		if _, ok := byType[typ]; !ok {
			t.Errorf("field type %q missing", typ)
		}
	}
	text := byType["text"]
	if text.Go != "TextField" || !strings.Contains(strings.Join(text.Options, ","), "max") || strings.Contains(strings.Join(text.Options, ","), "name") {
		t.Errorf("text = %+v", text)
	}
	if fts[0].Type != "text" {
		t.Errorf("first type = %s, want text", fts[0].Type)
	}
	tbl := fieldsTable(fts)
	if !strings.HasPrefix(tbl, fieldsStart) || !strings.HasSuffix(tbl, fieldsEnd) || !strings.Contains(tbl, "core/fields#coretextfield") {
		t.Errorf("table:\n%s", tbl)
	}
}
