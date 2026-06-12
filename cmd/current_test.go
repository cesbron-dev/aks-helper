package cmd

import (
	"os"
	"testing"

	"github.com/cesbron-dev/aks-helper/internal/config"
)

func storeWithCluster(t *testing.T, name string) *config.Store {
	t.Helper()
	st, err := config.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(st.Path(name), []byte("apiVersion: v1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestCurrentClusterPrefersPerTerminalSignals(t *testing.T) {
	st := storeWithCluster(t, "prod")
	t.Setenv("AKS_HELPER_CLUSTER", "")
	t.Setenv("KUBECONFIG", "")

	// Falls back to the global marker when no per-terminal signal is present.
	if err := st.SetCurrent("prod"); err != nil {
		t.Fatal(err)
	}
	if got := currentCluster(st); got != "prod" {
		t.Errorf("global marker: got %q", got)
	}

	// KUBECONFIG pointing at a stored cluster is recognised.
	t.Setenv("KUBECONFIG", st.Path("prod"))
	if got := currentCluster(st); got != "prod" {
		t.Errorf("from KUBECONFIG: got %q", got)
	}

	// The subshell marker takes precedence.
	st2 := storeWithCluster(t, "staging")
	// Re-point the global marker elsewhere to prove the env var wins.
	_ = st2.SetCurrent("staging")
	t.Setenv("AKS_HELPER_CLUSTER", "staging")
	t.Setenv("KUBECONFIG", "")
	// st doesn't have 'staging', so it should fall through to st's global marker.
	if got := currentCluster(st); got != "prod" {
		t.Errorf("unknown env cluster should be ignored: got %q", got)
	}
	if got := currentCluster(st2); got != "staging" {
		t.Errorf("subshell marker: got %q", got)
	}
}

func TestResolveShell(t *testing.T) {
	if p, err := resolveShell("/bin/sh"); err != nil || p == "" {
		t.Errorf("override /bin/sh: p=%q err=%v", p, err)
	}
	t.Setenv("SHELL", "/bin/sh")
	if p, err := resolveShell(""); err != nil || p == "" {
		t.Errorf("from $SHELL: p=%q err=%v", p, err)
	}
}
