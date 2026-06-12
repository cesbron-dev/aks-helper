package kubeconfig

import (
	"path/filepath"
	"testing"
)

const sample = `apiVersion: v1
kind: Config
current-context: az-generated-name
clusters:
- name: az-generated-name
  cluster:
    server: https://example:443
contexts:
- name: az-generated-name
  context:
    cluster: az-generated-name
    user: clusterUser_rg_cluster
users:
- name: clusterUser_rg_cluster
  user:
    token: secret
`

func TestLoadRenameSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	if err := writeFile(path, sample); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.ContextNames(); len(got) != 1 || got[0] != "az-generated-name" {
		t.Fatalf("ContextNames = %v", got)
	}

	cfg.Rename("friendly")
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.CurrentContext != "friendly" {
		t.Errorf("CurrentContext = %q", reloaded.CurrentContext)
	}
	if reloaded.Contexts[0].Name != "friendly" {
		t.Errorf("context name = %q", reloaded.Contexts[0].Name)
	}
	// The user reference inside the context must be preserved so auth keeps working.
	if reloaded.Contexts[0].Context.User != "clusterUser_rg_cluster" {
		t.Errorf("user ref lost: %q", reloaded.Contexts[0].Context.User)
	}
	if len(reloaded.Users) != 1 {
		t.Errorf("users lost: %+v", reloaded.Users)
	}
}
