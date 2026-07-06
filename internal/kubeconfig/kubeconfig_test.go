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

const multiContext = `apiVersion: v1
kind: Config
current-context: second
clusters:
- name: first
  cluster:
    server: https://first:443
- name: second
  cluster:
    server: https://second:443
contexts:
- name: first
  context:
    cluster: first
    user: user-first
- name: second
  context:
    cluster: second
    user: user-second
users:
- name: user-first
  user: {}
- name: user-second
  user: {}
`

func TestRenameMultiContext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	if err := writeFile(path, multiContext); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// The context named by current-context is renamed; the other is untouched.
	cfg.Rename("friendly")
	if cfg.CurrentContext != "friendly" {
		t.Errorf("CurrentContext = %q", cfg.CurrentContext)
	}
	if cfg.Contexts[0].Name != "first" || cfg.Contexts[1].Name != "friendly" {
		t.Errorf("contexts = %v", cfg.ContextNames())
	}

	// current-context must always name an existing context.
	found := false
	for _, n := range cfg.ContextNames() {
		if n == cfg.CurrentContext {
			found = true
		}
	}
	if !found {
		t.Errorf("current-context %q points at no context (%v)", cfg.CurrentContext, cfg.ContextNames())
	}
}

func TestRenameNoMatchingCurrentContext(t *testing.T) {
	cfg := &Config{
		CurrentContext: "missing",
		Contexts: []NamedContext{
			{Name: "a"}, {Name: "b"},
		},
	}
	cfg.Rename("friendly")
	if cfg.CurrentContext != "missing" {
		t.Errorf("CurrentContext changed to %q", cfg.CurrentContext)
	}
	if cfg.Contexts[0].Name != "a" || cfg.Contexts[1].Name != "b" {
		t.Errorf("contexts renamed: %v", cfg.ContextNames())
	}
}
