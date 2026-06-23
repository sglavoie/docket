package tui

import (
	"fmt"
	"maps"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *DailyReviewModel) View() tea.View {
	totalCount := m.getTotalTaskCount()

	var b strings.Builder

	// Title
	var subtitle string
	if m.reviewCompleted {
		subtitle = fmt.Sprintf("completed today at %s", m.completedAt.Format("15:04"))
	} else if totalCount > 0 {
		progressBar := ""
		if m.reviewedCount > 0 {
			pct := float64(m.reviewedCount) / float64(totalCount) * 100
			if pct > 100 {
				pct = 100
			}
			progressBar = " " + RenderProgressBar(pct, 8)
		}
		subtitle = fmt.Sprintf("(%d/%d reviewed)%s", m.reviewedCount, totalCount, progressBar)
	} else {
		subtitle = "all caught up"
	}
	b.WriteString(renderFullscreenTitle("Review", subtitle, m.AccentColor()))
	b.WriteString("\n")

	if totalCount == 0 {
		b.WriteString("\n")
		if m.HasActiveFilters() {
			b.WriteString(m.renderActiveFilterIndicator())
			b.WriteString("\n\n")
		}
		b.WriteString(renderFilterAwareEmptyState(m.HasActiveFilters(),
			"No tasks to review.", "Your inbox is clear. Press Esc to close.",
			"Press Esc to clear filters."))
		return tea.NewView(b.String())
	}

	// Tab bar + filter badge
	chrome, chromeLines := renderFullscreenChrome(
		m.renderReviewTabBar(),
		m.renderActiveFilterIndicator(),
		true,
	)
	b.WriteString(chrome)

	// Calculate available height for content
	contentHeight := fullscreenContentHeight(m.height, chromeLines)

	// Render all sections
	if m.totalTasks == 0 {
		b.WriteString(renderFilterAwareEmptyState(m.HasActiveFilters(),
			"No tasks in this tab.", "shift+←/→: tabs · ? help",
			"shift+←/→: tabs · Fx clear filters · Esc clear filters · ? help"))
		return tea.NewView(b.String())
	}

	lines, cursorLineIndex := m.renderAllSections()

	// Account for preview lines in scroll calculation
	if m.previewMode {
		task := m.getTaskAtCursor()
		if task != nil {
			previewCount := inlinePreviewLineCount(task.Task)
			contentHeight -= previewCount
			if contentHeight < 1 {
				contentHeight = 1
			}
		}
	}

	// Reserve one row for the selected-task metadata strip whenever the cursor
	// has something to land on; the strip text is empty on section headers,
	// but the reserved row keeps layout stable as the cursor moves.
	stripTask := m.getTaskAtCursor()
	stripReserved := len(m.sectionOrder) > 0
	if stripReserved {
		contentHeight = max(contentHeight-1, 1)
	}

	contentStr := strings.Join(applyPersistentScroll(lines, cursorLineIndex, contentHeight, &m.scrollOffset), "\n")
	b.WriteString(TabContentBgStyle.Width(m.width).Render(contentStr))

	if stripReserved {
		stripText := ""
		if stripTask != nil {
			section := m.sectionOrder[getCurrentSectionIdx(m.cursor, m.sectionStarts)]
			stripText = renderTaskMetadataStrip(TaskMetadataStripOpts{
				Task:    stripTask.Task,
				Project: stripTask.Project,
				Config:  m.config,
				Width:   m.width,
				AlreadyShown: TaskMetadataAlreadyShown{
					Project:  true,
					Priority: stripTask.Task.Priority >= 1 && stripTask.Task.Priority <= 2,
					Due:      stripTask.Task.DueDate != nil,
					Stale:    true,
					Updated:  section == SectionOld,
				},
			})
		}
		b.WriteString("\n" + stripText)
	}

	// Footer
	reviewHint := "R: mark reviewed"
	if m.reviewCompleted {
		reviewHint = "R: unmark"
	}
	left := HelpStyle.Render("shift+←/→ tabs · n/N section · " + reviewHint + " · ? help")
	if m.multiSelectMode {
		left = HelpStyle.Render("MULTI-SELECT: space toggle · a all · A deselect · ,/! status/priority · v exit")
	}
	right := MutedStyle.Render(fmt.Sprintf("%d tasks", totalCount))
	b.WriteString(renderFullscreenFooterBlock(m.statusMessage, m.err, left, right, m.width, m.AccentColor()))

	return tea.NewView(b.String())
}

