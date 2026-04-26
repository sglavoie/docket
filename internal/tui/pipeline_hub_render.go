package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	pipelineHubMinColWidth = 14
	pipelineHubColSep      = " │ "
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
	sepWidth := len(pipelineHubColSep) * (colCount - 1)
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

	// Entry rows.
	for ri := 0; ri < maxEntries; ri++ {
		cells := make([]string, colCount)
		for ci, col := range m.columns {
			if ri < len(col.entries) {
				plain := m.formatEntry(col.entries[ri], colWidth)
				if ci == m.activeCol && ri == m.activeRow {
					cells[ci] = lipgloss.NewStyle().
						Background(ColorSelectedBg).
						Foreground(ColorSelectedFg).
						Render(plain)
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

// renderSeparatorRow builds a ─┼─ separator line matching the column layout.
func (m *PipelineHubModel) renderSeparatorRow(colWidth int) string {
	dash := strings.Repeat("─", colWidth)
	sep := MutedStyle.Render("─┼─")
	parts := make([]string, len(m.columns))
	for i := range m.columns {
		parts[i] = MutedStyle.Render(dash)
	}
	return strings.Join(parts, sep)
}

// formatEntry formats a pipeline entry as a fixed-width plain string.
// Key is left-padded to 7 characters, followed by the label (with optional count).
func (m *PipelineHubModel) formatEntry(e pipelineEntry, colWidth int) string {
	label := e.label
	if e.count > 0 {
		label = fmt.Sprintf("%s (%d)", label, e.count)
	}
	// "%-7s %s" — key in 7 chars, then space, then label
	plain := fmt.Sprintf("%-7s %s", e.key, label)
	return padRight(plain, colWidth)
}

// padRight returns s padded with spaces to exactly width characters.
// Truncates with "…" if s is longer than width.
func padRight(s string, width int) string {
	n := len(s) // ASCII-safe: all keys/labels are ASCII
	if n >= width {
		if width > 1 {
			return s[:width-1] + "…"
		}
		return s[:width]
	}
	return s + strings.Repeat(" ", width-n)
}
