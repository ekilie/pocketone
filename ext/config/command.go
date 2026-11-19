package config

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"pocketone.dev/ext/extkit"
)

// NewCommand returns `config`: it lists every section this binary knows (owner's docs and defaults) and checks a
// config file. opts says where the files are (the root --config/--env-file flags).
func NewCommand(opts func() Options) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Lists the pocketone.yaml sections this binary understands, with their defaults",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			if asJSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(Describe())
			}
			for _, s := range extkit.Sections() {
				fmt.Fprintf(out, "# %s\n", strings.ReplaceAll(s.Doc, "\n", "\n# "))
				b, _ := yaml.Marshal(map[string]any{s.Name: s.Default})
				fmt.Fprintf(out, "%s\n", b)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print the sections as JSON (name, doc, owning extension, keys with types and defaults)")
	cmd.AddCommand(&cobra.Command{
		Use:   "check",
		Short: "Checks the config file: every section known and valid, every $VAR set",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := Read(opts())
			if err != nil {
				return err
			}
			if err := c.Check(); err != nil {
				return err
			}
			if c.Path == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "no config file: defaults only")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: ok (%d sections)", c.Path, len(c.sections))
			if c.EnvPath != "" {
				fmt.Fprintf(cmd.OutOrStdout(), ", secrets from %s", c.EnvPath)
			}
			fmt.Fprintln(cmd.OutOrStdout())
			return nil
		},
	})
	return cmd
}
