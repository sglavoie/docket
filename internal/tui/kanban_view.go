package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/models"
)

func (m *KanbanViewModel) View() string {
	if m.Project == nil {
		return MutedStyle.Render("No project loaded")
	}

	// Render completed view if in completed view mode
	if m.completedViewMode {
		return m.renderCompletedView()
	}

	// Render all view if in all view mode
	if m.allViewMode {
		return m.renderAllView()
	}

	var viewParts []string

	// Render custom section tab bar above preset tabs (when custom sections exist)
	hasCustomSections := m.customSectionTabs() != nil
	if hasCustomSections {
		focusIndicator := lipgloss.NewStyle().Foreground(ColorPrimary).Render("▐")
		unfocusedPad := " "

		csBar := RenderColoredTabBar(m.customSectionTabs(), m.customSectionTab, m.Width()-1, 1, m.Project.SectionColors)
		if m.customSectionRowFocused {
			csBar = focusIndicator + csBar
		} else {
			csBar = unfocusedPad + csBar
		}
		viewParts = append(viewParts, csBar)
	}

	// Render tab bar at the top
	presetBar := m.renderKanbanTabBar()
	if hasCustomSections {
		focusIndicator := lipgloss.NewStyle().Foreground(ColorPrimary).Render("▐")
		unfocusedPad := " "
		if !m.customSectionRowFocused {
			presetBar = focusIndicator + presetBar
		} else {
			presetBar = unfocusedPad + presetBar
		}
	}
	viewParts = append(viewParts, presetBar)

	// Render search input if in search mode
	if m.searchMode {
		searchLine := "Search: " + m.searchInput.View()
		viewParts = append(viewParts, searchLine)
	}

	// Render score filter custom value input
	if m.scoreFilterInputMode {
		scoreLine := SubtitleStyle.Render("Score threshold: ") + m.scoreFilterInput.View() + " " + HelpStyle.Render("(Enter: apply, Esc: cancel)")
		viewParts = append(viewParts, scoreLine)
	}

	var columns []string
	gap := " " // Single space gap between columns

	for i, section := range KanbanColumnOrder {
		columnContent := m.renderColumn(section, i == m.ActiveColumn)
		columns = append(columns, columnContent)
	}

	// Join columns horizontally with gaps
	var parts []string
	for i, col := range columns {
		parts = append(parts, col)
		if i < len(columns)-1 {
			parts = append(parts, gap)
		}
	}

	kanbanPane := lipgloss.JoinHorizontal(lipgloss.Top, parts...)

	// Check if all columns are empty (no tasks match filters)
	totalTasks := 0
	for _, section := range KanbanColumnOrder {
		totalTasks += len(m.Tasks[section])
	}
	if totalTasks == 0 && m.HasActiveFilters() {
		// Show message when filters result in no tasks
		emptyMsg := MutedStyle.Render("No tasks match current filters")
		kanbanPane = lipgloss.Place(m.Width(), m.kanbanHeight, lipgloss.Center, lipgloss.Center, emptyMsg)
	}

	viewParts = append(viewParts, kanbanPane)

	// Render the horizontal divider
	var dividerOpts []DividerOption
	if m.detailsPaneFocused {
		dividerOpts = append(dividerOpts, WithDividerColor(ColorPrimary))
	}
	divider := renderHorizontalDivider(m.Width(), dividerOpts...)
	viewParts = append(viewParts, divider)

	// Render the details panel for the selected task
	tabBarHeight := TabBarHeight
	if m.customSectionTabs() != nil {
		tabBarHeight += TabBarHeight
	}
	searchBarHeight := 0
	if m.searchMode {
		searchBarHeight = 1
	}
	quickAddHeight := 0
	if m.quickAddMode {
		quickAddHeight = m.getQuickAddHeight()
	}
	quickEditHeight := m.getQuickEditHeight()
	statusBarHeight := 0
	if m.statusMessage != "" {
		statusBarHeight = 1
	}
	detailsHeight := m.Height() - m.kanbanHeight - DividerHeight - tabBarHeight - searchBarHeight - quickAddHeight - quickEditHeight - statusBarHeight
	if detailsHeight < KanbanMinDetailsHeight {
		detailsHeight = KanbanMinDetailsHeight
	}
	selectedTask := m.getSelectedTask()
	detailsPane := m.renderDetailsPanel(selectedTask, m.Width(), detailsHeight)
	viewParts = append(viewParts, detailsPane)

	// Render quick add input if in quick add mode
	if m.quickAddMode {
		quickAddBar := m.renderQuickAddInput()
		viewParts = append(viewParts, quickAddBar)
	}

	// Render quick edit bar if in quick edit mode
	if m.quickEdit != nil && m.quickEdit.IsActive() {
		quickEditBar := m.renderQuickEditBar()
		viewParts = append(viewParts, quickEditBar)
	}

	// Render status message if set
	if m.statusMessage != "" {
		statusStyle := lipgloss.NewStyle().Foreground(ColorWarning)
		if m.confirm.IsActive() {
			statusStyle = lipgloss.NewStyle().Foreground(ColorError).Bold(true)
		}
		viewParts = append(viewParts, statusStyle.Render(m.statusMessage))
	}

	// Join vertically: filterBar + kanban + divider + details + quickAdd + status + popup
	return lipgloss.JoinVertical(lipgloss.Left, viewParts...)
}

