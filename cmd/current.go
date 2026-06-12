package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newCurrentCmd() *cobra.Command {
	var quiet bool
	cmd := &cobra.Command{
		Use:     "current",
		Aliases: []string{"cur"},
		Short:   "Show the currently selected cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := store()
			if err != nil {
				return err
			}
			name, err := st.Current()
			if err != nil {
				return err
			}
			if name == "" {
				if quiet {
					return nil
				}
				fmt.Fprintln(os.Stderr, "no cluster selected — run 'aks-helper use'")
				return nil
			}
			if quiet {
				fmt.Println(name)
				return nil
			}
			e, ok, _ := st.Get(name)
			if ok && e.ClusterName != "" {
				fmt.Printf("%s  (%s / %s, login=%s)\n", name, e.Subscription, e.ResourceGroup, e.LoginMode)
			} else {
				fmt.Println(name)
			}
			if env := os.Getenv("KUBECONFIG"); env != "" && env != st.Path(name) {
				fmt.Fprintf(os.Stderr, "note: KUBECONFIG points elsewhere (%s)\n", env)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "print only the name (for prompts/scripts)")
	return cmd
}
