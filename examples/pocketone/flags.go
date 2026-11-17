package main

import (
	"io"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// appCommands open this app (its pb_data, pb_hooks and pb_migrations) and take the app flags; serve also takes the
// serve flags. Every other command (deploy, link, config, upgrade, ...) takes only --config and --env-file, the
// flags every command reads. A command that isn't in this binary (its extension is off) is skipped.
var appCommands = []string{"serve", "migrate", "superuser", "pitr", "token", "oauth", "template"}

// coreFlags are PocketBase's own root flags. They stay on the root command, hidden, since core defines and reads them
// there (and every command still bootstraps the app); a visible copy sharing the same value goes in the app flags.
var coreFlags = []string{"dir", "dev", "encryptionEnv", "queryTimeout"}

// moveCoreFlags hides core's root flags and adds a visible copy of each to app.
func moveCoreFlags(root *cobra.Command, app *pflag.FlagSet) {
	for _, name := range coreFlags {
		f := root.PersistentFlags().Lookup(name)
		if f == nil {
			continue
		}
		f.Hidden = true
		visible := *f // same Value: setting either one sets the variable core reads
		visible.Hidden = false
		app.AddFlag(&visible)
	}
}

// parseEarly reads the app and serve flags before cobra runs, because the extensions are built from them first.
// Flags of other commands are skipped and errors are left to cobra, which parses again and rejects a flag the
// command doesn't take.
func parseEarly(args []string, sets ...*pflag.FlagSet) {
	fs := pflag.NewFlagSet("early", pflag.ContinueOnError)
	fs.ParseErrorsAllowlist.UnknownFlags = true
	fs.SetOutput(io.Discard)
	fs.BoolP("help", "h", false, "")
	fs.BoolP("version", "v", false, "")
	for _, s := range sets {
		fs.AddFlagSet(s)
	}
	_ = fs.Parse(args)
}

// attachFlags puts the app flags on appCommands (and their subcommands) and the serve flags on serve.
func attachFlags(root *cobra.Command, app, serve *pflag.FlagSet) {
	for _, name := range appCommands {
		for _, c := range root.Commands() {
			if c.Name() != name {
				continue
			}
			c.PersistentFlags().AddFlagSet(app)
			if name == "serve" {
				c.PersistentFlags().AddFlagSet(serve)
			}
		}
	}
}
