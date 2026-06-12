package cmd

import (
	"testing"

	"github.com/cesbron-dev/aks-helper/internal/azure"
)

func TestRenderName(t *testing.T) {
	sub := azure.Subscription{Name: "Prod Sub"}
	cl := azure.Cluster{Name: "my-cluster", ResourceGroup: "rg-1"}

	cases := map[string]string{
		"{cluster}":       "my-cluster",
		"{sub}-{cluster}": "Prod-Sub-my-cluster",
		"{rg}/{cluster}":  "rg-1-my-cluster",
	}
	for tmpl, want := range cases {
		if got := renderName(tmpl, sub, cl); got != want {
			t.Errorf("renderName(%q) = %q, want %q", tmpl, got, want)
		}
	}
}

func TestFilterClusters(t *testing.T) {
	clusters := []azure.Cluster{
		{Name: "prod-eu"},
		{Name: "prod-us"},
		{Name: "staging"},
	}
	if got := filterClusters(clusters, "prod"); len(got) != 2 {
		t.Errorf("expected 2 prod clusters, got %d", len(got))
	}
	if got := filterClusters(clusters, "STAGING"); len(got) != 1 {
		t.Errorf("case-insensitive exact match failed: %d", len(got))
	}
	if got := filterClusters(clusters, "none"); len(got) != 0 {
		t.Errorf("expected 0, got %d", len(got))
	}
}
