package tui

import (
	"strings"
	"testing"

	"github.com/cesbron-dev/aks-helper/internal/azure"
	"github.com/cesbron-dev/aks-helper/internal/config"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func newTestImport(t *testing.T) importModel {
	t.Helper()
	st, err := config.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return newImportModel(st, spinner.New())
}

func TestImportWizardFlow(t *testing.T) {
	im := newTestImport(t)
	if im.step != stepLoadingSubs {
		t.Fatalf("initial step = %v", im.step)
	}

	// Subscriptions arrive.
	im, _ = im.update(impSubsMsg{subs: []azure.Subscription{{ID: "s1", Name: "A"}, {ID: "s2", Name: "B"}}})
	if im.step != stepPickSub {
		t.Fatalf("after subs: step = %v", im.step)
	}

	// Move to the second subscription and select it.
	im, _ = im.update(tea.KeyMsg{Type: tea.KeyDown})
	im, _ = im.update(tea.KeyMsg{Type: tea.KeyEnter})
	if im.step != stepLoadingClusters || im.chosenSub.ID != "s2" {
		t.Fatalf("after sub select: step=%v sub=%q", im.step, im.chosenSub.ID)
	}

	// Clusters arrive.
	im, _ = im.update(impClustersMsg{clusters: []azure.Cluster{{Name: "c1"}, {Name: "c2"}}})
	if im.step != stepPickClusters {
		t.Fatalf("after clusters: step = %v", im.step)
	}

	// Toggle the first cluster, then confirm.
	im, _ = im.update(tea.KeyMsg{Type: tea.KeyTab})
	if !im.selected[0] {
		t.Fatal("tab should select the cluster under the cursor")
	}
	im, _ = im.update(tea.KeyMsg{Type: tea.KeyEnter})
	if im.step != stepRunning {
		t.Fatalf("after confirm: step = %v", im.step)
	}

	// Import finishes.
	im, _ = im.update(impDoneMsg{names: []string{"c1"}})
	if !im.done || !strings.Contains(im.status, "imported c1") {
		t.Fatalf("done=%v status=%q", im.done, im.status)
	}
}

func TestImportSelectAllToggle(t *testing.T) {
	im := newTestImport(t)
	im, _ = im.update(impClustersMsg{clusters: []azure.Cluster{{Name: "c1"}, {Name: "c2"}}})
	// ctrl+a selects all, ctrl+a again clears.
	im, _ = im.update(tea.KeyMsg{Type: tea.KeyCtrlA})
	if len(im.chosenClusters()) != 2 {
		t.Fatalf("expected all selected, got %d", len(im.chosenClusters()))
	}
	im, _ = im.update(tea.KeyMsg{Type: tea.KeyCtrlA})
	if len(im.chosenClusters()) != 0 {
		t.Fatalf("expected none selected, got %d", len(im.chosenClusters()))
	}
}

func TestImportFilter(t *testing.T) {
	im := newTestImport(t)
	im, _ = im.update(impSubsMsg{subs: []azure.Subscription{
		{ID: "s1", Name: "prod-eu"},
		{ID: "s2", Name: "prod-us"},
		{ID: "s3", Name: "staging"},
	}})
	if got := len(im.filteredIdx()); got != 3 {
		t.Fatalf("unfiltered: %d", got)
	}

	// Type "prod" to narrow to two subscriptions.
	for _, r := range "prod" {
		im, _ = im.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	idx := im.filteredIdx()
	if len(idx) != 2 {
		t.Fatalf("filtered: expected 2, got %d", len(idx))
	}

	// Selecting the first match resolves through the filtered index.
	im, _ = im.update(tea.KeyMsg{Type: tea.KeyEnter})
	if im.chosenSub.Name != "prod-eu" {
		t.Errorf("chosen sub = %q", im.chosenSub.Name)
	}
}

func TestImportCancelAndError(t *testing.T) {
	im := newTestImport(t)
	im, _ = im.update(impSubsMsg{subs: []azure.Subscription{{ID: "s1", Name: "A"}}})
	im, _ = im.update(tea.KeyMsg{Type: tea.KeyEsc})
	if !im.done || !strings.Contains(im.status, "cancelled") {
		t.Fatalf("cancel: done=%v status=%q", im.done, im.status)
	}

	im2 := newTestImport(t)
	im2, _ = im2.update(impSubsMsg{err: errString("not logged in")})
	if im2.step != stepError || !strings.Contains(im2.err, "not logged in") {
		t.Fatalf("error: step=%v err=%q", im2.step, im2.err)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
