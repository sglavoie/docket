package tui

import (
	"fmt"
	"maps"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *FocusViewModel) View() tea.View { //nolint:gocyclo
	var b strings.Builder

	// Title
	queueCount := len(*m.queue)
	subtitle := fmt.Sprintf("%d tasks", len(m.tasks))
	if queueCount > 0 {
		subtitle = fmt.Sprintf("%d queued, %d tasks", queueCount, len(m.tasks))
	}
	if len(m.filtered) != len(m.tasks) && m.tabs.Active != focusTabQueue {
		subtitle = fmt.Sprintf("%d/%d tasks", len(m.filtered), len(m.tasks))
		if queueCount > 0 {
			subtitle = fmt.Sprintf("%d queued, %d/%d tasks", queueCount, len(m.filtered), len(m.tasks))
		}
	}
	b.WriteString(renderFullscreenTitle("Backlog", subtitle, m.AccentColor()))
	b.WriteString("\n")

	if len(m.tasks) == 0 && len(*m.queue) == 0 {
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render("No actionable tasks found."))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Tasks appear here from all active projects (excluding completed, snoozed, and blocked)."))
		b.WriteString("\n")
		b.WriteString(HelpStyle.Render("Press Esc to return to dashboard."))
		return tea.NewView(b.String())
	}

	// Tab bar + filter badge
	chrome, chromeLines := renderFullscreenChrome(
		m.renderTabBar(),
		m.renderActiveFilterIndicator(),
		false,
	)
	b.WriteString(chrome)

	// Sub-tab bar
	subTabBar := m.renderSubTabBar()
	if subTabBar != "" {
		b.WriteString(subTabBar)
		b.WriteString("\n")
		chromeLines += lipgloss.Height(subTabBar)
	}

	// Pre-render status messages so their height is known
	statusBlock := renderStatusMessages(m.statusMessage, m.err)

	// Calculate available height for task list
	contentHeight := fullscreenContentHeight(m.height, chromeLines)
	if statusBlock != "" {
		contentHeight -= strings.Count(statusBlock, "\n")
	}

	// Render task lines based on active tab
	var lines []string

	switch m.tabs.Active {
	case focusTabQueue:
		lines, _ = m.renderQueueLines()
	case focusTabSuggested:
		lines, _ = m.renderSuggestedLines()
	default:
		lines, _ = m.renderStandardLines()
	}

	if len(lines) == 0 {
		switch m.tabs.Active {
		case focusTabQueue:
			lines = append(lines, MutedStyle.Render("Queue is empty. Switch to Suggested tab and press a to add tasks."))
		case focusTabSuggested:
			if m.HasActiveFilters() {
				lines = append(lines, MutedStyle.Render(EmptyStateFiltered))
			} else if len(m.dismissedIDs) > 0 && !m.showDismissed {
				lines = append(lines, MutedStyle.Render(fmt.Sprintf("All suggestions dismissed (%d hidden). Press D to show or r to refresh.", len(m.dismissedIDs))))
			} else {
				lines = append(lines, MutedStyle.Render("All suggestions dismissed. Press r to refresh."))
			}
		default:
			lines = append(lines, MutedStyle.Render(EmptyStateFiltered))
		}
	}

	// Scroll cursor maps cursor to line index (suggested tab has 2 lines per entry)
	scrollCursor := m.cursor
	if m.tabs.Active == focusTabSuggested {
		scrollCursor = m.cursor * 2
	}

	// Insert inline preview lines after cursor task
	if m.previewMode && m.cursor >= 0 && m.cursor < len(m.filtered) {
		task, _ := m.getTaskRef()
		if task != nil {
			indent := "        " // align with cross-project task line content
			pLines := inlinePreviewLines(task, indent, m.width)
			if len(pLines) > 0 {
				insertAt := scrollCursor + 1
				// For suggested tab, insert after reason line
				if m.tabs.Active == focusTabSuggested {
					insertAt = scrollCursor + 2
				}
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

	b.WriteString(strings.Join(applyScroll(lines, scrollCursor, contentHeight), "\n"))

	b.WriteString("\n\n")
	b.WriteString(statusBlock)

	// Footer
	var helpParts []string
	switch m.tabs.Active {
	case focusTabQueue:
		helpParts = []string{"j/k nav", "x remove", "J/K reorder", "r refresh", "[/] tabs"}
	case focusTabSuggested:
		if m.showDismissed {
			xLabel := "x dismiss"
			if ft := m.getTaskAtCursor(); ft != nil && m.dismissedIDs[ft.Task.ID] {
				xLabel = "x restore"
			}
			helpParts = []string{"j/k nav", "q add to queue", xLabel, "D hide dismissed", "[/] tabs"}
		} else {
			helpParts = []string{"j/k nav", "q add to queue", "x dismiss", "D show dismissed", "[/] tabs"}
		}
	default:
		helpParts = []string{"j/k nav", "[/] tabs", "(/) row focus", ",s/d/b/x status", "1-5 priority"}
	}
	left := HelpStyle.Render(strings.Join(helpParts, " · "))
	right := MutedStyle.Render(fmt.Sprintf("%d tasks", len(m.filtered)))
	b.WriteString(renderStatusBarFooter(left, right, m.width, m.AccentColor()))

	return tea.NewView(b.String())
}

// renderQueueLines renders the Queue tab with numbered items.
func (m *FocusViewModel) renderQueueLines() ([]string, int) {
	var lines []string
	maxTitleLen := max(m.width-FocusViewQueueTitlePadding, FocusViewMinTitleWidth)
	layout := m.computeColumnLayout()
	rowIndex := 0

	for i, ft := range m.filtered {
		isSelected := i == m.cursor
		// Number prefix for queue items
		numPrefix := MutedStyle.Render(fmt.Sprintf("%d. ", i+1))
		line := m.renderTaskLineWithPrefix(ft, isSelected, maxTitleLen, numPrefix, m.queueIDs[ft.Task.ID], m.dailyPlanIDs[ft.Task.ID], layout, rowIndex)
		lines = append(lines, line)
		rowIndex++
	}
	return lines, rowIndex
}

// renderSuggestedLines renders the Suggested tab with two-line format (task + reason).
func (m *FocusViewModel) renderSuggestedLines() ([]string, int) {
	var lines []string
	maxTitleLen := max(m.width-FocusViewQueueTitlePadding, FocusViewMinTitleWidth)
	layout := m.computeColumnLayout()
	rowIndex := 0

	// Build reason lookup map to avoid O(n) scan per filtered task
	reasonByID := make(map[string]string, len(m.ranked))
	for _, sft := range m.ranked {
		reasonByID[sft.Task.ID] = sft.Reason
	}

	starShown := false
	for i, ft := range m.filtered {
		isSelected := i == m.cursor
		isDismissed := m.showDismissed && m.dismissedIDs[ft.Task.ID]

		// Prefix for dismissed or star indicator
		prefix := ""
		if isDismissed {
			prefix = MutedStyle.Render("[dismissed] ")
		}

		// Star indicator on the first non-dismissed, non-queued item
		if !starShown && !isDismissed && !m.isInQueue(ft.Task.ID) {
			prefix = DueSoonStyle.Render("* ")
			starShown = true
		}

		line := m.renderTaskLineWithPrefix(ft, isSelected, maxTitleLen, prefix, m.isInQueue(ft.Task.ID), m.dailyPlanIDs[ft.Task.ID], layout, rowIndex)
		lines = append(lines, line)
		rowIndex++

		if reason := reasonByID[ft.Task.ID]; reason != "" {
			reasonLine := "    " + MutedStyle.Render(reason)
			lines = append(lines, reasonLine)
		}
	}
	return lines, rowIndex
}

// renderStandardLines renders the By Project and By Due Date tabs.
func (m *FocusViewModel) renderStandardLines() ([]string, int) {
	var lines []string
	maxTitleLen := max(m.width-FocusViewStandardTitlePadding, FocusViewMinTitleWidth)
	layout := m.computeColumnLayout()
	rowIndex := 0

	for i, ft := range m.filtered {
		isSelected := i == m.cursor
		line := m.renderTaskLineWithPrefix(ft, isSelected, maxTitleLen, "", m.queueIDs[ft.Task.ID], m.dailyPlanIDs[ft.Task.ID], layout, rowIndex)
		lines = append(lines, line)
		rowIndex++
	}
	return lines, rowIndex
}

// renderTabBar renders the tab bar for filtering.
func (m *FocusViewModel) renderTabBar() string {
	return renderTabBarWithOptionalSubTabs(focusTabNames, m.tabs.Active, m.subTabNames, m.subTabRowFocused, m.width, m.AccentColor())
}

// renderSubTabBar renders the dynamic sub-tab row when applicable.
func (m *FocusViewModel) renderSubTabBar() string {
	return renderDynamicSubTabBar(m.subTabNames, m.subTab, m.subTabRowFocused, m.width, m.AccentColor())
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *FocusViewModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	var currentStage string
	var currentPriority int
	if ft := m.getTaskAtCursor(); ft != nil {
		currentStage = ft.Task.Stage
		currentPriority = ft.Task.Priority
	}
	hints := CommonTaskActionPrefixHints(m.config.Stages, currentStage, currentPriority)
	hints["g"] = FocusViewNavigationPrefixHints
	hints["z"] = []PrefixHint{
		{Key: "p", Description: "preview", Detail: "Toggle inline preview"},
	}
	// Merge filter prefix hints (no DueDate category -- view has "By Due Date" tab)
	maps.Copy(hints, buildCommonFilterPrefixPopupHints(&m.filter, m.config, false, true))
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints:  hints,
	}
}

// FocusViewNavigationPrefixHints contains "g" prefix hints for focus view.
var FocusViewNavigationPrefixHints = []PrefixHint{
	{Key: "g", Description: "first", Detail: "Jump to first item"},
	{Key: "p", Description: "go to project", Detail: "Navigate to project"},
}

// computeColumnLayout builds a ColumnLayout from the current filtered task list.
// Returns nil when the terminal is too narrow for alignment.
func (m *FocusViewModel) computeColumnLayout() *ColumnLayout {
	if m.width < MinColumnAlignWidth {
		return nil
	}
	tasks := make([]*models.Task, 0, len(m.filtered))
	for _, ft := range m.filtered {
		tasks = append(tasks, ft.Task)
	}
	return ComputeTaskLineColumnLayout(tasks, false)
}

// renderTaskLineWithPrefix renders a single task line with an optional prefix.
func (m *FocusViewModel) renderTaskLineWithPrefix(ft FocusTask, isSelected bool, maxTitleLen int, prefix string, queued bool, dailyPlanned bool, layout *ColumnLayout, rowIndex int) string {
	task := ft.Task

	dueStr := ""
	if task.DueDate != nil {
		dueStr = FormatRelativeDueTask(task, true, m.config.GetTimeFormat())
	}

	return RenderCrossProjectTaskLine(task, ft.Project, nil, isSelected, m.width, maxTitleLen, CrossProjectTaskLineOpts{
		Queued:       queued,
		DailyPlanned: dailyPlanned,
		ShowTags:     true,
		Prefix:       prefix,
		DueStr:       dueStr,
		Flash:        m.cursorFlash,
		Layout:       layout,
		RowIndex:     rowIndex,
	})
}