// renderKanbanTabBar renders the tab bar at the top of the kanban view.
func (m *KanbanViewModel) renderKanbanTabBar() string {
	availableWidth := m.Width()
	if m.customSectionTabs() != nil {
		availableWidth-- // Reserve space for focus indicator
	}

	// When custom sections exist and focus is on the custom section row,
	// dim the preset tab bar to indicate it's not the focused row.
	dimmed := m.customSectionTabs() != nil && m.customSectionRowFocused

	activeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Padding(0, 1).
		BorderBottom(true).
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(ColorPrimary)

	if dimmed {
		activeStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Padding(0, 1).
			BorderBottom(true).
			BorderStyle(lipgloss.Border{Bottom: "─"}).
			BorderForeground(ColorBorder)
	}

	inactiveStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Padding(0, 1).
		BorderBottom(true).
		BorderStyle(lipgloss.Border{Bottom: "─"}).
		BorderForeground(ColorBorder)

	var tabs []string
	for i, name := range kanbanTabNames {
		if i == m.activeTab {
			tabs = append(tabs, activeStyle.Render(name))
		} else {
			tabs = append(tabs, inactiveStyle.Render(name))
		}
	}

	row := lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)

	tabWidth := lipgloss.Width(row)
	remaining := availableWidth - tabWidth
	if remaining > 0 {
		filler := lipgloss.NewStyle().
			BorderBottom(true).
			BorderStyle(lipgloss.Border{Bottom: "─"}).
			BorderForeground(ColorBorder).
			Width(remaining).
			Render("")
		row = lipgloss.JoinHorizontal(lipgloss.Bottom, row, filler)
	}

	return row
}

