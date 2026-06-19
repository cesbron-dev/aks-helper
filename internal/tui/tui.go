// Package tui implements an interactive, k9s-style terminal UI for browsing and
// acting on the stored AKS clusters.
package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cesbron-dev/aks-helper/internal/azure"
	"github.com/cesbron-dev/aks-helper/internal/config"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Options configures the UI. Behaviour that lives in the cmd layer (resolving a
// shell, the path to re-exec for sync/cleanup) is injected so this package stays
// free of import cycles.
type Options struct {
	Store        *config.Store
	Self         string                 // path to the aks-helper binary, for sync/cleanup
	ResolveShell func() (string, error) // shell to spawn on "shell"
}

// Run starts the TUI and blocks until the user quits.
func Run(opts Options) error {
	m, err := newModel(opts)
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("231")).
			Background(lipgloss.Color("63")).Padding(0, 1)
	keyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true)
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	runningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	stoppedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	goneStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

type reloadMsg struct{}
type execDoneMsg struct {
	action string
	err    error
}

// statusInfo is the live Azure state for one cluster.
type statusInfo struct {
	code    string // PowerState, "" if gone/unknown
	version string
	exists  bool
}

type statusesMsg struct {
	statuses map[string]statusInfo
	azErr    error
}

type model struct {
	opts    Options
	entries []config.Entry
	current string

	statuses     map[string]statusInfo
	statusLoaded bool
	azNote       string

	table  table.Model
	filter textinput.Model

	filtering bool
	confirm   string // non-empty => awaiting y/n for deleting this cluster
	status    string
	statusErr bool
	width     int
	height    int
}

func newModel(opts Options) (model, error) {
	fi := textinput.New()
	fi.Prompt = "/"
	fi.Placeholder = "filter"

	t := table.New(
		table.WithColumns(columns(0)),
		table.WithFocused(true),
	)
	t.SetStyles(tableStyles())

	m := model{opts: opts, table: t, filter: fi, statuses: map[string]statusInfo{}}
	m.reload()
	return m, nil
}

func columns(width int) []table.Column {
	rest := width - 2 - 8 - 10 - 11 // marker + state + version + login + padding
	if rest < 45 {
		rest = 66
	}
	name := rest / 3
	sub := rest / 3
	rg := rest - name - sub
	return []table.Column{
		{Title: "", Width: 2},
		{Title: "STATE", Width: 8},
		{Title: "NAME", Width: name},
		{Title: "VERSION", Width: 9},
		{Title: "SUBSCRIPTION", Width: sub},
		{Title: "RESOURCE GROUP", Width: rg},
		{Title: "LOGIN", Width: 9},
	}
}

func tableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("63")).BorderBottom(true).Bold(true)
	s.Selected = s.Selected.Foreground(lipgloss.Color("231")).
		Background(lipgloss.Color("63")).Bold(true)
	return s
}

func (m *model) reload() {
	entries, err := m.opts.Store.List()
	if err != nil {
		m.setErr(err.Error())
		return
	}
	m.current, _ = m.opts.Store.Current()
	m.entries = entries
	m.applyRows()
}

// applyRows rebuilds the visible rows, honouring the active filter.
func (m *model) applyRows() {
	q := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	var rows []table.Row
	for _, e := range m.entries {
		if q != "" && !matches(e, q) {
			continue
		}
		marker := " "
		if e.Name == m.current {
			marker = "●"
		}
		state, version := m.stateCell(e)
		rows = append(rows, table.Row{marker, state, e.Name, version, dash(e.Subscription), dash(e.ResourceGroup), dash(e.LoginMode)})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i][2] < rows[j][2] })
	m.table.SetRows(rows)
	if m.table.Cursor() >= len(rows) {
		m.table.SetCursor(maxInt(0, len(rows)-1))
	}
}

