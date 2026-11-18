package desktop

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"text/template"
	"time"
)

// AllTargets are the platforms --all builds.
var AllTargets = []string{"darwin/arm64", "darwin/amd64", "windows/amd64", "windows/arm64", "linux/amd64", "linux/arm64"}

// AppFolders are the app folders a desktop build embeds, when they exist.
var AppFolders = []string{"pb_hooks", "pb_migrations", "pb_public"}

// runtimeModule is the module the generated main package calls.
const runtimeModule = "github.com/pockethost/pocketone/ext/desktop/runtime"

// TemplateEnv names the build image's prepared module (`desktop prepare`): set, a build runs inside the image,
// offline, against that module's vendored source.
const TemplateEnv = "P1_DESKTOP_TEMPLATE"

// BuildOptions is one `desktop build`.
type BuildOptions struct {
	Config  Config
	Dir     string   // the app's folder; default the working directory
	All     bool     // every platform in AllTargets
	Targets []string // os/arch; default this machine's
	Out     string   // where the apps go, relative to Dir; default dist
	Local   bool     // build with this machine's Go instead of the build image
	Image   string   // the build image; default the one matching this pocketone (DefaultImage)
	Source  string   // --local: PocketOne source tree; default $P1_SOURCE, else the checkout Dir is in
	Log     io.Writer
}

// Build compiles the app in o.Dir into a desktop app for each target, in o.Out: in the build image (Docker) by
// default, with this machine's Go when o.Local, and offline against the prepared module when it runs in the image.
func Build(ctx context.Context, o BuildOptions) error {
	if o.Log == nil {
		o.Log = io.Discard
	}
	dir, err := filepath.Abs(orStr(o.Dir, "."))
	if err != nil {
		return err
	}
	o.Dir = dir
	out := orStr(o.Out, "dist")
	if !filepath.IsAbs(out) {
		out = filepath.Join(dir, out)
	}
	o.Out = out
	if o.All {
		o.Targets = AllTargets
	}
	if len(o.Targets) == 0 {
		o.Targets = []string{goruntime.GOOS + "/" + goruntime.GOARCH}
	}
	for _, t := range o.Targets {
		if !strings.Contains(t, "/") {
			return fmt.Errorf("--target %q: use os/arch, like darwin/arm64", t)
		}
	}
	if tmpl := os.Getenv(TemplateEnv); tmpl != "" {
		return compile(ctx, o, tmpl, "")
	}
	if !o.Local {
		return buildInDocker(ctx, o)
	}
	source, err := findSource(o.Source, dir)
	if err != nil {
		return err
	}
	work, err := os.MkdirTemp("", "pocketone-desktop-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	return compile(ctx, o, work, source)
}

// compile builds o's targets in work: a fresh folder that gets a go.mod against source, or (source "") the build
// image's prepared module, whose go.mod and vendor/ stay as they are.
func compile(ctx context.Context, o BuildOptions, work, source string) error {
	goBin, err := exec.LookPath("go")
	if err != nil {
		return errors.New("desktop build --local needs Go 1.27 or newer on your PATH (https://go.dev/dl); without --local it runs in Docker")
	}
	name, id := appNames(o.Config, o.Dir)
	embedded, err := copyApp(o.Dir, filepath.Join(work, "app"))
	if err != nil {
		return err
	}
	from := "the build image's vendored source"
	if source != "" {
		from = source
	}
	fmt.Fprintf(o.Log, "%s %s (%s): embedding %s, building against %s\n", stamp(), name, id, strings.Join(embedded, ", "), from)
	if err := writeMain(work, name, id, o.Config, embedded); err != nil {
		return err
	}
	if source != "" {
		if err := writeGoMod(work, source); err != nil {
			return err
		}
		if err := goCmd(ctx, goBin, work, nil, "mod", "tidy"); err != nil {
			return err
		}
	}
	for _, t := range o.Targets {
		goos, goarch, _ := strings.Cut(t, "/")
		start := time.Now()
		fmt.Fprintf(o.Log, "%s building %s/%s...\n", stamp(), goos, goarch)
		path, err := buildTarget(ctx, goBin, work, o.Out, name, id, goos, goarch)
		if err != nil {
			return fmt.Errorf("%s/%s: %w", goos, goarch, err)
		}
		fmt.Fprintf(o.Log, "%s   %s (%s)\n", stamp(), displayPath(o.Dir, path), time.Since(start).Round(100*time.Millisecond))
	}
	return nil
}

// appNames are the app's name and id: the desktop section's, else from the folder's name.
func appNames(c Config, dir string) (name, id string) {
	name = orStr(c.Name, filepath.Base(dir))
	return name, orStr(c.ID, "app.pocketone."+strings.ToLower(safeName(name)))
}

// copyApp copies the app folders that exist in dir into dst (emptied first) and returns their names.
func copyApp(dir, dst string) ([]string, error) {
	if err := os.RemoveAll(dst); err != nil {
		return nil, err
	}
	var embedded []string
	for _, f := range AppFolders {
		src := filepath.Join(dir, f)
		if fi, err := os.Stat(src); err != nil || !fi.IsDir() {
			continue
		}
		if err := os.CopyFS(filepath.Join(dst, f), os.DirFS(src)); err != nil {
			return nil, fmt.Errorf("copy %s: %w", f, err)
		}
		embedded = append(embedded, f)
	}
	if len(embedded) == 0 {
		return nil, fmt.Errorf("%s has none of %s: run desktop build in your app's folder", dir, strings.Join(AppFolders, ", "))
	}
	return embedded, nil
}

// displayPath is path relative to dir when it's inside it.
func displayPath(dir, path string) string {
	if rel, err := filepath.Rel(dir, path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return path
}

// buildTarget builds one platform and returns what it wrote: an .app bundle on macOS, an executable elsewhere.
func buildTarget(ctx context.Context, goBin, work, out, name, id, goos, goarch string) (string, error) {
	dest := filepath.Join(out, goos+"-"+goarch)
	if err := os.RemoveAll(dest); err != nil {
		return "", err
	}
	exe := safeName(name)
	bin := filepath.Join(dest, exe)
	ldflags := "-s -w"
	switch goos {
	case "windows":
		bin += ".exe"
		ldflags += " -H windowsgui" // no console window behind the app
	case "darwin":
		bin = filepath.Join(dest, name+".app", "Contents", "MacOS", exe)
	}
	args := []string{"build", "-trimpath", "-ldflags", ldflags, "-o", bin}
	if goos == "darwin" || goos == "linux" {
		args = append(args, "-tags", "purego") // Wails' cgo-free backends
	}
	args = append(args, ".")
	env := []string{"CGO_ENABLED=0", "GOOS=" + goos, "GOARCH=" + goarch}
	if err := goCmd(ctx, goBin, work, env, args...); err != nil {
		return "", err
	}
	if goos != "darwin" {
		return bin, nil
	}
	app := filepath.Join(dest, name+".app")
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), infoPlist(name, id, exe), 0o644); err != nil {
		return "", err
	}
	adhocSign(ctx, app)
	return app, nil
}

