package pages

import (
	_ "embed"
	"fmt"
	"path"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
	"pocketone.dev/ext/extkit/js"
)

// JSX/TSX pages and TypeScript modules (SPEC 4.5). esbuild, linked into the binary, strips types and turns JSX into
// calls to __h (classic transform); the runtime in jsxruntime.js renders those calls straight to HTML strings. There
// is no virtual DOM and nothing runs in the browser: a component is a function from props to markup. The driver
// renders .jsx/.tsx templates (renderJSX in driver.js).

//go:embed jsxruntime.js
var jsxRuntimeSource string

// tsExts are module extensions that go through esbuild.
var tsExts = map[string]api.Loader{".ts": api.LoaderTS, ".tsx": api.LoaderTSX, ".jsx": api.LoaderJSX}

// transpile turns TypeScript/JSX source into plain CommonJS for the JS engine. Other files are returned unchanged.
func transpile(file string, src []byte) (string, error) {
	loader, ok := tsExts[path.Ext(file)]
	if !ok {
		return string(src), nil
	}
	res := api.Transform(string(src), api.TransformOptions{
		Loader:      loader,
		Format:      api.FormatCommonJS,
		Target:      api.ES2017,
		JSX:         api.JSXTransform,
		JSXFactory:  "__h",
		JSXFragment: "__Fragment",
		Sourcefile:  file,
		Sourcemap:   api.SourceMapNone,
	})
	if len(res.Errors) > 0 {
		var b strings.Builder
		for i, m := range res.Errors {
			if i > 0 {
				b.WriteString("; ")
			}
			if m.Location != nil {
				fmt.Fprintf(&b, "%s:%d:%d: ", file, m.Location.Line, m.Location.Column)
			}
			b.WriteString(m.Text)
		}
		return "", fmt.Errorf("%s", b.String())
	}
	return string(res.Code), nil
}

// builtinProgram returns one of pages' own JS programs on this VM, evaluated once per VM: "jsx" (the JSX runtime, a
// function of the escaper) or "sdk" (the SDK module, a function of module and exports that returns the Client class).
func (r *request) builtinProgram(name string) (js.Value, error) {
	switch name {
	case "jsx":
		return r.program("p:jsx", "ext/pages/jsxruntime.js", jsxRuntimeSource)
	case "sdk":
		return r.program("p:sdk", "ext/pages/sdk/pocketbase.cjs.js", sdkWrapped)
	}
	return nil, fmt.Errorf("no program %q", name)
}
