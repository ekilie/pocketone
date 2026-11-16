package jsvm

/*
#include "bridge.h"
*/
import "C"

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"
)

// What goja_nodejs gives plugins/jsvm, on this engine: CommonJS require() (relative paths, node_modules, .json,
// index.js, package.json "main"), process.env/argv/platform, and a small Buffer.

const nodeJS = `
(() => {
	const cache = Object.create(null)
	const makeRequire = (dir) => function require(spec) {
		const m = __resolveModule(dir, String(spec))
		if (!m) throw new Error("Cannot find module '" + spec + "' from '" + dir + "'")
		if (cache[m.path]) return cache[m.path].exports
		const mod = { id: m.path, filename: m.path, exports: {}, loaded: false }
		cache[m.path] = mod
		if (m.path.endsWith(".json")) {
			mod.exports = JSON.parse(m.source)
		} else {
			const fn = __compileModule(m.path, "(function (exports, require, module, __filename, __dirname) {" + m.source + "\n})")
			fn.call(mod.exports, mod.exports, makeRequire(m.dir), mod, m.path, m.dir)
		}
		mod.loaded = true
		return mod.exports
	}
	Object.defineProperty(globalThis, "require", { value: makeRequire(__hooks), writable: true, configurable: true })

	class Buffer extends Uint8Array {
		static from(v, enc) {
			if (typeof v === "string") return Buffer.__wrap(__bufDecode(v, enc || "utf8"))
			if (v instanceof ArrayBuffer) return new Buffer(v)
			return Buffer.__wrap(Array.from(v))
		}
		static alloc(n, fill) { const b = new Buffer(n); if (fill !== undefined) b.fill(typeof fill === "string" ? fill.charCodeAt(0) : fill); return b }
		static isBuffer(v) { return v instanceof Buffer }
		static byteLength(s, enc) { return __bufDecode(String(s), enc || "utf8").length }
		static concat(list) { const n = list.reduce((a, b) => a + b.length, 0); const out = new Buffer(n); let o = 0; for (const b of list) { out.set(b, o); o += b.length } return out }
		static __wrap(bytes) { const b = new Buffer(bytes.length); for (let i = 0; i < bytes.length; i++) b[i] = bytes[i]; return b }
		toString(enc) { return __bufEncode(Array.from(this), enc || "utf8") }
		toJSON() { return { type: "Buffer", data: Array.from(this) } }
	}
	Object.defineProperty(globalThis, "Buffer", { value: Buffer, writable: true, configurable: true })
	Object.defineProperty(globalThis, "process", { value: __process, writable: true, configurable: true })
})();
`

type resolvedModule struct {
	Path   string `json:"path"`
	Dir    string `json:"dir"`
	Source string `json:"source"`
}

