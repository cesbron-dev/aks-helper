package cmd

import (
	"fmt"
	"io"

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
		Long: `Selects a stored cluster and points KUBECONFIG at it for the current shell.

Each shell keeps its own KUBECONFIG, so different terminals can target different
clusters at the same time.

A child process cannot change its parent shell's environment, so 'use' relies on
the wrapper function from 'shell-init' (loaded once in your shell profile):

  aks use            # fuzzy-pick
  aks use my-cluster # pick by name

If you do not want a shell function, 'aks-helper shell my-cluster' opens a
subshell scoped to a cluster — the most reliable option on Windows (PowerShell
and git-bash).`,
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
			printUseHint(cmd.ErrOrStderr(), name, path)
			return nil
		},
	}

	cmd.Flags().BoolVar(&printExport, "export", false, "print the KUBECONFIG statement for eval (used by the shell function)")
	cmd.Flags().StringVar(&shell, "shell", "posix", "syntax for --export: posix, fish, powershell")
	return cmd
}

// printUseHint is shown when 'use' runs outside the shell wrapper (so it could
// not set KUBECONFIG). It gives shell-correct options without guessing the shell.
func printUseHint(w io.Writer, name, path string) {
	posix, _ := exportStatement("posix", path)
	ps, _ := exportStatement("powershell", path)
	fmt.Fprintf(w,
		"KUBECONFIG was not changed (run through the 'aks' function, or use one of):\n"+
			"  • a subshell scoped to it (any shell, no setup):  aks-helper shell %s\n"+
			"  • set it yourself:\n"+
			"      bash/zsh/git-bash : %s\n"+
			"      PowerShell        : %s\n"+
			"  • load the wrapper once so 'aks use' does it:      aks-helper shell-init <shell>\n",
		name, posix, ps)
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
