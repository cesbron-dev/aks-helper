package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cesbron-dev/aks-helper/internal/azure"
	"github.com/cesbron-dev/aks-helper/internal/config"
	"github.com/cesbron-dev/aks-helper/internal/importer"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// importStep tracks where the import wizard is.
type importStep int

const (
	stepLoadingSubs importStep = iota
	stepPickSub
	stepLoadingClusters
	stepPickClusters
	stepRunning
	stepError
)

const importLoginMode = "azurecli"

// import wizard messages.
type (
	impSubsMsg struct {
		subs []azure.Subscription
		err  error
	}
	impClustersMsg struct {
		clusters []azure.Cluster
		err      error
	}
	impDoneMsg struct {
		names []string
		err   error
	}
)

// importModel is the interactive "import from Azure" wizard shown inside the UI.
// The pick steps are filterable (type to narrow, fzf-style) and scrollable, so
// they stay usable with hundreds of subscriptions or clusters.
type importModel struct {
	store   *config.Store
	spinner spinner.Model
	filter  textinput.Model

	step importStep
	err  string

	subs      []azure.Subscription
	chosenSub azure.Subscription
	clusters  []azure.Cluster
	selected  map[int]bool // keyed by index into clusters (stable across filtering)

	cursor int // position within the currently filtered list
	height int

	// done signals the parent to leave the wizard; status is shown afterwards.
	done   bool
	status string
}

func newImportModel(st *config.Store, sp spinner.Model) importModel {
	fi := textinput.New()
	fi.Prompt = ""
	fi.Placeholder = "type to filter"
	fi.Focus()
	return importModel{store: st, spinner: sp, filter: fi, step: stepLoadingSubs, selected: map[int]bool{}}
}

// init kicks off subscription loading plus the spinner and filter cursor.
func (im importModel) init() tea.Cmd {
	return tea.Batch(im.spinner.Tick, textinput.Blink, loadSubsCmd())
}