// renderReviewTabBar renders the tab bar for daily review sections.
func (m *DailyReviewModel) renderReviewTabBar() string {
	tabCounts := []int{
		len(m.overdue) + len(m.dueToday) + len(m.dueSoon) + len(m.dueLater) + len(m.future),
		len(m.inProgress) + len(m.blocked),
		len(m.inReview) + len(m.inStandby),
		len(m.old),
		len(m.completed),
	}

	labels := make([]string, len(m.tabs.Names))
	for i, name := range m.tabs.Names {
		labels[i] = fmt.Sprintf("%s (%d)", name, tabCounts[i])
	}

	emptyTabs := make([]bool, len(tabCounts))
	for i, c := range tabCounts {
		emptyTabs[i] = c == 0
	}
	return renderWrappingTabBarWithEmpty(labels, m.tabs.Active, emptyTabs, m.width, m.AccentColor())
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *DailyReviewModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	var currentStage string
	var currentPriority int
	if ct := m.getTaskAtCursor(); ct != nil {
		currentStage = ct.Task.Stage
		currentPriority = ct.Task.Priority
	}
	hints := CommonTaskActionPrefixHints(m.config.Stages, currentStage, currentPriority)
	// In DailyReview, marking done also auto-advances the next recurring occurrence
	hints[","] = overrideHintDetail(hints[","], "d", "done + auto-advance")
	hints["g"] = DailyReviewNavigationPrefixHints
	hints["f"] = []PrefixHint{
		{Key: "a", Description: "toggle", Detail: "Toggle section fold"},
		{Key: "p", Description: "preview", Detail: "Toggle inline preview"},
	}
	// Merge filter prefix hints (no DueDate or Status — sections group by these)
	maps.Copy(hints, buildCommonFilterPrefixPopupHints(&m.filter, m.config, false, false))
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints:  hints,
	}
}

// DailyReviewNavigationPrefixHints contains "g" prefix hints for daily review.
var DailyReviewNavigationPrefixHints = []PrefixHint{
	{Key: "g", Description: "first", Detail: "Jump to first item"},
	{Key: "p", Description: "go to project", Detail: "Navigate to project"},
	{Key: "d", Description: "overdue", Detail: "Focus Overdue section"},
	{Key: "t", Description: "today", Detail: "Focus Today section"},
	{Key: "s", Description: "due soon", Detail: "Focus Due Soon section"},
	{Key: "l", Description: "due later", Detail: "Focus Due Later section"},
	{Key: "f", Description: "future", Detail: "Focus Future section"},
	{Key: "r", Description: "in review", Detail: "Focus In Review section"},
	{Key: "S", Description: "standby", Detail: "Focus On Standby section"},
	{Key: "o", Description: "old", Detail: "Focus Old section"},
}

// renderAllSections renders all non-empty sections.
// Returns lines and the line index where the cursor is located.
func (m *DailyReviewModel) renderAllSections() ([]string, int) {
	var lines []string
	currentIndex := 0
	cursorLineIndex := 0
	separator := renderHorizontalDivider(m.width)

	// Compute column layout from all visible tasks.
	var colLayout *ColumnLayout
	if m.width >= MinColumnAlignWidth {
		var allTasks []*models.Task
		var allItems []CrossProjectTask
		for _, section := range m.sectionOrder {
			if !m.collapsedSections[int(section)] {
				for _, dt := range m.getSectionTasks(section) {
					allTasks = append(allTasks, dt.Task)
					allItems = append(allItems, dt)
				}
			}
		}
		colLayout = ComputeBadgeWidths(allTasks, BadgeWidthOpts{
			HideLowPriority: true,
			IncludeStatus:   true,
			IncludeType:     true,
			ProjectNameFor:  crossProjectNameLookup(allItems),
		})
	}

	rowIdx := 0
	for i, section := range m.sectionOrder {
		if i > 0 {
			lines = append(lines, separator)
		}

		tasks := m.getSectionTasks(section)
		collapsed := m.collapsedSections[int(section)]

		// Section header (navigable)
		headerSelected := currentIndex == m.cursor
		if headerSelected {
			cursorLineIndex = len(lines)
		}
		lines = append(lines, m.renderSectionHeader(section, len(tasks), collapsed, headerSelected, m.width))
		currentIndex++

		// Section tasks (if not collapsed)
		if !collapsed {
			for _, dt := range tasks {
				isSelected := currentIndex == m.cursor
				if isSelected {
					cursorLineIndex = len(lines)
				}
				line := m.renderTaskLine(dt, section, isSelected, max(m.width-40, 10), colLayout, rowIdx)
				lines = append(lines, line)
				currentIndex++
				rowIdx++
			}
		}
	}

	// Insert inline preview after cursor task
	if m.previewMode && cursorLineIndex >= 0 && cursorLineIndex < len(lines) {
		task := m.getTaskAtCursor()
		if task != nil {
			indent := "        " // align with cross-project task line content
			pLines := inlinePreviewLines(task.Task, indent, m.width)
			if len(pLines) > 0 {
				insertAt := cursorLineIndex + 1
				if insertAt <= len(lines) {
					tail := append([]string{}, lines[insertAt:]...)
					lines = append(lines[:insertAt], pLines...)
					lines = append(lines, tail...)
				}
			}
		}
	}

	return lines, cursorLineIndex
}

