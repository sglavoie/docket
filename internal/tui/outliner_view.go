package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// Tree connector characters for outliner rendering.
const (
	connectorBranch   = "├── "
	connectorLastItem = "└── "
	connectorPipe     = "│   "
	connectorSpace    = "    "
)

// outlinerRow represents a single flattened row for rendering.
type outlinerRow struct {
	task            *models.Task
	prefix          string
	IsSectionHeader bool   // True if this row is a section header (not a task)
	SectionName     string // Section name for header rows
	SectionCount    int    // Number of tasks in this section
}

// buildTreePrefix computes the connector prefix string from the ancestor
// is-last-child boolean stack. Each true entry means that ancestor was the
// last child of its parent (so we render spaces), while false means we need
// a continuation pipe.
func buildTreePrefix(ancestorIsLast []bool, isLast bool) string {
	var b strings.Builder
	for _, last := range ancestorIsLast {
		if last {
			b.WriteString(connectorSpace)
		} else {
			b.WriteString(connectorPipe)
		}
	}
	if isLast {
		b.WriteString(connectorLastItem)
	} else {
		b.WriteString(connectorBranch)
	}
	return b.String()
}

// flattenTaskTree recursively walks the task tree and produces a flat list of
// outlinerRows with pre-computed connector prefixes. The ancestorIsLast stack
// tracks whether each ancestor is its parent's last child, which determines
// whether continuation pipes or spaces are drawn.
func flattenTaskTree(tasks []*models.Task, ancestorIsLast []bool, rows *[]outlinerRow) {
	for i, task := range tasks {
		isLast := i == len(tasks)-1
		prefix := ""
		if len(ancestorIsLast) > 0 || len(tasks) > 0 {
			prefix = buildTreePrefix(ancestorIsLast, isLast)
		}
		*rows = append(*rows, outlinerRow{task: task, prefix: prefix})

		if len(task.Children) > 0 {
			flattenTaskTree(task.Children, append(ancestorIsLast, isLast), rows)
		}
	}
}

// matchesSearch checks if a task matches the search query by fuzzy-matching
// the title, tags, description, and comments.
func matchesSearch(task *models.Task, query string) bool {
	return TaskMatchesSearch(task, query)
}

// flattenTaskTreeWithFolds recursively walks the task tree and produces a flat
// list of outlinerRows, respecting fold state. Collapsed nodes (not in
// expandedNodes or explicitly false) stop recursion, showing only the parent.
// When expandedNodes is empty (default), all nodes are expanded.
func (m *OutlinerModel) flattenTaskTreeWithFolds(tasks []*models.Task, ancestorIsLast []bool, rows *[]outlinerRow) {
	for i, task := range tasks {
		isLast := i == len(tasks)-1
		prefix := ""
		if len(ancestorIsLast) > 0 || len(tasks) > 0 {
			prefix = buildTreePrefix(ancestorIsLast, isLast)
		}
		*rows = append(*rows, outlinerRow{task: task, prefix: prefix})

		if len(task.Children) > 0 && m.isExpanded(task.ID) {
			m.flattenTaskTreeWithFolds(task.Children, append(ancestorIsLast, isLast), rows)
		}
	}
}

// isExpanded returns true if the node should show its children.
// When the expandedNodes map is empty (default), all nodes are expanded.
func (m *OutlinerModel) isExpanded(taskID string) bool {
	if len(m.expandedNodes) == 0 {
		return true
	}
	expanded, exists := m.expandedNodes[taskID]
	if !exists {
		return true // Default to expanded
	}
	return expanded
}

