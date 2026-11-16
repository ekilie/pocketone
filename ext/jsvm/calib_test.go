//go:build cgo

package jsvm

import (
	"reflect"
	"runtime"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"
)

func heap() uint64 {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

func TestCalibrate(t *testing.T) {
	app, _ := tests.NewTestApp()
	defer app.Cleanup()
	col, _ := app.FindCollectionByNameOrId("demo2")
	const n = 20000
	before := heap()
	rs := make([]*core.Record, n)
	for i := range rs {
		rs[i] = core.NewRecord(col)
		rs[i].Set("title", "title number something")
	}
	real := int64(heap()-before) / n
	f, _ := sizers.Load(reflect.TypeOf(&core.Record{}))
	sz := f.(func(reflect.Value) int64)(reflect.ValueOf(rs[0]))
	deep := deepSize(reflect.ValueOf(rs), 1<<40, PBShared) / n
	t.Logf("new Record: real %d B/record, sizer %d, deep %d", real, sz, deep)

	before = heap()
	cols := make([][]*core.Collection, 50)
	for i := range cols {
		cols[i], _ = app.FindAllCollections()
	}
	realC := int64(heap()-before) / 50
	deepC := deepSize(reflect.ValueOf(cols[0]), 1<<40, PBShared)
	t.Logf("findAllCollections(): real %d B/call, deep %d", realC, deepC)
	runtime.KeepAlive(rs)
	runtime.KeepAlive(cols)
}
