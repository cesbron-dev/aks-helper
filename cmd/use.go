package cmd

import (
	"fmt"

	"github.com/cesbron-dev/aks-helper/internal/config"
	"github.com/cesbron-dev/aks-helper/internal/ui"
	"github.com/spf13/cobra"
)

func newUseCmd() *cobra.Command {
	var (
		printExport bool
		shell       string
	)

	cmd := &cobra.Command{
		Use:     "use [name]",
		Aliases: []string{"select", "switch"},
		Short:   "Select a cluster for the current shell",
		Long: `Selects a stored cluster and points KUBECONFIG at it.

Because a child process cannot mutate its parent's environment, 'use' prints a
statement that sets KUBECONFIG. The shell function installed by 'shell-init'
evaluates it automatically, so once that is set up you can simply run:

  aks use            # fuzzy-pick
  aks use my-cluster # pick by name

Without the shell function:
  bash/zsh:   eval "$(aks-helper use my-cluster --export)"
  fish:       aks-helper use my-cluster --export --shell fish | source
  powershell: aks-helper use my-cluster --export --shell powershell | iex`,
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
				return fmt.Errorf("no stored cluster named %q (run 'aks-helper list')", name)
			}
			if err := st.SetCurrent(name); err != nil {
				return err
			}
			path := st.Path(name)
			if printExport {
				// Consumed by `eval`/`Invoke-Expression`; one statement only.
				stmt, err := exportStatement(shell, path)
				if err != nil {
					return err
				}
				fmt.Println(stmt)
				return nil
			}
			fmt.Printf("selected %s\n", name)
			fmt.Fprintf(cmd.ErrOrStderr(),
				"KUBECONFIG is not set (no shell integration detected).\n"+
					"Run:  %s\n"+
					"Or install the shell function once:  eval \"$(aks-helper shell-init bash)\"\n",
				manualExportHint(shell, path))
			return nil
		},
	}

	cmd.Flags().BoolVar(&printExport, "export", false, "print the KUBECONFIG statement for eval (used by the shell function)")
	cmd.Flags().StringVar(&shell, "shell", "posix", "syntax for --export: posix, fish, powershell")
	return cmd
}

// resolveName returns the cluster name from args, or prompts for one.
func resolveName(st *config.Store, args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	entries, err := st.List()
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("no clusters stored yet — run 'aks-helper sync'")
	}
	current, _ := st.Current()
	idx, err := ui.Select(entries, "cluster>", func(e config.Entry) string {
		marker := "  "
		if e.Name == current {
			marker = "* "
		}
		if e.ClusterName != "" {
			return fmt.Sprintf("%s%s  (%s / %s)", marker, e.Name, e.Subscription, e.ResourceGroup)
		}
		return marker + e.Name
	})
	if err != nil {
		return "", err
	}
	return entries[idx].Name, nil
}
