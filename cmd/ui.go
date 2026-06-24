package cmd

import (
	"os"

	"github.com/cesbron-dev/aks-helper/internal/tui"
	"github.com/spf13/cobra"
)

func newUICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ui",
		Aliases: []string{"tui"},
		Short:   "Browse and manage clusters in an interactive TUI",
		Long: `Opens a k9s-style terminal UI listing your stored clusters, with a live
state icon (running/stopped/gone) and Kubernetes version fetched from Azure.

Keys: enter/k launch k9s on the highlighted cluster, s open a subshell, d
delete, i import from Azure (built-in wizard), c cleanup, r reload, / filter,
q quit.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := store()
			if err != nil {
				return err
			}
			self, err := os.Executable()
			if err != nil || self == "" {
				self = os.Args[0]
			}
			return tui.Run(tui.Options{
				Store:        st,
				Self:         self,
				ResolveShell: func() (string, error) { return resolveShell("") },
			})
		},
	}
	return cmd
}
