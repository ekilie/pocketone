package jsvm

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"
	"unsafe"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/routine"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/template"
	"github.com/fatih/color"
	"github.com/fsnotify/fsnotify"
	"github.com/pockethost/pocketone/ext/extkit/js"
)

const typesFileName = "types.d.ts"

//go:generate go run ./internal/typegen
//go:embed types.d.ts
var typesFile []byte

// MustRegister is Register that panics on error.
func MustRegister(app core.App, config Config) {
	if err := Register(app, config); err != nil {
		panic(err)
	}
}

// Register starts the app's VM, loads its migrations and hooks, and sets the app's js.Runner.
func Register(app core.App, config Config) error {
	p := &plugin{app: app, config: config}
	if p.config.HooksDir == "" {
		p.config.HooksDir = filepath.Join(app.DataDir(), "../pb_hooks")
	}
	if p.config.MigrationsDir == "" {
		p.config.MigrationsDir = filepath.Join(app.DataDir(), "../pb_migrations")
	}
	if p.config.HooksFilesPattern == "" {
		p.config.HooksFilesPattern = `^.*\.pb\.(js|cjs|mjs|ts|cts|mts)$`
	}
	if p.config.MigrationsFilesPattern == "" {
		p.config.MigrationsFilesPattern = `^.*\.(js|cjs|mjs|ts|cts|mts)$`
	}
	embedded := p.extension(func(e any) bool { _, ok := e.(js.HookFiler); return ok }) != nil
	if embedded {
		p.config.HooksWatch = false // the files don't come from HooksDir
	}
	// types.d.ts and tsconfig.json go in the project root, the folder holding pb_hooks (not pb_hooks itself: its
	// watcher restarts the app on any change there). Embedded hooks have no sources on disk to type, so nothing is
	// written unless TypesDir says where.
	writeTypes := p.config.TypesDir != "" || !embedded
	if p.config.TypesDir == "" {
		p.config.TypesDir = filepath.Dir(filepath.Clean(p.config.HooksDir))
	}
	if p.config.VM.Blocking == nil {
		p.config.VM.Blocking = PBBlocking
	}
	if p.config.VM.Shared == nil {
		p.config.VM.Shared = PBShared
	}
	p.config.VM.PumpSyncCalls = true

	app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error {
		if err := e.Next(); err != nil {
			return err
		}
		if !writeTypes {
			return nil
		}
		if err := p.refreshTypesFile(); err != nil {
			color.Yellow("Unable to refresh app types file: %v", err)
		}
		if err := p.writeTSConfig(); err != nil {
			color.Yellow("Unable to write tsconfig.json: %v", err)
		}
		return nil
	})

	vm, err := New(p.config.VM, p.setup)
	if err != nil {
		return err
	}
	p.vm = vm
	app.OnTerminate().BindFunc(func(e *core.TerminateEvent) error {
		err := e.Next()
		vm.Close()
		return err
	})
	js.SetRunner(app, vm)

	if err := p.registerMigrations(); err != nil {
		return fmt.Errorf("registerMigrations: %w", err)
	}
	if err := p.registerHooks(); err != nil {
		return fmt.Errorf("registerHooks: %w", err)
	}
	return nil
}

type plugin struct {
	app      core.App
	config   Config
	vm       *VM
	hooksDir string // absolute; __hooks and require()'s root

	loadingMigration string // the migration file being evaluated, for migrate()
}

func (p *plugin) extension(match func(any) bool) any {
	for _, e := range p.config.Extensions {
		if match(e) {
			return e
		}
	}
	return nil
}

