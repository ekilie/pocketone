package wasm_test

// The modules the tests run: Go source folders built for wasip1 (once per test run), the Rust and Zig examples kept
// built beside their sources (prebuilt), and a few modules written byte by byte (tiny, says, producers).

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

var (
	buildMu sync.Mutex
	built   = map[string][]byte{}
)

// module compiles a Go source folder (once per test run) and returns its .wasm.
func module(t *testing.T, src string) []byte { return build(t, src, "", false) }

// moduleTags builds a Go source folder with build tags.
func moduleTags(t *testing.T, src, tags string) []byte { return build(t, src, tags, false) }

// build builds a Go source folder for wasip1: a program (what a module is), or a library (-buildmode=c-shared).
func build(t *testing.T, src, tags string, library bool) []byte {
	t.Helper()
	if testing.Short() {
		t.Skip("builds wasm modules")
	}
	buildMu.Lock()
	defer buildMu.Unlock()
	key := fmt.Sprint(src, " ", tags, " ", library)
	if b, ok := built[key]; ok {
		return b
	}
	out := filepath.Join(t.TempDir(), "module.wasm")
	args := []string{"build", "-tags=" + tags, "-o", out}
	if library {
		args = append(args, "-buildmode=c-shared")
	}
	cmd := exec.Command("go", append(args, ".")...)
	cmd.Dir = src
	cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm", "GOTOOLCHAIN=auto")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building %s: %v\n%s", src, err, b)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	built[key] = b
	return b
}

// The example modules (Go, TinyGo, Rust, Zig, C, LLVM IR, WebAssembly text) are kept built, beside their sources (example/<folder>/<name>.pb.wasm), so
// every run of the tests runs them, with none of those compilers installed. They're built in Docker, with the
// toolchains pinned in example/toolchains/Dockerfile, so a rebuild on any machine gives the same bytes:
// `go test -run TestPrebuiltExamples -update` rebuilds them, and TestPrebuiltMatchSources checks them (both need
// Docker; without it the check skips).

// prebuilt lists them: the folder, the language, what the build makes, and the file kept (the module's name).
var prebuilt = []struct{ dir, lang, out, file string }{
	{"example/summary/go", "go", "summary", "summary.pb.wasm"},
	{"example/crypto", "go", "crypto", "crypto.pb.wasm"},
	{"example/summary/rust", "rust", "summary", "summary.pb.wasm"},
	{"example/summary/zig", "zig", "summary", "summary.pb.wasm"},
	{"example/summary/c", "c", "summary", "summary.pb.wasm"},
	{"example/summary/llvm", "llvm", "summary", "summary.pb.wasm"},
	{"example/summary/wat", "wat", "summary", "summary.pb.wasm"},
	{"example/summary/tinygo", "tinygo", "summary", "summary.pb.wasm"},
	{"example/summary/assemblyscript", "assemblyscript", "summary", "summary.pb.wasm"},
	{"example/stats-tinygo", "tinygo", "stats", "stats.pb.wasm"},
	{"example/edifact-orders-rust", "rust", "orders", "edifact-orders-rust.pb.wasm"},
	{"example/order-xml", "rust", "order-xml", "order-xml.pb.wasm"},
	{"example/waflows", "rust", "waflows", "waflows.pb.wasm"},
	{"example/devil", "rust", "devil", "devil.pb.wasm"},
	{"example/memhog", "rust", "memhog", "memhog.pb.wasm"},
	{"example/markdown", "rust", "markdown", "markdown.pb.wasm"},
	{"example/sanitize-html", "rust", "sanitize-html", "sanitize-html.pb.wasm"},
	{"example/qrcode", "rust", "qrcode", "qrcode.pb.wasm"},
	{"example/thumbnail", "rust", "thumbnail", "thumbnail.pb.wasm"},
	{"example/diff", "rust", "diff", "diff.pb.wasm"},
	{"example/csv", "rust", "csv", "csv.pb.wasm"},
}

// prebuiltModule is the kept build of an example folder.
func prebuiltModule(t *testing.T, dir string) []byte {
	t.Helper()
	for _, p := range prebuilt {
		if p.dir == dir {
			b, err := os.ReadFile(filepath.Join(p.dir, p.file))
			if err != nil {
				t.Fatalf("%v (go test -run TestPrebuiltExamples -update builds it)", err)
			}
			return b
		}
	}
	t.Fatalf("%s isn't a prebuilt example", dir)
	return nil
}

