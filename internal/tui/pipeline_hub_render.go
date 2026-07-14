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

	// Next Action recommendation strip — pinned above the grid so the user
	// sees "what should I do next" without scanning every column.
	if strip := m.renderRecommendationStrip(); strip != "" {
		b.WriteString(strip)
		b.WriteString("\n\n")
	}

	// Column layout
	colCount := len(m.columns)
	sepWidth := ansi.StringWidth(pipelineHubColSep) * (colCount - 1)
	colWidth := max((m.width-sepWidth)/colCount, 1)
	if m.width >= pipelineHubMinColWidth*colCount+sepWidth {
		colWidth = max(colWidth, pipelineHubMinColWidth)
	}

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

	// Footer — session 11 caps contextual hints at three and pins the
	// standard escape-hatch cluster so navigation/help discovery matches
	// Today and Dashboard. Esc is dropped from the contextual cluster
	// because it is a universal escape and would otherwise spend the
	// fourth slot on a recoverable key.
	helpText := composeFullHeightFooterLeft("Enter open · h/l columns · j/k rows")
	b.WriteString(renderStatusBarFooter(helpText, "", m.width, m.AccentColor()))

	return tea.NewView(b.String())
}

// renderRecommendationStrip returns the styled Next Action strip for the hub,
// or empty when the renderer suppresses it (e.g., width below the strip's
// minimum). The strip uses the same engine + renderer the Today cockpit and
// Dashboard use so all three surfaces speak the same vocabulary.
func (m *PipelineHubModel) renderRecommendationStrip() string {
	return renderNextActionStrip(NextActionStripOpts{
		Action:      m.recommendation,
		Width:       m.width,
		HasProjects: m.hasActiveProjects(),
		OpenLoops:   m.openLoops,
	})
}

func (m *PipelineHubModel) hasActiveProjects() bool {
	if m.runtime == nil {
		return false
	}
	return len(m.runtime.ActiveProjects()) > 0
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
	keyPad := max(keyWidth-ansi.StringWidth(e.key), 0)
	plain := e.key + strings.Repeat(" ", keyPad) + "  " + label
	return padRight(plain, colWidth)
}

// padRight returns s padded with spaces to exactly width visual characters.
// Truncates with "…" if s is wider than width.
func padRight(s string, width int) string {
	n := ansi.StringWidth(s)
	if n >= width {
		if width > 1 {
			return truncateIntentional(s, width, "…")
		}
		return truncateIntentional(s, width, "")
	}
	return s + strings.Repeat(" ", width-n)
}
