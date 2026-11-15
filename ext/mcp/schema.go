package mcp

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
)

// checkArgs checks args against the parts of a JSON Schema object that catch most model mistakes: required
// properties, top-level types, enums, and additionalProperties: false. The endpoint still validates everything.
func checkArgs(schema map[string]any, args map[string]any) error {
	if schema == nil {
		return nil
	}
	props, _ := schema["properties"].(map[string]any)
	missing := []string{}
	for _, r := range toStrings(schema["required"]) {
		if v, ok := args[r]; !ok || v == nil {
			missing = append(missing, r)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required argument(s): %s", strings.Join(missing, ", "))
	}
	if schema["additionalProperties"] == false {
		extra := []string{}
		for k := range args {
			if _, ok := props[k]; !ok {
				extra = append(extra, k)
			}
		}
		if len(extra) > 0 {
			sort.Strings(extra)
			return fmt.Errorf("unknown argument(s): %s", strings.Join(extra, ", "))
		}
	}
	for k, v := range args {
		p, _ := props[k].(map[string]any)
		if p == nil || v == nil {
			continue
		}
		if types := toStrings(p["type"]); len(types) > 0 && !slices.ContainsFunc(types, func(t string) bool { return isType(v, t) }) {
			return fmt.Errorf("argument %q must be %s", k, strings.Join(types, " or "))
		}
		if enum, ok := p["enum"].([]any); ok && !slices.ContainsFunc(enum, func(e any) bool { return fmt.Sprint(e) == fmt.Sprint(v) }) {
			return fmt.Errorf("argument %q must be one of %v", k, enum)
		}
	}
	return nil
}

func isType(v any, t string) bool {
	switch t {
	case "string":
		_, ok := v.(string)
		return ok
	case "number":
		_, ok := v.(float64)
		return ok
	case "integer":
		f, ok := v.(float64)
		return ok && f == math.Trunc(f)
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "object":
		_, ok := v.(map[string]any)
		return ok
	case "array":
		_, ok := v.([]any)
		return ok
	case "null":
		return v == nil
	}
	return true
}

func toStrings(v any) []string {
	switch x := v.(type) {
	case string:
		return []string{x}
	case []string:
		return x
	case []any:
		out := make([]string, 0, len(x))
		for _, s := range x {
			out = append(out, fmt.Sprint(s))
		}
		return out
	}
	return nil
}

func argStr(args map[string]any, key string) string {
	switch v := args[key].(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		if v == math.Trunc(v) {
			return fmt.Sprintf("%.0f", v)
		}
		return fmt.Sprint(v)
	default:
		return fmt.Sprint(v)
	}
}
