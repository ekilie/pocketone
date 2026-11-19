package standard

import (
	"fmt"
	"strconv"
	"text/tabwriter"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/spf13/cobra"
)

// setCommands are the set's own commands: extension migrations and pb_cache. They work on whatever extensions run on
// app (plan.On).
func setCommands(app core.App, plan *extkit.Plan) []*cobra.Command {
	migrators := func() map[string][]extkit.Migration {
		out := map[string][]extkit.Migration{}
		for _, e := range plan.On {
			if m, ok := e.(extkit.Migrator); ok {
				out[e.Name()] = m.Migrations()
			}
		}
		return out
	}
	names := func() []string {
		var out []string
		for _, e := range plan.On {
			if _, ok := e.(extkit.Migrator); ok {
				out = append(out, e.Name())
			}
		}
		return out
	}

	ext := &cobra.Command{Use: "extensions", Short: "Manages the extensions' own schema (their migrations)"}
	ext.AddCommand(&cobra.Command{
		Use:   "migrations",
		Short: "Lists each extension's migrations and whether they're applied",
		Long: "Extensions apply their migrations at startup, after PocketBase's own and before the app's pb_migrations, " +
			"and record them in _extension_migrations (apart from the app's _migrations).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			all := migrators()
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "EXTENSION\tMIGRATION\tAPPLIED")
			for _, name := range names() {
				st, err := extkit.MigrationStatus(app, name, all[name])
				if err != nil {
					return err
				}
				for _, s := range st {
					applied := "pending"
					if s.Applied != nil {
						applied = s.Applied.UTC().Format("2006-01-02 15:04:05Z")
					}
					fmt.Fprintf(w, "%s\t%s\t%s\n", name, s.Name, applied)
				}
			}
			return w.Flush()
		},
	})
	migrate := &cobra.Command{Use: "migrate", Short: "Applies or reverts an extension's migrations"}
	migrate.AddCommand(&cobra.Command{
		Use:   "down <extension> [n]",
		Short: "Reverts an extension's last n migrations (default 1; 0: all of them)",
		Long: "Reverting drops what the migrations created, data included. The next start applies them again, " +
			"unless the extension is turned off (`<extension>: {enabled: false}` in pocketone.yaml).",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			list, ok := migrators()[args[0]]
			if !ok {
				return fmt.Errorf("%s isn't running here or has no migrations (see `extensions migrations`)", args[0])
			}
			n := 1
			if len(args) == 2 {
				v, err := strconv.Atoi(args[1])
				if err != nil || v < 0 {
					return fmt.Errorf("n must be a number, 0 or more: %q", args[1])
				}
				n = v
			}
			reverted, err := extkit.MigrateDown(app, args[0], list, n)
			for _, r := range reverted {
				fmt.Fprintf(cmd.OutOrStdout(), "reverted %s %s\n", args[0], r)
			}
			if err == nil && len(reverted) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "%s has no applied migrations\n", args[0])
			}
			return err
		},
	})
	migrate.AddCommand(&cobra.Command{
		Use:   "up [extension]",
		Short: "Applies pending extension migrations (startup does this too)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			all := migrators()
			for _, name := range names() {
				if len(args) == 1 && args[0] != name {
					continue
				}
				applied, err := extkit.MigrateUp(app, name, all[name])
				for _, a := range applied {
					fmt.Fprintf(cmd.OutOrStdout(), "applied %s %s\n", name, a)
				}
				if err != nil {
					return err
				}
			}
			return nil
		},
	})
	ext.AddCommand(migrate)

	cache := &cobra.Command{
		Use:   "cache",
		Short: "Shows each extension's folder in pb_cache and its size",
		Long: "pb_cache, next to pb_data, holds files extensions can rebuild (one folder each). It's never backed up " +
			"or deployed, and emptying it is always safe.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root := extkit.CacheRoot(app)
			usages, err := extkit.CacheUsages(root)
			if err != nil {
				return err
			}
			if len(usages) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "%s is empty\n", root)
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "EXTENSION\tFILES\tSIZE\tFOLDER")
			for _, u := range usages {
				fmt.Fprintf(w, "%s\t%d\t%s\t%s\n", u.Extension, u.Files, humanBytes(u.Bytes), u.Dir)
			}
			return w.Flush()
		},
	}
	cache.AddCommand(&cobra.Command{
		Use:   "clear [extension...]",
		Short: "Empties pb_cache (every extension's folder, or the ones named)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cleared, err := extkit.ClearCache(extkit.CacheRoot(app), args...)
			for _, c := range cleared {
				fmt.Fprintf(cmd.OutOrStdout(), "cleared pb_cache/%s\n", c)
			}
			if err == nil && len(cleared) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "nothing to clear")
			}
			return err
		},
	})
	return []*cobra.Command{ext, cache}
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