// renderColumn renders a single Kanban column with header and task list.
// Applies viewport scrolling when tasks exceed the available height.
func (m *KanbanViewModel) renderColumn(section string, isActive bool) string {
	var b strings.Builder

	// Render header with section color
	header := m.renderColumnHeader(section, isActive)
	b.WriteString(header)
	b.WriteString("\n")

	// Render visible nodes in this column
	visibleNodes := m.visibleNodesForSection(section)
	cs := m.Columns[section]

	// Calculate viewport height for tasks (kanbanHeight minus header line)
	viewportHeight := m.kanbanHeight - 1
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	// Always validate with reduced height to ensure cursor remains visible when
	// the "↑ N more" indicator appears. The indicator takes 1 line, so we
	// conservatively assume it will be shown during validation.
	reducedHeight := viewportHeight - 1
	if reducedHeight < 1 {
		reducedHeight = 1
	}
	cs.ValidateScroll(len(visibleNodes), reducedHeight)

	// Determine if indicator is actually needed after validation
	hasUpIndicator := cs.ScrollOffset > 0
	effectiveHeight := viewportHeight
	if hasUpIndicator {
		effectiveHeight = reducedHeight
	}

	if len(visibleNodes) == 0 {
		b.WriteString(MutedStyle.Render("(empty)"))
	} else {
		// Calculate visible range based on scroll offset
		startIdx := cs.ScrollOffset
		endIdx := startIdx + viewportHeight
		if endIdx > len(visibleNodes) {
			endIdx = len(visibleNodes)
		}

		// Show "↑ N more" indicator if tasks are hidden above
		if hasUpIndicator {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("↑ %d more", startIdx)))
			b.WriteString("\n")
			// Adjust visible range since we used a line for the indicator
			endIdx = startIdx + effectiveHeight
			if endIdx > len(visibleNodes) {
				endIdx = len(visibleNodes)
			}
		}

		// Render visible nodes
		for i := startIdx; i < endIdx; i++ {
			node := visibleNodes[i]
			isSelected := i == cs.Cursor && isActive && !m.detailsPaneFocused
			line := m.renderTaskRow(node, isSelected)
			b.WriteString(line)
			if i < endIdx-1 {
				b.WriteString("\n")
			}
		}

		// Show "↓ N more" indicator if tasks are hidden below
		if endIdx < len(visibleNodes) {
			b.WriteString("\n")
			b.WriteString(MutedStyle.Render(fmt.Sprintf("↓ %d more", len(visibleNodes)-endIdx)))
		}
	}

	// Pad column to exact height for consistent layout
	content := b.String()
	lines := strings.Split(content, "\n")
	for len(lines) < m.kanbanHeight {
		lines = append(lines, "")
	}
	if len(lines) > m.kanbanHeight {
		lines = lines[:m.kanbanHeight]
	}

	// Apply column width styling
	columnStyle := lipgloss.NewStyle().Width(m.ColumnWidth)
	return columnStyle.Render(strings.Join(lines, "\n"))
}

// renderColumnHeader renders the column header with appropriate section color.
// Uses the same ViewMode styles as the project tree view mode indicator for consistency.
func (m *KanbanViewModel) renderColumnHeader(section string, isActive bool) string {
	displayName := sectionDisplayName(section)
	visibleCount := m.visibleNodeCount(section)
	totalCount := len(m.Tasks[section])

	// Show visible/total if different (collapsed nodes)
	var headerText string
	if visibleCount != totalCount {
		headerText = fmt.Sprintf("%s (%d/%d)", displayName, visibleCount, totalCount)
	} else {
		headerText = fmt.Sprintf("%s (%d)", displayName, totalCount)
	}

	// Append sort indicator if this column has a non-default sort
	sortField := m.getSortFieldForColumn(section)
	if sortField != SortByDefault {
		direction := "↑"
		if isActive && !m.sortAscending {
			direction = "↓"
		}
		headerText += " " + direction + sortField.ShortString()
	}

	// Use the same ViewMode style as the view mode indicator
	style := GetViewModeStyleForSection(section)

	// Add indicator if this column is active
	if isActive {
		headerText = "▸ " + headerText
	}

	return style.Render(headerText)
}

