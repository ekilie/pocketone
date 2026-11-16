package jsvm

// Modules: CommonJS require() and ES import share one resolver.
//
//   - .mjs, .mts and .ts are ES modules; .cjs, .cts and .json are CommonJS. A plain .js is CommonJS (as in PocketBase),
//     except inside node_modules, where Node's rule applies ("type": "module" in the package.json).
//   - Hook files: *.pb.mjs, *.pb.mts and *.pb.ts run as ES modules (their top-level names are private to the file;
//     share through export/import), *.pb.js, *.pb.cjs and *.pb.cts run as scripts in the app's one global scope.
//     Migrations the same way.
//   - require() of an ES module works when the module has no top-level await (as in Node 22): it returns the module's
//     namespace.
//   - import of a CommonJS module gives its module.exports as the default export, plus its identifier keys as named
//     exports, and is the same instance require() returns.

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

type moduleState struct {
	root string                       // the hooks folder: where bare names start looking for node_modules
	load func(string) ([]byte, error) // reads a file (disk, or an extension's js.Moduler source)
	base map[string]string            // a hook or migration file's label (pb_hooks/x.pb.mjs) -> folder its imports resolve from
}

func (m *moduleState) read(p string) ([]byte, bool) {
	b, err := m.load(p)
	return b, err == nil
}

// resolve finds spec from dir the way Node does: a relative or absolute path (trying the extensions and index files),
// else a package in the nearest node_modules. esm picks the "import" condition of a package's "exports".
func (m *moduleState) resolve(dir, spec string, esm bool) (string, []byte, bool) {
	if strings.HasPrefix(spec, "file://") {
		spec = strings.TrimPrefix(spec, "file://")
	}
	if strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") || strings.HasPrefix(spec, "/") || filepath.IsAbs(spec) {
		p := spec
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		return m.tryFile(filepath.Clean(p), esm)
	}
	for d := dir; ; d = filepath.Dir(d) {
		if f, b, ok := m.tryFile(filepath.Join(d, "node_modules", spec), esm); ok {
			return f, b, true
		}
		if filepath.Dir(d) == d {
			return "", nil, false
		}
	}
}

func (m *moduleState) tryFile(p string, esm bool) (string, []byte, bool) {
	cands := []string{p, p + ".js", p + ".mjs", p + ".cjs", p + ".json", p + ".ts", p + ".mts", p + ".cts"}
	// TypeScript imports name the compiled file: "./util.js" means ./util.ts when there's no util.js
	if ts, ok := map[string]string{".js": ".ts", ".mjs": ".mts", ".cjs": ".cts"}[filepath.Ext(p)]; ok {
		cands = append(cands, strings.TrimSuffix(p, filepath.Ext(p))+ts)
	}
	for _, cand := range cands {
		if b, ok := m.read(cand); ok {
			return cand, b, true
		}
	}
	if pkg, ok := m.read(filepath.Join(p, "package.json")); ok {
		var meta struct {
			Exports any    `json:"exports"`
			Main    string `json:"main"`
		}
		if json.Unmarshal(pkg, &meta) == nil {
			entries := []string{}
			if e := pickExport(meta.Exports, esm); e != "" {
				entries = append(entries, e)
			}
			if meta.Main != "" {
				entries = append(entries, meta.Main)
			}
			for _, e := range entries {
				main := filepath.Join(p, e)
				for _, cand := range []string{main, main + ".js", main + ".mjs", main + ".cjs", main + ".ts", filepath.Join(main, "index.js")} {
					if b, ok := m.read(cand); ok {
						return cand, b, true
					}
				}
			}
		}
	}
	for _, idx := range []string{"index.js", "index.mjs", "index.cjs", "index.json", "index.ts", "index.mts", "index.cts"} {
		if b, ok := m.read(filepath.Join(p, idx)); ok {
			return filepath.Join(p, idx), b, true
		}
	}
	return "", nil, false
}

// pickExport is a package's main entry from its "exports": a string, {".": ...}, or conditions (import/require,
// node, default), nested.
func pickExport(v any, esm bool) string {
	switch x := v.(type) {
	case string:
		return x
	case map[string]any:
		if dot, ok := x["."]; ok {
			return pickExport(dot, esm)
		}
		conds := []string{"require", "node", "default"}
		if esm {
			conds = []string{"import", "node", "default"}
		}
		for _, c := range conds {
			if e := pickExport(x[c], esm); e != "" {
				return e
			}
		}
	}
	return ""
}

// isESM: .mjs, .mts and .ts (TypeScript is written as ES modules; .cts is the CommonJS exception), or a .js file in a
// node_modules package whose package.json says "type": "module".
func (m *moduleState) isESM(p string) bool {
	switch filepath.Ext(p) {
	case ".mjs", ".mts", ".ts":
		return true
	case ".js":
		if !strings.Contains(p, string(filepath.Separator)+"node_modules"+string(filepath.Separator)) {
			return false
		}
		for d := filepath.Dir(p); filepath.Base(d) != "node_modules" && filepath.Dir(d) != d; d = filepath.Dir(d) {
			if b, ok := m.read(filepath.Join(d, "package.json")); ok {
				var meta struct {
					Type string `json:"type"`
				}
				_ = json.Unmarshal(b, &meta)
				return meta.Type == "module"
			}
		}
	}
	return false
}

