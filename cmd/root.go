package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/cesbron-dev/aks-helper/internal/config"
	"github.com/spf13/cobra"
)

// version is overridden at build time via -ldflags "-X .../cmd.version=...".
var version = "dev"

// resolveVersion falls back to the module version recorded by `go install`
// when no ldflags override was provided.
func resolveVersion() string {
	if version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return version
}

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
		Version:       resolveVersion(),
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
		newSkillCmd(),
		newShellInitCmd(),
	)
	return root
}

// Execute is the entry point used by main. Ctrl+C / SIGTERM cancel the command
// context, which aborts any in-flight az subprocess.
func Execute() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// store is a small helper used by every command.
func store() (*config.Store, error) {
	return config.Default()
}