// renderTaskRow renders a single task row with hierarchy indicators.
// Shows indentation, expand/collapse icons, and children-elsewhere badges.
// In multi-select mode, shows [x] or [ ] checkbox prefix.
func (m *KanbanViewModel) renderTaskRow(node *KanbanNode, isSelected bool) string {
	task := node.Task
	statusIcon := task.Status.Icon()
	title := task.Title

	// Build selection checkbox for multi-select mode
	var checkbox string
	if m.multiSelectMode {
		if m.isTaskSelected(task.ID) {
			checkbox = SelectionStyle.Render("[x]") + " "
		} else {
			checkbox = MutedStyle.Render("[ ]") + " "
		}
	}

	// Build indent prefix: "└" for nested items
	var indent string
	if node.Level > 0 {
		indent = strings.Repeat(" ", node.Level) + "└"
	}

	// Build expand/collapse icon
	var expandIcon string
	if node.HasChildrenHere {
		if node.Expanded {
			expandIcon = "▼"
		} else {
			expandIcon = "▶"
		}
	} else {
		expandIcon = " "
	}

	// Build children-elsewhere badge
	var badge string
	if node.ChildrenElsewhere > 0 {
		badge = MutedStyle.Render(fmt.Sprintf("+%d", node.ChildrenElsewhere))
	}

	// Calculate available space for title
	// Layout: cursor (3) + checkbox (4 if multi-select) + indent + expand (1) + space (1) + status (1) + space (1) + title + badge
	checkboxLen := 0
	if m.multiSelectMode {
		checkboxLen = KanbanCheckboxWidth
	}
	prefixLen := KanbanCursorWidth + checkboxLen + len(indent) + 1 + 1 + 1 + 1
	badgeLen := 0
	if badge != "" {
		badgeLen = len(fmt.Sprintf("+%d", node.ChildrenElsewhere)) + 1 // +1 for space before badge
	}
	scoreBadgeLen := 0
	if m.ColumnWidth >= 25 && m.config != nil {
		if profile := m.config.GetProjectScoringProfile(m.Project.ID); profile != nil {
			_ = profile // score badge takes ~7-8 chars: [S:NN] or [S:~NN]
			scoreBadgeLen = 8
		}
	}
	maxTitleLen := m.ColumnWidth - prefixLen - badgeLen - scoreBadgeLen
	if maxTitleLen < KanbanMinTitleWidth {
		maxTitleLen = KanbanMinTitleWidth
	}
	title = truncateWithEllipsis(title, maxTitleLen)

	// Apply active/pinned title styling
	if isSelected {
		if task.Active {
			title = ActiveStyle.Bold(true).Render(title)
		} else if task.Pinned {
			title = BoldStyle.Foreground(ColorSuccess).Render(title)
		}
	} else if task.Active {
		title = ActiveStyle.Render(title)
	} else if task.Pinned {
		title = lipgloss.NewStyle().Foreground(ColorSuccess).Render(title)
	}

	// Score badge (only if column wide enough and profile assigned)
	scoreBadgeStr := ""
	if m.ColumnWidth >= 25 && m.config != nil {
		if profile := m.config.GetProjectScoringProfile(m.Project.ID); profile != nil {
			scoreBadgeStr = RenderScoreBadge(task, profile)
		}
	}

	statusStyle := GetStatusStyle(string(task.Status))
	content := checkbox + indent + expandIcon + " " + statusStyle.Render(statusIcon) + " " + title
	if scoreBadgeStr != "" {
		content += " " + scoreBadgeStr
	}
	if badge != "" {
		content += " " + badge
	}

	// Build cursor prefix with active marker (matches RenderSelectableRow)
	activeMarker := ActiveStyle.Bold(true).Render("*")
	if isSelected && task.Active {
		return SelectionCursorStyle.Render(">") + activeMarker + " " + content
	} else if isSelected {
		return SelectionCursorStyle.Render(">") + "  " + content
	} else if task.Active {
		return " " + activeMarker + " " + content
	}
	return "   " + content
}

