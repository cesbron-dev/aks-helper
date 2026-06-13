package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/cesbron-dev/aks-helper/internal/azure"
	"github.com/cesbron-dev/aks-helper/internal/config"
	"github.com/cesbron-dev/aks-helper/internal/kubeconfig"
	"github.com/cesbron-dev/aks-helper/internal/ui"
	"github.com/spf13/cobra"
)

// clusterState is the outcome of checking a stored cluster against Azure.
type clusterState string

const (
	stateOK      clusterState = "ok"      // exists and the stored credentials still match
	stateStale   clusterState = "stale"   // exists but was recreated/rotated — creds outdated
	stateGone    clusterState = "gone"    // no longer exists in Azure
	stateUnknown clusterState = "unknown" // not enough metadata to check
)

func newCleanupCmd() *cobra.Command {
	var (
		prune   bool
		refresh bool
		yes     bool
	)

	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Check stored clusters against Azure and remove or refresh stale ones",
		Long: `Verifies every stored cluster against Azure:

  ok       exists and the stored kubeconfig still matches
  stale    exists but was recreated or rotated (same name, new API server) —
           the stored credentials are outdated
  gone     no longer exists in Azure
  unknown  imported without metadata, so it can't be checked

By default 'cleanup' only reports. Use --prune to remove 'gone' clusters and
--refresh to re-fetch credentials for 'stale' ones.`,
		Example: `  aks-helper cleanup            # report only
  aks-helper cleanup --prune    # also remove deleted clusters
  aks-helper cleanup --prune --refresh --yes`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			st, err := store()
			if err != nil {
				return err
			}
			az, err := azure.New()
			if err != nil {
				return err
			}
			entries, err := st.List()
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				fmt.Fprintln(os.Stderr, "no clusters stored — nothing to check")
				return nil
			}

			var gone, stale []config.Entry
			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME\tSTATE\tDETAIL")
			for _, e := range entries {
				state, detail := checkEntry(ctx, az, st, e)
				fmt.Fprintf(tw, "%s\t%s\t%s\n", e.Name, state, detail)
				switch state {
				case stateGone:
					gone = append(gone, e)
				case stateStale:
					stale = append(stale, e)
				}
			}
			tw.Flush()

			if len(gone) == 0 && len(stale) == 0 {
				fmt.Fprintln(os.Stderr, "\neverything is up to date")
				return nil
			}

			if len(gone) > 0 {
				if prune {
					if yes || ui.Confirm(fmt.Sprintf("\nRemove %d deleted cluster(s)?", len(gone)), false) {
						for _, e := range gone {
							if err := st.Remove(e.Name); err != nil {
								fmt.Fprintf(os.Stderr, "warning: removing %s: %v\n", e.Name, err)
								continue
							}
							fmt.Printf("removed %s\n", e.Name)
						}
					}
				} else {
					fmt.Fprintf(os.Stderr, "\n%d cluster(s) gone — run with --prune to remove them.\n", len(gone))
				}
			}

			if len(stale) > 0 {
				if refresh {
					if yes || ui.Confirm(fmt.Sprintf("Refresh credentials for %d cluster(s)?", len(stale)), false) {
						for _, e := range stale {
							if err := refreshEntry(ctx, az, st, e); err != nil {
								fmt.Fprintf(os.Stderr, "warning: refreshing %s: %v\n", e.Name, err)
								continue
							}
							fmt.Printf("refreshed %s\n", e.Name)
						}
					}
				} else {
					fmt.Fprintf(os.Stderr, "%d cluster(s) stale — run with --refresh to update their credentials.\n", len(stale))
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&prune, "prune", false, "remove clusters that no longer exist in Azure")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "re-fetch credentials for recreated/rotated clusters")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "do not ask for confirmation")
	return cmd
}

func checkEntry(ctx context.Context, az *azure.Client, st *config.Store, e config.Entry) (clusterState, string) {
	if e.SubscriptionID == "" || e.ResourceGroup == "" || e.ClusterName == "" {
		return stateUnknown, "no metadata (imported manually?)"
	}
	detail, exists, err := az.Show(ctx, e.SubscriptionID, e.ResourceGroup, e.ClusterName)
	if err != nil {
		return stateUnknown, "check failed: " + firstLine(err.Error())
	}
	if !exists {
		return stateGone, fmt.Sprintf("%s/%s not found", e.ResourceGroup, e.ClusterName)
	}
	cfg, err := kubeconfig.Load(st.Path(e.Name))
	if err != nil {
		return stateUnknown, "unreadable kubeconfig: " + firstLine(err.Error())
	}
	if isStale(cfg.Server(), detail.Fqdn, detail.PrivateFQDN) {
		return stateStale, "recreated or rotated (API server changed)"
	}
	return stateOK, fmt.Sprintf("%s, %s", strings.TrimSpace(detail.PowerState.Code), detail.ProvisioningState)
}

// isStale reports whether the stored API server URL no longer matches the
// cluster's current FQDN(s). It is conservative: when no FQDN is known it
// returns false rather than flag a healthy cluster.
func isStale(storedServer, fqdn, privateFqdn string) bool {
	if storedServer == "" {
		return false
	}
	server := strings.ToLower(storedServer)
	for _, f := range []string{fqdn, privateFqdn} {
		if f == "" {
			continue
		}
		if strings.Contains(server, strings.ToLower(f)) {
			return false // matches a current FQDN -> not stale
		}
	}
	// Only declare stale when we had at least one FQDN to compare against.
	return fqdn != "" || privateFqdn != ""
}

// refreshEntry re-imports a cluster's credentials in place, reusing its stored
// metadata and login mode.
func refreshEntry(ctx context.Context, az *azure.Client, st *config.Store, e config.Entry) error {
	sub := azure.Subscription{ID: e.SubscriptionID, Name: e.Subscription}
	cl := azure.Cluster{Name: e.ClusterName, ResourceGroup: e.ResourceGroup}
	admin := e.LoginMode == "admin"
	loginMode := e.LoginMode
	if loginMode == "" || admin {
		loginMode = "azurecli"
	}
	return importCluster(ctx, az, st, sub, cl, e.Name, loginMode, admin)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
