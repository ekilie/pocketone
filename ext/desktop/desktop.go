// Package desktop adds `pocketone desktop build`: it compiles the PocketOne app in the current folder (pb_hooks,
// pb_migrations, pb_public) into a native desktop app, one executable per platform, that runs the app's server on a
// private port and shows it in a native window (ext/desktop/runtime, Wails on the system webview). No cgo: every
// platform builds from any machine with Go installed.
package desktop

import (
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/spf13/pflag"

	clicmdv1 "github.com/pockethost/pocketone/ext/contracts/cli/commands/v1"
	"github.com/pockethost/pocketone/ext/extkit"
)

// Name is the extension's name.
const Name = "desktop"

// Config is pocketone.yaml's desktop section.
type Config struct {
	Name   string `yaml:"name" doc:"The app's name: the window title and the executable's name. Default: the folder's name."`
	ID     string `yaml:"id" doc:"Reverse-DNS app id (dev.example.notes): names the app's data folder on each user's machine and its single-instance lock. Default: app.pocketone.<name>."`
	Width  int    `yaml:"width" doc:"The window's starting width. Default: 1024."`
	Height int    `yaml:"height" doc:"The window's starting height. Default: 768."`
}

// Section is pocketone.yaml's desktop section.
var Section = extkit.NewSection(Name, Config{}, "How `pocketone desktop build` names and sizes the desktop app built from this folder.")

// Extension provides the `desktop build` command.
type Extension struct{}

// New is the extension.
func New() *Extension { return &Extension{} }

// Name implements extkit.Extension.
func (*Extension) Name() string { return Name }

// Contracts implements extkit.Declarer.
func (*Extension) Contracts() extkit.Contracts {
	return extkit.Contracts{Implements: extkit.IDs("cli.commands/v1")}
}

// Register implements extkit.Extension.
func (*Extension) Register(app core.App, _ extkit.Config) error {
	clicmdv1.Contract.Provide(app, clicmdv1.Command{
		Path:  "desktop build",
		Short: "Build this folder's app as a native desktop app (this platform; --all for every platform)",
		Long: `Compiles the PocketOne app in the current folder into a desktop app: pb_hooks, pb_migrations and pb_public are
embedded in the executable, which runs the app's server on a private 127.0.0.1 port and shows it in a native window.
Each user's data lives in their own app-data folder, named by the app id.

Without flags it builds for this machine. --all builds macOS (arm64, amd64), Windows (amd64, arm64) and Linux
(amd64, arm64); --target picks some. The apps land in dist/. Needs Go; no C compiler, no Docker.`,
		Flags: func(fs *pflag.FlagSet) {
			fs.Bool("all", false, "build for every platform")
			fs.StringSlice("target", nil, "platforms to build, as os/arch (darwin/arm64,windows/amd64,...)")
			fs.String("out", "dist", "the folder the apps are written to")
			fs.String("source", "", "a PocketOne source tree to build against (default $P1_SOURCE, else the checkout this folder is in)")
		},
		Run: func(c *clicmdv1.Ctx, _ []string) error {
			fs := c.Flags()
			all, _ := fs.GetBool("all")
			targets, _ := fs.GetStringSlice("target")
			out, _ := fs.GetString("out")
			source, _ := fs.GetString("source")
			cfg, _ := Section.Get()
			return Build(c, BuildOptions{Config: cfg, All: all, Targets: targets, Out: out, Source: source, Log: c.Out})
		},
	})
	return nil
}