// breadcrumb returns the rendered breadcrumb string showing the ancestor chain
// from the first zoom level to the current zoom target. Returns empty string
// when not zoomed. If the breadcrumb exceeds maxWidth, leftmost segments are
// dropped and replaced with "..." until it fits.
func (m *OutlinerModel) breadcrumb(maxWidth int) string {
	if len(m.zoomStack) == 0 || m.project == nil {
		return ""
	}

	separator := " > "
	var segments []string
	for _, entry := range m.zoomStack {
		if t := m.project.GetTaskByID(entry.TaskID); t != nil {
			segments = append(segments, t.Title)
		}
	}
	if len(segments) == 0 {
		return ""
	}

	full := strings.Join(segments, separator)
	rendered := MutedStyle.Render(full)
	if lipgloss.Width(rendered) <= maxWidth {
		return rendered
	}

	// Truncate from the left, replacing dropped segments with "...".
	ellipsis := "..."
	for len(segments) > 1 {
		segments = segments[1:]
		candidate := ellipsis + separator + strings.Join(segments, separator)
		rendered = MutedStyle.Render(candidate)
		if lipgloss.Width(rendered) <= maxWidth {
			return rendered
		}
	}

	// Even the last segment alone with ellipsis doesn't fit; render it anyway.
	return MutedStyle.Render(ellipsis + separator + segments[0])
}

// View implements tea.Model. It renders the full outliner tree with connector
// prefixes computed on each render pass from the project's task hierarchy.
func (m *OutlinerModel) View() string {
	if m.project == nil {
		return "No project loaded"
	}

	// Rebuild flat row list with perspective filter and clamp cursor.
	m.rebuildRows()

	if m.sidebar.IsEnabled() {
		return m.renderWithSidebar()
	}
	return m.renderFullWidth()
}

// renderFullWidth renders the outliner tree using the full terminal width.
func (m *OutlinerModel) renderFullWidth() string {
	return m.renderOutlinerTreeContent(m.width)
}

// renderWithSidebar renders the split-pane view with tree on left and sidebar on right.
func (m *OutlinerModel) renderWithSidebar() string {
	treeWidth := (m.width * 55) / 100
	if treeWidth < TreeMinWidth {
		treeWidth = TreeMinWidth
	}

	dividerWidth := 1
	sidebarWidth := m.width - treeWidth - dividerWidth
	if sidebarWidth < SidebarMinWidth {
		sidebarWidth = SidebarMinWidth
		treeWidth = m.width - sidebarWidth - dividerWidth
	}

	treeContent := m.renderOutlinerTreeContent(treeWidth)

	var selectedTask *models.Task
	if len(m.rows) > 0 && m.cursor >= 0 && m.cursor < len(m.rows) {
		row := m.rows[m.cursor]
		if !row.IsSectionHeader {
			selectedTask = row.task
		}
	}

	sidebarContent := renderSidebarContent(m, selectedTask, sidebarWidth-SidebarContentOverhead, m.height-CardVerticalBorderOverhead)
	treeContentStyled := lipgloss.NewStyle().Width(treeWidth).Render(treeContent)
	divider := renderVerticalDivider(m.height - 2)

	return lipgloss.JoinHorizontal(lipgloss.Top, treeContentStyled, divider, sidebarContent)
}

