package jsvm

import (
	"testing"
)

// Go constructors (setCtor) must behave like JS constructors for instanceof, as in goja:
// `x instanceof FormData` must not throw (the bundled PocketBase JS SDK does exactly that
// on every request with a body, which broke ctx.pb() writes in pages), and instances
// made with `new` must be instances of their constructor.
func TestCtorInstanceof(t *testing.T) {
	vm := testVM(t)
	BindCore(vm)
	BindHTTP(vm)

	cases := map[string]bool{
		// plain values are not instances (and asking must not throw)
		`({}) instanceof FormData`:     false,
		`"x" instanceof FormData`:      false,
		`null instanceof FormData`:     false,
		`({}) instanceof Record`:       false,
		`({}) instanceof DynamicModel`: false,
		// instances made with new are
		`new FormData() instanceof FormData`:                  true,
		`new DateTime() instanceof DateTime`:                  true,
		`new Collection() instanceof Collection`:              true,
		`new Collection({ name: "x" }) instanceof Collection`: true,
		`new TextField() instanceof TextField`:                true,
		`new MailerMessage({}) instanceof MailerMessage`:      true,
		// and not of another constructor
		`new FormData() instanceof Collection`: false,
		`new Collection() instanceof FormData`: false,
		// each constructor has a prototype that points back to it
		`typeof FormData.prototype === "object" && FormData.prototype.constructor === FormData`: true,
		`new FormData().constructor === FormData`:                                               true,
		// instances keep the Go object behavior from the shared prototype
		`typeof new FormData().toJSON === "function"`:                                  true,
		`JSON.stringify(new Collection({ name: "posts" })).includes('"name":"posts"')`: true,
		`String(new DateTime("2026-11-20 10:00:00.000Z")).startsWith("2026-11-20")`:    true,
		// Go slices returned by a constructor's methods still use the slice prototype
		`(() => { const f = new FormData(); f.append("a", "1"); f.append("a", "2"); return f.keys().map((k) => k + "!").join() === "a!" })()`: true,
	}
	for code, want := range cases {
		v, err := vm.RunString(code)
		if err != nil {
			t.Errorf("%s: %v", code, err)
			continue
		}
		if got, _ := v.Export().(bool); got != want {
			t.Errorf("%s = %v, want %v", code, v.Export(), want)
		}
	}
}

// The SDK's own check, verbatim in spirit: isFormData(body) on a plain object.
func TestCtorInstanceofSDKFormDataCheck(t *testing.T) {
	vm := testVM(t)
	BindHTTP(vm)
	v, err := vm.RunString(`
		const isFormData = (b) => b && (b.constructor?.name === "FormData" || (typeof FormData !== "undefined" && b instanceof FormData))
		isFormData({ title: "x" }) === false && isFormData(new FormData()) === true
	`)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := v.Export().(bool); !got {
		t.Fatalf("isFormData check = %v, want true", v.Export())
	}
}
