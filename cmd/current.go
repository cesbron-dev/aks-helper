package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cesbron-dev/aks-helper/internal/config"
	"github.com/spf13/cobra"
)

func newCurrentCmd() *cobra.Command {
	var quiet bool
	cmd := &cobra.Command{
		Use:     "current",
		Aliases: []string{"cur"},
		Short:   "Show the cluster selected in this shell",
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := store()
			if err != nil {
				return err
			}
			name := currentCluster(st)
			if name == "" {
				if quiet {
					return nil
				}
				fmt.Fprintln(os.Stderr, "no cluster selected — run 'aks-helper use' or 'aks-helper shell'")
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
			return nil
		},
	}
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "print only the name (for prompts/scripts)")
	return cmd
}

// currentCluster determines the active cluster for THIS shell, preferring
// per-terminal signals (the subshell marker, then KUBECONFIG) over the global
// last-selected marker — so parallel terminals each report their own cluster.
func currentCluster(st *config.Store) string {
	if name := os.Getenv("AKS_HELPER_CLUSTER"); name != "" && st.Exists(name) {
		return name
	}
	if kc := os.Getenv("KUBECONFIG"); kc != "" {
		// Take the first entry if it is a path list.
		first := strings.Split(kc, string(os.PathListSeparator))[0]
		name := strings.TrimSuffix(filepath.Base(first), ".yaml")
		if st.Exists(name) && filepath.Clean(first) == filepath.Clean(st.Path(name)) {
			return name
		}
	}
	name, _ := st.Current()
	return name
}
