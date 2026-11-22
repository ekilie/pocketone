package jsvm

import (
	"reflect"
	"sync"
)

// handleBase is what one handle costs on the Go side: the table slot plus the reflect.Value.
const handleBase = 160

// deepSize estimates the Go memory reachable from v: structs, strings, slices and maps, each pointer counted once.
// It stops at shared types (the app, DB pools: alive anyway) and once past limit.
//
// It runs on the VM goroutine without anyone else's locks, so it must not iterate a map another goroutine may be
// writing: Go aborts the whole process on "concurrent map iteration and map write" (mothership, 2026-11-21).
//   - A type with a registered sizer (or a Sizer) is charged by it; sizers read under the type's own locks.
//   - A struct that holds a sync.Mutex or sync.RWMutex owns lock-guarded state: charged its size, not walked.
//   - A map reached through a pointer may be shared: charged from its length, not iterated.
//
// Only maps the call itself returned (not behind a pointer) are walked entry by entry.
func deepSize(v reflect.Value, limit int64, shared func(reflect.Type) bool) int64 {
	seen := map[uintptr]struct{}{}
	var total int64
	nodes := 0
	var walk func(v reflect.Value, owned bool)
	walk = func(v reflect.Value, owned bool) {
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
			if n, ok := sizerSize(v); ok {
				total += n
				return
			}
			total += int64(v.Type().Elem().Size())
			walk(v.Elem(), false)
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
			walk(e, owned)
		case reflect.Struct:
			if hasLock(v.Type()) {
				return
			}
			for i := 0; i < v.NumField(); i++ {
				walk(v.Field(i), owned)
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
					walk(v.Index(i), owned)
				}
			}
		case reflect.Array:
			if hasIndirect(v.Type().Elem()) {
				for i := 0; i < v.Len(); i++ {
					walk(v.Index(i), owned)
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
			if !owned {
				return
			}
			ki, ei := hasIndirect(kt), hasIndirect(et)
			if ki || ei {
				it := v.MapRange()
				for it.Next() {
					if ki {
						walk(it.Key(), owned)
					}
					if ei {
						walk(it.Value(), owned)
					}
				}
			}
		}
	}
	walk(v, true)
	return total
}

// sizerSize charges a value through its registered sizer or Sizer method, if it has one.
// Values reached through unexported fields can't be handed to Go code (Interface panics): those are walked instead.
func sizerSize(v reflect.Value) (int64, bool) {
	if !v.CanInterface() {
		return 0, false
	}
	if f, ok := sizers.Load(v.Type()); ok {
		return f.(func(reflect.Value) int64)(v), true
	}
	if s, ok := v.Interface().(Sizer); ok {
		return s.JSSize(), true
	}
	return 0, false
}

var (
	mutexType   = reflect.TypeOf(sync.Mutex{})
	rwMutexType = reflect.TypeOf(sync.RWMutex{})
	lockTypes   sync.Map // reflect.Type -> bool
)

// hasLock reports whether a struct holds a sync.Mutex or sync.RWMutex field (embedded or not): its other fields are
// guarded by a lock deepSize doesn't take.
func hasLock(t reflect.Type) bool {
	if b, ok := lockTypes.Load(t); ok {
		return b.(bool)
	}
	b := false
	for i := 0; i < t.NumField(); i++ {
		if ft := t.Field(i).Type; ft == mutexType || ft == rwMutexType {
			b = true
			break
		}
	}
	lockTypes.Store(t, b)
	return b
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
