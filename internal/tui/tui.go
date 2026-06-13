// Package tui implements an interactive, k9s-style terminal UI for browsing and
// acting on the stored AKS clusters.
package tui

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

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
	ResolveShell func() (string, error) // shell to spawn on "use"
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
)

type reloadMsg struct{}
type execDoneMsg struct {
	action string
	err    error
}

type model struct {
	opts    Options
	entries []config.Entry
	current string

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

	m := model{opts: opts, table: t, filter: fi}
	m.reload()
	return m, nil
}

func columns(width int) []table.Column {
	// Distribute the remaining width across the text columns.
	rest := width - 4 - 6 - 8 // marker + login + padding budget
	if rest < 40 {
		rest = 60
	}
	name := rest / 3
	sub := rest / 3
	rg := rest - name - sub
	return []table.Column{
		{Title: "", Width: 2},
		{Title: "NAME", Width: name},
		{Title: "SUBSCRIPTION", Width: sub},
		{Title: "RESOURCE GROUP", Width: rg},
		{Title: "LOGIN", Width: 10},
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
		rows = append(rows, table.Row{marker, e.Name, dash(e.Subscription), dash(e.ResourceGroup), dash(e.LoginMode)})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i][1] < rows[j][1] })
	m.table.SetRows(rows)
	if m.table.Cursor() >= len(rows) {
		m.table.SetCursor(maxInt(0, len(rows)-1))
	}
}

func matches(e config.Entry, q string) bool {
	return strings.Contains(strings.ToLower(e.Name), q) ||
		strings.Contains(strings.ToLower(e.Subscription), q) ||
		strings.Contains(strings.ToLower(e.ResourceGroup), q)
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.table.SetColumns(columns(msg.Width))
		m.table.SetHeight(maxInt(3, msg.Height-6))
		m.applyRows()
		return m, nil

	case execDoneMsg:
		if msg.err != nil {
			m.setErr(fmt.Sprintf("%s: %v", msg.action, msg.err))
		} else {
			m.setOK(msg.action + " done")
		}
		m.reload()
		return m, nil

	case reloadMsg:
		m.reload()
		return m, nil

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
		m.setOK("reloaded")
		return m, nil
	case "enter", "s":
		return m.useSelected()
	case "d":
		if name := m.selectedName(); name != "" {
			m.confirm = name
			m.status = ""
		}
		return m, nil
	case "S":
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

func (m model) useSelected() (tea.Model, tea.Cmd) {
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
	c.Env = append(os.Environ(),
		"KUBECONFIG="+m.opts.Store.Path(name),
		"AKS_HELPER_CLUSTER="+name,
	)
	return m, tea.ExecProcess(c, func(err error) tea.Msg {
		return execDoneMsg{action: "shell (" + name + ")", err: err}
	})
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
	if len(row) < 2 {
		return ""
	}
	return row[1]
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
		b.WriteString(keyStyle.Render("S"))
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
	default:
		b.WriteString(dimStyle.Render("↑/↓ move  •  type / to filter"))
	}
	b.WriteString("\n")
	b.WriteString(m.helpView())
	return b.String()
}

func (m model) helpView() string {
	pairs := [][2]string{
		{"enter/s", "shell"},
		{"d", "delete"},
		{"S", "sync"},
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