// renderSectionHeader renders a section header with collapse indicator.
func (m *DailyReviewModel) renderSectionHeader(section DailyReviewSection, count int, collapsed bool, isSelected bool, width int) string {
	var style lipgloss.Style
	switch section {
	case SectionOverdue:
		style = OverdueStyle
	case SectionDueToday:
		style = DueTodayStyle
	case SectionCompleted:
		style = SuccessStyle
	case SectionBlocked:
		style = BlockedStyle
	case SectionInReview:
		style = ReviewStyle
	case SectionOnStandby:
		style = StandbyStyle
	case SectionOld:
		style = MutedStyle
	default:
		style = SectionHeaderStyle
	}
	return renderCollapsibleHeader(getSectionName(section), count, collapsed, isSelected, m.cursorFlash, style, width)
}

// renderTaskLine renders a single task line.
func (m *DailyReviewModel) renderTaskLine(dt CrossProjectTask, section DailyReviewSection, isSelected bool, maxTitleLen int, layout *ColumnLayout, rowIndex int) string {
	task := dt.Task

	// Custom status badge for blocked tasks
	var statusBadge string
	if section == SectionBlocked {
		if dt.Project.HasIncompleteDependencies(task) {
			statusBadge = BlockedByDepsStyle.Render(task.Status.Icon())
		} else {
			statusBadge = BlockedStyle.Render(task.Status.Icon())
		}
	} else if task.Status.IsComplete() {
		statusBadge = GetStatusStyle(string(task.Status)).Render(task.Status.Icon())
	}

	line := RenderCrossProjectTaskLine(task, dt.Project, m.config, isSelected, m.width, maxTitleLen, CrossProjectTaskLineOpts{
		Queued:          m.queueIDs[task.ID],
		DailyPlanned:    m.dailyPlanIDs[task.ID],
		StatusBadge:     statusBadge,
		HideLowPriority: true,
		DueStr:          m.sectionContextStr(dt, section),
		ShowRecurrence:  true,
		ShowDescription: m.config.IsDescriptionColumnVisible(),
		Prefix:          m.Checkbox(task.ID),
		Flash:           m.cursorFlash,
		Layout:          layout,
		RowIndex:        rowIndex,
	})
	if dt.SuggestEscalation {
		line += "\n" + "      " + InfoStatusStyle.Render("↑ Due soon — consider P2")
	}
	return line
}

// sectionContextStr returns the right-aligned context string for a task in a section.
func (m *DailyReviewModel) sectionContextStr(dt CrossProjectTask, section DailyReviewSection) string {
	task := dt.Task
	switch section {
	case SectionCompleted:
		if task.CompletedAt != nil {
			return MutedStyle.Render(formatRelativeTime(*task.CompletedAt))
		}
	case SectionOverdue, SectionDueToday, SectionDueSoon, SectionDueLater, SectionFuture:
		if task.DueDate != nil {
			return m.formatDueDateContext(task, section)
		}
	case SectionInReview:
		if task.DueDate != nil {
			return DueDateStyle.Render(task.DueDate.Format("Jan 2"))
		}
	case SectionOnStandby:
		if task.DueDate != nil {
			return DueDateStyle.Render(task.DueDate.Format("Jan 2"))
		}
	case SectionBlocked:
		if dt.Project.HasIncompleteDependencies(task) {
			deps := dt.Project.GetIncompleteDependencies(task)
			if len(deps) > 0 {
				return MutedStyle.Render(fmt.Sprintf("waiting on #%d", deps[0].ShortID))
			}
		} else {
			return MutedStyle.Render("blocked")
		}
	case SectionOld:
		displayTime := task.UpdatedAt
		if displayTime.IsZero() {
			displayTime = task.CreatedAt
		}
		return MutedStyle.Render(formatRelativeTime(displayTime))
	default:
	}
	return ""
}

// formatDueDateContext formats due date with appropriate styling for context.
func (m *DailyReviewModel) formatDueDateContext(task *models.Task, section DailyReviewSection) string {
	if task.DueDate == nil {
		return ""
	}

	// Future section uses MutedStyle with full date (always includes year).
	if section == SectionFuture {
		dateStr := task.DueDate.Format("Jan 2, 2006")
		dateStr = AppendTimeIfHasTime(task, dateStr, m.config.GetTimeFormat())
		return MutedStyle.Render(dateStr)
	}

	return FormatRelativeDueTask(task, true, m.config.GetTimeFormat())
}
