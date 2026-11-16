package jsvm

import (
	"reflect"
)

// handleBase is what one handle costs on the Go side: the table slot plus the reflect.Value.
const handleBase = 160

// deepSize estimates the Go memory reachable from v: structs, strings, slices and maps, each pointer counted once.
// It stops at shared types (the app, DB pools: alive anyway) and once past limit.
func deepSize(v reflect.Value, limit int64, shared func(reflect.Type) bool) int64 {
	seen := map[uintptr]struct{}{}
	var total int64
	nodes := 0
	var walk func(v reflect.Value)
	walk = func(v reflect.Value) {
		if total > limit || nodes > 500000 || !v.IsValid() {
			return
		}
		nodes++
		switch v.Kind() {
		case reflect.Pointer:
			if v.IsNil() || (shared != nil && shared(v.Type())) {
				return
			}
			p := v.Pointer()
			if _, ok := seen[p]; ok {
				return
			}
			seen[p] = struct{}{}
			total += int64(v.Type().Elem().Size())
			walk(v.Elem())
		case reflect.Interface:
			if v.IsNil() {
				return
			}
			e := v.Elem()
			if shared != nil && shared(e.Type()) {
				return
			}
			if e.Kind() != reflect.Pointer {
				total += int64(e.Type().Size())
			}
			walk(e)
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				walk(v.Field(i))
			}
		case reflect.String:
			total += int64(v.Len())
		case reflect.Slice:
			if v.IsNil() {
				return
			}
			p := v.Pointer()
			if _, ok := seen[p]; ok {
				return
			}
			seen[p] = struct{}{}
			total += int64(v.Cap()) * int64(v.Type().Elem().Size())
			if hasIndirect(v.Type().Elem()) {
				for i := 0; i < v.Len(); i++ {
					walk(v.Index(i))
				}
			}
		case reflect.Array:
			if hasIndirect(v.Type().Elem()) {
				for i := 0; i < v.Len(); i++ {
					walk(v.Index(i))
				}
			}
		case reflect.Map:
			if v.IsNil() {
				return
			}
			p := v.Pointer()
			if _, ok := seen[p]; ok {
				return
			}
			seen[p] = struct{}{}
			kt, et := v.Type().Key(), v.Type().Elem()
			total += 48 + int64(v.Len())*(int64(kt.Size()+et.Size())+8)*2
			ki, ei := hasIndirect(kt), hasIndirect(et)
			if ki || ei {
				it := v.MapRange()
				for it.Next() {
					if ki {
						walk(it.Key())
					}
					if ei {
						walk(it.Value())
					}
				}
			}
		}
	}
	walk(v)
	return total
}

func hasIndirect(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.String, reflect.Slice, reflect.Map:
		return true
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			if hasIndirect(t.Field(i).Type) {
				return true
			}
		}
	case reflect.Array:
		return hasIndirect(t.Elem())
	}
	return false
}
