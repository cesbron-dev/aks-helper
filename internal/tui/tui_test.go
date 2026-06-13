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
