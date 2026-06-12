package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cesbron-dev/aks-helper/internal/azure"
	"github.com/cesbron-dev/aks-helper/internal/config"
	"github.com/cesbron-dev/aks-helper/internal/kubeconfig"
	"github.com/cesbron-dev/aks-helper/internal/ui"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	var (
		subFilter   string
		clusterName string
		loginMode   string
		admin       bool
		all         bool
		nameTmpl    string
	)

	cmd := &cobra.Command{
		Use:     "sync",
		Aliases: []string{"get-cred", "get-credentials", "creds", "import"},
		Short:   "Import AKS cluster credentials from Azure",
		Long: `Lists your Azure subscriptions and AKS clusters, then stores a kubeconfig for
each selected cluster under ~/.kube/aks.

Interactive (default): fuzzy-pick one or more subscriptions, then one or more
clusters (Tab toggles, Enter confirms).

Non-interactive (for automation / coding agents): pass --subscription and
optionally --cluster, or --all to import every cluster in the matched
subscriptions without prompting.`,
		Example: `  aks-helper sync
  aks-helper sync --subscription "Prod" --all
  aks-helper sync --subscription 00000000-0000-0000-0000-000000000000 --cluster my-cluster`,
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

			subs, err := selectSubscriptions(ctx, az, subFilter, all || clusterName != "")
			if err != nil {
				return err
			}
			if len(subs) == 0 {
				return fmt.Errorf("no subscriptions selected")
			}

			imported := 0
			for _, sub := range subs {
				clusters, err := az.Clusters(ctx, sub.ID)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: listing clusters in %q: %v\n", sub.Name, err)
					continue
				}
				if clusterName != "" {
					clusters = filterClusters(clusters, clusterName)
				}
				if len(clusters) == 0 {
					if clusterName != "" {
						fmt.Fprintf(os.Stderr, "no cluster matching %q in %q\n", clusterName, sub.Name)
					}
					continue
				}

				chosen := clusters
				if !all && clusterName == "" {
					chosen, err = pickClusters(clusters, sub.Name)
					if err != nil {
						return err
					}
				}

				for _, cl := range chosen {
					name := renderName(nameTmpl, sub, cl)
					if err := importCluster(ctx, az, st, sub, cl, name, loginMode, admin); err != nil {
						fmt.Fprintf(os.Stderr, "warning: importing %s: %v\n", name, err)
						continue
					}
					fmt.Printf("imported %s\n", name)
					imported++
				}
			}

			if imported == 0 {
				return fmt.Errorf("no clusters imported")
			}
			fmt.Fprintf(os.Stderr, "\n%d cluster(s) ready. Run 'aks-helper use' to select one.\n", imported)
			return nil
		},
	}

	cmd.Flags().StringVarP(&subFilter, "subscription", "s", "", "subscription name or id (substring match); skips the subscription prompt")
	cmd.Flags().StringVarP(&clusterName, "cluster", "c", "", "cluster name (substring match); skips the cluster prompt")
	cmd.Flags().StringVarP(&loginMode, "login", "l", "azurecli", "kubelogin login mode: azurecli, devicecode, interactive, spn, msi, workloadidentity")
	cmd.Flags().BoolVar(&admin, "admin", false, "use admin (local) credentials instead of Azure AD")
	cmd.Flags().BoolVar(&all, "all", false, "import every cluster without prompting")
	cmd.Flags().StringVar(&nameTmpl, "name-template", "{cluster}", "stored name template; placeholders: {cluster} {rg} {sub}")
	return cmd
}

func selectSubscriptions(ctx context.Context, az *azure.Client, filter string, nonInteractive bool) ([]azure.Subscription, error) {
	subs, err := az.Subscriptions(ctx)
	if err != nil {
		return nil, err
	}
	if len(subs) == 0 {
		return nil, fmt.Errorf("no subscriptions found for this account")
	}
	if filter != "" {
		var matched []azure.Subscription
		for _, s := range subs {
			if strings.EqualFold(s.ID, filter) || strings.Contains(strings.ToLower(s.Name), strings.ToLower(filter)) {
				matched = append(matched, s)
			}
		}
		if len(matched) == 0 {
			return nil, fmt.Errorf("no subscription matching %q", filter)
		}
		return matched, nil
	}
	if nonInteractive {
		return subs, nil
	}
	idxs, err := ui.SelectMulti(subs, "subscriptions>", func(s azure.Subscription) string {
		def := ""
		if s.IsDefault {
			def = " (default)"
		}
		return fmt.Sprintf("%s%s  [%s]", s.Name, def, s.ID)
	})
	if err != nil {
		return nil, err
	}
	out := make([]azure.Subscription, len(idxs))
	for i, idx := range idxs {
		out[i] = subs[idx]
	}
	return out, nil
}

func pickClusters(clusters []azure.Cluster, subName string) ([]azure.Cluster, error) {
	idxs, err := ui.SelectMulti(clusters, "clusters in "+subName+">", func(c azure.Cluster) string {
		state := strings.TrimSpace(c.PowerState.Code)
		if state == "" {
			state = "?"
		}
		return fmt.Sprintf("%s  (rg=%s, v%s, %s, %s)", c.Name, c.ResourceGroup, c.K8sVersion, c.Location, state)
	})
	if err != nil {
		return nil, err
	}
	out := make([]azure.Cluster, len(idxs))
	for i, idx := range idxs {
		out[i] = clusters[idx]
	}
	return out, nil
}

func filterClusters(clusters []azure.Cluster, name string) []azure.Cluster {
	var out []azure.Cluster
	for _, c := range clusters {
		if strings.EqualFold(c.Name, name) || strings.Contains(strings.ToLower(c.Name), strings.ToLower(name)) {
			out = append(out, c)
		}
	}
	return out
}

func importCluster(ctx context.Context, az *azure.Client, st *config.Store, sub azure.Subscription, cl azure.Cluster, name, loginMode string, admin bool) error {
	dest := st.Path(name)
	if err := az.GetCredentials(ctx, sub.ID, cl.ResourceGroup, cl.Name, dest, admin); err != nil {
		return err
	}
	// Admin credentials are certificate-based and need no kubelogin conversion.
	if !admin {
		if err := az.ConvertKubeconfig(ctx, dest, loginMode); err != nil {
			return err
		}
	}
	cfg, err := kubeconfig.Load(dest)
	if err != nil {
		return err
	}
	cfg.Rename(name)
	if err := cfg.Save(dest); err != nil {
		return err
	}
	mode := loginMode
	if admin {
		mode = "admin"
	}
	return st.Save(config.Entry{
		Name:           name,
		SubscriptionID: sub.ID,
		Subscription:   sub.Name,
		ResourceGroup:  cl.ResourceGroup,
		ClusterName:    cl.Name,
		LoginMode:      mode,
	})
}

func renderName(tmpl string, sub azure.Subscription, cl azure.Cluster) string {
	r := strings.NewReplacer(
		"{cluster}", cl.Name,
		"{rg}", cl.ResourceGroup,
		"{sub}", sub.Name,
	)
	name := r.Replace(tmpl)
	// Keep the name filesystem- and KUBECONFIG-friendly.
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "/", "-")
	return name
}
