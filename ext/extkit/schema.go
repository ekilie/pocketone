package extkit

import (
	"encoding/json"
	"reflect"
	"strings"
	"time"
)

var (
	timeType = reflect.TypeOf(time.Time{})
	rawType  = reflect.TypeOf(json.RawMessage(nil))
)

// SchemaOf is a JSON Schema for v's type, or for v itself when it is a reflect.Type (an operation's In): a struct
// with json tags, for an MCP tool's arguments; nil for nil. Fields without omitempty are required.
func SchemaOf(v any) map[string]any {
	if v == nil {
		return nil
	}
	if t, ok := v.(reflect.Type); ok {
		if t == nil {
			return nil
		}
		return schemaOfType(t, 0)
	}
	return schemaOfType(reflect.TypeOf(v), 0)
}

func schemaOfType(t reflect.Type, depth int) map[string]any {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch {
	case t == timeType:
		return map[string]any{"type": "string", "format": "date-time"}
	case t == rawType:
		return map[string]any{} // any JSON
	}
	switch t.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Slice, reflect.Array:
		return map[string]any{"type": "array", "items": schemaOfType(t.Elem(), depth+1)}
	case reflect.Map:
		return map[string]any{"type": "object"}
	case reflect.Struct:
		if depth > 5 {
			return map[string]any{"type": "object"}
		}
		props := map[string]any{}
		var required []string
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			name, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
			if name == "-" {
				continue
			}
			if name == "" {
				name = f.Name
			}
			props[name] = schemaOfType(f.Type, depth+1)
			if !strings.Contains(opts, "omitempty") {
				required = append(required, name)
			}
		}
		s := map[string]any{"type": "object", "properties": props}
		if len(required) > 0 {
			s["required"] = required
		}
		return s
	}
	return map[string]any{}
}