// truncateWithEllipsis truncates a string to maxLen characters, adding "..."
// suffix if truncation occurs. Returns original string if it fits within maxLen.
func truncateWithEllipsis(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	// Need at least 4 chars to show something meaningful with ellipsis (1 char + "...")
	if maxLen < 4 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// sectionDisplayName returns the display name for a section.
// The Active section (empty string) displays as "Active".
func sectionDisplayName(section string) string {
	if section == "" {
		return "Active"
	}
	return section
}

// renderDetailsPanel renders the task details panel for the bottom half of the split view.
// Displays task info in cards: Task | Info+Due stacked | Description (double width)
func (m *KanbanViewModel) renderDetailsPanel(task *models.Task, width, height int) string {
	if task == nil {
		// Center "No task selected" message
		emptyMsg := MutedStyle.Render("No task selected")
		lines := make([]string, height)
		midLine := height / 2
		for i := range lines {
			if i == midLine {
				// Center horizontally
				padding := (width - lipgloss.Width(emptyMsg)) / 2
				if padding < 0 {
					padding = 0
				}
				lines[i] = strings.Repeat(" ", padding) + emptyMsg
			} else {
				lines[i] = ""
			}
		}
		return strings.Join(lines, "\n")
	}

	// Get section color for accent
	// Prefer custom section color when available
	sectionColor := m.Project.GetSectionColor(task.Section)
	if task.CustomSection != "" {
		sectionColor = m.Project.GetSectionColor(task.CustomSection)
	}

	// Calculate column widths: 3 columns, but Description takes 2 columns worth
	col1Width := width / 4                     // Task card
	col2Width := width / 4                     // Info + Due stacked
	col3Width := width - col1Width - col2Width // Description (remaining ~half)

	// Column 1: Task card
	taskCard := m.renderDetailsTaskCard(task, col1Width, sectionColor)

	// Column 2: Info + Due stacked vertically
	infoCard := m.renderDetailsInfoCard(task, col2Width, sectionColor)
	var col2Content string
	if task.DueDate != nil {
		dueCard := m.renderDetailsDueCard(task, col2Width)
		col2Content = lipgloss.JoinVertical(lipgloss.Left, infoCard, dueCard)
	} else {
		col2Content = infoCard
	}

	// Column 3: Description (double width)
	var descCard string
	if task.Description != "" {
		descCard = m.renderDetailsDescriptionCard(task, col3Width, height)
	} else {
		descCard = renderSidebarCard("Description", MutedStyle.Render("(none)"), col3Width, false, "")
	}

	// Join columns horizontally
	content := lipgloss.JoinHorizontal(lipgloss.Top, taskCard, col2Content, descCard)

	// Pad to exact height (minus 1 for help hint line which is part of app status bar)
	return m.padDetailsToHeight(content, width, height)
}

// renderDetailsTaskCard renders the main task info card for the details panel.
func (m *KanbanViewModel) renderDetailsTaskCard(task *models.Task, width int, sectionColor string) string {
	typeStyle := GetTypeStyle(string(task.Type))
	statusStyle := GetStatusStyle(string(task.Status))

	if m.Project.HasIncompleteDependencies(task) {
		statusStyle = BlockedByDepsStyle
	}

	title := SidebarHeaderStyle.Render(task.Title)
	typeLine := GetTypeBadge(string(task.Type), task.Type.Icon()) + " " + typeStyle.Render(task.Type.HeadingPrefix())
	statusLine := statusStyle.Render(task.Status.Icon() + " " + string(task.Status))
	priorityBadge := renderPriorityBadge(task.Priority)
	idLine := fmt.Sprintf("#%d %s", task.ShortID, priorityBadge)

	content := title + "\n" + typeLine + " " + statusLine + " " + idLine
	return renderSidebarCard("Task", content, width, m.detailsPaneFocused, sectionColor)
}

// renderDetailsInfoCard renders tags and section info for the details panel.
func (m *KanbanViewModel) renderDetailsInfoCard(task *models.Task, width int, sectionColor string) string {
	var lines []string

	// Tags line
	if len(task.Tags) > 0 {
		var tagParts []string
		for _, tag := range task.Tags {
			tagParts = append(tagParts, TagInlineStyle.Render("#"+tag))
		}
		lines = append(lines, strings.Join(tagParts, " "))
	}

	// Section line (GTD section)
	sectionName := task.Section
	if sectionName == "" {
		sectionName = "Active"
	}
	sectionStyle := GetSectionColorStyle(sectionColor)
	sectionLine := MutedStyle.Render("Section: ") + sectionStyle.Render(sectionName)
	lines = append(lines, sectionLine)

	// Custom section line
	if task.CustomSection != "" {
		csColor := m.Project.GetSectionColor(task.CustomSection)
		csStyle := GetSectionColorStyle(csColor)
		csLine := MutedStyle.Render("Custom:  ") + csStyle.Render(task.CustomSection)
		lines = append(lines, csLine)
	}

	return renderSidebarCard("Info", strings.Join(lines, "\n"), width, false, sectionColor)
}

// renderDetailsDueCard renders the due date card for the details panel.
func (m *KanbanViewModel) renderDetailsDueCard(task *models.Task, width int) string {
	dueAbsolute := task.DueDate.Format("2006-01-02")
	dueRelative := formatRelativeDueDateVerbose(task.DueDate)
	dueLine := fmt.Sprintf("%s (%s)", dueAbsolute, dueRelative)

	if isOverdue(task) && !task.Status.IsComplete() {
		dueLine = OverdueStyle.Render(dueLine)
	} else if isDueToday(task) {
		dueLine = DueTodayStyle.Render(dueLine)
	} else {
		dueLine = DueDateStyle.Render(dueLine)
	}

	return renderSidebarCard("Due", dueLine, width, false, "")
}

// renderDetailsDescriptionCard renders the scrollable description card.
func (m *KanbanViewModel) renderDetailsDescriptionCard(task *models.Task, width, availableHeight int) string {
	lines := strings.Split(task.Description, "\n")

	// Apply scroll offset
	if m.detailsScroll < len(lines) {
		lines = lines[m.detailsScroll:]
	} else {
		lines = []string{}
	}

	// Render with all visible lines, then iteratively reduce until the card
	// fits within availableHeight. This accounts for text wrapping by lipgloss.
	count := len(lines)
	for count > 0 {
		content := strings.Join(lines[:count], "\n")
		if count < len(lines) {
			content += "\n" + MutedStyle.Render("...")
		}
		rendered := renderSidebarCard("Description", content, width, false, "")
		if lipgloss.Height(rendered) <= availableHeight {
			return rendered
		}
		count--
	}

	return renderSidebarCard("Description", MutedStyle.Render("..."), width, false, "")
}

// padDetailsToHeight ensures the details panel fills the exact height.
func (m *KanbanViewModel) padDetailsToHeight(content string, width, height int) string {
	lines := strings.Split(content, "\n")

	// Truncate if too tall
	if len(lines) > height {
		lines = lines[:height]
	}

	// Pad if too short
	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// ShouldCaptureEsc implements ViewCapabilities. Returns true when in any
// internal mode that should capture Esc instead of allowing app-level
// navigation. Currently tracks pending status prefix mode, details pane focus,
// search mode, quick add mode, delete confirmation, note conversion confirmation,
// archive confirmation, focus mode, multi-select mode, quick edit mode, completed view mode, and all view mode.
func (m *KanbanViewModel) ShouldCaptureEsc() bool {
	return m.pendingPrefix != "" || m.detailsPaneFocused || m.searchMode || m.quickAddMode || m.confirm.IsActive() || m.FocusedTask != nil || m.multiSelectMode || (m.quickEdit != nil && m.quickEdit.IsActive()) || m.completedViewMode || m.allViewMode || m.scoreFilterInputMode
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *KanbanViewModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints: PrefixHints{
			",":   StatusPrefixHints,
			"!":   PriorityPrefixHints,
			"d":   DueDatePrefixHints,
			"g":   KanbanNavigationPrefixHints,
			"c":   CreateLinkedPrefixHints,
			"t":   TaskPrefixHints,
			"b":   BlockerPrefixHints,
			"z":   KanbanFoldPrefixHints,
			"F":   m.buildKanbanFilterPrefixHints(),
			"Ft":  FilterTypePrefixHints,
			"Fs":  FilterStatusPrefixHints,
			"Fd":  FilterDuePrefixHints,
			"Fa":  FilterAttrPrefixHints,
			"Fas": FilterAttrSeverityPrefixHints,
			"Far": FilterAttrReproduciblePrefixHints,
			"Fae": FilterAttrEffortPrefixHints,
			"Fac": FilterAttrCategoryPrefixHints,
			"Fav": FilterAttrValuePrefixHints,
			"Faw": FilterAttrWorktypePrefixHints,
			"Fao": FilterAttrScopePrefixHints,
			"Fap": FilterAttrPointsPrefixHints,
			"FS":  BuildFilterScorePrefixHints(m.config),
		},
	}
}