// setup runs on the loop: every bind, then the extensions, then OnInit.
func (p *plugin) setup(vm *VM) error {
	vm.Log = func(args ...any) { fmt.Fprintln(os.Stdout, args...) }

	root, err := filepath.Abs(p.config.HooksDir)
	if err != nil {
		return err
	}
	var load func(string) ([]byte, error)
	if m, ok := p.extension(func(e any) bool { _, ok := e.(js.Moduler); return ok }).(js.Moduler); ok {
		root, load = m.Modules()
	}
	p.hooksDir = root
	if load == nil {
		load = os.ReadFile
	}

	BindCore(vm)
	BindDbx(vm)
	BindSecurity(vm)
	BindOS(vm)
	BindFilepath(vm)
	BindHTTP(vm)
	BindFilesystem(vm)
	BindForms(vm)
	BindMails(vm)
	BindApis(vm)
	hooksBinds(p.app, vm)
	cronBinds(p.app, vm)
	routerBinds(p.app, vm)
	vm.setGo("$app", p.app)
	vm.setGo("$template", template.NewRegistry())
	vm.setGo("__hooks", root)
	vm.setFunc("migrate", p.migrate)
	if err := bindNode(vm, root, load); err != nil {
		return err
	}

	rt := jsRuntime{vm}
	for _, e := range p.config.Extensions {
		if h, ok := e.(js.HTTPClienter); ok && vm.HTTPClient == nil {
			vm.HTTPClient = h.HTTPClient(rt)
		}
		if h, ok := e.(js.Hider); ok && vm.Hider == nil {
			vm.Hider = h
		}
	}
	for _, e := range p.config.Extensions {
		if i, ok := e.(js.Initer); ok {
			i.OnInit(rt)
		}
		if b, ok := e.(js.Binder); ok {
			b.BindJS(p.app, rt)
		}
	}
	var callers []js.Caller
	for _, e := range p.config.Extensions {
		if c, ok := e.(js.Caller); ok {
			callers = append(callers, c)
		}
	}
	if len(callers) > 0 {
		vm.OnTask = func() (func(), error) {
			var dones []func()
			for _, c := range callers {
				done, err := c.OnCall(rt)
				if err != nil {
					for i := len(dones) - 1; i >= 0; i-- {
						dones[i]()
					}
					return nil, err
				}
				if done != nil {
					dones = append(dones, done)
				}
			}
			return func() {
				for i := len(dones) - 1; i >= 0; i-- {
					dones[i]()
				}
			}, nil
		}
	}
	if p.config.OnInit != nil {
		p.config.OnInit(rt)
	}
	return nil
}

// migrate(up, down) registers the migration file being loaded on the app's own list.
func (p *plugin) migrate(c *call) (any, error) {
	file := p.loadingMigration
	var up, down *jsRef
	if c.isFunc(0) {
		up = p.vm.ref(c.args[0])
	}
	if c.isFunc(1) {
		down = p.vm.ref(c.args[1])
	}
	run := func(fn *jsRef) func(txApp core.App) error {
		return func(txApp core.App) error {
			return routine.SafeWrap(func() error {
				if fn == nil {
					return nil
				}
				return p.vm.runHandler(fn, txApp, txApp)
			})()
		}
	}
	core.AppMigrationsOf(p.app).Register(run(up), run(down), file)
	return nil, nil
}

func (p *plugin) registerMigrations() error {
	files, err := p.files(func(e any) bool { _, ok := e.(js.MigrationFiler); return ok }, func(e any) (map[string][]byte, error) {
		return e.(js.MigrationFiler).MigrationFiles(p.config.MigrationsFilesPattern)
	}, p.config.MigrationsDir, p.config.MigrationsFilesPattern)
	if err != nil {
		return err
	}
	for _, name := range sortedKeys(files) {
		var runErr error
		err := p.vm.Do(func() {
			p.loadingMigration = name
			defer func() { p.loadingMigration = "" }()
			// each migration file gets its own scope, as with a VM per file in plugins/jsvm (an ES module has its own)
			label := "pb_migrations/" + name
			dir := filepath.Dir(filepath.Join(p.migrationsDir(), name))
			module := isModuleFile(name)
			code := string(files[name])
			if isTS(name) {
				format, offset := tsScript, 1
				if module {
					format, offset = tsESM, 0
				}
				if code, runErr = p.vm.transpile(code, label, format, offset); runErr != nil {
					return
				}
			}
			if module {
				runErr = p.vm.evalModuleHere(code, label, dir)
				return
			}
			p.vm.mods.base[label] = dir
			runErr = p.vm.evalHere("(function(){\n"+code+"\n})()", label)
		})
		if err == nil {
			err = runErr
		}
		if err != nil {
			return fmt.Errorf("failed to run migration %s: %w", name, err)
		}
	}
	return nil
}

