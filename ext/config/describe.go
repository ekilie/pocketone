package config

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"pocketone.dev/ext/extkit"
)

// SectionDoc describes one config section for docs (`pocketone config --json`, used by pocketone.dev).
type SectionDoc struct {
	Name      string   `json:"name"`
	Doc       string   `json:"doc"`
	Owner     string   `json:"owner"`     // import path of the owning package
	Extension string   `json:"extension"` // the ext/<extension> that owns it ("" for none)
	Keys      []KeyDoc `json:"keys"`
	Default   string   `json:"default"` // the section's defaults as YAML
}

// KeyDoc is one key of a section. Dotted for nested keys (dns.provider). Doc comes from an optional `doc:"…"` struct
// tag on the field.
type KeyDoc struct {
	Key     string `json:"key"`
	Type    string `json:"type"`
	Default string `json:"default"`
	Doc     string `json:"doc,omitempty"`
}

// Describe lists every registered section with its keys.
func Describe() []SectionDoc {
	var out []SectionDoc
	for _, s := range extkit.Sections() {
		b, _ := yaml.Marshal(map[string]any{s.Name: s.Default})
		d := SectionDoc{Name: s.Name, Doc: s.Doc, Owner: s.Owner, Extension: extensionOf(s.Owner), Default: string(b), Keys: []KeyDoc{}}
		describeKeys(reflect.ValueOf(s.Default), "", &d.Keys)
		out = append(out, d)
	}
	return out
}

// extensionOf turns ".../ext/mothership/mirror" into "mothership".
func extensionOf(pkg string) string {
	i := strings.Index(pkg, "/ext/")
	if i < 0 {
		return ""
	}
	return strings.SplitN(pkg[i+len("/ext/"):], "/", 2)[0]
}

var durationType = reflect.TypeOf(time.Duration(0))

func describeKeys(v reflect.Value, prefix string, out *[]KeyDoc) {
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name, opts, _ := strings.Cut(f.Tag.Get("yaml"), ",")
		if name == "-" {
			continue
		}
		fv := v.Field(i)
		if strings.Contains(opts, "inline") {
			describeKeys(fv, prefix, out)
			continue
		}
		if name == "" {
			name = strings.ToLower(f.Name)
		}
		key := prefix + name
		ft := f.Type
		for ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		if ft.Kind() == reflect.Struct && ft != durationType {
			var nested []KeyDoc
			describeKeys(reflect.New(ft).Elem(), key+".", &nested)
			if len(nested) > 0 {
				if !(fv.Kind() == reflect.Pointer && fv.IsNil()) {
					nested = nested[:0]
					describeKeys(fv, key+".", &nested)
				}
				*out = append(*out, nested...)
				continue
			}
		}
		*out = append(*out, KeyDoc{Key: key, Type: typeName(f.Type), Default: defaultOf(fv), Doc: f.Tag.Get("doc")})
	}
}

func typeName(t reflect.Type) string {
	if t == durationType {
		return "duration"
	}
	switch t.Kind() {
	case reflect.Pointer:
		return typeName(t.Elem())
	case reflect.Bool:
		return "bool"
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "int"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Slice, reflect.Array:
		return "list of " + typeName(t.Elem())
	case reflect.Map:
		return "map of " + typeName(t.Elem())
	case reflect.Struct:
		return "object"
	}
	return t.Kind().String()
}

func defaultOf(v reflect.Value) string {
	if v.Kind() == reflect.Pointer && v.IsNil() {
		return ""
	}
	if v.Type() == durationType {
		if d := v.Interface().(time.Duration); d != 0 {
			return d.String()
		}
		return ""
	}
	if v.IsZero() {
		return ""
	}
	b, err := yaml.Marshal(v.Interface())
	if err != nil {
		return fmt.Sprint(v.Interface())
	}
	return strings.TrimSpace(string(b))
}
