package modetest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// The superuser every instance has, created before its first serve.
const (
	SuperuserEmail    = "admin@example.com"
	SuperuserPassword = "password1234"
)

var (
	binOnce sync.Once
	binPath string
	binErr  error
)

// Bin is the pocketone binary: P1_MODETEST_BIN when set, otherwise examples/pocketone built once per test process
// into a temp folder (about 2s with a warm build cache). It skips the test under -short.
func Bin(t testing.TB) string {
	t.Helper()
	if testing.Short() {
		t.Skip("mode test: builds and runs the pocketone binary")
	}
	binOnce.Do(func() {
		if p := os.Getenv("P1_MODETEST_BIN"); p != "" {
			binPath, binErr = filepath.Abs(p)
			return
		}
		src, err := findBinarySource()
		if err != nil {
			binErr = err
			return
		}
		// os.TempDir, not t.TempDir: the binary outlives this test. It stays under the temp folder, so it runs in dev
		// mode (osutils.IsProbablyGoRun). Left for the OS to clean, like go test's own build output.
		dir, err := os.MkdirTemp("", "pocketone-modetest-")
		if err != nil {
			binErr = err
			return
		}
		binPath = filepath.Join(dir, "pocketone")
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		start := time.Now()
		fmt.Fprintf(os.Stderr, "modetest: building %s\n", src)
		cmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, ".")
		cmd.Dir = src
		if out, err := cmd.CombinedOutput(); err != nil {
			binErr = fmt.Errorf("modetest: build %s: %v\n%s", src, err, out)
			return
		}
		fmt.Fprintf(os.Stderr, "modetest: built %s in %s\n", binPath, time.Since(start).Round(time.Millisecond))
	})
	if binErr != nil {
		t.Fatal(binErr)
	}
	return binPath
}

// findBinarySource walks up from the working directory (the package under test) to the repo's examples/pocketone.
func findBinarySource() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		src := filepath.Join(dir, "examples", "pocketone")
		if _, err := os.Stat(filepath.Join(src, "main.go")); err == nil {
			return src, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("modetest: no examples/pocketone above the working directory; set P1_MODETEST_BIN to a pocketone binary")
		}
		dir = parent
	}
}
