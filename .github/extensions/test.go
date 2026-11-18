package main

// `test` runs locally what CI runs on a pull request, so an extension author knows before pushing that CI will pass:
// the extension rules, then build, vet and `go test -race` for each affected module, then a check that the site can
// be built from the READMEs. Nothing it checks needs a file outside the author's extension folder: generated files
// (the site, llms.txt, contract indexes) are regenerated on main.

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// moduleTags are the build tags a module builds and tests with, here and in CI (extensions.yml's GOFLAGS): Wails'
// cgo-free backend for the desktop runtime, as `pocketone desktop build` uses.
var moduleTags = map[string]string{"ext/desktop/runtime": "purego"}

func runTest(dirs []string) error {
	start := time.Now()
	step := func(format string, a ...any) {
		fmt.Printf("\n== %s  %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, a...))
	}
	if len(dirs) == 0 {
		base, err := exec.Command("git", "merge-base", "origin/main", "HEAD").Output()
		if err != nil {
			return fmt.Errorf("git merge-base origin/main HEAD (run `git fetch origin` first, or name the dirs): %w", err)
		}
		if dirs, err = affected(strings.TrimSpace(string(base)), ""); err != nil {
			return err
		}
		fmt.Printf("modules changed since origin/main (committed or not): %s\n", strings.Join(dirs, " "))
	}

	step("extension rules (go run ./.github/extensions check)")
	mods, err := modules()
	if err != nil {
		return err
	}
	problems := append(check(mods), checkContracts()...)
	problems = append(problems, checkOwnership()...)
	problems = append(problems, checkChangelogs()...)
	for _, p := range problems {
		fmt.Println("  problem:", p)
	}
	if len(problems) > 0 {
		return fmt.Errorf("%d problems with the extension rules", len(problems))
	}
	fmt.Println("  ok")

	for _, dir := range dirs {
		for _, args := range [][]string{
			{"build", "-o", os.DevNull, "./..."},
			{"vet", "./..."},
			{"test", "-race", "-count=1", "-timeout", "10m", "./..."},
		} {
			step("%s: go %s", dir, strings.Join(args, " "))
			cmd := exec.Command("go", args...)
			cmd.Dir = dir
			if tags, ok := moduleTags[dir]; ok {
				cmd.Env = append(os.Environ(), "GOFLAGS=-tags="+tags)
			}
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("%s: go %s: %w", dir, args[0], err)
			}
		}
	}

	step("site builds from the READMEs (cd site/sync && go run . --validate)")
	cmd := exec.Command("go", "run", ".", "--validate")
	cmd.Dir = "site/sync"
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("site/sync: %w", err)
	}

	fmt.Printf("\nok: CI's pull request checks pass for %d modules (%s)\n", len(dirs), time.Since(start).Round(time.Second))
	return nil
}
