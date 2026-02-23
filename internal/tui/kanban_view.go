package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
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

	// Render swimlane board when swimlanes are active
	if m.swimlaneEnabled() {
		return m.renderSwimlaneBoardView()
	}

	var viewParts []string

	// Render custom section tab bar above preset tabs (when custom sections exist and in section mode)
	hasCustomSections := m.customSectionTabs() != nil && IsSectionGroupBy(m.groupBy)
	if hasCustomSections {
		unfocusedPad := " "

		csBar := RenderColoredTabBar(m.customSectionTabs(), m.customSectionTab, m.Width()-1, 1, m.Project.SectionColors)
		if m.customSectionRowFocused {
			csBar = FocusIndicator + csBar
		} else {
			csBar = unfocusedPad + csBar
		}
		viewParts = append(viewParts, csBar)
	}

	// Render tab bar at the top
	presetBar := m.renderKanbanTabBar()
	if hasCustomSections {
		unfocusedPad := " "
		if !m.customSectionRowFocused {
			presetBar = FocusIndicator + presetBar
		} else {
			presetBar = unfocusedPad + presetBar
		}
	}
	viewParts = append(viewParts, presetBar)

	// Render group-by indicator when not in default section mode
	if !IsSectionGroupBy(m.groupBy) {
		indicator := MutedStyle.Render("Grouped by: ") + lipgloss.NewStyle().Foreground(ColorPrimary).Render(GroupByLabel(m.groupBy))
		viewParts = append(viewParts, indicator)
	}

	// Render search input if in search mode
	if m.IsSearchMode() {
		searchLine := "Search: " + m.SearchInputView()
		viewParts = append(viewParts, searchLine)
	}

	// Render score filter custom value input
	if m.scoreFilterInput.IsActive() {
		scoreLine := SubtitleStyle.Render("Score threshold: ") + m.scoreFilterInput.View() + " " + HelpStyle.Render("(Enter: apply, Esc: cancel)")
		viewParts = append(viewParts, scoreLine)
	}

	var columns []string
	gap := " " // Single space gap between columns

	// Only render the visible column range (horizontal scrolling)
	visStart, visEnd := m.visibleColumnRange()

	// Render scroll position line when horizontal scrolling is active
	if len(m.columnDefs) > m.maxVisibleColumns() {
		var scrollParts []string
		if visStart > 0 {
			scrollParts = append(scrollParts, fmt.Sprintf("◀ %d", visStart))
		}
		scrollParts = append(scrollParts, fmt.Sprintf("Columns %d–%d / %d", visStart+1, visEnd, len(m.columnDefs)))
		if visEnd < len(m.columnDefs) {
			scrollParts = append(scrollParts, fmt.Sprintf("%d ▶", len(m.columnDefs)-visEnd))
		}
		scrollLine := MutedStyle.Render(strings.Join(scrollParts, "  "))
		viewParts = append(viewParts, scrollLine)
	}

	for i := visStart; i < visEnd; i++ {
		def := m.columnDefs[i]
		columnContent := m.renderColumn(def.Key, i == m.ActiveColumn)
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
	for _, def := range m.columnDefs {
		totalTasks += len(m.Tasks[def.Key])
	}
	if totalTasks == 0 && m.HasActiveFilters() {
		// Show message when filters result in no tasks
		emptyMsg := MutedStyle.Render(EmptyStateFiltered)
		kanbanPane = lipgloss.Place(m.Width(), m.kanbanHeight, lipgloss.Center, lipgloss.Center, emptyMsg)
	}

	viewParts = append(viewParts, kanbanPane)

	// Render the horizontal divider
	var dividerOpts []DividerOption
	if m.detailsPane.Focused {
		dividerOpts = append(dividerOpts, WithDividerColor(ColorPrimary))
	}
	divider := renderHorizontalDivider(m.Width(), dividerOpts...)
	viewParts = append(viewParts, divider)

	// Render the details panel for the selected task
	tabBarHeight := TabBarHeight
	if m.customSectionTabs() != nil && IsSectionGroupBy(m.groupBy) {
		tabBarHeight += TabBarHeight
	}
	if !IsSectionGroupBy(m.groupBy) {
		tabBarHeight++ // Group-by indicator line
	}
	if len(m.columnDefs) > m.maxVisibleColumns() {
		tabBarHeight++ // Horizontal scroll position line
	}
	searchBarHeight := 0
	if m.IsSearchMode() {
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
	m.detailsPane.Height = detailsHeight
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
	if msg := renderConfirmAwareStatus(m.statusMessage, m.confirm.IsActive()); msg != "" {
		viewParts = append(viewParts, msg)
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

	active := activeTabStyle(1, true)
	if dimmed {
		active = inactiveTabStyle(1, true)
	}
	inactive := inactiveTabStyle(1, true)

	var tabs []string
	for i, name := range kanbanTabNames {
		if i == m.tabs.Active {
			tabs = append(tabs, active.Render(name))
		} else {
			tabs = append(tabs, inactive.Render(name))
		}
	}

	row := lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)

	remaining := availableWidth - lipgloss.Width(row)
	if filler := renderTabFiller(remaining); filler != "" {
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
			b.WriteString(ScrollUpIndicator(startIdx))
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
			isSelected := i == cs.Cursor && isActive && !m.detailsPane.Focused
			line := m.renderTaskRow(node, isSelected)
			b.WriteString(line)
			if i < endIdx-1 {
				b.WriteString("\n")
			}
		}

		// Show "↓ N more" indicator if tasks are hidden below
		if endIdx < len(visibleNodes) {
			b.WriteString("\n")
			b.WriteString(ScrollDownIndicator(len(visibleNodes) - endIdx))
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

// renderColumnHeader renders the column header with appropriate color.
// Uses group-by-aware styles: section colors for section mode,
// status/priority/type colors for other modes.
func (m *KanbanViewModel) renderColumnHeader(section string, isActive bool) string {
	displayName := m.columnLabel(section)
	visibleCount := m.visibleNodeCount(section)
	totalCount := len(m.Tasks[section])

	// Show visible/total if different (collapsed nodes)
	var headerText string
	if visibleCount != totalCount {
		headerText = fmt.Sprintf("%s (%d/%d)", displayName, visibleCount, totalCount)
	} else {
		headerText = fmt.Sprintf("%s (%d)", displayName, totalCount)
	}

	// In status mode, show total completed count for Done/Canceled columns
	if m.groupBy == GroupByStatus && IsCompletedStatusKey(section) && m.Project != nil {
		allCount := 0
		for _, task := range m.Project.Tasks {
			if string(task.Status) == section {
				allCount++
			}
		}
		if allCount > totalCount {
			headerText = fmt.Sprintf("%s (%d/%d)", displayName, totalCount, allCount)
		}
	}

	// Append sort indicator if this column has a non-default sort
	sortField := m.getSortFieldForColumn(section)
	if sortField != SortByDefault {
		direction := "↑"
		if isActive && !m.sort.Ascending {
			direction = "↓"
		}
		headerText += " " + direction + sortField.ShortString()
	}

	// Use group-by-aware style
	style := GetColumnHeaderStyle(m.groupBy, section)

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
		if m.IsSelected(task.ID) {
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
			scoreBadgeLen = 9
		}
	}
	maxTitleLen := m.ColumnWidth - prefixLen - badgeLen - scoreBadgeLen
	if maxTitleLen < KanbanMinTitleWidth {
		maxTitleLen = KanbanMinTitleWidth
	}
	title = util.Truncate(title, maxTitleLen)

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
		title = SuccessStyle.Render(title)
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

	return RenderCursorPrefix(isSelected, task.Active, task.Pinned) + content
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

	// Optional rows: checklist, comments, attachments, deps, activity cards
	// Rendered in rows of up to 3 cards, with width distributed evenly per row.
	type cardRenderer func(width int) string
	var cards []cardRenderer
	if len(task.Checklist) > 0 {
		cards = append(cards, func(w int) string { return renderSidebarChecklistCard(task, w) })
	}
	if len(task.Comments) > 0 {
		cards = append(cards, func(w int) string { return renderSidebarCommentsCard(task, w) })
	}
	if task.HasAttachments() {
		cards = append(cards, func(w int) string { return renderSidebarAttachmentsCard(task, w) })
	}
	if depsCard := m.renderDetailsDepsCard(task, 0); depsCard != "" {
		cards = append(cards, func(w int) string { return m.renderDetailsDepsCard(task, w) })
	}
	cards = append(cards, func(w int) string { return renderSidebarActivityCard(task, w) })

	const maxCardsPerRow = 3
	for i := 0; i < len(cards); i += maxCardsPerRow {
		end := i + maxCardsPerRow
		if end > len(cards) {
			end = len(cards)
		}
		chunk := cards[i:end]
		n := len(chunk)
		cardWidth := width / n
		var rowCards []string
		for j, c := range chunk {
			w := cardWidth
			if j == n-1 {
				w = width - cardWidth*(n-1)
			}
			rowCards = append(rowCards, c(w))
		}
		row := lipgloss.JoinHorizontal(lipgloss.Top, rowCards...)
		content = lipgloss.JoinVertical(lipgloss.Left, content, row)
	}

	// Pad to exact height (minus 1 for help hint line which is part of app status bar)
	return m.padDetailsToHeight(content, width, height)
}

// renderDetailsTaskCard renders the main task info card for the details panel.
func (m *KanbanViewModel) renderDetailsTaskCard(task *models.Task, width int, sectionColor string) string {
	content := renderTaskHeaderContent(task, m.Project.HasIncompleteDependencies(task), false)
	return renderSidebarCard("Task", content, width, m.detailsPane.Focused, sectionColor)
}

// renderDetailsInfoCard renders tags and section info for the details panel.
func (m *KanbanViewModel) renderDetailsInfoCard(task *models.Task, width int, sectionColor string) string {
	content := renderMetadataContent(task, m.Project, width-6, false)
	return renderSidebarCard("Info", content, width, false, sectionColor)
}

// renderDetailsDueCard renders the due date card for the details panel.
func (m *KanbanViewModel) renderDetailsDueCard(task *models.Task, width int) string {
	return renderSidebarCard("Due", renderDueDateContent(task), width, false, "")
}

// renderDetailsDepsCard renders a simplified (non-interactive) dependencies card
// for the kanban details panel. Returns empty string if no dependencies exist.
func (m *KanbanViewModel) renderDetailsDepsCard(task *models.Task, width int) string {
	blockedByTasks := m.Project.GetBlockedByTasks(task)
	if len(task.DependsOn) == 0 && len(blockedByTasks) == 0 {
		return ""
	}

	var lines []string

	if len(task.DependsOn) > 0 {
		lines = append(lines, MutedStyle.Render("Blocked by:"))
		for _, depRef := range task.DependsOn {
			depTask := m.Project.GetTaskByID(depRef)
			if depTask != nil {
				depStatus := GetStatusStyle(string(depTask.Status))
				lines = append(lines, fmt.Sprintf("  #%-3d %s %s", depTask.ShortID, depStatus.Render(depTask.Status.Icon()), depTask.Title))
			} else {
				lines = append(lines, "  "+ValidationErrorStyle.Render("(missing)"))
			}
		}
	}

	if len(blockedByTasks) > 0 {
		if len(task.DependsOn) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, MutedStyle.Render("Blocking:"))
		for _, blocked := range blockedByTasks {
			depStatus := GetStatusStyle(string(blocked.Status))
			lines = append(lines, fmt.Sprintf("  #%-3d %s %s", blocked.ShortID, depStatus.Render(blocked.Status.Icon()), blocked.Title))
		}
	}

	return renderSidebarCard("Deps", strings.Join(lines, "\n"), width, false, "")
}

// renderDetailsDescriptionCard renders the description card.
func (m *KanbanViewModel) renderDetailsDescriptionCard(task *models.Task, width, availableHeight int) string {
	lines := strings.Split(task.Description, "\n")

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
// Applies detailsScroll offset to enable scrolling through the full content.
func (m *KanbanViewModel) padDetailsToHeight(content string, width, height int) string {
	lines := strings.Split(content, "\n")

	// Clamp scroll to valid range
	maxScroll := len(lines) - height
	if maxScroll < 0 {
		maxScroll = 0
	}
	m.detailsPane.MaxScroll = maxScroll
	m.detailsPane.ClampScroll()

	// Apply scroll offset
	if m.detailsPane.Scroll > 0 {
		lines = lines[m.detailsPane.Scroll:]
	}

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
// internal mode that should capture Esc instead of allowing app-level navigation.
func (m *KanbanViewModel) ShouldCaptureEsc() bool {
	return primaryViewShouldCaptureEsc(&m.InteractiveViewState, &m.BaseSearchableView, m.multiSelectMode, m.quickEdit != nil && m.quickEdit.IsActive(), m.scoreFilterInput.IsActive()) ||
		m.detailsPane.Focused || m.quickAddMode || AnyEscStepActive(m.escSteps())
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
			"m":   KanbanMovePrefixHints,
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
