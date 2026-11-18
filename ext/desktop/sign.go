package desktop

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"
)

// signing is what a build does with its output on this machine, after it's built: sign each platform that
// Config.Sign names (unless --no-sign), and on a Mac sign the rest of the macOS bundles ad hoc and make a DMG of each.
type signing struct {
	cfg     SignConfig
	name    string
	targets []string
}

func (o BuildOptions) signPlan() signing {
	s := signing{targets: o.Targets}
	s.name, _ = appNames(o.Config, o.Dir)
	if !o.NoSign {
		s.cfg = o.Config.Sign
	}
	return s
}

func (s signing) has(goos string) bool {
	for _, t := range s.targets {
		if strings.HasPrefix(t, goos+"/") {
			return true
		}
	}
	return false
}

var onMac = goruntime.GOOS == "darwin"

// preflight checks, before anything is built, that every tool and key the signing needs is here, so a missing one
// fails in a second instead of after the build.
func (s signing) preflight(ctx context.Context, log io.Writer) error {
	if s.has("darwin") && !onMac {
		if s.cfg.MacOS != nil {
			fmt.Fprintf(log, "%s note: macOS builds are signed and notarized only on a Mac; here they stay unsigned\n", stamp())
		}
		fmt.Fprintf(log, "%s note: DMGs are made only on a Mac; here macOS builds are .app bundles only\n", stamp())
	}
	if m := s.cfg.MacOS; m != nil && s.has("darwin") && onMac {
		if m.Identity == "" {
			return errors.New("desktop.sign.macos.identity is empty: set it to your Developer ID (security find-identity -v -p codesigning)")
		}
		for _, tool := range []string{"codesign", "xcrun", "security"} {
			if _, err := exec.LookPath(tool); err != nil {
				return fmt.Errorf("signing macOS builds needs %s (Xcode's command line tools: xcode-select --install)", tool)
			}
		}
		out, _ := exec.CommandContext(ctx, "security", "find-identity", "-v", "-p", "codesigning").Output()
		if !bytes.Contains(out, []byte(`"`+m.Identity+`"`)) {
			return fmt.Errorf("desktop.sign.macos.identity %q isn't in your keychain; security find-identity -v -p codesigning lists the ones that are", m.Identity)
		}
	}
	if w := s.cfg.Windows; w != nil && s.has("windows") {
		if !strings.Contains(w.Command, "{file}") {
			return errors.New("desktop.sign.windows.command needs {file} where the executable's path goes")
		}
		if tool := firstWord(w.Command); tool != "" {
			if _, err := exec.LookPath(tool); err != nil {
				return fmt.Errorf("desktop.sign.windows.command runs %s, which isn't on your PATH", tool)
			}
		}
	}
	if l := s.cfg.Linux; l != nil && s.has("linux") {
		if _, err := exec.LookPath("gpg"); err != nil {
			return errors.New("desktop.sign.linux needs gpg on your PATH")
		}
		if err := exec.CommandContext(ctx, "gpg", "--batch", "--list-secret-keys", l.GPGKey).Run(); err != nil {
			return fmt.Errorf("desktop.sign.linux.gpg_key %q has no secret key here (gpg --list-secret-keys)", l.GPGKey)
		}
	}
	return nil
}

// finish signs and packages each target's build in o.Out.
func (s signing) finish(ctx context.Context, o BuildOptions) error {
	for _, t := range s.targets {
		goos, goarch, _ := strings.Cut(t, "/")
		dest := filepath.Join(o.Out, goos+"-"+goarch)
		var err error
		switch goos {
		case "darwin":
			err = s.finishMac(ctx, o, dest)
		case "windows":
			err = s.finishWindows(ctx, o, dest)
		case "linux":
			err = s.finishLinux(ctx, o, dest)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", t, err)
		}
	}
	return nil
}