// stateCell returns the state icon/label and the version for an entry, based on
// whatever live status has been fetched so far.
func (m *model) stateCell(e config.Entry) (string, string) {
	if e.SubscriptionID == "" || e.ResourceGroup == "" || e.ClusterName == "" {
		return dimStyle.Render("— n/a"), ""
	}
	info, ok := m.statuses[e.Name]
	if !ok {
		if m.statusLoaded {
			return dimStyle.Render("? unk"), "" // checked but no result (e.g. az missing)
		}
		return dimStyle.Render("…"), ""
	}
	if !info.exists {
		return goneStyle.Render("✖ gone"), ""
	}
	v := info.version
	switch {
	case strings.EqualFold(info.code, "Running"):
		return runningStyle.Render("● run"), v
	case strings.HasPrefix(info.code, "Stop"), strings.HasPrefix(info.code, "Dealloc"):
		return stoppedStyle.Render("○ stop"), v
	case info.code == "":
		return runningStyle.Render("● ok"), v
	default:
		return stoppedStyle.Render("○ " + strings.ToLower(info.code)), v
	}
}

func matches(e config.Entry, q string) bool {
	return strings.Contains(strings.ToLower(e.Name), q) ||
		strings.Contains(strings.ToLower(e.Subscription), q) ||
		strings.Contains(strings.ToLower(e.ResourceGroup), q)
}

func (m model) Init() tea.Cmd { return m.fetchStatusesCmd() }

// fetchStatusesCmd queries Azure for the live state of every entry that has
// enough metadata, concurrently and with a timeout, then delivers them at once.
func (m model) fetchStatusesCmd() tea.Cmd {
	entries := m.entries
	return func() tea.Msg {
		az, err := azure.New()
		if err != nil {
			return statusesMsg{azErr: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()

		out := make(map[string]statusInfo)
		var mu sync.Mutex
		var wg sync.WaitGroup
		sem := make(chan struct{}, 6)
		for _, e := range entries {
			if e.SubscriptionID == "" || e.ResourceGroup == "" || e.ClusterName == "" {
				continue
			}
			wg.Add(1)
			go func(e config.Entry) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				d, exists, err := az.Show(ctx, e.SubscriptionID, e.ResourceGroup, e.ClusterName)
				if err != nil {
					return // leave unknown
				}
				mu.Lock()
				if exists {
					out[e.Name] = statusInfo{code: d.PowerState.Code, version: d.KubernetesVersion, exists: true}
				} else {
					out[e.Name] = statusInfo{exists: false}
				}
				mu.Unlock()
			}(e)
		}
		wg.Wait()
		return statusesMsg{statuses: out}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.table.SetColumns(columns(msg.Width))
		m.table.SetHeight(maxInt(3, msg.Height-6))
		m.applyRows()
		return m, nil

	case statusesMsg:
		m.statusLoaded = true
		if msg.azErr != nil {
			m.azNote = "cluster state unavailable: " + firstLine(msg.azErr.Error())
		} else {
			m.statuses = msg.statuses
		}
		m.applyRows()
		return m, nil

	case execDoneMsg:
		if msg.err != nil {
			m.setErr(fmt.Sprintf("%s: %v", msg.action, msg.err))
		} else {
			m.setOK(msg.action + " done")
		}
		m.reload()
		return m, m.fetchStatusesCmd()

	case reloadMsg:
		m.reload()
		return m, m.fetchStatusesCmd()

	case tea.KeyMsg:
		if m.filtering {
			return m.updateFiltering(msg)
		}
		if m.confirm != "" {
			return m.updateConfirm(msg)
		}
		return m.updateNormal(msg)
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "/":
		m.filtering = true
		m.filter.Focus()
		return m, textinput.Blink
	case "r":
		m.reload()
		m.setOK("reloading…")
		return m, m.fetchStatusesCmd()
	case "enter", "k":
		return m.launchK9s()
	case "s":
		return m.openShell()
	case "d":
		if name := m.selectedName(); name != "" {
			m.confirm = name
			m.status = ""
		}
		return m, nil
	case "i":
		return m, m.execSelf("sync", "import")
	case "c":
		return m, m.execSelf("cleanup", "cleanup")
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) updateFiltering(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "esc":
		m.filtering = false
		m.filter.Blur()
		if msg.String() == "esc" {
			m.filter.SetValue("")
			m.applyRows()
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	m.applyRows()
	return m, cmd
}

func (m model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		name := m.confirm
		m.confirm = ""
		if err := m.opts.Store.Remove(name); err != nil {
			m.setErr(err.Error())
		} else {
			m.setOK("removed " + name)
			m.reload()
		}
		return m, nil
	default:
		m.confirm = ""
		return m, nil
	}
}

// launchK9s opens k9s scoped to the highlighted cluster.
func (m model) launchK9s() (tea.Model, tea.Cmd) {
	name := m.selectedName()
	if name == "" {
		return m, nil
	}
	bin, err := exec.LookPath("k9s")
	if err != nil {
		m.setErr("k9s not found on PATH — install it from https://k9scli.io")
		return m, nil
	}
	c := exec.Command(bin)
	c.Env = clusterEnv(m.opts.Store, name)
	return m, tea.ExecProcess(c, func(err error) tea.Msg {
		return execDoneMsg{action: "k9s (" + name + ")", err: err}
	})
}

// openShell drops into a subshell scoped to the highlighted cluster.
func (m model) openShell() (tea.Model, tea.Cmd) {
	name := m.selectedName()
	if name == "" {
		return m, nil
	}
	sh, err := m.opts.ResolveShell()
	if err != nil {
		m.setErr(err.Error())
		return m, nil
	}
	c := exec.Command(sh)
	c.Env = clusterEnv(m.opts.Store, name)
	return m, tea.ExecProcess(c, func(err error) tea.Msg {
		return execDoneMsg{action: "shell (" + name + ")", err: err}
	})
}

func clusterEnv(st *config.Store, name string) []string {
	return append(os.Environ(),
		"KUBECONFIG="+st.Path(name),
		"AKS_HELPER_CLUSTER="+name,
	)
}

// execSelf re-runs the aks-helper binary for an interactive subcommand (sync,
// cleanup), suspending the TUI while it runs.
func (m model) execSelf(sub, label string) tea.Cmd {
	c := exec.Command(m.opts.Self, sub)
	c.Env = os.Environ()
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return execDoneMsg{action: label, err: err}
	})
}

