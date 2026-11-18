// Package desktop adds `pocketone desktop build`: it compiles the PocketOne app in the current folder (pb_hooks,
// pb_migrations, pb_public) into a native desktop app, one executable per platform, that runs the app's server on a
// private port and shows it in a native window (ext/desktop/runtime, Wails on the system webview). No cgo: every
// platform builds from one machine, in Docker (the build image) or with your own Go (--local).
package desktop

import (
	"os"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/spf13/cobra"
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
(amd64, arm64); --target picks some. The apps land in dist/.

It runs in Docker, in the build image matching this pocketone (Go and PocketOne's source, vendored, so it needs
nothing else and works offline once the image is here). --local builds with your own Go and a PocketOne checkout.`,
		Flags: func(fs *pflag.FlagSet) {
			fs.Bool("all", false, "build for every platform")
			fs.StringSlice("target", nil, "platforms to build, as os/arch (darwin/arm64,windows/amd64,...)")
			fs.String("out", "dist", "the folder the apps are written to")
			fs.Bool("local", false, "build with your own Go instead of the build image")
			fs.String("image", "", "the build image (default "+DefaultImage()+")")
			fs.String("source", "", "a PocketOne checkout: --local builds against it; a development pocketone builds its image from it (default $P1_SOURCE, else the checkout this folder is in)")
		},
		Run: func(c *clicmdv1.Ctx, _ []string) error {
			fs := c.Flags()
			o := BuildOptions{Log: c.Out}
			o.Config, _ = Section.Get()
			o.All, _ = fs.GetBool("all")
			o.Targets, _ = fs.GetStringSlice("target")
			o.Out, _ = fs.GetString("out")
			o.Local, _ = fs.GetBool("local")
			o.Image, _ = fs.GetString("image")
			o.Source, _ = fs.GetString("source")
			return Build(c, o)
		},
	})
	clicmdv1.Contract.Provide(app, clicmdv1.Command{
		Path:  "desktop image",
		Short: "Build the desktop build image from a PocketOne checkout",
		Long: `Builds the image desktop build runs in (Go plus this checkout's source, vendored) and tags it. A release
publishes it as ` + ImageRepo + `:v<version>; a development pocketone builds ` + DevImage + ` on first use.`,
		Flags: func(fs *pflag.FlagSet) {
			fs.String("source", "", "the PocketOne checkout (default $P1_SOURCE, else the one this folder is in)")
			fs.String("tag", DevImage, "the image's tag")
		},
		Run: func(c *clicmdv1.Ctx, _ []string) error {
			src, _ := c.Flags().GetString("source")
			tag, _ := c.Flags().GetString("tag")
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			source, err := findSource(src, wd)
			if err != nil {
				return err
			}
			return BuildImage(c, source, tag, c.Out)
		},
	})
	clicmdv1.Contract.Provide(app, clicmdv1.Command{
		Path:  "desktop prepare",
		Use:   "<dir>",
		Short: "Write the build image's vendored module into <dir> (the build image's Dockerfile runs this)",
		Args:  cobra.ExactArgs(1),
		Flags: func(fs *pflag.FlagSet) {
			fs.String("source", "", "the PocketOne source tree")
		},
		Run: func(c *clicmdv1.Ctx, args []string) error {
			src, _ := c.Flags().GetString("source")
			return Prepare(c, src, args[0])
		},
	})
	return nil
}