// toolchainImages builds each stage of example/toolchains/Dockerfile once per test run (Docker's cache makes it quick
// after the first time) and names its image.
var (
	imageMu sync.Mutex
	images  = map[string]string{}
)

// toolchainImage is the image of a Dockerfile stage (rust, zig, tinygo, go, wabt, assemblyscript). It skips the test, or with -update fails it,
// when Docker isn't there.
func toolchainImage(t *testing.T, stage string) string {
	t.Helper()
	stop := t.Skipf
	if *update {
		stop = t.Fatalf
	}
	imageMu.Lock()
	defer imageMu.Unlock()
	if img, ok := images[stage]; ok {
		return img
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		stop("no Docker: the prebuilt examples are built in example/toolchains/Dockerfile")
		return ""
	}
	img := "pocketone-wasm-" + stage
	cmd := exec.Command("docker", "build", "-q", "--target", stage, "-t", img, "example/toolchains")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building the %s toolchain image: %v\n%s", stage, err, b)
	}
	images[stage] = img
	return img
}

// buildExample builds a prebuilt example from its source, as the kept file was built: in its toolchain's container,
// with the source mounted read-only and the module copied out. Rust's crate paths are remapped to /cargo, so the
// module holds no path of the container either; the crates are cached in the Docker volume pocketone-wasm-cargo.
func buildExample(t *testing.T, dir string) []byte {
	t.Helper()
	if testing.Short() {
		t.Skip("builds wasm modules")
	}
	var p struct{ dir, lang, out, file string }
	for _, q := range prebuilt {
		if q.dir == dir {
			p = q
		}
	}
	buildMu.Lock()
	defer buildMu.Unlock()
	if b, ok := built[dir]; ok {
		return b
	}
	var stage, script string
	var extra []string
	switch p.lang {
	case "rust":
		stage = "rust"
		extra = []string{"-v", "pocketone-wasm-cargo:/usr/local/cargo/registry",
			"-e", "CARGO_TARGET_DIR=/tmp/target", "-e", "CARGO_ENCODED_RUSTFLAGS=--remap-path-prefix=/usr/local/cargo/registry/src=/cargo"}
		script = "cargo build --release --locked --target wasm32-wasip1 && cp /tmp/target/wasm32-wasip1/release/" + p.out + ".wasm /out/module.wasm"
	case "zig":
		stage = "zig"
		script = "zig build -Doptimize=ReleaseSmall --cache-dir /tmp/zig-local --prefix /tmp/zig-out && cp /tmp/zig-out/bin/" + p.out + ".wasm /out/module.wasm"
	case "c":
		stage = "zig"
		script = "zig cc -target wasm32-wasi -std=c23 -Os -s -Wl,-z,stack-size=1048576 -o /out/module.wasm main.c"
	case "llvm":
		stage = "zig"
		script = "zig cc -target wasm32-wasi -Os -s -Wl,-z,stack-size=1048576 -o /out/module.wasm summary.ll"
	case "wat":
		stage = "wabt"
		script = "wat2wasm summary.wat -o /out/module.wasm"
	case "tinygo":
		stage = "tinygo"
		script = "tinygo build -target=wasip1 -opt=z -no-debug -scheduler=none -o /out/module.wasm ."
	case "assemblyscript":
		stage = "assemblyscript"
		// the source is mounted read-only: build a copy, with the image's pinned packages as its node_modules
		script = "cp -r /src /tmp/b && cd /tmp/b && rm -rf node_modules && ln -s /opt/as/node_modules node_modules && " +
			"npx asc assembly/index.ts --target release --outFile /out/module.wasm"
	case "go":
		stage = "go"
		script = "go build -o /out/module.wasm ." // GOOS, GOARCH and -trimpath are set in the image
	default:
		t.Fatalf("%s isn't a prebuilt example", dir)
	}
	img := toolchainImage(t, stage)
	src, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	args := append([]string{"run", "--rm", "-v", src + ":/src:ro", "-v", out + ":/out", "-w", "/src"}, extra...)
	cmd := exec.Command("docker", append(args, "--entrypoint", "sh", img, "-c", script)...)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building %s: %v\n%s", dir, err, b)
	}
	b, err := os.ReadFile(filepath.Join(out, "module.wasm"))
	if err != nil {
		t.Fatal(err)
	}
	built[dir] = b
	return b
}

