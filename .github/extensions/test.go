package main

// `test` runs locally what CI runs on a pull request, so an author knows before pushing that CI will
// pass: the extension rules, then tidy, build, vet and `go test -race` for each affected module, then a check that the
// site can be built from the READMEs. Nothing it checks needs a file outside the author's extension folder: generated
// files (the site, llms.txt, contract indexes) are regenerated on main.
//
// `quick` is the part of it that takes seconds, not minutes: the rules, `go mod tidy -diff` and `go vet` (which compiles
// every package and its tests) for each affected module. It catches what breaks a module outright, such as a go.mod
// or go.sum left stale by a change in a module it replaces, and leaves the tests to CI. Run it before a push when you
// want a fast sanity check; nothing runs it for you.

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

func step(format string, a ...any) {
	fmt.Printf("\n== %s  %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, a...))
}

// changedModules is the modules a change since origin/main affects. committedOnly leaves out uncommitted work (what a
// push sends); otherwise the working tree counts too.
func changedModules(committedOnly bool) ([]string, error) {
	base, err := exec.Command("git", "merge-base", "origin/main", "HEAD").Output()
	if err != nil {
		return nil, fmt.Errorf("git merge-base origin/main HEAD (run `git fetch origin` first, or name the dirs): %w", err)
	}
	head := ""
	if committedOnly {
		head = "HEAD"
	}
	return affected(strings.TrimSpace(string(base)), head)
}

func checkRules() error {
	step("extension rules (go run ./.github/extensions check)")
	mods, err := modules()
	if err != nil {
		return err
	}
	problems := append(check(mods), checkContracts()...)
	problems = append(problems, checkOwnership()...)
	problems = append(problems, checkChangelogs()...)
	problems = append(problems, checkTablePrefixes()...)
	for _, p := range problems {
		fmt.Println("  problem:", p)
	}
	if len(problems) > 0 {
		return fmt.Errorf("%d problems with the extension rules", len(problems))
	}
	fmt.Println("  ok")
	return nil
}

// goSteps runs each `go` command in each module, stopping at the first failure.
func goSteps(dirs []string, steps [][]string) error {
	for _, dir := range dirs {
		for _, args := range steps {
			step("%s: go %s", dir, strings.Join(args, " "))
			cmd := exec.Command("go", args...)
			cmd.Dir = dir
			if tags, ok := moduleTags[dir]; ok {
				cmd.Env = append(os.Environ(), "GOFLAGS=-tags="+tags)
			}
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
			if err := cmd.Run(); err != nil {
				if args[0] == "mod" {
					return fmt.Errorf("%s: go.mod or go.sum is out of date; run `go mod tidy` there and commit it", dir)
				}
				return fmt.Errorf("%s: go %s: %w", dir, args[0], err)
			}
		}
	}
	return nil
}

var tidyStep = []string{"mod", "tidy", "-diff"}

func runTest(dirs []string) error {
	start := time.Now()
	if len(dirs) == 0 {
		var err error
		if dirs, err = changedModules(false); err != nil {
			return err
		}
		fmt.Printf("modules changed since origin/main (committed or not): %s\n", strings.Join(dirs, " "))
	}
	if err := checkRules(); err != nil {
		return err
	}
	if err := goSteps(dirs, [][]string{
		tidyStep,
		{"build", "-o", os.DevNull, "./..."},
		{"vet", "./..."},
		{"test", "-race", "-count=1", "-timeout", "10m", "./..."},
	}); err != nil {
		return err
	}

	step("site builds from the READMEs (cd site/sync && go run . --validate)")
	cmd := exec.Command("go", "run", ".", "--validate")
	cmd.Dir = "site/sync"
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("site/sync: %w", err)
	}

	fmt.Printf("\nok: CI's checks pass for %d modules (%s)\n", len(dirs), time.Since(start).Round(time.Second))
	return nil
}

func runQuick(dirs []string) error {
	start := time.Now()
	if len(dirs) == 0 {
		var err error
		if dirs, err = changedModules(true); err != nil {
			return err
		}
		fmt.Printf("modules changed since origin/main (committed): %s\n", strings.Join(dirs, " "))
	}
	if err := checkRules(); err != nil {
		return err
	}
	if err := goSteps(dirs, [][]string{tidyStep, {"vet", "./..."}}); err != nil {
		return err
	}
	fmt.Printf("\nok: %d modules are tidy and compile, tests included (%s); CI runs the tests\n", len(dirs), time.Since(start).Round(time.Second))
	return nil
}
