package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newPathCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "path [name]",
		Short: "Print the kubeconfig path for a stored cluster",
		Long: `Prints the absolute path of a stored cluster's kubeconfig.

Handy for tools that take an explicit --kubeconfig flag:

  k9s --kubeconfig "$(aks-helper path prod)"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := store()
			if err != nil {
				return err
			}
			name, err := resolveName(st, args)
			if err != nil {
				return err
			}
			if !st.Exists(name) {
				return fmt.Errorf("no stored cluster named %q", name)
			}
			fmt.Println(st.Path(name))
			return nil
		},
	}
	return cmd
}
