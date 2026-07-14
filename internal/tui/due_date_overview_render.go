package tui

import (
	"fmt"
	"maps"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *DueDateOverviewModel) View() tea.View {
	var b strings.Builder

	allTotal := len(m.overdue) + len(m.today) + len(m.thisWeek) + len(m.later)

	// Title
	subtitle := fmt.Sprintf("%d tasks across all projects", allTotal)
	b.WriteString(renderFullscreenTitle("Due Dates", subtitle, m.AccentColor()))
	b.WriteString("\n")

	if allTotal == 0 {
		b.WriteString("\n")
		if m.HasActiveFilters() {
			b.WriteString(m.renderActiveFilterIndicator())
			b.WriteString("\n\n")
		}
		b.WriteString(renderFilterAwareEmptyState(m.HasActiveFilters(),
			"No tasks with due dates found.", "Press Esc to close",
			"Press Esc to clear filters."))
		return tea.NewView(b.String())
	}

	// Tab bar + filter badge
	chrome, chromeLines := renderFullscreenChrome(
		m.tabs.RenderBar(m.Width(), m.AccentColor()),
		m.renderActiveFilterIndicator(),
		true,
	)
	b.WriteString(chrome)

	if m.totalTasks == 0 {
		b.WriteString(renderFilterAwareEmptyState(m.HasActiveFilters(),
			"No tasks in this category.", "Tab tabs · Esc close",
			"Tab tabs · Fx clear filters · Esc clear filters"))
		return tea.NewView(b.String())
	}

	// Render groups with scroll awareness
	contentHeight := fullscreenContentHeight(m.Height(), chromeLines)

	// Reserve one row for the selected-task metadata strip whenever the cursor
	// is in bounds. Keeps layout stable as the selection moves.
	var stripTask *models.Task
	var stripProject *models.Project
	if m.cursor >= 0 && m.cursor < m.totalTasks {
		stripTask, stripProject = m.getTaskRef()
		contentHeight = max(contentHeight-1, 1)
	}

	cursorLineIdx := 0
	currentIndex := 0
	lines := m.renderAllGroups(&currentIndex, &cursorLineIdx)

	// Insert inline preview lines after cursor task
	if m.previewMode && m.cursor >= 0 && m.cursor < m.totalTasks {
		task, _ := m.getTaskRef()
		if task != nil {
			indent := "        " // align with cross-project task line content
			pLines := inlinePreviewLines(task, indent, m.Width())
			if len(pLines) > 0 {
				insertAt := cursorLineIdx + 1
				if insertAt <= len(lines) {
					tail := append([]string{}, lines[insertAt:]...)
					lines = append(lines[:insertAt], pLines...)
					lines = append(lines, tail...)
					contentHeight -= len(pLines)
					if contentHeight < 1 {
						contentHeight = 1
					}
				}
			}
		}
	}

	visibleLines, _, _ := persistentLineWindow(lines, cursorLineIdx, contentHeight, &m.viewScrollOffset)
	b.WriteString(strings.Join(visibleLines, "\n"))

	if stripTask != nil {
		strip := renderTaskMetadataStrip(TaskMetadataStripOpts{
			Task:    stripTask,
			Project: stripProject,
			Runtime: m.runtime,
			Width:   m.Width(),
			AlreadyShown: TaskMetadataAlreadyShown{
				Project:  true,
				Priority: true,
				Due:      true,
				Stale:    true,
			},
		})
		b.WriteString("\n")
		b.WriteString(strip)
	}

	// Footer
	left := HelpStyle.Render("Tab tabs · ↑/↓ navigate · ,s/d/b/x/o status · 1-5 priority · dt/dm/dw due · e edit · ? help")
	if m.multiSelectMode {
		left = HelpStyle.Render("MULTI-SELECT: space toggle · a all · A deselect · ,/! status/priority · v exit")
	}
	right := MutedStyle.Render(fmt.Sprintf("%d tasks", m.totalTasks))
	b.WriteString(renderFullscreenFooterBlock(m.statusMessage, m.Error(), left, right, m.Width(), m.AccentColor()))

	return tea.NewView(b.String())
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *DueDateOverviewModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.Dispatcher == nil || !m.Dispatcher.HasPendingPrefix() {
		return nil
	}
	prefix := m.Dispatcher.PendingPrefix()
	hints := PrefixHints{}
	maps.Copy(hints, buildCommonFilterPrefixPopupHints(&m.filter, m.runtime.Machine(), true, true, false))
	if _, ok := hints[prefix]; !ok {
		fallback := make([]PrefixHint, 0, len(m.Dispatcher.PrefixHints()))
		for _, h := range m.Dispatcher.PrefixHints() {
			fallback = append(fallback, PrefixHint{Key: h.Key, Description: h.Description})
		}
		hints[prefix] = fallback
	}
	return &PrefixPopupInfo{
		Prefix: prefix,
		Hints:  hints,
	}
}

