package pages

import (
	"fmt"

	"github.com/dop251/goja"
)

// oldPluginHooks are 0.x plugin hooks v1 doesn't call, with what to use instead (SPEC 9.2). A plugin that still has
// one is an error rather than silently half-working.
var oldPluginHooks = map[string]string{
	"onExtendContextApi": "use extendContext(ctx)",
	"onRender":           "use transform({ html, ctx, file })",
	"onResponse":         "use transform, or ctx.res in onRequest",
	"handles":            "add a page or +api.js for the path instead",
}

// builtinPlugins are the plugins turned on by name in +config.js (SPEC 9.1).
var builtinPlugins = map[string]bool{"auth": true, "realtime": true}

// pagesConfig is +config.js.
type pagesConfig struct {
	Debug    bool
	Builtins map[string]bool
}

// loadConfig runs +config.js (if any), validates it and returns the config and the JS plugin objects.
func (r *request) loadConfig(file string) (pagesConfig, []goja.Value, error) {
	var cfg pagesConfig
	if file == "" {
		return cfg, nil, nil
	}
	v := r.load(file)
	obj, ok := v.(*goja.Object)
	if !ok || goja.IsUndefined(v) || goja.IsNull(v) {
		return cfg, nil, fmt.Errorf("%s: must export an object", file)
	}
	if _, isFn := goja.AssertFunction(v); isFn {
		// 0.x allowed module.exports = (api) => ({...}); a function has no keys, so it would be silently ignored
		return cfg, nil, fmt.Errorf("%s: must export an object (module.exports = { ... }), not a function", file)
	}
	var plugins []goja.Value
	seen := map[string]bool{}
	for _, key := range obj.Keys() {
		switch key {
		case "debug":
			cfg.Debug = obj.Get(key).ToBoolean()
		case "plugins":
			list, ok := obj.Get(key).Export().([]any)
			if !ok && !goja.IsUndefined(obj.Get(key)) {
				return cfg, nil, fmt.Errorf("%s: plugins must be an array", file)
			}
			arr := obj.Get(key).ToObject(r.vm)
			for i := range list {
				item := arr.Get(fmt.Sprint(i))
				// A string is a module name, required from +config.js (SPEC 7): a _private file, a node_modules
				// package or a path. Only when no module by that name exists in pb_pages is it a built-in.
				if name, ok := item.Export().(string); ok && (!builtinPlugins[name] || r.moduleExists(file, name)) {
					v, err := r.requirePlugin(file, name)
					if err != nil {
						return cfg, nil, err
					}
					item = v
				} else if ok {
					if seen[name] {
						return cfg, nil, fmt.Errorf("%s: plugin %q listed twice", file, name)
					}
					seen[name] = true
					if cfg.Builtins == nil {
						cfg.Builtins = map[string]bool{}
					}
					cfg.Builtins[name] = true
					continue
				}
				factory, options := item, r.vm.NewObject()
				if o, ok := item.(*goja.Object); ok {
					if f := o.Get("factory"); f != nil && !goja.IsUndefined(f) {
						factory = f
						for _, k := range o.Keys() {
							if k != "factory" {
								_ = options.Set(k, o.Get(k))
							}
						}
					}
				}
				fn, ok := goja.AssertFunction(factory)
				if !ok {
					return cfg, nil, fmt.Errorf("%s: plugins[%d] must be a built-in name, a module name or a plugin factory", file, i)
				}
				p, err := fn(goja.Undefined(), options)
				if err != nil {
					return cfg, nil, err
				}
				po, ok := p.(*goja.Object)
				if !ok {
					return cfg, nil, fmt.Errorf("%s: plugins[%d]: the factory must return an object", file, i)
				}
				name := po.Get("name")
				if name == nil || goja.IsUndefined(name) || name.String() == "" {
					return cfg, nil, fmt.Errorf("%s: plugins[%d]: a plugin needs a name", file, i)
				}
				for _, k := range po.Keys() {
					if hint, ok := oldPluginHooks[k]; ok {
						return cfg, nil, fmt.Errorf("%s: plugin %q: %s is a 0.x hook that v1 never calls; %s", file, name.String(), k, hint)
					}
				}
				if seen[name.String()] {
					return cfg, nil, fmt.Errorf("%s: plugin %q listed twice", file, name.String())
				}
				seen[name.String()] = true
				plugins = append(plugins, po)
			}
		default:
			return cfg, nil, fmt.Errorf("%s: unknown key %q", file, key)
		}
	}
	return cfg, plugins, nil
}

// requirePlugin loads a plugin listed by module name in +config.js. The module's export (or its `default`, for a
// module compiled from ESM) must be a plugin factory; loadConfig checks that.
func (r *request) requirePlugin(file, name string) (v goja.Value, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("%s: plugin %q: require failed (built-in plugins: auth, realtime): %w", file, name, panicErr(rec))
		}
	}()
	v = r.requireFrom(file, name)
	if o, ok := v.(*goja.Object); ok {
		if _, isFn := goja.AssertFunction(v); !isFn {
			if d := o.Get("default"); d != nil {
				if _, isFn := goja.AssertFunction(d); isFn {
					return d, nil
				}
			}
		}
	}
	return v, nil
}

// moduleExists reports whether a built-in's name is also a module in pb_pages (a _private file or a node_modules
// package, SPEC 3.2), which then wins over the built-in.
func (r *request) moduleExists(file, name string) bool {
	_, ok := r.resolveBare(file, name)
	return ok
}
