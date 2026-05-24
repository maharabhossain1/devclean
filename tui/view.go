package tui

import (
	"fmt"
	"strings"

	"github.com/maharabhossain1/devclean/engine"
)

func (m Model) View() string {
	switch m.state {
	case stateDone:
		return m.viewDone()
	case stateConfirm:
		return m.viewList() + "\n" + m.viewConfirmDialog()
	case stateTypeName:
		return m.viewList() + "\n" + m.viewTypeNameDialog()
	default:
		return m.viewList()
	}
}

func (m Model) viewList() string {
	if len(m.items) == 0 {
		return styleTitle.Render("DevClean") + "\n\n" +
			styleGreen.Render("✓ Nothing to clean — system is already tidy.") + "\n"
	}

	var b strings.Builder

	// Header
	selectedCount, selectedSize := m.selectionStats()
	header := styleTitle.Render("DevClean — Interactive Clean")
	stats := styleDim.Render(fmt.Sprintf("  %d items  |  selected: %d (%s)",
		len(m.items), selectedCount, fmtSize(selectedSize)))
	b.WriteString(header + stats + "\n")
	b.WriteString(styleDim.Render(strings.Repeat("─", min(m.width, 78))) + "\n")

	// Visible window
	visible := m.visibleRange()
	for i := visible.start; i < visible.end; i++ {
		b.WriteString(m.renderRow(i))
	}

	// Footer
	b.WriteString("\n")
	b.WriteString(m.viewHelp())

	return b.String()
}

func (m Model) renderRow(i int) string {
	it := m.items[i]

	cursor := "  "
	if i == m.cursor {
		cursor = styleCursor.Render("▶ ")
	}

	checkbox := "[ ]"
	if it.Selected {
		checkbox = styleCheck.Render("[✓]")
	}

	risk := riskMarker(it.Trace.Risk)
	path := shortenPath(it.Trace.Path)
	if len(path) > 48 {
		path = "…" + path[len(path)-47:]
	}

	size := styleDim.Render(fmtSize(it.Trace.SizeBytes))
	riskLabel := riskStyle(it.Trace.Risk).Render(string(it.Trace.Risk))

	row := fmt.Sprintf("%s%s %s %-50s %-8s %s\n",
		cursor, checkbox, risk, path, size, riskLabel)

	if i == m.cursor {
		row = styleSelected.Render(row)
	}
	return row
}

type visibleRange struct{ start, end int }

func (m Model) visibleRange() visibleRange {
	listHeight := m.height - 6 // header + footer
	if listHeight < 5 {
		listHeight = 5
	}
	start := m.cursor - listHeight/2
	if start < 0 {
		start = 0
	}
	end := start + listHeight
	if end > len(m.items) {
		end = len(m.items)
		start = end - listHeight
		if start < 0 {
			start = 0
		}
	}
	return visibleRange{start, end}
}

func (m Model) selectionStats() (int, int64) {
	var count int
	var size int64
	for _, it := range m.items {
		if it.Selected {
			count++
			size += it.Trace.SizeBytes
		}
	}
	return count, size
}

func (m Model) viewConfirmDialog() string {
	selected := m.selectedItems()
	var size int64
	for _, it := range selected {
		size += it.Trace.SizeBytes
	}

	hasYellow := false
	for _, it := range selected {
		if it.Trace.Risk == engine.RiskYellow {
			hasYellow = true
		}
	}

	msg := fmt.Sprintf("Remove %d items (%s)?", len(selected), fmtSize(size))
	if hasYellow {
		msg = styleYellow.Render("⚠  ") + styleBold.Render(msg) +
			styleDim.Render("  (includes Yellow items)")
	} else {
		msg = styleBold.Render(msg)
	}
	return "\n" + msg + "  " +
		styleGreen.Render("[y]") + styleDim.Render("es  ") +
		styleRed.Render("[n]") + styleDim.Render("o\n")
}

func (m Model) viewTypeNameDialog() string {
	var redItem engine.Trace
	for _, it := range m.selectedItems() {
		if it.Trace.Risk == engine.RiskRed {
			redItem = it.Trace
			break
		}
	}
	name := itemName(redItem.Path)
	prompt := styleRed.Render("⚠  Unknown item — type the path to confirm deletion:\n")
	prompt += styleDim.Render("  " + name + "\n\n")
	prompt += "  > " + m.input + styleCursor.Render("█") + "\n"
	prompt += styleDim.Render("  esc to cancel\n")
	return "\n" + prompt
}

func (m Model) viewDone() string {
	if m.err != nil {
		return styleRed.Render("Error: "+m.err.Error()) + "\n"
	}
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(styleGreen.Render("✓ Done!") + "\n\n")
	b.WriteString(fmt.Sprintf("  Removed: %d items\n", m.deleted))
	b.WriteString(fmt.Sprintf("  Freed:   %s\n", fmtSize(m.freed)))
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  Undo log saved. Run `devclean undo` to restore.\n"))
	return b.String()
}

func (m Model) viewHelp() string {
	parts := []string{
		styleDim.Render("↑↓") + " navigate",
		styleDim.Render("space") + " select",
		styleDim.Render("a") + " select all green",
		styleDim.Render("d") + " delete",
		styleDim.Render("q") + " quit",
	}
	return styleDim.Render("  " + strings.Join(parts, "  ·  ") + "\n")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