// adhocSign gives a macOS bundle an ad-hoc signature, so macOS runs it on this machine. It does nothing off macOS
// (inside the build image, the host signs the bundles after the build).
func adhocSign(ctx context.Context, app string) {
	if goruntime.GOOS != "darwin" {
		return
	}
	if cs, err := exec.LookPath("codesign"); err == nil {
		_ = exec.CommandContext(ctx, cs, "--force", "--deep", "--sign", "-", app).Run()
	}
}

// findSource is the PocketOne source tree to build against: the flag, $P1_SOURCE, else the checkout dir is in.
func findSource(flag, dir string) (string, error) {
	if s := orStr(flag, os.Getenv("P1_SOURCE")); s != "" {
		abs, err := filepath.Abs(s)
		if err != nil {
			return "", err
		}
		if !isSource(abs) {
			return "", fmt.Errorf("%s isn't a PocketOne source tree (no ext/desktop/runtime)", abs)
		}
		return abs, nil
	}
	for d := dir; ; d = filepath.Dir(d) {
		if isSource(d) {
			return d, nil
		}
		if filepath.Dir(d) == d {
			break
		}
	}
	return "", errors.New("desktop build --local needs a PocketOne source tree: pass --source <checkout> or set " +
		"P1_SOURCE, or drop --local to build in the build image")
}