// dirOf is the folder imports from the module or script named name resolve from.
func (m *moduleState) dirOf(name string) string {
	if d, ok := m.base[name]; ok {
		return d
	}
	if filepath.IsAbs(name) {
		return filepath.Dir(name)
	}
	return m.root
}

// normalizeModule resolves an import of spec from the module or script base to the imported file's path.
func (vm *VM) normalizeModule(base, spec string) (string, error) {
	dir := vm.mods.dirOf(base)
	found, _, ok := vm.mods.resolve(dir, spec, true)
	if !ok {
		return "", fmt.Errorf("Cannot find module '%s' from '%s'", spec, dir)
	}
	return found, nil
}

// errThrown means a JS exception is pending.
var errThrown = errors.New("thrown")

var identRe2 = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

// reserved words can't be names in `export const { ... }`.
var reserved = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`break case catch class const continue debugger default delete do else enum export
		extends false finally for function if import in instanceof new null return super switch this throw true try
		typeof var void while with yield let static implements interface package private protected public await
		arguments eval`) {
		reserved[w] = true
	}
}

// loadModule compiles the module at path: an ES module as is, a CommonJS or JSON one as a small ES module that
// re-exports what require() returns.
func (vm *VM) loadModule(path string) (uintptr, error) {
	if vm.mods.isESM(path) {
		src, ok := vm.mods.read(path)
		if !ok {
			return 0, fmt.Errorf("Cannot read module '%s'", path)
		}
		code := string(src)
		if isTS(path) {
			var err error
			if code, err = vm.transpile(code, path, tsESM, 0); err != nil {
				return 0, err
			}
		}
		if m := C.avm_compile_module(vm.ctx, code, path, importMeta(path)); m != 0 {
			return m, nil
		}
		return 0, errThrown
	}

	q, _ := json.Marshal(path)
	var names []string
	if filepath.Ext(path) != ".json" {
		keys, err := vm.evalString(fmt.Sprintf(`JSON.stringify(Object.keys(Object(__requireAbs(%s))))`, q), path)
		if err != nil {
			return 0, err
		}
		var all []string
		_ = json.Unmarshal([]byte(keys), &all)
		for _, k := range all {
			if identRe2.MatchString(k) && !reserved[k] {
				names = append(names, k)
			}
		}
	}
	src := fmt.Sprintf("const __cjs = __requireAbs(%s);\nexport default __cjs;\n", q)
	if len(names) > 0 {
		src += "export const { " + strings.Join(names, ", ") + " } = __cjs;\n"
	}
	if m := C.avm_compile_module(vm.ctx, src, path, nil); m != 0 {
		return m, nil
	}
	return 0, errThrown
}

// evalString evaluates code as a script and returns its value as a string.
func (vm *VM) evalString(code, file string) (string, error) {
	cc, cf := C.CString(code), C.CString(file)
	defer C.free(cc)
	defer C.free(cf)
	r := C.avm_eval(vm.ctx, cc, uint64(len(code)), cf)
	if C.avm_is_exception(r) != 0 {
		return "", errThrown
	}
	defer vm.free_(r)
	return vm.goString(r), nil
}

// evalModuleHere runs code as the ES module label (pb_hooks/x.pb.mjs), whose imports resolve from dir, and waits
// for its top-level await (pumping the loop) before returning.
func (vm *VM) evalModuleHere(code, label, dir string) error {
	vm.mods.base[label] = dir
	r := C.avm_eval_module(vm.ctx, code, label, importMeta(filepath.Join(dir, filepath.Base(label))))
	if C.avm_is_exception(r) != 0 {
		return vm.exception()
	}
	return vm.awaitHere(r)
}

// awaitHere takes ownership of v and, when it's a promise, waits on the loop until it settles.
func (vm *VM) awaitHere(v JSValue) error {
	if C.avm_is_promise(vm.ctx, v) == 0 {
		vm.free_(v)
		vm.runJobs()
		return nil
	}
	s, ch := vm.settlePromise(v, nil)
	if ch != nil {
		s = vm.waitSettled(ch)
	}
	return s.err
}

// importMeta is import.meta for the module file at path: url, filename and dirname, as in Node.
func importMeta(path string) map[string]string {
	return map[string]string{"url": "file://" + filepath.ToSlash(path), "filename": path, "dirname": filepath.Dir(path)}
}

// isModuleFile reports a hook or migration file that runs as an ES module.
func isModuleFile(name string) bool {
	switch filepath.Ext(name) {
	case ".mjs", ".mts":
		return true
	case ".ts":
		return !strings.HasSuffix(name, ".d.ts")
	}
	return false
}
