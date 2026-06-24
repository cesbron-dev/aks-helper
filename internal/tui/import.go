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
type importModel struct {
	store   *config.Store
	spinner spinner.Model

	step importStep
	err  string

	subs      []azure.Subscription
	subCursor int
	chosenSub azure.Subscription

	clusters []azure.Cluster
	clCursor int
	selected map[int]bool

	// done signals the parent to leave the wizard; status is shown afterwards.
	done   bool
	status string
}

func newImportModel(st *config.Store, sp spinner.Model) importModel {
	return importModel{store: st, spinner: sp, step: stepLoadingSubs, selected: map[int]bool{}}
}

// init returns the command that kicks off subscription loading plus the spinner.
func (im importModel) init() tea.Cmd {
	return tea.Batch(im.spinner.Tick, loadSubsCmd())
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
		im.clCursor, im.selected = 0, map[int]bool{}
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
	return im, nil
}

func (im importModel) updateKey(msg tea.KeyMsg) (importModel, tea.Cmd) {
	key := msg.String()
	// Esc / q / ctrl+c cancels from any non-running step.
	if key == "esc" || key == "ctrl+c" || (key == "q" && im.step != stepPickClusters) {
		if im.step == stepRunning {
			return im, nil
		}
		im.done, im.status = true, "import cancelled"
		return im, nil
	}

	switch im.step {
	case stepPickSub:
		switch key {
		case "up", "k":
			if im.subCursor > 0 {
				im.subCursor--
			}
		case "down", "j":
			if im.subCursor < len(im.subs)-1 {
				im.subCursor++
			}
		case "enter":
			im.chosenSub = im.subs[im.subCursor]
			im.step = stepLoadingClusters
			return im, tea.Batch(im.spinner.Tick, loadClustersCmd(im.chosenSub.ID))
		}
	case stepPickClusters:
		switch key {
		case "up", "k":
			if im.clCursor > 0 {
				im.clCursor--
			}
		case "down", "j":
			if im.clCursor < len(im.clusters)-1 {
				im.clCursor++
			}
		case " ":
			im.selected[im.clCursor] = !im.selected[im.clCursor]
		case "a":
			all := len(im.selected) < len(im.clusters)
			im.selected = map[int]bool{}
			if all {
				for i := range im.clusters {
					im.selected[i] = true
				}
			}
		case "enter":
			chosen := im.chosenClusters()
			if len(chosen) == 0 {
				chosen = []azure.Cluster{im.clusters[im.clCursor]}
			}
			im.step = stepRunning
			return im, tea.Batch(im.spinner.Tick, runImportCmd(im.store, im.chosenSub, chosen))
		}
	case stepError:
		// Any key returns to the main list.
		im.done, im.status = true, "import: "+im.err
	}
	return im, nil
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

func (im importModel) view() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(" aks-helper  •  import from Azure "))
	b.WriteString("\n\n")

	switch im.step {
	case stepLoadingSubs:
		b.WriteString("  " + im.spinner.View() + " loading subscriptions…\n")
	case stepPickSub:
		b.WriteString(dimStyle.Render("  Select a subscription:") + "\n\n")
		for i, s := range im.subs {
			b.WriteString(renderChoice(i == im.subCursor, false, false, s.Name+dimStyle.Render("  ["+s.ID+"]")))
		}
		b.WriteString("\n" + helpStyle.Render("  ↑/↓ move  •  enter select  •  esc cancel"))
	case stepLoadingClusters:
		b.WriteString("  " + im.spinner.View() + " loading clusters in " + im.chosenSub.Name + "…\n")
	case stepPickClusters:
		b.WriteString(dimStyle.Render("  Select clusters in "+im.chosenSub.Name+":") + "\n\n")
		for i, c := range im.clusters {
			label := c.Name + dimStyle.Render(fmt.Sprintf("  (rg=%s, v%s, %s)", c.ResourceGroup, c.K8sVersion, c.Location))
			b.WriteString(renderChoice(i == im.clCursor, true, im.selected[i], label))
		}
		b.WriteString("\n" + helpStyle.Render("  ↑/↓ move  •  space toggle  •  a all  •  enter import  •  esc cancel"))
	case stepRunning:
		b.WriteString("  " + im.spinner.View() + " importing… (fetching credentials, converting with kubelogin)\n")
	case stepError:
		b.WriteString(warnStyle.Render("  "+im.err) + "\n\n")
		b.WriteString(helpStyle.Render("  press any key to go back"))
	}
	return b.String()
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
