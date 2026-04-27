package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	pipelineHubMinColWidth = 14
	pipelineHubColSep      = "  "
)

// View implements tea.Model.
func (m *PipelineHubModel) View() tea.View {
	if len(m.columns) == 0 {
		return tea.NewView("No pipeline columns defined.")
	}

	var b strings.Builder

	// Title
	b.WriteString(renderFullscreenTitle("Pipeline Overview", "", m.AccentColor()))
	b.WriteString("\n\n")

	// Column layout
	colCount := len(m.columns)
	sepWidth := ansi.StringWidth(pipelineHubColSep) * (colCount - 1)
	colWidth := max((m.width-sepWidth)/colCount, pipelineHubMinColWidth)

	// Max entries across all columns (for row-count alignment)
	maxEntries := 0
	for _, col := range m.columns {
		if len(col.entries) > maxEntries {
			maxEntries = len(col.entries)
		}
	}

	// Header row: column names in their accent color.
	headerCells := make([]string, colCount)
	for ci, col := range m.columns {
		name := strings.ToUpper(col.name)
		padded := padRight(name, colWidth)
		style := lipgloss.NewStyle().Bold(true).Foreground(col.accent)
		if ci == m.activeCol {
			style = style.Underline(true)
		}
		headerCells[ci] = style.Render(padded)
	}
	b.WriteString(strings.Join(headerCells, pipelineHubColSep))
	b.WriteString("\n")

	// Separator row: horizontal rule with cross-joints at column boundaries.
	sepLine := m.renderSeparatorRow(colWidth)
	b.WriteString(sepLine)
	b.WriteString("\n")

	// Pre-compute max key visual width per column.
	colKeyWidth := make([]int, colCount)
	for ci, col := range m.columns {
		for _, entry := range col.entries {
			if w := ansi.StringWidth(entry.key); w > colKeyWidth[ci] {
				colKeyWidth[ci] = w
			}
		}
	}

	// Entry rows.
	for ri := 0; ri < maxEntries; ri++ {
		cells := make([]string, colCount)
		for ci, col := range m.columns {
			if ri < len(col.entries) {
				entry := col.entries[ri]
				plain := m.formatEntry(entry, colWidth, colKeyWidth[ci])
				isSelected := ci == m.activeCol && ri == m.activeRow
				noAction := entry.viewType == 0
				if isSelected {
					cells[ci] = lipgloss.NewStyle().
						Width(colWidth).
						Background(ColorSelectedBg).
						Foreground(ColorSelectedFg).
						Render(strings.TrimRight(plain, " "))
				} else if noAction {
					cells[ci] = MutedStyle.Render(plain)
				} else {
					cells[ci] = plain
				}
			} else {
				cells[ci] = strings.Repeat(" ", colWidth)
			}
		}
		b.WriteString(strings.Join(cells, pipelineHubColSep))
		b.WriteString("\n")
	}

	// Footer
	helpText := HelpStyle.Render("Enter open · h/l column · j/k row · Esc close")
	b.WriteString(renderStatusBarFooter("", helpText, m.width, m.AccentColor()))

	return tea.NewView(b.String())
}

// renderSeparatorRow builds a separator line matching the column layout.
func (m *PipelineHubModel) renderSeparatorRow(colWidth int) string {
	dash := strings.Repeat("─", colWidth)
	sep := MutedStyle.Render("──")
	parts := make([]string, len(m.columns))
	for i := range m.columns {
		parts[i] = MutedStyle.Render(dash)
	}
	return strings.Join(parts, sep)
}

// formatEntry formats a pipeline entry as a fixed-width plain string.
// Keys are left-padded to keyWidth (the longest key in the column), then 2 spaces, then label.
func (m *PipelineHubModel) formatEntry(e pipelineEntry, colWidth, keyWidth int) string {
	label := e.label
	if e.count > 0 {
		label = fmt.Sprintf("%s (%d)", label, e.count)
	}
	keyPad := keyWidth - ansi.StringWidth(e.key)
	if keyPad < 0 {
		keyPad = 0
	}
	plain := e.key + strings.Repeat(" ", keyPad) + "  " + label
	return padRight(plain, colWidth)
}

// padRight returns s padded with spaces to exactly width visual characters.
// Truncates with "…" if s is wider than width.
func padRight(s string, width int) string {
	n := ansi.StringWidth(s)
	if n >= width {
		if width > 1 {
			return ansi.Truncate(s, width, "…")
		}
		return ansi.Truncate(s, width, "")
	}
	return s + strings.Repeat(" ", width-n)
}
