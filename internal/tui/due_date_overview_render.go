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
	b.WriteString(renderFullscreenTitle("Due Dates", subtitle))
	b.WriteString("\n")

	if allTotal == 0 {
		b.WriteString("\n")
		if m.HasActiveFilters() {
			b.WriteString(m.renderActiveFilterIndicator())
			b.WriteString("\n\n")
		}
		b.WriteString(renderFilterAwareEmptyState(m.HasActiveFilters(),
			"No tasks with due dates found.", "Press q or Esc to close",
			"Press q or Esc to clear filters."))
		return tea.NewView(b.String())
	}

	// Tab bar + filter badge
	chrome, chromeLines := renderFullscreenChrome(
		m.tabs.RenderBar(m.Width()),
		m.renderActiveFilterIndicator(),
		true,
	)
	b.WriteString(chrome)

	if m.totalTasks == 0 {
		b.WriteString(renderFilterAwareEmptyState(m.HasActiveFilters(),
			"No tasks in this category.", "[/] h/l tabs · q/Esc close",
			"[/] h/l tabs · Fx clear filters · q/Esc clear filters"))
		return tea.NewView(b.String())
	}

	// Render groups with scroll awareness
	contentHeight := fullscreenContentHeight(m.Height(), chromeLines)
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

	b.WriteString(strings.Join(applyScroll(lines, cursorLineIdx, contentHeight), "\n"))

	// Footer
	left := HelpStyle.Render("[/] tabs · ↑/↓ navigate · ,s/d/b/x/o status · 1-5 priority · dt/dm/dw due · e edit · ? help")
	right := MutedStyle.Render(fmt.Sprintf("%d tasks", m.totalTasks))
	b.WriteString(renderFullscreenFooterBlock(m.statusMessage, m.Error(), left, right, m.Width()))

	return tea.NewView(b.String())
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *DueDateOverviewModel) GetPrefixPopupInfo() *PrefixPopupInfo {
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
	hints["g"] = DueDateOverviewNavigationPrefixHints
	hints["z"] = []PrefixHint{
		{Key: "p", Description: "preview", Detail: "Toggle inline preview"},
	}
	// Merge filter prefix hints (no DueDate category — the view IS organized by due date)
	maps.Copy(hints, buildCommonFilterPrefixPopupHints(&m.filter, m.config, false, true))
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
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

	overdue, today, thisWeek, later := m.visibleGroups()

	if len(overdue) > 0 {
		lines = append(lines, renderGroupHeader("Overdue", len(overdue), true, m.Width()))
		lines = append(lines, m.renderGroup(overdue, currentIndex, cursorLineIdx, len(lines))...)
		hasContent = true
	}

	if len(today) > 0 {
		if hasContent {
			lines = append(lines, separator)
		}
		lines = append(lines, renderGroupHeader("Today", len(today), false, m.Width()))
		lines = append(lines, m.renderGroup(today, currentIndex, cursorLineIdx, len(lines))...)
		hasContent = true
	}

	if len(thisWeek) > 0 {
		if hasContent {
			lines = append(lines, separator)
		}
		lines = append(lines, renderGroupHeader("This Week", len(thisWeek), false, m.Width()))
		lines = append(lines, m.renderGroup(thisWeek, currentIndex, cursorLineIdx, len(lines))...)
		hasContent = true
	}

	if len(later) > 0 {
		if hasContent {
			lines = append(lines, separator)
		}
		lines = append(lines, renderGroupHeader("Later", len(later), false, m.Width()))
		lines = append(lines, m.renderGroup(later, currentIndex, cursorLineIdx, len(lines))...)
	}

	return lines
}

// renderGroup renders a group of tasks.
func (m *DueDateOverviewModel) renderGroup(tasks []CrossProjectTask, currentIndex *int, cursorLineIdx *int, baseLine int) []string {
	var lines []string
	maxTitleLen := m.Width() - 35 // Account for date, project, icons

	for _, dt := range tasks {
		isSelected := *currentIndex == m.cursor
		if isSelected && cursorLineIdx != nil {
			*cursorLineIdx = baseLine + len(lines)
		}
		line := m.renderTaskLine(dt, isSelected, maxTitleLen)
		lines = append(lines, line)
		*currentIndex++
	}

	return lines
}

// renderTaskLine renders a single task line.
func (m *DueDateOverviewModel) renderTaskLine(dt CrossProjectTask, isSelected bool, maxTitleLen int) string {
	dateStr := m.formatDueDate(dt.Task)
	return RenderCrossProjectTaskLine(dt.Task, dt.Project, nil, isSelected, m.Width(), maxTitleLen, CrossProjectTaskLineOpts{
		MaxProjectLen: 15,
		Prefix:        dateStr + " ",
		Flash:         m.cursorFlash,
	})
}

// formatDueDate formats the due date for display.
func (m *DueDateOverviewModel) formatDueDate(task *models.Task) string {
	if task.DueDate == nil {
		return "         "
	}
	dateStr, style := ClassifyDueDate(task.DueDate)
	return style.Render(fmt.Sprintf("%-9s", dateStr))
}
