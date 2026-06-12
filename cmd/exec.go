package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func newExecCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "exec [-c name] -- command [args...]",
		Short: "Run a command with KUBECONFIG set to a stored cluster",
		Long: `Runs an arbitrary command (kubectl, k9s, helm, ...) with KUBECONFIG pointing at
the chosen cluster, without modifying the current shell.

This is the recommended way to use aks-helper from scripts and coding agents,
since it needs no shell integration:

  aks-helper exec --cluster prod -- kubectl get nodes
  aks-helper exec -- kubectl get pods -A   # uses the currently selected cluster`,
		Args:               cobra.MinimumNArgs(1),
		DisableFlagParsing: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := store()
			if err != nil {
				return err
			}
			if name == "" {
				name, err = st.Current()
				if err != nil {
					return err
				}
				if name == "" {
					return fmt.Errorf("no cluster selected and --cluster not given (run 'aks-helper use' or pass --cluster)")
				}
			}
			if !st.Exists(name) {
				return fmt.Errorf("no stored cluster named %q", name)
			}

			bin, err := exec.LookPath(args[0])
			if err != nil {
				return fmt.Errorf("command not found: %s", args[0])
			}
			sub := exec.Command(bin, args[1:]...)
			sub.Stdin = os.Stdin
			sub.Stdout = os.Stdout
			sub.Stderr = os.Stderr
			sub.Env = append(os.Environ(), "KUBECONFIG="+st.Path(name))
			if err := sub.Run(); err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					os.Exit(exitErr.ExitCode())
				}
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&name, "cluster", "c", "", "stored cluster name (defaults to the current selection)")
	return cmd
}