// DueDateOverviewNavigationPrefixHints contains "g" prefix hints for due date overview.
var DueDateOverviewNavigationPrefixHints = []PrefixHint{
	{Key: "g", Description: "first", Detail: "Jump to first item"},
	{Key: "p", Description: "go to project", Detail: "Navigate to project"},
}

// renderAllGroups renders all task groups and returns the lines.
func (m *DueDateOverviewModel) renderAllGroups(currentIndex *int, cursorLineIdx *int) []string {
	var lines []string
	hasContent := false
	separator := renderHorizontalDivider(m.Width())
	rowIdx := 0

	overdue, today, thisWeek, later := m.visibleGroups()

	// Compute column layout from all visible tasks.
	var colLayout *ColumnLayout
	if m.Width() >= MinColumnAlignWidth {
		var allTasks []*models.Task
		var allItems []CrossProjectTask
		for _, g := range [][]CrossProjectTask{overdue, today, thisWeek, later} {
			for _, dt := range g {
				allTasks = append(allTasks, dt.Task)
				allItems = append(allItems, dt)
			}
		}
		colLayout = ComputeBadgeWidths(allTasks, BadgeWidthOpts{
			IncludeStatus:  true,
			IncludeType:    true,
			ProjectNameFor: crossProjectNameLookup(allItems),
			MaxProjectLen:  15,
		})
	}

	type taskGroup struct {
		name      string
		tasks     []CrossProjectTask
		isOverdue bool
	}
	groups := []taskGroup{
		{"Overdue", overdue, true},
		{"Today", today, false},
		{"This Week", thisWeek, false},
		{"Later", later, false},
	}
	for _, g := range groups {
		if len(g.tasks) == 0 {
			continue
		}
		if hasContent {
			lines = append(lines, separator)
		}
		lines = append(lines, renderGroupHeader(g.name, len(g.tasks), g.isOverdue, m.Width()))
		lines = append(lines, m.renderGroup(g.tasks, currentIndex, cursorLineIdx, len(lines), colLayout, &rowIdx)...)
		hasContent = true
	}

	return lines
}

// renderGroup renders a group of tasks.
func (m *DueDateOverviewModel) renderGroup(tasks []CrossProjectTask, currentIndex *int, cursorLineIdx *int, baseLine int, layout *ColumnLayout, rowIdx *int) []string {
	var lines []string
	maxTitleLen := m.Width() - 35 // Account for date, project, icons

	for _, dt := range tasks {
		isSelected := *currentIndex == m.cursor
		if isSelected && cursorLineIdx != nil {
			*cursorLineIdx = baseLine + len(lines)
		}
		line := m.renderTaskLine(dt, isSelected, maxTitleLen, layout, *rowIdx)
		lines = append(lines, line)
		*currentIndex++
		*rowIdx++
	}

	return lines
}

// renderTaskLine renders a single task line.
func (m *DueDateOverviewModel) renderTaskLine(dt CrossProjectTask, isSelected bool, maxTitleLen int, layout *ColumnLayout, rowIndex int) string {
	dateStr := m.formatDueDate(dt.Task)
	return RenderCrossProjectTaskLine(dt.Task, dt.Project, m.runtime, isSelected, m.Width(), maxTitleLen, CrossProjectTaskLineOpts{
		Queued:          m.queueIDs[dt.Task.ID],
		DailyPlanned:    m.dailyPlanIDs[dt.Task.ID],
		ShowDescription: m.runtime.Machine().IsDescriptionColumnVisible(),
		MaxProjectLen:   15,
		Prefix:          m.Checkbox(dt.Task.ID) + dateStr + " ",
		Flash:           m.cursorFlash,
		Layout:          layout,
		RowIndex:        rowIndex,
	})
}

// formatDueDate formats the due date for display.
func (m *DueDateOverviewModel) formatDueDate(task *models.Task) string {
	if task.DueDate == nil {
		return "         "
	}
	dateStr, style := ClassifyDueDate(task.DueDate)
	dateStr = AppendTimeIfHasTime(task, dateStr, m.runtime.Settings().GetTimeFormat())
	return style.Render(fmt.Sprintf("%-9s", dateStr))
}
