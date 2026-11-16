package pages

// +config.js is loaded and checked by the driver (loadConfig in driver.js): debug, plugins (factories, module names
// and the built-ins auth and realtime, SPEC 9.1), and the 0.x plugin hooks v1 refuses (SPEC 9.2). Go applies the
// result in the rt object's configure.

// moduleExists reports whether a built-in's name is also a module in pb_pages (a _private file or a node_modules
// package, SPEC 3.2), which then wins over the built-in.
func (r *request) moduleExists(file, name string) bool {
	_, ok := r.resolveBare(file, name)
	return ok
}
