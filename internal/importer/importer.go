// Package importer holds the shared logic to import a single AKS cluster's
// credentials into the local store, used by both the `sync`/`cleanup` commands
// and the interactive TUI.
package importer

import (
	"context"

	"github.com/cesbron-dev/aks-helper/internal/azure"
	"github.com/cesbron-dev/aks-helper/internal/config"
	"github.com/cesbron-dev/aks-helper/internal/kubeconfig"
)

// Import fetches credentials for a cluster, converts them with kubelogin (unless
// admin), stores a standalone kubeconfig under the given name and records the
// metadata in the store's index.
func Import(ctx context.Context, az *azure.Client, st *config.Store, sub azure.Subscription, cl azure.Cluster, name, loginMode string, admin bool) error {
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