// renderOutlinerTreeContent renders the tree portion of the outliner, constrained to
// the given width. Used by both full-width and split-pane layouts.
func (m *OutlinerModel) renderOutlinerTreeContent(width int) string {
	// Capture and clear the transient zoom notice (shown for one render).
	notice := m.zoomNotice
	m.zoomNotice = ""

	perspective := m.currentPerspective()

	// Show perspective label only for non-All perspectives.
	perspectiveLabel := ""
	if perspective.Name != "All" {
		perspectiveLabel = "  " + MutedStyle.Render("["+perspective.Name+"]")
	}

	bc := m.breadcrumb(width)

	// Build search/filter indicators
	searchIndicator := ""
	if m.IsSearchMode() {
		searchIndicator = "  " + MutedStyle.Render("Search: ") + m.SearchInputView()
	} else if m.SearchQuery() != "" {
		searchIndicator = "  " + MutedStyle.Render(fmt.Sprintf("Search: \"%s\"", m.SearchQuery()))
	}

	filterBadge := ""
	if hasActiveCommonFilters(&m.filter.CommonFilterState) || m.filter.Actionable || len(m.filter.Sections) > 0 {
		filterBadge = "  " + MutedStyle.Render("[filtered]")
	}

	sortBadge := ""
	if m.sort.Field != SortByDefault {
		direction := "↑"
		if !m.sort.Ascending {
			direction = "↓"
		}
		sortBadge = "  " + MutedStyle.Render(fmt.Sprintf("Sort: %s %s", m.sort.Field, direction))
	}

	if len(m.rows) == 0 {
		header := TitleStyle.Render(m.project.Name) + perspectiveLabel + filterBadge + sortBadge + searchIndicator
		emptyMsg := EmptyStateNoTasks
		if perspective.Name != "All" && len(m.zoomStack) > 0 {
			topID := m.zoomStack[len(m.zoomStack)-1].TaskID
			if t := m.project.GetTaskByID(topID); t != nil {
				emptyMsg = fmt.Sprintf("No tasks match the %s perspective within %q.", perspective.Name, t.Title)
			} else {
				emptyMsg = fmt.Sprintf("No tasks match the %s perspective.", perspective.Name)
			}
		} else if perspective.Name != "All" {
			emptyMsg = fmt.Sprintf("No tasks match the %s perspective.", perspective.Name)
		}
		result := header + "\n"
		if tabs := m.customSectionTabs(); tabs != nil {
			result += RenderColoredTabBar(tabs, m.customSectionTab, width, 1, m.project.SectionColors) + "\n"
		}
		if bc != "" {
			result += bc + "\n"
		}
		result += MutedStyle.Render(emptyMsg)
		if notice != "" {
			result += "\n\n" + PreviewWarningStyle.Render(notice)
		}
		return result
	}

	var b strings.Builder
	b.WriteString(TitleStyle.Render(m.project.Name))
	b.WriteString(perspectiveLabel)
	b.WriteString(filterBadge)
	b.WriteString(sortBadge)
	b.WriteString(searchIndicator)
	if m.multiSelectMode {
		count := m.SelectedCount()
		b.WriteString("  ")
		b.WriteString(SelectionCountStyle.Render(fmt.Sprintf("[MULTI-SELECT: %d selected]", count)))
	}
	b.WriteString("\n")

	// Custom section tab bar
	if tabs := m.customSectionTabs(); tabs != nil {
		b.WriteString(RenderColoredTabBar(tabs, m.customSectionTab, width, 1, m.project.SectionColors))
		b.WriteString("\n")
	}

	if bc != "" {
		b.WriteString(bc)
		b.WriteString("\n")
	}

	// Compute scroll window.
	visibleRows := m.contentHeight()
	startIdx := calcScrollOffset(m.cursor, visibleRows)
	endIdx := min(startIdx+visibleRows, len(m.rows))

	if startIdx > 0 {
		b.WriteString(ScrollUpIndicator(startIdx))
		b.WriteString("\n")
	}

	for i := startIdx; i < endIdx; i++ {
		row := m.rows[i]
		selected := i == m.cursor

		// Handle section header rows
		if row.IsSectionHeader {
			// Display name for empty section (Active)
			displayName := row.SectionName
			if displayName == "" {
				displayName = "Active"
			}

			// Get colored style for this section
			sectionStyle := GetViewModeStyleForSection(row.SectionName)

			// Triangle indicator (▾ expanded, ▸ collapsed)
			indicator := "▾"
			if m.collapsedSections[row.SectionName] {
				indicator = "▸"
			}

			// Cursor prefix
			cursorPrefix := "  "
			if selected {
				cursorPrefix = SelectionCursorStyle.Render(">") + " "
			}

			// Render colored badge with @ symbol at the very left
			badge := sectionStyle.Render(fmt.Sprintf("@ %s", displayName))
			line := fmt.Sprintf("%s%s %s (%d)", cursorPrefix, MutedStyle.Render(indicator), badge, row.SectionCount)

			if selected {
				line = SelectionRowStyle.Width(width).Render(line)
			}

			b.WriteString(line)
			if i < endIdx-1 || endIdx < len(m.rows) {
				b.WriteString("\n")
			}
			continue
		}

		// Regular task row rendering
		task := row.task
		// Fold indicator for parent tasks
		foldIndicator := ""
		if len(task.Children) > 0 {
			if m.isExpanded(task.ID) {
				foldIndicator = MutedStyle.Render("▾") + " "
			} else {
				foldIndicator = MutedStyle.Render("▸") + " "
			}
		}

		typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())
		statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())
		if m.project.HasIncompleteDependencies(task) {
			statusIcon = BlockedByDepsStyle.Render(task.Status.Icon())
		}

		// Title with priority coloring
		titleStyle := NormalStyle
		if task.Status == models.StatusCanceled {
			titleStyle = CanceledStyle
		} else if task.Active {
			titleStyle = ActiveStyle
		} else if task.Pinned {
			titleStyle = SuccessStyle
		}

		// Build inline indicators reusing existing helpers
		var indicators strings.Builder

		// Short ID
		if m.columnConfig.IsVisible(ColShortID) {
			indicators.WriteString(" ")
			indicators.WriteString(MutedStyle.Render(fmt.Sprintf("#%d", task.ShortID)))
		}

		// Priority badge
		if m.columnConfig.ShouldShowPriority(task.Priority) {
			indicators.WriteString(" ")
			indicators.WriteString(renderPriorityBadge(task.Priority))
		}

		// Due date badge
		if m.columnConfig.IsVisible(ColDueDate) {
			if dueStr := formatDueDate(task); dueStr != "" {
				indicators.WriteString(" ")
				indicators.WriteString(dueStr)
			}
		}

		// Snooze indicator
		if m.columnConfig.IsVisible(ColSnooze) && task.SnoozedUntil != nil {
			snoozeStr := task.SnoozedUntil.Format("Jan 2")
			if task.SnoozedUntil.Year() != time.Now().Year() {
				snoozeStr = task.SnoozedUntil.Format("Jan 2 '06")
			}
			indicators.WriteString(" ")
			indicators.WriteString(MutedStyle.Render("zzz " + snoozeStr))
		}

		// Tag badges (first 2 tags, truncated)
		if m.columnConfig.IsVisible(ColTags) {
			if s := renderTruncatedTags(task.Tags, 2, 8); s != "" {
				indicators.WriteString(" ")
				indicators.WriteString(s)
			}
		}

		// Score badge (only when scoring profile is active and task has scores)
		if m.columnConfig.IsVisible(ColScore) && m.config != nil {
			if profile := m.config.GetProjectScoringProfile(m.project.ID); profile != nil {
				if HasManualScores(task, profile) {
					indicators.WriteString(" ")
					indicators.WriteString(RenderScoreBadge(task, profile))
				}
			}
		}

		// Comment indicator
		if m.columnConfig.IsVisible(ColComments) {
			if s := renderCommentIndicator(task); s != "" {
				indicators.WriteString(" ")
				indicators.WriteString(s)
			}
		}

		// Attachment indicator
		if m.columnConfig.IsVisible(ColAttachments) {
			if s := renderAttachmentIndicator(task); s != "" {
				indicators.WriteString(" ")
				indicators.WriteString(s)
			}
		}

		// Checklist indicator
		if m.columnConfig.IsVisible(ColChecklist) {
			if s := renderChecklistIndicator(task); s != "" {
				indicators.WriteString(" ")
				indicators.WriteString(s)
			}
		}

		// Recurrence indicator (with streak count when > 0)
		if m.columnConfig.IsVisible(ColRecurrence) && task.IsRecurring() {
			recurStr := "↻"
			if task.Recurrence.Streak > 0 {
				recurStr = fmt.Sprintf("↻%d", task.Recurrence.Streak)
			}
			indicators.WriteString(" ")
			indicators.WriteString(MutedStyle.Render(recurStr))
		}

		// Dependency indicators (only when sidebar is hidden)
		if m.columnConfig.IsVisible(ColDependencies) && !m.sidebar.IsEnabled() {
			if s := renderDependencyIndicators(task, m.project); s != "" {
				indicators.WriteString(" ")
				indicators.WriteString(s)
			}
		}

		// Custom section badge
		if m.columnConfig.IsVisible(ColCustomSection) && task.CustomSection != "" {
			csColor := m.project.GetSectionColor(task.CustomSection)
			csStyle := GetSectionColorStyle(csColor)
			indicators.WriteString(" ")
			indicators.WriteString(csStyle.Render("@ " + task.CustomSection))
		}

		// Progress for parent tasks
		if m.columnConfig.IsVisible(ColProgress) {
			if frac := renderProgressFraction(task); frac != "" {
				indicators.WriteString(" ")
				indicators.WriteString(frac)
			}
		}

		// Section badge (only in non-All perspectives to show which section the task belongs to)
		p := m.currentPerspective()
		if p.Name != "All" {
			sectionName := task.Section
			if sectionName == "" {
				sectionName = "Active"
			}
			sectionStyle := GetViewModeStyleForSection(task.Section)
			indicators.WriteString(" ")
			indicators.WriteString(sectionStyle.Render("@ " + sectionName))
		}

		cursorPrefix := RenderCursorPrefix(selected, task.Active, task.Pinned)

		// Selection checkbox (only in multi-select mode)
		checkbox := ""
		if m.multiSelectMode {
			if m.IsSelected(task.ID) {
				checkbox = SelectionCheckStyle.Render("[x]") + " "
			} else {
				checkbox = MutedStyle.Render("[ ]") + " "
			}
		}

		// Truncate title to fit available width
		availableWidth := max(width-lipgloss.Width(row.prefix)-lipgloss.Width(cursorPrefix)-lipgloss.Width(checkbox)-12-lipgloss.Width(indicators.String()), 10)
		title := titleStyle.Render(util.Truncate(task.Title, availableWidth))

		line := fmt.Sprintf("%s%s%s%s%s %s %s%s",
			cursorPrefix,
			checkbox,
			MutedStyle.Render(row.prefix),
			foldIndicator,
			typeIcon, statusIcon, title, indicators.String())

		if selected {
			line = SelectionRowStyle.Width(width).Render(line)
		}

		b.WriteString(line)
		if i < endIdx-1 || endIdx < len(m.rows) {
			b.WriteString("\n")
		}
	}

	if endIdx < len(m.rows) {
		b.WriteString(ScrollDownIndicator(len(m.rows) - endIdx))
	}

	if notice != "" {
		b.WriteString("\n")
		b.WriteString(PreviewWarningStyle.Render(notice))
		b.WriteString("\n")
	}

	// Render quick edit bar if active
	if m.quickEdit.IsActive() {
		b.WriteString("\n")
		b.WriteString(m.renderQuickEditBar())
		b.WriteString("\n")
	}

	// Render status message or confirm prompt
	if m.statusMessage != "" {
		b.WriteString("\n")
		if m.confirm.IsActive() {
			b.WriteString(PreviewWarningStyle.Render(m.statusMessage))
		} else {
			b.WriteString(MutedStyle.Render(m.statusMessage))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderQuickEditBar renders the quick edit input bar at the bottom of the view.
func (m *OutlinerModel) renderQuickEditBar() string {
	if !m.quickEdit.IsActive() {
		return ""
	}

	var lines []string

	mode := "Quick Edit"
	if m.quickEdit.IsRenameOnly() {
		mode = "Rename"
	}
	modeLabel := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render(mode + ":")

	inputLine := modeLabel + " " + m.quickEdit.GetInput().View()
	lines = append(lines, inputLine)

	// Preview line (only for full quick edit mode)
	if !m.quickEdit.IsRenameOnly() {
		preview := m.quickEdit.GetPreview()
		original := m.quickEdit.GetOriginal()
		if preview != nil && original != nil {
			previewStr := renderQuickEditPreview(preview, original)
			if previewStr != "" {
				lines = append(lines, MutedStyle.Render(previewStr))
			}
		}
	}

	helpHint := MutedStyle.Render("Enter=save  Esc=cancel")
	if !m.quickEdit.IsRenameOnly() {
		helpHint = MutedStyle.Render("!1-5=priority #tag @date /section $status · Enter=save Esc=cancel")
	}
	lines = append(lines, helpHint)

	return strings.Join(lines, "\n")
}
