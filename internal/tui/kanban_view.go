package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// kanbanRender renders the full kanban view as a string.
func kanbanRender(m *KanbanModel) string {
	if m.width == 0 || m.height == 0 {
		return "Kanban loading..."
	}

	var sb strings.Builder

	// === Title bar ===
	modeIcon := KanbanModeIcons[m.mode]
	modeName := KanbanModeNames[m.mode]
	totalCards := 0
	for _, col := range m.columns {
		totalCards += len(col.Tasks)
	}
	subtitle := fmt.Sprintf("%d tasks", totalCards)
	if m.HasActiveFilters() {
		n := m.ActiveFilterCount()
		if n == 1 {
			subtitle += " · 1 filter"
		} else {
			subtitle += fmt.Sprintf(" · %d filters", n)
		}
	}
	title := renderFullscreenTitle(modeIcon+" "+modeName, subtitle, m.AccentColor())
	// When terminal is too narrow for more than one column, show a position
	// indicator instead of the mode-cycling hint so the user can orient themselves.
	// minColWidth=20 matches the constant defined in the column layout section below.
	maxVisForHint := (m.width + 1) / 21
	if maxVisForHint < 1 {
		maxVisForHint = 1
	}
	var hint string
	if maxVisForHint == 1 && len(m.columns) > 1 {
		hint = MutedStyle.Render(fmt.Sprintf("col %d/%d  h/l: prev/next", m.activeCol+1, len(m.columns)))
	} else {
		hint = MutedStyle.Render("ctrl+g: cycle mode")
	}
	gap := m.width - lipgloss.Width(title) - lipgloss.Width(hint)
	if gap > 0 {
		title += strings.Repeat(" ", gap) + hint
	}
	sb.WriteString(title + "\n")

	numCols := len(m.columns)
	if numCols == 0 {
		sb.WriteString("\n" + MutedStyle.Render("  No tasks to display."))
		return sb.String()
	}

	// === Compute visible columns ===
	const minColWidth = 20
	const colSep = "│"
	const divSep = "┼"

	// Max columns that fit at minimum width: each col needs minColWidth + 1 (separator),
	// except the last which needs no trailing separator.
	maxVisible := (m.width + 1) / (minColWidth + 1)
	if maxVisible < 1 {
		maxVisible = 1
	}
	numVisible := numCols
	if numVisible > maxVisible {
		numVisible = maxVisible
	}

	// Clamp horizontal scroll offset.
	startCol := m.colScrollOff
	if startCol+numVisible > numCols {
		startCol = numCols - numVisible
	}
	if startCol < 0 {
		startCol = 0
	}
	endCol := startCol + numVisible

	// Column width: divide remaining space equally after separators.
	colWidth := (m.width - (numVisible - 1)) / numVisible
	if colWidth < minColWidth {
		colWidth = minColWidth
	}

	// === Height budget ===
	// title(1) + blank(1) + header(1) + divider(1) = 4 fixed lines
	const fixedOverhead = 4
	bodyRows := m.height - fixedOverhead
	if m.detailPaneVisible {
		// separator(1) + configurable pane height
		bodyRows -= 1 + m.detailPaneHeight()
	}
	if bodyRows < 0 {
		bodyRows = 0
	}

	// === Blank separator after title ===
	sb.WriteString("\n")

	// === Scroll hint runes (1 char each) ===
	leftHintWidth := 0
	rightHintWidth := 0
	leftHint := ""
	rightHint := ""
	if startCol > 0 {
		leftHint = MutedStyle.Render("◀")
		leftHintWidth = 1
	}
	if endCol < numCols {
		rightHint = MutedStyle.Render("▶")
		rightHintWidth = 1
	}
	_ = rightHintWidth // used indirectly via rightHint appending

	// === Column headers ===
	headerParts := make([]string, 0, numVisible)
	for i := startCol; i < endCol; i++ {
		col := m.columns[i]
		label := fmt.Sprintf(" %s %s (%d)", col.Icon, col.Label, len(col.Tasks))
		label = truncateToVisualWidth(label, colWidth)
		var hdrStyle lipgloss.Style
		if i == m.activeCol {
			hdrStyle = lipgloss.NewStyle().
				Foreground(m.AccentColor()).
				Bold(true).
				Width(colWidth)
		} else {
			hdrStyle = MutedStyle.Width(colWidth)
		}
		headerParts = append(headerParts, hdrStyle.Render(label))
	}
	sb.WriteString(leftHint + strings.Join(headerParts, colSep) + rightHint + "\n")

	// === Divider ===
	divParts := make([]string, numVisible)
	for i := range divParts {
		divParts[i] = strings.Repeat("─", colWidth)
	}
	sb.WriteString(strings.Repeat(" ", leftHintWidth) + strings.Join(divParts, divSep) + "\n")

	// === Card rows ===
	for row := 0; row < bodyRows; row++ {
		rowParts := make([]string, 0, numVisible)
		for i := startCol; i < endCol; i++ {
			col := m.columns[i]
			cardIdx := col.ScrollOffset + row

			// Last visible row: show overflow indicator if tasks are cut off.
			if row == bodyRows-1 && len(col.Tasks)-col.ScrollOffset > bodyRows {
				overflow := len(col.Tasks) - col.ScrollOffset - (bodyRows - 1)
				overflowStr := fmt.Sprintf("  ...+%d more", overflow)
				rowParts = append(rowParts, MutedStyle.Width(colWidth).Render(overflowStr))
				continue
			}

			if cardIdx >= len(col.Tasks) {
				// Empty cell
				rowParts = append(rowParts, strings.Repeat(" ", colWidth))
				continue
			}

			card := col.Tasks[cardIdx]
			statusBadge := GetStatusBadge(string(card.Task.Status), card.Task.Status.Icon())
			badgeWidth := lipgloss.Width(statusBadge)

			// Selection indicator (1 char + 1 space, or 2 spaces when not in multi-select).
			selIndicator := "  "
			isChecked := m.IsSelected(card.Task.ID)
			if m.multiSelectMode {
				if isChecked {
					selIndicator = "● "
				} else {
					selIndicator = "○ "
				}
			}
			selWidth := lipgloss.Width(selIndicator)

			// Project badge in cross-project mode (appended after title).
			projBadge := ""
			projBadgeWidth := 0
			if m.crossProject && card.Project != nil {
				projBadge = MutedStyle.Render(fmt.Sprintf("[%s]", truncateToVisualWidth(card.Project.Name, 8)))
				projBadgeWidth = lipgloss.Width(projBadge) + 1 // +1 for space before badge
			}

			// Format: "[sel][badge] title [proj]"
			prefixWidth := selWidth + badgeWidth + 1
			maxTitleWidth := colWidth - prefixWidth - projBadgeWidth
			if maxTitleWidth < 0 {
				maxTitleWidth = 0
			}
			title := truncateToVisualWidth(card.Task.Title, maxTitleWidth)
			line := selIndicator + statusBadge + " " + title
			if projBadge != "" {
				line += " " + projBadge
			}

			isFocused := i == m.activeCol && cardIdx == col.Cursor
			isFlash := m.flashCardID != "" && card.Task.ID == m.flashCardID
			var cell string
			switch {
			case isFocused:
				cell = SelectedStyle.Width(colWidth).Render(line)
			case isFlash:
				cell = HighlightStyle.Width(colWidth).Render(line)
			case isChecked:
				cell = ActiveStyle.Width(colWidth).Render(line)
			default:
				cell = lipgloss.NewStyle().Width(colWidth).Render(line)
			}
			rowParts = append(rowParts, cell)
		}
		sb.WriteString(strings.Repeat(" ", leftHintWidth) + strings.Join(rowParts, colSep) + "\n")
	}

	// Append detail pane (separator + info rows)
	sb.WriteString(m.renderKanbanDetailPane())

	result := sb.String()

	// Overlay axis manager popup when active
	if m.showAxisManager {
		popup := m.renderAxisManagerPopup()
		result = OverlayPopupDimmed(result, popup, m.width, m.height)
	}

	return result
}
