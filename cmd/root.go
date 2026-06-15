package cmd

import (
	"fmt"
	"os"

	"github.com/cesbron-dev/aks-helper/internal/config"
	"github.com/spf13/cobra"
)

// version is overridden at build time via -ldflags "-X .../cmd.version=...".
var version = "dev"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "aks-helper",
		Short: "Manage connections to Azure Kubernetes Service (AKS) clusters",
		Long: `aks-helper imports AKS cluster credentials from Azure and lets you switch
between them with a single command.

Each cluster is stored as a standalone kubeconfig under ~/.kube/aks. The 'use'
command points KUBECONFIG at the selected one so kubectl, k9s, helm and friends
just work.

Typical workflow:

  aks-helper sync          # pick subscriptions + clusters to import (interactive)
  eval "$(aks-helper shell-init bash)"   # once, in your shell rc
  aks use                  # fuzzy-pick a cluster for the current shell
  kubectl get nodes`,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		newSyncCmd(),
		newUseCmd(),
		newUICmd(),
		newListCmd(),
		newCurrentCmd(),
		newRemoveCmd(),
		newCleanupCmd(),
		newExecCmd(),
		newPathCmd(),
		newShellCmd(),
		newShellInitCmd(),
	)
	return root
}

// Execute is the entry point used by main.
func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// store is a small helper used by every command.
func store() (*config.Store, error) {
	return config.Default()
}
