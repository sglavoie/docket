package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// joinWithActiveColBorders joins parts with sep, coloring separators adjacent
// to activeVisIdx with the accent color to visually indicate the focused column.
func joinWithActiveColBorders(parts []string, sep string, activeVisIdx int, accent lipgloss.Style) string {
	var sb strings.Builder
	for i, part := range parts {
		if i > 0 {
			if i == activeVisIdx || i == activeVisIdx+1 {
				sb.WriteString(accent.Render(sep))
			} else {
				sb.WriteString(sep)
			}
		}
		sb.WriteString(part)
	}
	return sb.String()
}

// kanbanRender renders the full kanban view as a string.
//
//nolint:gocyclo // rendering hub with many layout branches
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
		hint = MutedStyle.Render("`/~: cycle mode")
	}
	const alertReserve = 10
	gap := m.width - lipgloss.Width(title) - lipgloss.Width(hint) - alertReserve
	if gap > 0 {
		title += strings.Repeat(" ", gap) + hint
	} else {
		title += "  " + hint
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
	numVisible := min(numCols, maxVisible)

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
	// title(1) + blank(1) + header(1) + divider(1) + footer(2) = 6 fixed lines
	const fixedOverhead = 6
	bodyRows := m.height - fixedOverhead
	if m.detailPaneVisible {
		// separator(1) + configurable pane height
		bodyRows -= 1 + m.detailPaneHeight()
	}
	if bodyRows < 1 {
		bodyRows = 1
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
	activeVisIdx := m.activeCol - startCol
	accentSepStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	headerParts := make([]string, 0, numVisible)
	for i := startCol; i < endCol; i++ {
		col := m.columns[i]
		if m.mode == KanbanModeSection {
			sectionStyle := GetViewModeStyleForSection(col.Label)
			if i == m.activeCol {
				sectionStyle = sectionStyle.Bold(true)
			}
			countStr := fmt.Sprintf(" (%d)", len(col.Tasks))
			countWidth := lipgloss.Width(MutedStyle.Render(countStr))
			maxBadgeWidth := colWidth - countWidth
			if maxBadgeWidth < 0 {
				maxBadgeWidth = 0
			}
			iconLabel := fmt.Sprintf(" %s %s", col.Icon, col.Label)
			iconLabel = truncateToVisualWidth(iconLabel, maxBadgeWidth)
			badge := sectionStyle.Render(iconLabel)
			countStyle := MutedStyle
			if i == m.activeCol {
				countStyle = accentSepStyle
			}
			count := countStyle.Render(countStr)
			combined := badge + count
			w := lipgloss.Width(combined)
			padding := colWidth - w
			if padding < 0 {
				padding = 0
			}
			headerParts = append(headerParts, combined+strings.Repeat(" ", padding))
		} else {
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
	}
	sb.WriteString(leftHint + joinWithActiveColBorders(headerParts, colSep, activeVisIdx, accentSepStyle) + rightHint + "\n")

	// === Divider ===
	divParts := make([]string, numVisible)
	for i := range divParts {
		line := strings.Repeat("─", colWidth)
		if i == activeVisIdx {
			line = accentSepStyle.Render(line)
		}
		divParts[i] = line
	}
	sb.WriteString(strings.Repeat(" ", leftHintWidth) + joinWithActiveColBorders(divParts, divSep, activeVisIdx, accentSepStyle) + "\n")

	// === Build display rows per column (with project group headers in cross-project mode) ===
	type displayRow struct {
		isHeader bool   // true = project group header
		header   string // project name for header rows
		card     *KanbanCard
		taskIdx  int // index into col.Tasks (for cursor matching)
	}
	colDisplayRows := make([][]displayRow, numCols)
	for i := range m.columns {
		col := &m.columns[i]
		if m.crossProject && len(col.Tasks) > 0 {
			// Group tasks by project, insert header rows.
			var rows []displayRow
			lastProj := ""
			for ti := range col.Tasks {
				card := &col.Tasks[ti]
				projName := ""
				if card.Project != nil {
					projName = card.Project.Name
				}
				if projName != lastProj {
					rows = append(rows, displayRow{isHeader: true, header: projName})
					lastProj = projName
				}
				rows = append(rows, displayRow{card: card, taskIdx: ti})
			}
			colDisplayRows[i] = rows
		} else {
			// No grouping — 1:1 mapping.
			rows := make([]displayRow, len(col.Tasks))
			for ti := range col.Tasks {
				rows[ti] = displayRow{card: &col.Tasks[ti], taskIdx: ti}
			}
			colDisplayRows[i] = rows
		}
	}

	// === Card rows ===
	for row := 0; row < bodyRows; row++ {
		rowParts := make([]string, 0, numVisible)
		for i := startCol; i < endCol; i++ {
			col := m.columns[i]
			dRows := colDisplayRows[i]
			dispIdx := col.ScrollOffset + row

			// Last visible row: show overflow indicator if display rows are cut off.
			if row == bodyRows-1 && len(dRows)-col.ScrollOffset > bodyRows {
				overflow := len(dRows) - col.ScrollOffset - (bodyRows - 1)
				overflowStr := fmt.Sprintf("...+%d more", overflow)
				rowParts = append(rowParts, MutedStyle.Width(colWidth).Render(overflowStr))
				continue
			}

			if dispIdx >= len(dRows) {
				// Empty cell
				rowParts = append(rowParts, strings.Repeat(" ", colWidth))
				continue
			}

			dr := dRows[dispIdx]

			// Project group header row.
			if dr.isHeader {
				headerText := truncateToVisualWidth(dr.header, colWidth-2)
				headerStyle := lipgloss.NewStyle().
					Background(ColorDimmedBg).
					Foreground(ColorDimmedFg).
					Bold(true).
					Width(colWidth)
				rowParts = append(rowParts, headerStyle.Render(headerText))
				continue
			}

			card := dr.card
			statusBadge := GetStatusBadge(string(card.Task.Status), card.Task.Status.Icon())
			badgeWidth := lipgloss.Width(statusBadge)
			typeBadge := GetTypeBadge(string(card.Task.Type), card.Task.Type.Icon())
			typeBadgeWidth := lipgloss.Width(typeBadge)

			// Selection indicator: only shown in multi-select mode.
			selIndicator := ""
			isChecked := m.IsSelected(card.Task.ID)
			if m.multiSelectMode {
				if isChecked {
					selIndicator = "● "
				} else {
					selIndicator = "○ "
				}
			}
			selWidth := lipgloss.Width(selIndicator)

			// Format: "[sel]typeBadge statusBadge title"
			prefixWidth := selWidth + typeBadgeWidth + badgeWidth + 1
			maxTitleWidth := colWidth - prefixWidth
			if maxTitleWidth < 0 {
				maxTitleWidth = 0
			}
			title := truncateToVisualWidth(card.Task.Title, maxTitleWidth)
			line := selIndicator + typeBadge + statusBadge + " " + title

			isFocused := i == m.activeCol && dr.taskIdx == col.Cursor
			isFlash := m.flashCardID != "" && card.Task.ID == m.flashCardID
			var cell string
			switch {
			case isFocused:
				cell = SelectedLine(line, colWidth)
			case isFlash:
				cell = SelectedFlashLine(line, colWidth)
			case isChecked:
				cell = ActiveStyle.Width(colWidth).Render(line)
			default:
				cell = lipgloss.NewStyle().Width(colWidth).Render(line)
			}
			rowParts = append(rowParts, cell)
		}
		sb.WriteString(strings.Repeat(" ", leftHintWidth) + joinWithActiveColBorders(rowParts, colSep, activeVisIdx, accentSepStyle) + "\n")
	}

	// Append detail pane (separator + info rows)
	sb.WriteString(m.renderKanbanDetailPane())

	// Footer help bar
	left := HelpStyle.Render("h/l columns · j/k navigate · H/L move task · Enter open · Tab detail · ? help")
	right := MutedStyle.Render(fmt.Sprintf("%d tasks", totalCards))
	sb.WriteString(renderStatusBarFooter(left, right, m.width, m.AccentColor()))

	result := sb.String()

	// Overlay mode picker popup when active
	if m.showModePicker {
		popup := RenderPrefixPopup("`", m.buildModePickerHints(), m.width, m.height)
		result = OverlayPopupDimmed(result, popup, m.width, m.height)
	}

	// Overlay axis manager popup when active
	if m.showAxisManager {
		popup := m.renderAxisManagerPopup()
		result = OverlayPopupDimmed(result, popup, m.width, m.height)
	}

	return result
}