func loadSubsCmd() tea.Cmd {
	return func() tea.Msg {
		az, err := azure.New()
		if err != nil {
			return impSubsMsg{err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		subs, err := az.Subscriptions(ctx)
		return impSubsMsg{subs: subs, err: err}
	}
}

func loadClustersCmd(subID string) tea.Cmd {
	return func() tea.Msg {
		az, err := azure.New()
		if err != nil {
			return impClustersMsg{err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		clusters, err := az.Clusters(ctx, subID)
		return impClustersMsg{clusters: clusters, err: err}
	}
}

func runImportCmd(st *config.Store, sub azure.Subscription, clusters []azure.Cluster) tea.Cmd {
	return func() tea.Msg {
		az, err := azure.New()
		if err != nil {
			return impDoneMsg{err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		var names []string
		for _, cl := range clusters {
			name := importName(sub, cl)
			if err := importer.Import(ctx, az, st, sub, cl, name, importLoginMode, false); err != nil {
				return impDoneMsg{names: names, err: fmt.Errorf("%s: %w", cl.Name, err)}
			}
			names = append(names, name)
		}
		return impDoneMsg{names: names}
	}
}

// importName mirrors the default name template used by the sync command.
func importName(_ azure.Subscription, cl azure.Cluster) string {
	name := strings.ReplaceAll(cl.Name, " ", "-")
	return strings.ReplaceAll(name, "/", "-")
}

func (im importModel) update(msg tea.Msg) (importModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		im.height = msg.Height
		return im, nil

	case spinner.TickMsg:
		if im.step != stepLoadingSubs && im.step != stepLoadingClusters && im.step != stepRunning {
			return im, nil
		}
		var cmd tea.Cmd
		im.spinner, cmd = im.spinner.Update(msg)
		return im, cmd

	case impSubsMsg:
		if msg.err != nil {
			im.step, im.err = stepError, firstLine(msg.err.Error())
			return im, nil
		}
		if len(msg.subs) == 0 {
			im.step, im.err = stepError, "no subscriptions found for this account"
			return im, nil
		}
		im.subs, im.step = msg.subs, stepPickSub
		im.cursor = 0
		im.filter.SetValue("")
		return im, nil

	case impClustersMsg:
		if msg.err != nil {
			im.step, im.err = stepError, firstLine(msg.err.Error())
			return im, nil
		}
		if len(msg.clusters) == 0 {
			im.step, im.err = stepError, "no AKS clusters in "+im.chosenSub.Name
			return im, nil
		}
		im.clusters, im.step = msg.clusters, stepPickClusters
		im.cursor, im.selected = 0, map[int]bool{}
		im.filter.SetValue("")
		return im, nil

	case impDoneMsg:
		im.done = true
		switch {
		case msg.err != nil:
			im.status = "import failed: " + firstLine(msg.err.Error())
		case len(msg.names) == 1:
			im.status = "imported " + msg.names[0]
		default:
			im.status = fmt.Sprintf("imported %d clusters", len(msg.names))
		}
		return im, nil

	case tea.KeyMsg:
		return im.updateKey(msg)
	}

	// Forward anything else (e.g. cursor blink) to the filter on pick steps.
	if im.step == stepPickSub || im.step == stepPickClusters {
		var cmd tea.Cmd
		im.filter, cmd = im.filter.Update(msg)
		return im, cmd
	}
	return im, nil
}

func (im importModel) updateKey(msg tea.KeyMsg) (importModel, tea.Cmd) {
	switch im.step {
	case stepPickSub, stepPickClusters:
		return im.updatePick(msg)
	case stepError:
		im.done, im.status = true, "import: "+im.err
		return im, nil
	default: // loading / running
		if msg.String() == "esc" || msg.String() == "ctrl+c" {
			if im.step == stepRunning {
				return im, nil // don't abandon an in-flight import
			}
			im.done, im.status = true, "import cancelled"
		}
		return im, nil
	}
}

func (im importModel) updatePick(msg tea.KeyMsg) (importModel, tea.Cmd) {
	multi := im.step == stepPickClusters
	idx := im.filteredIdx()

	switch msg.String() {
	case "esc", "ctrl+c":
		im.done, im.status = true, "import cancelled"
		return im, nil
	case "up", "ctrl+p":
		if im.cursor > 0 {
			im.cursor--
		}
		return im, nil
	case "down", "ctrl+n":
		if im.cursor < len(idx)-1 {
			im.cursor++
		}
		return im, nil
	case "tab":
		if multi && len(idx) > 0 {
			j := idx[im.cursor]
			im.selected[j] = !im.selected[j]
		}
		return im, nil
	case "ctrl+a":
		if multi {
			im.toggleAll(idx)
		}
		return im, nil
	case "enter":
		return im.confirmPick(idx)
	}

	// Any other key edits the filter; keep the cursor in range.
	var cmd tea.Cmd
	im.filter, cmd = im.filter.Update(msg)
	if n := len(im.filteredIdx()); im.cursor >= n {
		im.cursor = maxInt(0, n-1)
	}
	return im, cmd
}

func (im importModel) confirmPick(idx []int) (importModel, tea.Cmd) {
	if len(idx) == 0 {
		return im, nil
	}
	if im.step == stepPickSub {
		im.chosenSub = im.subs[idx[im.cursor]]
		im.step = stepLoadingClusters
		im.filter.SetValue("")
		im.cursor = 0
		return im, tea.Batch(im.spinner.Tick, loadClustersCmd(im.chosenSub.ID))
	}
	chosen := im.chosenClusters()
	if len(chosen) == 0 {
		chosen = []azure.Cluster{im.clusters[idx[im.cursor]]}
	}
	im.step = stepRunning
	return im, tea.Batch(im.spinner.Tick, runImportCmd(im.store, im.chosenSub, chosen))
}

// filteredIdx returns the indices of the items matching the filter for the
// current pick step, in their original order.
func (im importModel) filteredIdx() []int {
	q := strings.ToLower(strings.TrimSpace(im.filter.Value()))
	var out []int
	if im.step == stepPickSub {
		for i, s := range im.subs {
			if q == "" || strings.Contains(strings.ToLower(s.Name), q) || strings.Contains(strings.ToLower(s.ID), q) {
				out = append(out, i)
			}
		}
		return out
	}
	for i, c := range im.clusters {
		if q == "" ||
			strings.Contains(strings.ToLower(c.Name), q) ||
			strings.Contains(strings.ToLower(c.ResourceGroup), q) ||
			strings.Contains(strings.ToLower(c.Location), q) {
			out = append(out, i)
		}
	}
	return out
}

func (im *importModel) toggleAll(idx []int) {
	allSelected := true
	for _, j := range idx {
		if !im.selected[j] {
			allSelected = false
			break
		}
	}
	for _, j := range idx {
		im.selected[j] = !allSelected
	}
}

func (im importModel) chosenClusters() []azure.Cluster {
	var out []azure.Cluster
	for i, cl := range im.clusters {
		if im.selected[i] {
			out = append(out, cl)
		}
	}
	return out
}

func (im importModel) visibleHeight() int {
	h := im.height - 8 // title, header, help, margins
	if h < 5 {
		return 12
	}
	return h
}

func (im importModel) view() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(" aks-helper  •  import from Azure "))
	b.WriteString("\n\n")

	switch im.step {
	case stepLoadingSubs:
		b.WriteString("  " + im.spinner.View() + " loading subscriptions…\n")
	case stepPickSub:
		idx := im.filteredIdx()
		b.WriteString(im.pickHeader("Select a subscription", len(im.subs), len(idx)))
		items := make([]string, len(idx))
		for i, j := range idx {
			s := im.subs[j]
			items[i] = s.Name + dimStyle.Render("  ["+s.ID+"]")
		}
		im.renderRows(&b, items, nil, false)
		b.WriteString("\n" + helpStyle.Render("  ↑/↓ move  •  type to filter  •  enter select  •  esc cancel"))
	case stepLoadingClusters:
		b.WriteString("  " + im.spinner.View() + " loading clusters in " + im.chosenSub.Name + "…\n")
	case stepPickClusters:
		idx := im.filteredIdx()
		b.WriteString(im.pickHeader("Clusters in "+im.chosenSub.Name, len(im.clusters), len(idx)))
		items := make([]string, len(idx))
		checked := make([]bool, len(idx))
		for i, j := range idx {
			c := im.clusters[j]
			items[i] = c.Name + dimStyle.Render(fmt.Sprintf("  (rg=%s, v%s, %s)", c.ResourceGroup, c.K8sVersion, c.Location))
			checked[i] = im.selected[j]
		}
		im.renderRows(&b, items, checked, true)
		b.WriteString("\n" + helpStyle.Render("  ↑/↓ move  •  type to filter  •  tab toggle  •  ctrl+a all  •  enter import  •  esc cancel"))
	case stepRunning:
		b.WriteString("  " + im.spinner.View() + " importing… (fetching credentials, converting with kubelogin)\n")
	case stepError:
		b.WriteString(warnStyle.Render("  "+im.err) + "\n\n")
		b.WriteString(helpStyle.Render("  press any key to go back"))
	}
	return b.String()
}

func (im importModel) pickHeader(label string, total, shown int) string {
	count := fmt.Sprintf("%d", total)
	if shown != total {
		count = fmt.Sprintf("%d/%d", shown, total)
	}
	return fmt.Sprintf("  %s (%s)\n  %s %s\n\n",
		dimStyle.Render(label), dimStyle.Render(count),
		dimStyle.Render("filter:"), im.filter.View())
}

// renderRows renders the visible window of items around the cursor, with
// "more" indicators when the list is scrolled.
func (im importModel) renderRows(b *strings.Builder, items []string, checked []bool, multi bool) {
	n := len(items)
	if n == 0 {
		b.WriteString(dimStyle.Render("  (no matches)") + "\n")
		return
	}
	h := im.visibleHeight()
	start := 0
	if im.cursor >= h {
		start = im.cursor - h + 1
	}
	end := start + h
	if end > n {
		end = n
	}
	if start > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  ↑ %d more", start)) + "\n")
	}
	for i := start; i < end; i++ {
		ck := false
		if checked != nil {
			ck = checked[i]
		}
		b.WriteString(renderChoice(i == im.cursor, multi, ck, items[i]))
	}
	if end < n {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  ↓ %d more", n-end)) + "\n")
	}
}

// renderChoice renders one selectable line with a cursor and, for multi-select
// lists, a checkbox.
func renderChoice(cursor, multi, checked bool, label string) string {
	point := "    "
	if cursor {
		point = "  " + keyStyle.Render("▸") + " "
	}
	box := ""
	if multi {
		if checked {
			box = statusStyle.Render("[x] ")
		} else {
			box = "[ ] "
		}
	}
	return point + box + label + "\n"
}