func (p *plugin) registerHooks() error {
	files, err := p.files(func(e any) bool { _, ok := e.(js.HookFiler); return ok }, func(e any) (map[string][]byte, error) {
		return e.(js.HookFiler).HookFiles(p.config.HooksFilesPattern)
	}, p.config.HooksDir, p.config.HooksFilesPattern)
	if err != nil {
		return err
	}

	if p.extension(func(e any) bool { _, ok := e.(js.HookFiler); return ok }) == nil {
		for name, content := range files {
			if len(content) != 0 {
				continue
			}
			path := filepath.Join(p.config.HooksDir, name)
			directive := `/// <reference path="` + p.relativeTypesPath(p.config.HooksDir) + `" />`
			if err := prependToEmptyFile(path, directive+"\n\n"); err != nil {
				color.Yellow("Unable to prepend the types reference: %v", err)
			}
		}
	}

	if p.config.HooksWatch {
		if err := p.watchHooks(); err != nil {
			color.Yellow("Unable to init hooks watcher: %v", err)
		}
	}
	if len(files) == 0 {
		return nil
	}

	p.app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		e.Router.BindFunc(p.normalizeServeExceptions)
		return e.Next()
	})

	var loaders []js.Loader
	for _, e := range p.config.Extensions {
		if l, ok := e.(js.Loader); ok {
			loaders = append(loaders, l)
		}
	}

	for _, name := range sortedKeys(files) {
		var runErr error
		err := p.vm.Do(func() {
			rt := jsRuntime{p.vm}
			var dones []func()
			var mappers []func(error) error
			for _, l := range loaders {
				done, mapErr := l.OnLoad(rt, p.hooksDir, name, files[name])
				if done != nil {
					dones = append(dones, done)
				}
				if mapErr != nil {
					mappers = append(mappers, mapErr)
				}
			}
			// a .pb.js/.pb.cjs hook file runs in the app's one global scope: its top-level functions and variables
			// are visible to every handler. A .pb.mjs file is an ES module: private top level, export/import.
			label := "pb_hooks/" + name
			dir := filepath.Dir(filepath.Join(p.hooksDir, name))
			module := isModuleFile(name)
			code := string(files[name])
			if isTS(name) {
				format := tsScript
				if module {
					format = tsESM
				}
				code, runErr = p.vm.transpile(code, label, format, 0)
			}
			switch {
			case runErr != nil:
			case module:
				runErr = p.vm.evalModuleHere(code, label, dir)
			default:
				p.vm.mods.base[label] = dir
				runErr = p.vm.evalHere(code, label)
			}
			for _, m := range mappers {
				runErr = m(runErr)
			}
			for i := len(dones) - 1; i >= 0; i-- {
				dones[i]()
			}
		})
		if err == nil {
			err = runErr
		}
		if err != nil {
			if !p.app.IsBootstrapped() && strings.Contains(err.Error(), "nil pointer dereference") {
				// most likely $app's database, which isn't open while hook files load (as in PocketBase)
				err = fmt.Errorf("%w\n   hook files run before the app opens its database: read data in onBootstrap (after e.next()) or in a handler", err)
			}
			fmtErr := fmt.Errorf("failed to execute %s:\n - %w", name, err)
			if p.config.HooksWatch {
				color.Red("%v", fmtErr)
				continue
			}
			return fmtErr
		}
	}
	return nil
}

// files reads from the first extension that has the source, else from dir.
func (p *plugin) files(has func(any) bool, read func(any) (map[string][]byte, error), dir, pattern string) (map[string][]byte, error) {
	if e := p.extension(has); e != nil {
		return read(e)
	}
	return filesContent(dir, pattern)
}

func sortedKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// normalizeServeExceptions unwraps a Go error a JS handler threw.
func (p *plugin) normalizeServeExceptions(e *core.RequestEvent) error {
	err := e.Next()
	if err == nil || e.Written() {
		return err
	}
	var jsErr *JSError
	if errors.As(err, &jsErr) {
		return err
	}
	return err
}

// watchHooks restarts the app when a file in HooksDir changes (not on Windows).
func (p *plugin) watchHooks() error {
	watchDir := p.config.HooksDir
	info, err := os.Lstat(p.config.HooksDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink == os.ModeSymlink {
		if watchDir, err = filepath.EvalSymlinks(p.config.HooksDir); err != nil {
			return fmt.Errorf("failed to resolve hooksDir symlink: %w", err)
		}
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	var debounce *time.Timer
	stop := func() {
		if debounce != nil {
			debounce.Stop()
			debounce = nil
		}
	}
	p.app.OnTerminate().BindFunc(func(e *core.TerminateEvent) error {
		watcher.Close()
		stop()
		return e.Next()
	})
	routine.FireAndForget(func() {
		defer stop()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				stop()
				debounce = time.AfterFunc(50*time.Millisecond, func() {
					if runtime.GOOS == "windows" {
						color.Yellow("File %s changed, please restart the app manually", event.Name)
					} else {
						color.Yellow("File %s changed, restarting...", event.Name)
						if err := p.app.Restart(); err != nil {
							color.Red("Failed to restart the app:", err)
						}
					}
				})
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				color.Red("Watch error:", err)
			}
		}
	})
	dirsErr := filepath.WalkDir(watchDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.IsDir() {
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "node_modules" {
			return filepath.SkipDir
		}
		return watcher.Add(path)
	})
	if dirsErr != nil {
		watcher.Close()
	}
	return dirsErr
}

