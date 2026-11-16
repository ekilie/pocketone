package jsvm

import (
	"strings"
	"testing"
	"time"
)

// QuickJS never checks the interrupt handler inside a built-in's own loop, so before the guard each of these ran
// for minutes on one CPU (new Array(1e9).join took ~80 s, the pocker sandbox test hung under -race) while neither
// TaskBudget nor the memory limit could stop it.
func TestArrayWalksAreBounded(t *testing.T) {
	walks := []string{
		`new Array(1e9).join("x")`,
		`new Array(1e9).join("")`,
		`new Array(1e9).toString()`,
		`new Array(1e9) + ""`,
		`new Array(1e9).toLocaleString()`,
		`new Array(1e9).indexOf(1)`,
		`new Array(1e9).lastIndexOf(1)`,
		`new Array(1e9).includes(1)`,
		`new Array(1e9).slice()`,
		`new Array(1e9).splice(0, 1)`,
		`new Array(1e9).reverse()`,
		`new Array(1e9).sort()`,
		`new Array(1e9).shift()`,
		`new Array(1e9).unshift(1)`,
		`new Array(1e9).copyWithin(0, 1)`,
		`new Array(1e9).forEach(Boolean)`,
		`new Array(1e9).map(Boolean)`,
		`new Array(1e9).filter(Boolean)`,
		`new Array(1e9).every(Boolean)`,
		`new Array(1e9).some(Boolean)`,
		`new Array(1e9).reduce(Math.max, 0)`,
		`new Array(1e9).reduceRight(Math.max, 0)`,
		`new Array(1e9).flat()`,
		`[[new Array(1e9)]].flat(2)`,
		`new Array(1e9).flatMap(Boolean)`,
		`[0].flatMap(() => new Array(1e9))`,
		`[].concat(new Array(1e9))`,
		`[].concat({ length: 1e9, [Symbol.isConcatSpreadable]: true })`,
		`Array.prototype.indexOf.call({ length: 2 ** 53 - 1 }, 1)`,
		`Array.prototype.join.call({ length: 1e9 }, "")`,
		`takeSlice(new Array(1e9))`,
	}
	vm := newVM(t, VMConfig{TaskBudget: time.Second, TaskWall: 3 * time.Second})
	if err := vm.Set("takeSlice", func(a []any) int { return len(a) }); err != nil {
		t.Fatal(err)
	}
	for _, code := range walks {
		start := time.Now()
		_, err := vm.RunString(code)
		took := time.Since(start)
		if err == nil || !strings.Contains(err.Error(), "over this VM's limit") {
			t.Errorf("%s: want the length limit, got %v after %s", code, err, took)
		}
		if took > 500*time.Millisecond {
			t.Errorf("%s: took %s", code, took)
		}
	}
	// the VM still works, and an array the limit allows still walks
	r, err := vm.RunString(`new Array(1000).join("x").length`)
	if err != nil || r.Export() != int64(999) {
		t.Fatalf("after the refusals: %v %v", r, err)
	}
}

// The wrappers behave like the built-ins for ordinary arrays.
func TestArrayGuardKeepsBehavior(t *testing.T) {
	vm := newVM(t, VMConfig{})
	if err := vm.Set("goSlice", []string{"a", "b", "c"}); err != nil {
		t.Fatal(err)
	}
	checks := map[string]any{
		`[1, 2, 3].join("-")`: "1-2-3",
		`String([1, [2, 3]])`: "1,2,3",
		`[3, 1, 2].sort().concat([4], 5, { length: 1, 0: 6, [Symbol.isConcatSpreadable]: true }).join()`: "1,2,3,4,5,6",
		`[1, [2, [3, [4]]]].flat(Infinity).join()`:                                                       "1,2,3,4",
		`[1, 2].flatMap(function (x) { return [x, this.k] }, { k: 0 }).join()`:                           "1,0,2,0",
		`[1, 2, 3].map(x => x * 2).filter(x => x > 2).reduce((a, b) => a + b, 0)`:                        int64(10),
		`Array.prototype.indexOf.call("abc", "b")`:                                                       int64(1),
		`goSlice.map(s => s.toUpperCase()).join("")`:                                                     "ABC",
		`goSlice.join("+")`: "a+b+c",
		`[].join.name + [].join.length + [].flatMap.name + [].flatMap.length + [].map.length`:               "join1flatMap11",
		`Object.keys(Array.prototype).length`:                                                               int64(0),
		`(() => { try { Array.prototype.join.call(null) } catch (e) { return e instanceof TypeError } })()`: true,
		`(() => { try { [1].flatMap(1) } catch (e) { return e instanceof TypeError } })()`:                  true,
		`const a = []; a[5] = 1; a.indexOf(1)`:                                                              int64(5),
	}
	for code, want := range checks {
		r, err := vm.RunString(code)
		if err != nil {
			t.Errorf("%s: %v", code, err)
			continue
		}
		got := r.Export()
		if f, ok := got.(float64); ok && f == float64(int64(f)) {
			got = int64(f)
		}
		if got != want {
			t.Errorf("%s: want %#v, got %#v", code, want, got)
		}
	}
}