// tiny is a WASI program made by hand, a few dozen bytes: a _start that returns at once, or, with spin, one that
// loops until the call's time limit. pad bytes in a custom section make the module as big as a test needs.
func tiny(spin bool, pad int) []byte {
	body := []byte{0x00, 0x0b} // no locals; end
	if spin {
		body = []byte{0x00, 0x03, 0x40, 0x0c, 0x00, 0x0b, 0x0b} // no locals; loop: br 0; end; end
	}
	sec := func(id byte, content []byte) []byte {
		return append(append([]byte{id}, uleb(len(content))...), content...)
	}
	b := []byte{0x00, 'a', 's', 'm', 0x01, 0x00, 0x00, 0x00}
	b = append(b, sec(1, []byte{0x01, 0x60, 0x00, 0x00})...)                              // one type: () -> ()
	b = append(b, sec(3, []byte{0x01, 0x00})...)                                          // one function of it
	b = append(b, sec(7, append(append([]byte{0x01, 0x06}, "_start"...), 0x00, 0x00))...) // exported as _start
	b = append(b, sec(10, append([]byte{0x01}, append(uleb(len(body)), body...)...))...)  // its body
	if pad > 0 {
		b = append(b, sec(0, append([]byte{0x03, 'p', 'a', 'd'}, make([]byte, pad)...))...)
	}
	return b
}

// says is a module that prints out on stdout and exits 0, whatever it's asked: a module whose --describe prints out.
func says(out string) []byte {
	sec := func(id byte, content []byte) []byte {
		return append(append([]byte{id}, uleb(len(content))...), content...)
	}
	str := func(s string) []byte { return append(uleb(len(s)), s...) }
	le := func(n int) []byte { return []byte{byte(n), byte(n >> 8), byte(n >> 16), byte(n >> 24)} }
	b := []byte{0x00, 'a', 's', 'm', 0x01, 0x00, 0x00, 0x00}
	// types: fd_write's (i32, i32, i32, i32) -> i32, and () -> ()
	b = append(b, sec(1, []byte{0x02, 0x60, 0x04, 0x7f, 0x7f, 0x7f, 0x7f, 0x01, 0x7f, 0x60, 0x00, 0x00})...)
	b = append(b, sec(2, append(append(append([]byte{0x01}, str("wasi_snapshot_preview1")...), str("fd_write")...), 0x00, 0x00))...)
	b = append(b, sec(3, []byte{0x01, 0x01})...)                                       // _start, of type 1
	b = append(b, sec(5, append([]byte{0x01, 0x00}, uleb((16+len(out))>>16+1)...))...) // memory: enough pages
	b = append(b, sec(7, append(append(append(append([]byte{0x02}, str("_start")...), 0x00, 0x01), str("memory")...), 0x02, 0x00))...)
	// _start: fd_write(1, iovec at 0, 1 iovec, nwritten at 8); drop
	body := []byte{0x00, 0x41, 0x01, 0x41, 0x00, 0x41, 0x01, 0x41, 0x08, 0x10, 0x00, 0x1a, 0x0b}
	b = append(b, sec(10, append([]byte{0x01}, append(uleb(len(body)), body...)...))...)
	// data at 0: the iovec {16, len(out)}, 8 bytes for nwritten, then out
	data := append(append(append(le(16), le(len(out))...), make([]byte, 8)...), out...)
	b = append(b, sec(11, append(append([]byte{0x01, 0x00, 0x41, 0x00, 0x0b}, uleb(len(data))...), data...))...)
	return b
}

// uleb is n in unsigned LEB128, as wasm writes sizes.
func uleb(n int) []byte {
	var b []byte
	for {
		c := byte(n & 0x7f)
		if n >>= 7; n == 0 {
			return append(b, c)
		}
		b = append(b, c|0x80)
	}
}

// producers builds a producers section: field name, then name/version pairs, then the next field name, ...
func producers(parts ...string) []byte {
	type field struct {
		name   string
		values [][2]string
	}
	var fields []field
	for i := 0; i < len(parts); {
		f := field{name: parts[i]}
		i++
		for i+1 < len(parts) && parts[i] != "processed-by" && parts[i] != "language" {
			f.values = append(f.values, [2]string{parts[i], parts[i+1]})
			i += 2
		}
		fields = append(fields, f)
	}
	str := func(b []byte, s string) []byte { return append(append(b, byte(len(s))), s...) }
	b := []byte{byte(len(fields))}
	for _, f := range fields {
		b = append(str(b, f.name), byte(len(f.values)))
		for _, v := range f.values {
			b = str(str(b, v[0]), v[1])
		}
	}
	return b
}