func (p *plugin) fullTypesPath() string { return filepath.Join(p.config.TypesDir, typesFileName) }

func (p *plugin) relativeTypesPath(basepath string) string {
	return relPath(basepath, p.fullTypesPath())
}

// relPath is target relative to base with forward slashes (as TypeScript wants them), or target when it has none.
func relPath(base, target string) string {
	absBase, err1 := filepath.Abs(base)
	absTarget, err2 := filepath.Abs(target)
	if err1 != nil || err2 != nil {
		return filepath.ToSlash(target)
	}
	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return filepath.ToSlash(absTarget)
	}
	return filepath.ToSlash(rel)
}

const tsconfigFileName = "tsconfig.json"

// writeTSConfig writes tsconfig.json next to types.d.ts so editors type hooks, migrations and pages, only when the
// project has none: an existing one is the developer's and never touched.
func (p *plugin) writeTSConfig() error {
	path := filepath.Join(p.config.TypesDir, tsconfigFileName)
	if _, err := os.Stat(path); err == nil || !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.WriteFile(path, p.tsconfigContent(), 0644)
}

// tsconfigContent includes types.d.ts, the hooks and migrations folders (wherever they are) and pb_pages. "jsx":
// "preserve" lets editors check PocketPages .tsx pages without a React runtime.
func (p *plugin) tsconfigContent() []byte {
	glob := func(dir string) string {
		rel := relPath(p.config.TypesDir, dir)
		if rel == "." {
			return "**/*"
		}
		return rel + "/**/*"
	}
	include := []string{typesFileName}
	for _, g := range []string{glob(p.config.HooksDir), glob(p.config.MigrationsDir), "pb_pages/**/*"} {
		if !slices.Contains(include, g) {
			include = append(include, g)
		}
	}
	inc, _ := json.Marshal(include)
	return []byte(`{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "lib": ["ES2022"],
    "strict": true,
    "noEmit": true,
    "allowImportingTsExtensions": true,
    "skipLibCheck": true,
    "jsx": "preserve"
  },
  "include": ` + strings.ReplaceAll(string(inc), `","`, `", "`) + `
}
`)
}

// refreshTypesFile writes the TS declarations: the engine's (generated, embedded) and every extension's (js.Typer),
// only when they changed.
func (p *plugin) refreshTypesFile() error {
	full := p.fullTypesPath()
	if err := os.MkdirAll(filepath.Dir(full), os.ModePerm); err != nil {
		return err
	}
	content := p.typesContent()
	if old, err := os.ReadFile(full); err == nil && bytes.Equal(old, content) {
		return nil
	}
	return os.WriteFile(full, content, 0644)
}

// typesContent is types.d.ts for this app: the engine's declarations, then each extension's, then passed through
// each extension's filter (js.TypesFilter).
func (p *plugin) typesContent() []byte {
	out := append([]byte{}, typesFile...)
	for _, e := range p.config.Extensions {
		if t, ok := e.(js.Typer); ok {
			if d := strings.TrimSpace(t.TypeScript()); d != "" {
				out = append(out, '\n')
				out = append(out, d...)
				out = append(out, '\n')
			}
		}
	}
	for _, e := range p.config.Extensions {
		if f, ok := e.(js.TypesFilter); ok {
			out = []byte(f.FilterTypes(string(out)))
		}
	}
	return out
}

func prependToEmptyFile(path, text string) error {
	info, err := os.Stat(path)
	if err == nil && info.Size() == 0 {
		return os.WriteFile(path, []byte(text), 0644)
	}
	return err
}

// filesContent returns the files directly in dirPath that match pattern (empty map if the dir is missing).
func filesContent(dirPath string, pattern string) (map[string][]byte, error) {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string][]byte{}, nil
		}
		return nil, err
	}
	var exp *regexp.Regexp
	if pattern != "" {
		if exp, err = regexp.Compile(pattern); err != nil {
			return nil, err
		}
	}
	result := map[string][]byte{}
	for _, f := range files {
		if f.IsDir() || (exp != nil && !exp.MatchString(f.Name())) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dirPath, f.Name()))
		if err != nil {
			return nil, err
		}
		result[f.Name()] = raw
	}
	return result, nil
}

var _ = unsafe.Pointer(nil)

// migrationsDir is the migrations folder as an absolute path (where a migration's imports resolve from).
func (p *plugin) migrationsDir() string {
	if abs, err := filepath.Abs(p.config.MigrationsDir); err == nil {
		return abs
	}
	return p.config.MigrationsDir
}