// bindNode installs require, process and Buffer. load reads a module file by its path (disk by default, or an
// extension's js.Moduler source); root is __hooks.
func bindNode(vm *VM, root string, load func(string) ([]byte, error)) error {
	exists := func(p string) ([]byte, bool) {
		b, err := load(p)
		if err != nil {
			return nil, false
		}
		return b, true
	}
	tryFile := func(p string) (string, []byte, bool) {
		for _, cand := range []string{p, p + ".js", p + ".json", path.Join(p, "index.js"), path.Join(p, "index.json")} {
			if b, ok := exists(cand); ok && !strings.HasSuffix(cand, "/") {
				return cand, b, true
			}
		}
		if pkg, ok := exists(path.Join(p, "package.json")); ok {
			var meta struct {
				Main string `json:"main"`
			}
			if json.Unmarshal(pkg, &meta) == nil && meta.Main != "" {
				main := path.Join(p, meta.Main)
				for _, cand := range []string{main, main + ".js", path.Join(main, "index.js")} {
					if b, ok := exists(cand); ok {
						return cand, b, true
					}
				}
			}
		}
		return "", nil, false
	}
	join := func(dir, p string) string {
		if filepath.IsAbs(p) {
			return filepath.Clean(p)
		}
		return filepath.Join(dir, p)
	}

	vm.setHidden("__resolveModule", vm.nativeValue(func(c *call) (any, error) {
		dir, spec := c.str(0), c.str(1)
		var found string
		var src []byte
		var ok bool
		if strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") || strings.HasPrefix(spec, "/") || filepath.IsAbs(spec) {
			found, src, ok = tryFile(join(dir, spec))
		} else {
			for d := dir; ; d = filepath.Dir(d) {
				if found, src, ok = tryFile(filepath.Join(d, "node_modules", spec)); ok {
					break
				}
				if parent := filepath.Dir(d); parent == d {
					break
				}
			}
		}
		if !ok {
			return nil, nil
		}
		return &resolvedModule{Path: found, Dir: filepath.Dir(found), Source: string(src)}, nil
	}, false))

	vm.setHidden("__compileModule", vm.nativeValue(func(c *call) (any, error) {
		file, src := c.str(0), c.str(1)
		cs := C.CString(src)
		cf := C.CString(file)
		defer C.free(unsafe.Pointer(cs))
		defer C.free(unsafe.Pointer(cf))
		r := C.avm_eval(vm.ctx, cs, C.size_t(len(src)), cf)
		if C.avm_is_exception(r) != 0 {
			return nil, vm.exception()
		}
		return r, nil
	}, false))

	vm.setHidden("__bufDecode", vm.nativeValue(func(c *call) (any, error) {
		s, enc := c.str(0), strings.ToLower(c.str(1))
		var b []byte
		var err error
		switch enc {
		case "base64":
			b, err = base64.StdEncoding.DecodeString(s)
			if err != nil {
				b, err = base64.RawStdEncoding.DecodeString(s)
			}
		case "base64url":
			b, err = base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
		case "hex":
			b, err = hex.DecodeString(s)
		case "latin1", "binary":
			for _, r := range s {
				b = append(b, byte(r))
			}
		default:
			b = []byte(s)
		}
		if err != nil {
			return nil, err
		}
		out := make([]any, len(b))
		for i, x := range b {
			out[i] = int64(x)
		}
		return vm.jsArray(out)
	}, false))

	vm.setHidden("__bufEncode", vm.nativeValue(func(c *call) (any, error) {
		v, err := c.into(0, nil)
		if err != nil {
			return nil, err
		}
		items, _ := v.Interface().([]any)
		b := make([]byte, len(items))
		for i, x := range items {
			switch n := x.(type) {
			case int64:
				b[i] = byte(n)
			case float64:
				b[i] = byte(n)
			}
		}
		switch strings.ToLower(c.str(1)) {
		case "base64":
			return base64.StdEncoding.EncodeToString(b), nil
		case "base64url":
			return base64.RawURLEncoding.EncodeToString(b), nil
		case "hex":
			return hex.EncodeToString(b), nil
		case "latin1", "binary":
			r := make([]rune, len(b))
			for i, x := range b {
				r[i] = rune(x)
			}
			return string(r), nil
		}
		return string(b), nil
	}, false))

	env := map[string]any{}
	for _, kv := range os.Environ() {
		if k, v, ok := strings.Cut(kv, "="); ok {
			env[k] = v
		}
	}
	argv := make([]any, len(os.Args))
	for i, a := range os.Args {
		argv[i] = a
	}
	proc, err := vm.jsPlain(map[string]any{"env": env, "argv": argv, "platform": runtime.GOOS})
	if err != nil {
		return err
	}
	vm.setHidden("__process", proc)
	return vm.evalHere(nodeJS, "<node>")
}

// jsPlain converts plain Go data (maps, slices, primitives) into plain JS values through JSON.
func (vm *VM) jsPlain(v any) (C.JSValue, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return C.avm_undefined(), err
	}
	cs := C.CString(string(b))
	defer C.free(unsafe.Pointer(cs))
	r := C.avm_parse_json(vm.ctx, cs, C.size_t(len(b)))
	if C.avm_is_exception(r) != 0 {
		return C.avm_undefined(), vm.exception()
	}
	return r, nil
}

func (vm *VM) jsArray(items []any) (C.JSValue, error) { return vm.jsPlain(items) }

var _ = errors.New
var _ fs.FS