func isSource(d string) bool {
	mod, err := os.ReadFile(filepath.Join(d, "go.mod"))
	if err != nil || !bytes.HasPrefix(mod, []byte("module ganigeorgiev.com/pocketbase/pocketbase\n")) {
		return false
	}
	_, err = os.Stat(filepath.Join(d, "ext", "desktop", "runtime", "go.mod"))
	return err == nil
}

// writeGoMod writes the generated module: the runtime's go.mod and go.sum, with every replace pointing into source.
func writeGoMod(work, source string) error {
	rtDir := filepath.Join(source, "ext", "desktop", "runtime")
	mod, err := os.ReadFile(filepath.Join(rtDir, "go.mod"))
	if err != nil {
		return err
	}
	var b strings.Builder
	for _, line := range strings.Split(string(mod), "\n") {
		switch {
		case strings.HasPrefix(line, "module "):
			line = "module pocketoneapp"
		case strings.Contains(line, "=> ../"), strings.HasSuffix(line, "=> .."):
			before, rel, _ := strings.Cut(line, "=> ")
			line = before + "=> " + filepath.Join(rtDir, strings.TrimSpace(rel))
		}
		b.WriteString(line + "\n")
	}
	fmt.Fprintf(&b, "\nrequire %s v0.0.0-00010101000000-000000000000\n\nreplace %s => %s\n", runtimeModule, runtimeModule, rtDir)
	if err := os.WriteFile(filepath.Join(work, "go.mod"), []byte(b.String()), 0o644); err != nil {
		return err
	}
	sum, err := os.ReadFile(filepath.Join(rtDir, "go.sum"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.WriteFile(filepath.Join(work, "go.sum"), sum, 0o644)
}

var mainTmpl = template.Must(template.New("main").Parse(`// Code generated by pocketone desktop build. DO NOT EDIT.
package main

import (
	"embed"
	"io/fs"

	desktop "` + runtimeModule + `"
)

//go:embed all:app
var files embed.FS

func sub(dir string) fs.FS {
	f, err := fs.Sub(files, "app/"+dir)
	if err != nil {
		return nil
	}
	if _, err := fs.Stat(f, "."); err != nil {
		return nil
	}
	return f
}

func main() {
	desktop.Run(desktop.App{
		Name:   {{printf "%q" .Name}},
		ID:     {{printf "%q" .ID}},
		Width:  {{.Width}},
		Height: {{.Height}},
{{- range .Folders}}
		{{.Field}}: sub({{printf "%q" .Dir}}),
{{- end}}
	})
}
`))

func writeMain(work, name, id string, c Config, embedded []string) error {
	fields := map[string]string{"pb_hooks": "Hooks", "pb_migrations": "Migrations", "pb_public": "Public"}
	type folder struct{ Field, Dir string }
	var folders []folder
	for _, f := range embedded {
		folders = append(folders, folder{fields[f], f})
	}
	var b bytes.Buffer
	err := mainTmpl.Execute(&b, map[string]any{"Name": name, "ID": id, "Width": c.Width, "Height": c.Height, "Folders": folders})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(work, "main.go"), b.Bytes(), 0o644)
}

func infoPlist(name, id, exe string) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key><string>` + xmlEscape(name) + `</string>
	<key>CFBundleDisplayName</key><string>` + xmlEscape(name) + `</string>
	<key>CFBundleIdentifier</key><string>` + xmlEscape(id) + `</string>
	<key>CFBundleExecutable</key><string>` + xmlEscape(exe) + `</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleVersion</key><string>1.0.0</string>
	<key>CFBundleShortVersionString</key><string>1.0.0</string>
	<key>LSMinimumSystemVersion</key><string>11.0</string>
	<key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
`)
}

// goCmd runs go in dir, its output shown only when it fails.
func goCmd(ctx context.Context, goBin, dir string, env []string, args ...string) error {
	cmd := exec.CommandContext(ctx, goBin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(out.String()))
	}
	return nil
}

// safeName is name as a file name: letters, digits, '-', '_' and '.', the rest dropped.
func safeName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r == '-' || r == '_' || r == '.' || ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') || ('0' <= r && r <= '9') {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "app"
	}
	return b.String()
}

var xmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")

func xmlEscape(s string) string { return xmlEscaper.Replace(s) }

func stamp() string { return time.Now().Format("15:04:05") }

func orStr(v, def string) string {
	if v != "" {
		return v
	}
	return def
}
