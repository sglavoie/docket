package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/sglavoie/tt/internal/util"
)

func (m *FocusViewModel) View() string {
	var b strings.Builder

	// Title
	queueCount := len(*m.queue)
	subtitle := fmt.Sprintf("%d tasks", len(m.tasks))
	if queueCount > 0 {
		subtitle = fmt.Sprintf("%d queued, %d tasks", queueCount, len(m.tasks))
	}
	if len(m.filtered) != len(m.tasks) && m.activeTab != focusTabQueue {
		subtitle = fmt.Sprintf("%d/%d tasks", len(m.filtered), len(m.tasks))
		if queueCount > 0 {
			subtitle = fmt.Sprintf("%d queued, %d/%d tasks", queueCount, len(m.filtered), len(m.tasks))
		}
	}
	b.WriteString(renderFullscreenTitle("What's Next", subtitle))
	b.WriteString("\n")

	if len(m.tasks) == 0 && len(*m.queue) == 0 {
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render("No actionable tasks found."))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Tasks appear here from all active projects (excluding completed, snoozed, and blocked)."))
		b.WriteString("\n")
		b.WriteString(HelpStyle.Render("Press q to return to dashboard."))
		return b.String()
	}

	// Tab bar
	tabBar := m.renderTabBar()
	b.WriteString(tabBar)
	b.WriteString("\n")

	// Filter badge
	filterBadge := m.renderActiveFilterIndicator()
	if filterBadge != "" {
		b.WriteString(filterBadge)
		b.WriteString("\n")
	}

	// Sub-tab bar
	subTabBar := m.renderSubTabBar()
	if subTabBar != "" {
		b.WriteString(subTabBar)
		b.WriteString("\n")
	}

	// Pre-render status messages so their height is known
	statusBlock := renderStatusMessages(m.statusMessage, m.err)

	// Calculate available height for task list
	// Fixed overhead: title(1) + blank after tab bar(1) + blank before footer(1) + footer(2) = 5
	contentHeight := m.height - 5 - lipgloss.Height(tabBar)
	if filterBadge != "" {
		contentHeight--
	}
	if subTabBar != "" {
		contentHeight -= lipgloss.Height(subTabBar)
	}
	if statusBlock != "" {
		contentHeight -= strings.Count(statusBlock, "\n")
	}

	// Render task lines based on active tab
	var lines []string

	switch m.activeTab {
	case focusTabQueue:
		lines = m.renderQueueLines()
	case focusTabSuggested:
		lines = m.renderSuggestedLines()
	default:
		lines = m.renderStandardLines()
	}

	if len(lines) == 0 {
		switch m.activeTab {
		case focusTabQueue:
			lines = append(lines, MutedStyle.Render("Queue is empty. Switch to Suggested tab and press a to add tasks."))
		case focusTabSuggested:
			if m.HasActiveFilters() {
				lines = append(lines, MutedStyle.Render("No tasks match current filters. Press q or Esc to clear."))
			} else {
				lines = append(lines, MutedStyle.Render("All suggestions dismissed. Press r to refresh."))
			}
		default:
			if m.HasActiveFilters() {
				lines = append(lines, MutedStyle.Render("No tasks match current filters. Press q or Esc to clear."))
			} else {
				lines = append(lines, MutedStyle.Render("No matching tasks."))
			}
		}
	}

	// Suggested tab uses two lines per task (task + reason), so adjust cursor
	// index for scroll calculation to account for the double-height entries.
	scrollCursor := m.cursor
	if m.activeTab == focusTabSuggested {
		scrollCursor = m.cursor * 2
	}
	b.WriteString(strings.Join(applyScroll(lines, scrollCursor, contentHeight), "\n"))
	b.WriteString("\n\n")

	b.WriteString(statusBlock)

	// Footer
	var helpParts []string
	switch m.activeTab {
	case focusTabQueue:
		helpParts = []string{"j/k nav", "x remove", "J/K reorder", "a add suggestion", "r refresh", "[/] tabs"}
	case focusTabSuggested:
		helpParts = []string{"j/k nav", "a accept", "x dismiss", "[/] tabs", ",s/d/b/x status"}
	default:
		helpParts = []string{"j/k nav", "[/] tabs", "(/) row focus", ",s/d/b/x status", "1-5 priority"}
	}
	left := HelpStyle.Render(strings.Join(helpParts, " | "))
	right := MutedStyle.Render(fmt.Sprintf("%d tasks", len(m.filtered)))
	b.WriteString(renderStatusBarFooter(left, right, m.width))

	return b.String()
}

