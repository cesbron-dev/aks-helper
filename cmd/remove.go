package cmd

import (
	"fmt"

	"github.com/cesbron-dev/aks-helper/internal/ui"
	"github.com/spf13/cobra"
)

func newRemoveCmd() *cobra.Command {
	var force, yes bool
	cmd := &cobra.Command{
		Use:     "remove [name...]",
		Aliases: []string{"rm", "delete"},
		Short:   "Remove one or more stored clusters",
		Long:    "Deletes the stored kubeconfig(s). The cluster itself in Azure is never touched.",
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := store()
			if err != nil {
				return err
			}
			names := args
			if len(names) == 0 {
				name, err := resolveName(st, nil)
				if err != nil {
					return err
				}
				names = []string{name}
			}
			for _, name := range names {
				if !st.Exists(name) {
					return fmt.Errorf("no stored cluster named %q", name)
				}
			}
			if !force && !yes && len(names) > 0 {
				if !ui.Confirm(fmt.Sprintf("Remove %d stored cluster(s)?", len(names)), false) {
					return fmt.Errorf("aborted")
				}
			}
			for _, name := range names {
				if err := st.Remove(name); err != nil {
					return err
				}
				fmt.Printf("removed %s\n", name)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "do not ask for confirmation")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "do not ask for confirmation (alias of --force)")
	return cmd
}
