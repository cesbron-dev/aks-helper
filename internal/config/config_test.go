package config

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return st
}

func TestSaveListGet(t *testing.T) {
	st := newTestStore(t)
	// A kubeconfig file must exist for List to surface the entry.
	if err := os.WriteFile(st.Path("prod"), []byte("apiVersion: v1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := st.Save(Entry{Name: "prod", Subscription: "Sub", ResourceGroup: "rg", ClusterName: "prod"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	entries, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "prod" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
	if entries[0].UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set by Save")
	}

	e, ok, err := st.Get("prod")
	if err != nil || !ok {
		t.Fatalf("Get: ok=%v err=%v", ok, err)
	}
	if e.Subscription != "Sub" {
		t.Errorf("Subscription = %q", e.Subscription)
	}
}

func TestListIncludesUnindexedFiles(t *testing.T) {
	st := newTestStore(t)
	if err := os.WriteFile(st.Path("manual"), []byte("apiVersion: v1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	names, err := st.Names()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "manual" {
		t.Fatalf("expected [manual], got %v", names)
	}
}

func TestCurrentRoundTrip(t *testing.T) {
	st := newTestStore(t)
	if cur, _ := st.Current(); cur != "" {
		t.Fatalf("expected empty current, got %q", cur)
	}
	if err := st.SetCurrent("dev"); err != nil {
		t.Fatal(err)
	}
	if cur, _ := st.Current(); cur != "dev" {
		t.Fatalf("Current = %q", cur)
	}
}

func TestRemoveClearsCurrent(t *testing.T) {
	st := newTestStore(t)
	if err := os.WriteFile(st.Path("dev"), []byte("apiVersion: v1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = st.Save(Entry{Name: "dev"})
	_ = st.SetCurrent("dev")

	if err := st.Remove("dev"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if st.Exists("dev") {
		t.Error("kubeconfig should be gone")
	}
	if cur, _ := st.Current(); cur != "" {
		t.Errorf("current should be cleared, got %q", cur)
	}
	if _, err := os.Stat(filepath.Join(st.Dir, "dev.yaml")); !os.IsNotExist(err) {
		t.Error("file should not exist")
	}
}
