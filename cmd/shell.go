package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
)

func newShellCmd() *cobra.Command {
	var shellPath string

	cmd := &cobra.Command{
		Use:     "shell [name]",
		Aliases: []string{"spawn"},
		Short:   "Open a subshell scoped to a cluster (per-terminal, no setup)",
		Long: `Launches a new shell with KUBECONFIG set to the chosen cluster.

The selection lasts only for that subshell, so different terminals can target
different clusters at the same time. It needs no profile setup and no eval, which
makes it the most reliable option on Windows (PowerShell and git-bash).

Type 'exit' to return to your previous context.`,
		Example: `  aks-helper shell prod
  aks-helper shell           # fuzzy-pick
  aks-helper shell prod --shell pwsh`,
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

			sh, err := resolveShell(shellPath)
			if err != nil {
				return err
			}
			child := exec.Command(sh)
			child.Stdin = os.Stdin
			child.Stdout = os.Stdout
			child.Stderr = os.Stderr
			// Per-terminal isolation: only this subshell sees the cluster.
			child.Env = append(os.Environ(),
				"KUBECONFIG="+st.Path(name),
				"AKS_HELPER_CLUSTER="+name,
			)

			fmt.Fprintf(cmd.ErrOrStderr(), "aks: entering subshell for %q (type 'exit' to leave)\n", name)
			err = child.Run()
			fmt.Fprintf(cmd.ErrOrStderr(), "aks: left subshell for %q\n", name)
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			return err
		},
	}

	cmd.Flags().StringVar(&shellPath, "shell", "", "shell to launch (default: $SHELL, else an OS default)")
	return cmd
}

// resolveShell picks the shell binary to spawn: an explicit override, then
// $SHELL, then a sensible per-OS default. It returns the resolved path.
func resolveShell(override string) (string, error) {
	candidates := []string{}
	if override != "" {
		candidates = append(candidates, override)
	}
	if sh := os.Getenv("SHELL"); sh != "" {
		candidates = append(candidates, sh)
	}
	if runtime.GOOS == "windows" {
		candidates = append(candidates, "pwsh.exe", "powershell.exe", "cmd.exe")
	} else {
		candidates = append(candidates, "/bin/bash", "/bin/sh")
	}
	for _, c := range candidates {
		if p, err := exec.LookPath(c); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("could not find a shell to launch (set --shell or $SHELL)")
}