func (m model) selectedName() string {
	row := m.table.SelectedRow()
	if len(row) < 3 {
		return ""
	}
	return row[2]
}

func (m *model) setOK(s string)  { m.status, m.statusErr = s, false }
func (m *model) setErr(s string) { m.status, m.statusErr = s, true }

func (m model) View() string {
	var b strings.Builder

	count := len(m.entries)
	b.WriteString(titleStyle.Render(fmt.Sprintf(" aks-helper  •  %d cluster(s) ", count)))
	b.WriteString("\n\n")

	if count == 0 {
		b.WriteString(dimStyle.Render("  No clusters stored yet.\n\n"))
		b.WriteString("  Press ")
		b.WriteString(keyStyle.Render("i"))
		b.WriteString(" to import from Azure, or ")
		b.WriteString(keyStyle.Render("q"))
		b.WriteString(" to quit.\n")
		return b.String()
	}

	b.WriteString(m.table.View())
	b.WriteString("\n")

	switch {
	case m.confirm != "":
		b.WriteString(warnStyle.Render(fmt.Sprintf("Remove %q? (y/N)", m.confirm)))
	case m.filtering:
		b.WriteString(m.filter.View())
	case m.status != "":
		st := statusStyle
		if m.statusErr {
			st = warnStyle
		}
		b.WriteString(st.Render(m.status))
	case m.azNote != "":
		b.WriteString(dimStyle.Render(m.azNote))
	default:
		b.WriteString(dimStyle.Render("↑/↓ move  •  / filter"))
	}
	b.WriteString("\n")
	b.WriteString(m.helpView())
	return b.String()
}

func (m model) helpView() string {
	pairs := [][2]string{
		{"enter/k", "k9s"},
		{"s", "shell"},
		{"d", "delete"},
		{"i", "import"},
		{"c", "cleanup"},
		{"r", "reload"},
		{"/", "filter"},
		{"q", "quit"},
	}
	var parts []string
	for _, p := range pairs {
		parts = append(parts, keyStyle.Render(p[0])+" "+helpStyle.Render(p[1]))
	}
	return helpStyle.Render("  ") + strings.Join(parts, helpStyle.Render("  •  "))
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
