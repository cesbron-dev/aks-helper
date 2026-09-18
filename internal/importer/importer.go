// Package importer holds the shared logic to import a single AKS cluster's
// credentials into the local store, used by both the `sync`/`cleanup` commands
// and the interactive TUI.
package importer

import (
	"context"
	"os"

	"github.com/cesbron-dev/aks-helper/internal/azure"
	"github.com/cesbron-dev/aks-helper/internal/config"
	"github.com/cesbron-dev/aks-helper/internal/hooks"
	"github.com/cesbron-dev/aks-helper/internal/kubeconfig"
)

// Import fetches credentials for a cluster, converts them with kubelogin (unless
// admin), runs the post-import hook if one is configured, stores a standalone
// kubeconfig under the given name and records the metadata in the store's index.
//
// When the hook fails the freshly written kubeconfig is removed and the import
// reports failure, so a cluster is never stored with credentials the
// site-specific step (e.g. a proxy URL rewrite) could not process.
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

	if _, err := hooks.Run(ctx, st.Dir, hooks.Event{
		Name:           hooks.PostImport,
		KubeconfigPath: dest,
		Env: map[string]string{
			"AKS_HELPER_STORE_DIR":         st.Dir,
			"AKS_HELPER_CLUSTER":           name,
			"AKS_HELPER_AZURE_CLUSTER":     cl.Name,
			"AKS_HELPER_RESOURCE_GROUP":    cl.ResourceGroup,
			"AKS_HELPER_SUBSCRIPTION_ID":   sub.ID,
			"AKS_HELPER_SUBSCRIPTION_NAME": sub.Name,
			"AKS_HELPER_LOGIN_MODE":        mode,
		},
	}); err != nil {
		_ = os.Remove(dest)
		return err
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