// renderQueueLines renders the Queue tab with numbered items.
func (m *FocusViewModel) renderQueueLines() []string {
	var lines []string
	maxTitleLen := m.width - 50

	for i, ft := range m.filtered {
		isSelected := i == m.cursor
		// Number prefix for queue items
		numPrefix := MutedStyle.Render(fmt.Sprintf("%d. ", i+1))
		line := m.renderTaskLineWithPrefix(ft, isSelected, maxTitleLen, numPrefix)
		lines = append(lines, line)
	}
	return lines
}

// renderSuggestedLines renders the Suggested tab with two-line format (task + reason).
func (m *FocusViewModel) renderSuggestedLines() []string {
	var lines []string
	maxTitleLen := m.width - 50

	// Build reason lookup map to avoid O(n) scan per filtered task
	reasonByID := make(map[string]string, len(m.ranked))
	for _, sft := range m.ranked {
		reasonByID[sft.Task.ID] = sft.Reason
	}

	starShown := false
	for i, ft := range m.filtered {
		isSelected := i == m.cursor

		// Check if task is already in queue
		prefix := ""
		if m.isInQueue(ft.Task.ID) {
			prefix = lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true).Render("[Q] ")
		}

		// Star indicator on the first non-queued item
		if !starShown && prefix == "" {
			prefix = DueSoonStyle.Render("* ")
			starShown = true
		}

		line := m.renderTaskLineWithPrefix(ft, isSelected, maxTitleLen, prefix)
		lines = append(lines, line)

		if reason := reasonByID[ft.Task.ID]; reason != "" {
			reasonLine := "    " + MutedStyle.Render(reason)
			lines = append(lines, reasonLine)
		}
	}
	return lines
}

// renderStandardLines renders the By Project and By Due Date tabs.
func (m *FocusViewModel) renderStandardLines() []string {
	var lines []string
	maxTitleLen := m.width - 45

	for i, ft := range m.filtered {
		isSelected := i == m.cursor
		line := m.renderTaskLine(ft, isSelected, maxTitleLen)
		lines = append(lines, line)
	}
	return lines
}

// renderTabBar renders the tab bar for filtering.
func (m *FocusViewModel) renderTabBar() string {
	return renderTabBarWithOptionalSubTabs(focusTabNames, m.activeTab, m.subTabNames, m.subTabRowFocused, m.width)
}

// renderSubTabBar renders the dynamic sub-tab row when applicable.
func (m *FocusViewModel) renderSubTabBar() string {
	return renderDynamicSubTabBar(m.subTabNames, m.subTab, m.subTabRowFocused, m.width)
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *FocusViewModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	hints := PrefixHints{
		",": StatusPrefixHints,
		"d": DueDatePrefixHints,
		"g": FocusViewNavigationPrefixHints,
		"m": SecondaryViewMovePrefixHints,
		"c": CreateLinkedPrefixHints,
	}
	// Merge filter prefix hints (no DueDate category -- view has "By Due Date" tab)
	for k, v := range buildCommonFilterPrefixPopupHints(&m.filter, m.config, false, true) {
		hints[k] = v
	}
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

// renderTaskLine renders a single task line.
func (m *FocusViewModel) renderTaskLine(ft FocusTask, isSelected bool, maxTitleLen int) string {
	return m.renderTaskLineWithPrefix(ft, isSelected, maxTitleLen, "")
}

// renderTaskLineWithPrefix renders a single task line with an optional prefix.
func (m *FocusViewModel) renderTaskLineWithPrefix(ft FocusTask, isSelected bool, maxTitleLen int, prefix string) string {
	task := ft.Task
	projectName := util.Truncate(ft.Project.Name, 12)

	dueStr := ""
	if task.DueDate != nil {
		dueStr = FormatRelativeDue(task.DueDate, true)
	}

	preBadges := prefix + MutedStyle.Render(fmt.Sprintf("[%s]", projectName)) + " "

	return TaskLine{
		Task:       task,
		IsSelected: isSelected,
		MaxTitle:   maxTitleLen,
		PreBadges:  preBadges,
		PostTitle:  renderInlineTags(task.Tags),
		DueStr:     dueStr,
		ViewWidth:  m.width,
	}.Render()
}
