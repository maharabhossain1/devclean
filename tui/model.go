package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/maharabhossain1/devclean/cleaner"
	"github.com/maharabhossain1/devclean/engine"
)

type viewState int

const (
	stateList    viewState = iota
	stateConfirm           // Yellow: explicit "yes" required
	stateTypeName          // Red: must type item name
	stateDone
)

// Item wraps a Trace with selection state.
type Item struct {
	Trace    engine.Trace
	Selected bool
}

type Model struct {
	items    []Item
	cursor   int
	state    viewState
	input    string   // text input for Red confirmation
	session  *cleaner.Session
	home     string
	err      error
	deleted  int
	freed    int64
	width    int
	height   int
	autoMode bool // --auto flag skips interactive selection
}

// New builds a Model from a scan result.
func New(result *engine.ScanResult, home string, autoMode bool) Model {
	var items []Item
	for _, t := range result.Orphans {
		items = append(items, Item{Trace: t})
	}
	for _, t := range result.DeadAgents {
		items = append(items, Item{Trace: t})
	}
	for _, t := range result.DevCaches {
		items = append(items, Item{Trace: t})
	}
	// Unknowns are included but never auto-selected
	for _, t := range result.Unknowns {
		items = append(items, Item{Trace: t})
	}

	m := Model{
		items:    items,
		session:  cleaner.NewSession(home),
		home:     home,
		autoMode: autoMode,
	}

	if autoMode {
		// Pre-select all Green items
		for i := range m.items {
			if m.items[i].Trace.Risk == engine.RiskGreen {
				m.items[i].Selected = true
			}
		}
	}

	return m
}

// --- tea.Model interface ---

func (m Model) Init() tea.Cmd {
	if m.autoMode {
		return tea.Batch(tea.WindowSize(), deleteSelected(&m))
	}
	return tea.WindowSize()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		return m.handleKey(msg)

	case deleteDoneMsg:
		m.deleted = msg.count
		m.freed = msg.freed
		m.err = msg.err
		_ = m.session.Save()
		m.state = stateDone
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateList:
		return m.handleListKey(msg)
	case stateConfirm:
		return m.handleConfirmKey(msg)
	case stateTypeName:
		return m.handleTypeNameKey(msg)
	}
	// stateDone
	if key.Matches(msg, keys.Quit) || msg.String() == "q" || msg.String() == "enter" {
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}

	case key.Matches(msg, keys.Down):
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}

	case key.Matches(msg, keys.Select):
		if len(m.items) > 0 {
			m.items[m.cursor].Selected = !m.items[m.cursor].Selected
		}

	case key.Matches(msg, keys.SelectAll):
		// Toggle all Green items
		allGreenSelected := true
		for _, it := range m.items {
			if it.Trace.Risk == engine.RiskGreen && !it.Selected {
				allGreenSelected = false
				break
			}
		}
		for i := range m.items {
			if m.items[i].Trace.Risk == engine.RiskGreen {
				m.items[i].Selected = !allGreenSelected
			}
		}

	case key.Matches(msg, keys.Delete):
		return m.startDelete()
	}
	return m, nil
}

func (m Model) startDelete() (Model, tea.Cmd) {
	selected := m.selectedItems()
	if len(selected) == 0 {
		return m, nil
	}

	// Check if any Yellow or Red selected — require confirmation
	hasYellow, hasRed := false, false
	for _, it := range selected {
		switch it.Trace.Risk {
		case engine.RiskYellow:
			hasYellow = true
		case engine.RiskRed:
			hasRed = true
		}
	}

	if hasRed {
		m.state = stateTypeName
		m.input = ""
		return m, nil
	}
	if hasYellow {
		m.state = stateConfirm
		return m, nil
	}

	// All Green — single confirmation via enter
	m.state = stateConfirm
	return m, nil
}

func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		return m, deleteSelected(&m)
	case "n", "N", "q", "esc":
		m.state = stateList
	}
	return m, nil
}

func (m Model) handleTypeNameKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Check if any selected Red item's name matches the input
		for _, it := range m.selectedItems() {
			if it.Trace.Risk == engine.RiskRed {
				name := itemName(it.Trace.Path)
				if strings.TrimSpace(m.input) != name {
					m.input = ""
					return m, nil
				}
			}
		}
		return m, deleteSelected(&m)
	case "esc", "ctrl+c":
		m.state = stateList
		m.input = ""
	case "backspace":
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.input += msg.String()
		}
	}
	return m, nil
}

func (m *Model) selectedItems() []Item {
	var out []Item
	for _, it := range m.items {
		if it.Selected {
			out = append(out, it)
		}
	}
	return out
}

// --- async deletion ---

type deleteDoneMsg struct {
	count int
	freed int64
	err   error
}

func deleteSelected(m *Model) tea.Cmd {
	items := m.selectedItems()
	session := m.session
	return func() tea.Msg {
		var freed int64
		for _, it := range items {
			size := it.Trace.SizeBytes
			if err := session.Remove(it.Trace.Path, size); err != nil {
				return deleteDoneMsg{err: err}
			}
			freed += size
		}
		return deleteDoneMsg{count: len(items), freed: freed}
	}
}

// --- helpers ---

func itemName(path string) string {
	home, _ := os.UserHomeDir()
	if strings.HasPrefix(path, home) {
		path = "~" + path[len(home):]
	}
	return path
}

func shortenPath(path string) string {
	home, _ := os.UserHomeDir()
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

func fmtSize(bytes int64) string {
	if bytes == 0 {
		return "—"
	}
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.0f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.0f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// --- styles ---

var (
	styleTitle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	styleSelected = lipgloss.NewStyle().Background(lipgloss.Color("236"))
	styleCursor   = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	styleGreen    = lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
	styleYellow   = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	styleRed      = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	styleDim      = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleBold     = lipgloss.NewStyle().Bold(true)
	styleCheck    = lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Bold(true)
)

func riskStyle(r engine.RiskLevel) lipgloss.Style {
	switch r {
	case engine.RiskGreen:
		return styleGreen
	case engine.RiskYellow:
		return styleYellow
	case engine.RiskRed:
		return styleRed
	default:
		return styleDim
	}
}

func riskMarker(r engine.RiskLevel) string {
	switch r {
	case engine.RiskGreen:
		return styleGreen.Render("●")
	case engine.RiskYellow:
		return styleYellow.Render("▲")
	case engine.RiskRed:
		return styleRed.Render("⚠")
	default:
		return " "
	}
}