// finishMac signs the bundle (Developer ID, or ad hoc), makes a DMG of it, signs the DMG, and notarizes and staples
// it. Off a Mac there are no tools for any of it, and the bundle stays as built.
func (s signing) finishMac(ctx context.Context, o BuildOptions, dest string) error {
	if !onMac {
		return nil
	}
	app := filepath.Join(dest, s.name+".app")
	m := s.cfg.MacOS
	if m == nil {
		adhocSign(ctx, app)
	} else {
		fmt.Fprintf(o.Log, "%s   signing %s\n", stamp(), displayPath(o.Dir, app))
		if err := tool(ctx, "codesign", "--force", "--options", "runtime", "--timestamp", "--sign", m.Identity, app); err != nil {
			return err
		}
		if err := tool(ctx, "codesign", "--verify", "--strict", app); err != nil {
			return err
		}
	}
	dmg := filepath.Join(dest, s.name+".dmg")
	if err := makeDMG(ctx, o.Log, s.name, app, dmg); err != nil {
		return err
	}
	if m != nil {
		if err := tool(ctx, "codesign", "--force", "--timestamp", "--sign", m.Identity, dmg); err != nil {
			return err
		}
	}
	if m != nil && m.NotaryProfile != "" {
		start := time.Now()
		fmt.Fprintf(o.Log, "%s   notarizing %s (Apple usually takes a few minutes)\n", stamp(), displayPath(o.Dir, dmg))
		var out bytes.Buffer
		cmd := exec.CommandContext(ctx, "xcrun", "notarytool", "submit", dmg, "--keychain-profile", m.NotaryProfile, "--wait")
		cmd.Stdout, cmd.Stderr = io.MultiWriter(o.Log, &out), io.MultiWriter(o.Log, &out)
		err := cmd.Run()
		if err != nil || !strings.Contains(out.String(), "status: Accepted") {
			return fmt.Errorf("notarization didn't succeed (xcrun notarytool log <id> --keychain-profile %s shows why): %v", m.NotaryProfile, err)
		}
		if err := tool(ctx, "xcrun", "stapler", "staple", dmg); err != nil {
			return err
		}
		// the DMG's notarization covers the app inside it too: staple it, so the raw .app also opens offline
		if err := tool(ctx, "xcrun", "stapler", "staple", app); err != nil {
			fmt.Fprintf(o.Log, "%s   note: couldn't staple %s; it still opens online: %v\n", stamp(), filepath.Base(app), err)
		}
		fmt.Fprintf(o.Log, "%s   notarized and stapled (%s)\n", stamp(), time.Since(start).Round(time.Second))
	}
	fmt.Fprintf(o.Log, "%s   %s\n", stamp(), displayPath(o.Dir, dmg))
	return nil
}

// makeDMG makes a compressed disk image holding the app and a link to /Applications, to drag it onto.
func makeDMG(ctx context.Context, log io.Writer, name, app, dmg string) error {
	stage, err := os.MkdirTemp("", "pocketone-dmg-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := tool(ctx, "ditto", app, filepath.Join(stage, filepath.Base(app))); err != nil {
		return err
	}
	if err := os.Symlink("/Applications", filepath.Join(stage, "Applications")); err != nil {
		return err
	}
	return tool(ctx, "hdiutil", "create", "-quiet", "-volname", name, "-srcfolder", stage, "-ov", "-format", "UDZO", dmg)
}

// finishWindows runs the configured command on each executable.
func (s signing) finishWindows(ctx context.Context, o BuildOptions, dest string) error {
	w := s.cfg.Windows
	if w == nil {
		return nil
	}
	exes, _ := filepath.Glob(filepath.Join(dest, "*.exe"))
	for _, exe := range exes {
		fmt.Fprintf(o.Log, "%s   signing %s\n", stamp(), displayPath(o.Dir, exe))
		line := strings.ReplaceAll(w.Command, "{file}", shellQuote(exe))
		var cmd *exec.Cmd
		if goruntime.GOOS == "windows" {
			cmd = exec.CommandContext(ctx, "cmd", "/C", line)
		} else {
			cmd = exec.CommandContext(ctx, "sh", "-c", line)
		}
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("desktop.sign.windows.command failed: %w\n%s", err, strings.TrimSpace(out.String()))
		}
	}
	return nil
}

// finishLinux writes a checksum and a detached GPG signature beside each binary.
func (s signing) finishLinux(ctx context.Context, o BuildOptions, dest string) error {
	l := s.cfg.Linux
	if l == nil {
		return nil
	}
	bin := filepath.Join(dest, safeName(s.name))
	data, err := os.ReadFile(bin)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	if err := os.WriteFile(bin+".sha256", []byte(hex.EncodeToString(sum[:])+"  "+filepath.Base(bin)+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(o.Log, "%s   signing %s\n", stamp(), displayPath(o.Dir, bin))
	return tool(ctx, "gpg", "--batch", "--yes", "--local-user", l.GPGKey, "--armor", "--detach-sign", "--output", bin+".asc", bin)
}

// tool runs a signing tool, its output shown only when it fails.
func tool(ctx context.Context, name string, args ...string) error {
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, strings.TrimSpace(out.String()))
	}
	return nil
}

func firstWord(s string) string {
	f := strings.Fields(s)
	if len(f) == 0 {
		return ""
	}
	return strings.Trim(f[0], `"'`)
}

func shellQuote(p string) string {
	if goruntime.GOOS == "windows" {
		return `"` + p + `"`
	}
	return "'" + strings.ReplaceAll(p, "'", `'\''`) + "'"
}
