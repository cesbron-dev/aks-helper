// Package azure wraps the external `az` and `kubelogin` CLIs.
//
// aks-helper intentionally shells out to the official tools rather than talking
// to the Azure REST API directly: it reuses the user's existing `az login`
// session and keeps behaviour identical to the scripts users already trust.
package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Subscription is a subset of `az account list -o json`.
type Subscription struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
	State     string `json:"state"`
	TenantID  string `json:"tenantId"`
}

// Cluster is a subset of `az aks list -o json`.
type Cluster struct {
	Name          string `json:"name"`
	ResourceGroup string `json:"resourceGroup"`
	Location      string `json:"location"`
	K8sVersion    string `json:"kubernetesVersion"`
	PowerState    struct {
		Code string `json:"code"`
	} `json:"powerState"`
}

// Client runs az / kubelogin commands.
type Client struct {
	AzPath        string
	KubeloginPath string
}

// New locates the az and kubelogin binaries on PATH.
func New() (*Client, error) {
	az, err := exec.LookPath("az")
	if err != nil {
		return nil, fmt.Errorf("the Azure CLI ('az') was not found on PATH: %w", err)
	}
	// kubelogin is only required for the conversion step; resolve it lazily so
	// read-only commands still work without it installed.
	kubelogin, _ := exec.LookPath("kubelogin")
	return &Client{AzPath: az, KubeloginPath: kubelogin}, nil
}

func (c *Client) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), msg)
	}
	return stdout.Bytes(), nil
}

// Subscriptions lists the enabled subscriptions for the logged-in account.
func (c *Client) Subscriptions(ctx context.Context) ([]Subscription, error) {
	out, err := c.run(ctx, c.AzPath, "account", "list", "--all", "--output", "json")
	if err != nil {
		if strings.Contains(err.Error(), "az login") || strings.Contains(err.Error(), "AADSTS") {
			return nil, fmt.Errorf("not logged in to Azure — run 'az login' first")
		}
		return nil, err
	}
	var subs []Subscription
	if err := json.Unmarshal(out, &subs); err != nil {
		return nil, fmt.Errorf("decoding subscriptions: %w", err)
	}
	enabled := subs[:0]
	for _, s := range subs {
		if s.State == "" || strings.EqualFold(s.State, "Enabled") {
			enabled = append(enabled, s)
		}
	}
	return enabled, nil
}

// Clusters lists the AKS clusters in a subscription.
func (c *Client) Clusters(ctx context.Context, subscriptionID string) ([]Cluster, error) {
	out, err := c.run(ctx, c.AzPath, "aks", "list", "--subscription", subscriptionID, "--output", "json")
	if err != nil {
		return nil, err
	}
	var clusters []Cluster
	if err := json.Unmarshal(out, &clusters); err != nil {
		return nil, fmt.Errorf("decoding clusters: %w", err)
	}
	return clusters, nil
}

// GetCredentials writes a standalone kubeconfig for a single cluster to dest.
func (c *Client) GetCredentials(ctx context.Context, subscriptionID, resourceGroup, name, dest string, admin bool) error {
	_ = os.Remove(dest)
	args := []string{
		"aks", "get-credentials",
		"--subscription", subscriptionID,
		"--resource-group", resourceGroup,
		"--name", name,
		"--file", dest,
		"--overwrite-existing",
	}
	if admin {
		args = append(args, "--admin")
	}
	_, err := c.run(ctx, c.AzPath, args...)
	return err
}

// ClusterDetail is a subset of `az aks show -o json`.
type ClusterDetail struct {
	Name              string `json:"name"`
	ResourceGroup     string `json:"resourceGroup"`
	Fqdn              string `json:"fqdn"`
	PrivateFQDN       string `json:"privateFqdn"`
	ProvisioningState string `json:"provisioningState"`
	PowerState        struct {
		Code string `json:"code"`
	} `json:"powerState"`
}

// Show returns details for a single cluster. The boolean is false (with a nil
// error) when the cluster does not exist, so callers can distinguish a deleted
// cluster from a transient failure.
func (c *Client) Show(ctx context.Context, subscriptionID, resourceGroup, name string) (*ClusterDetail, bool, error) {
	out, err := c.run(ctx, c.AzPath,
		"aks", "show",
		"--subscription", subscriptionID,
		"--resource-group", resourceGroup,
		"--name", name,
		"--output", "json",
	)
	if err != nil {
		if isNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var d ClusterDetail
	if err := json.Unmarshal(out, &d); err != nil {
		return nil, false, fmt.Errorf("decoding cluster: %w", err)
	}
	return &d, true, nil
}

// isNotFound reports whether an az error indicates a missing cluster or resource
// group (as opposed to auth or network problems).
func isNotFound(err error) bool {
	msg := strings.ToLower(err.Error())
	for _, s := range []string{"resourcenotfound", "resourcegroupnotfound", "was not found", "could not be found", "not found"} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

// ConvertKubeconfig rewrites the exec credential plugin in dest to use kubelogin
// with the given login mode (azurecli, devicecode, interactive, spn, ...).
//
// azurecli is the friendliest default: it reuses the existing `az` session and
// is fully non-interactive, which matters for automation and coding agents.
func (c *Client) ConvertKubeconfig(ctx context.Context, dest, loginMode string) error {
	if c.KubeloginPath == "" {
		return fmt.Errorf("'kubelogin' was not found on PATH — install it to use login mode %q (Azure AD clusters)", loginMode)
	}
	_, err := c.run(ctx, c.KubeloginPath,
		"convert-kubeconfig",
		"--kubeconfig", dest,
		"--login", loginMode,
	)
	return err
}

// HasKubelogin reports whether kubelogin is available.
func (c *Client) HasKubelogin() bool { return c.KubeloginPath != "" }
