package jsvm

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"
)

// mapExposure is one place JS can get a live Go map: an exported map field, or a method that returns a map.
type mapExposure struct {
	Owner  string // the type the field or method is on
	Member string // Field or Method()
	Map    string // the map type
	Locked bool   // the owner holds a sync.Mutex/RWMutex (its state is shared and lock-guarded)
}

// jsMapExposures walks every type JS can reach from roots through exported fields, methods, method parameters
// (event callbacks) and results, the way membersFor exposes them, and lists where a live map comes out.
func jsMapExposures(roots ...reflect.Type) []mapExposure {
	seen := map[reflect.Type]bool{}
	var out []mapExposure
	var walk func(t reflect.Type)
	add := func(owner reflect.Type, member string, mt reflect.Type) {
		st := owner
		for st.Kind() == reflect.Pointer {
			st = st.Elem()
		}
		out = append(out, mapExposure{
			Owner:  owner.String(),
			Member: member,
			Map:    mt.String(),
			Locked: st.Kind() == reflect.Struct && hasLock(st),
		})
	}
	methods := func(t reflect.Type) {
		for i := 0; i < t.NumMethod(); i++ {
			m := t.Method(i)
			if !m.IsExported() {
				continue
			}
			ft := m.Type
			first := 0
			if t.Kind() != reflect.Interface {
				first = 1 // skip the receiver
			}
			for j := first; j < ft.NumIn(); j++ {
				walk(ft.In(j))
			}
			for j := 0; j < ft.NumOut(); j++ {
				o := ft.Out(j)
				if o.Kind() == reflect.Map {
					add(t, m.Name+"()", o)
				}
				walk(o)
			}
		}
	}
	walk = func(t reflect.Type) {
		if t == nil || seen[t] {
			return
		}
		seen[t] = true
		switch t.Kind() {
		case reflect.Pointer:
			methods(t)
			walk(t.Elem())
		case reflect.Interface:
			methods(t)
		case reflect.Struct:
			methods(t)
			for _, f := range reflect.VisibleFields(t) {
				if !f.IsExported() || f.Anonymous {
					continue
				}
				if f.Type.Kind() == reflect.Map {
					add(t, f.Name, f.Type)
				}
				walk(f.Type)
			}
		case reflect.Slice, reflect.Array, reflect.Chan:
			walk(t.Elem())
		case reflect.Map:
			walk(t.Key())
			walk(t.Elem())
		case reflect.Func:
			for j := 0; j < t.NumIn(); j++ {
				walk(t.In(j))
			}
			for j := 0; j < t.NumOut(); j++ {
				walk(t.Out(j))
			}
		default:
			if t.Kind() != reflect.Struct {
				methods(t) // named scalars and maps with methods (e.g. types.JSONMap)
			}
		}
	}
	for _, r := range roots {
		walk(r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Owner != out[j].Owner {
			return out[i].Owner < out[j].Owner
		}
		return out[i].Member < out[j].Member
	})
	return out
}

// reviewedLockedExposures are the maps JS can reach on lock-guarded owners, each checked to be safe. A new entry
// fails TestAuditJSMapExposures until someone confirms JS gets a copy (or a map nobody writes concurrently).
var reviewedLockedExposures = map[string]string{
	"*core.RequestEvent.GetAll()":                "router.Event.GetAll clones the event store under its lock",
	"*core.Settings.DBExport()":                  "builds a fresh map",
	"*store.Store[string,interface {}].GetAll()": "clones under RLock",
	"core.RequestEvent.Extra":                    "per-request event, owned by the request goroutine while the hook runs",
	"http.Server.TLSNextProto":                   "set before the server starts, read-only after",
	"tls.Config.NameToCertificate":               "deprecated, unused by PocketBase, read-only after startup",
}

// TestAuditJSMapExposures walks every type JS can reach (from $app, the hook events and every Go global the binds
// set) and fails on a map exposed by a lock-guarded owner that nobody has reviewed: JS reading or writing a live map
// another goroutine writes is a fatal Go error, not a panic. JSVM_MAP_AUDIT=1 prints every exposure.
func TestAuditJSMapExposures(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	roots := []reflect.Type{reflect.TypeOf((*core.App)(nil)).Elem()}
	onSetGo = func(_ string, v any) {
		if m, ok := v.(map[string]any); ok { // a namespace: its members are what JS calls
			for _, mv := range m {
				roots = append(roots, reflect.TypeOf(mv))
			}
			return
		}
		roots = append(roots, reflect.TypeOf(v))
	}
	defer func() { onSetGo = nil }()
	if err := BindApp(testVM(t), app); err != nil {
		t.Fatal(err)
	}
	list := jsMapExposures(roots...)
	for _, e := range list {
		key := e.Owner + "." + e.Member
		if !e.Locked || strings.Contains(e.Owner, "tests.TestApp") { // the test app stands in for the real app
			continue
		}
		if _, ok := reviewedLockedExposures[key]; !ok {
			t.Errorf("JS can reach %s (%s) on a lock-guarded owner: confirm it returns a copy, then add it to reviewedLockedExposures", key, e.Map)
		}
	}
	if os.Getenv("JSVM_MAP_AUDIT") == "" {
		return
	}
	var b strings.Builder
	for _, e := range list {
		lock := ""
		if e.Locked {
			lock = "  [owner locked]"
		}
		fmt.Fprintf(&b, "%s.%s -> %s%s\n", e.Owner, e.Member, e.Map, lock)
	}
	t.Logf("%d map exposures:\n%s", len(list), b.String())
}

// The 2026-11-21 mothership crash: a hook called $app.store(), and charging the result walked the whole app store
// while other goroutines wrote to it. The store holds a mutex, so deepSize must stop at it.
func TestDeepSizeAppStoreConcurrentWrites(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	st := app.Store()
	type nested struct{ m map[string]*nested }
	for i := 0; i < 32; i++ {
		st.Set("k"+strconv.Itoa(i), &nested{m: map[string]*nested{"a": {}}})
	}
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			st.Set("w"+strconv.Itoa(i%64), &nested{})
			st.Remove("w" + strconv.Itoa((i+32)%64))
		}
	}()
	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		deepSize(reflect.ValueOf(st), 1<<40, PBShared)
	}
	close(stop)
	wg.Wait()
}
