package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/cesbron-dev/aks-helper/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

func testModel(t *testing.T) model {
	t.Helper()
	st, err := config.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"prod", "staging"} {
		if err := os.WriteFile(st.Path(n), []byte("apiVersion: v1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		_ = st.Save(config.Entry{Name: n, Subscription: "Sub", ResourceGroup: "rg-" + n, LoginMode: "azurecli"})
	}
	_ = st.SetCurrent("prod")

	m, err := newModel(Options{
		Store:        st,
		Self:         "aks-helper",
		ResolveShell: func() (string, error) { return "/bin/sh", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return nm.(model)
}

func TestModelRenders(t *testing.T) {
	m := testModel(t)
	out := m.View()
	for _, want := range []string{"prod", "staging", "cluster(s)", "shell", "filter"} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing %q\n%s", want, out)
		}
	}
}

func TestFilterNarrowsRows(t *testing.T) {
	m := testModel(t)
	if got := len(m.table.Rows()); got != 2 {
		t.Fatalf("expected 2 rows, got %d", got)
	}
	m.filter.SetValue("stag")
	m.applyRows()
	if got := len(m.table.Rows()); got != 1 {
		t.Fatalf("filter: expected 1 row, got %d", got)
	}
	if name := m.selectedName(); name != "staging" {
		t.Errorf("selectedName = %q", name)
	}
}

func TestEmptyStoreView(t *testing.T) {
	st, _ := config.New(t.TempDir())
	m, _ := newModel(Options{Store: st, Self: "aks-helper", ResolveShell: func() (string, error) { return "/bin/sh", nil }})
	out := m.View()
	if !strings.Contains(out, "No clusters stored") {
		t.Errorf("empty view unexpected:\n%s", out)
	}
}

func TestStateCell(t *testing.T) {
	st, err := config.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(st.Path("prod"), []byte("apiVersion: v1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = st.Save(config.Entry{Name: "prod", SubscriptionID: "sub-id", Subscription: "Sub", ResourceGroup: "rg", ClusterName: "prod", LoginMode: "azurecli"})

	m, err := newModel(Options{Store: st, Self: "aks-helper", ResolveShell: func() (string, error) { return "/bin/sh", nil }})
	if err != nil {
		t.Fatal(err)
	}
	e := m.entries[0]

	// Before any fetch: the loading cell shows the Braille spinner frame.
	if got, _ := m.stateCell(e); got != m.spinner.View() {
		t.Errorf("loading: got %q, want spinner frame %q", got, m.spinner.View())
	}

	// Running with a version.
	m.statusLoaded = true
	m.statuses = map[string]statusInfo{"prod": {code: "Running", version: "1.29.2", exists: true}}
	if got, ver := m.stateCell(e); !strings.Contains(got, "run") || ver != "1.29.2" {
		t.Errorf("running: state=%q version=%q", got, ver)
	}

	// Deleted cluster.
	m.statuses = map[string]statusInfo{"prod": {exists: false}}
	if got, _ := m.stateCell(e); !strings.Contains(got, "gone") {
		t.Errorf("gone: got %q", got)
	}

	// Entry without metadata is reported as n/a.
	if got, _ := m.stateCell(config.Entry{Name: "manual"}); !strings.Contains(got, "n/a") {
		t.Errorf("n/a: got %q", got)
	}
}
