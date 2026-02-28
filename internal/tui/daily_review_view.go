package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func (m *DailyReviewModel) View() string {
	totalCount := m.getTotalTaskCount()

	var b strings.Builder

	// Title
	var subtitle string
	if m.reviewCompleted {
		subtitle = fmt.Sprintf("completed today at %s", m.completedAt.Format("15:04"))
	} else if totalCount > 0 {
		subtitle = fmt.Sprintf("%d tasks across %d sections", totalCount, len(m.sectionOrder))
	} else {
		subtitle = "all caught up"
	}
	b.WriteString(renderFullscreenTitle("Review", subtitle))
	b.WriteString("\n")

	if totalCount == 0 {
		b.WriteString("\n")
		if m.HasActiveFilters() {
			b.WriteString(m.renderActiveFilterIndicator())
			b.WriteString("\n\n")
		}
		b.WriteString(renderFilterAwareEmptyState(m.HasActiveFilters(),
			"No tasks to review.", "Your inbox is clear. Press q or Esc to close.",
			"Press q or Esc to clear filters."))
		return b.String()
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
			"No tasks in this tab.", "[/]: tabs · ? help",
			"[/]: tabs · Fx clear filters · q/Esc clear filters · ? help"))
		return b.String()
	}

	lines, cursorLineIndex := m.renderAllSections()

	b.WriteString(strings.Join(applyPersistentScroll(lines, cursorLineIndex, contentHeight, &m.scrollOffset), "\n"))

	// Footer
	reviewHint := "R: mark reviewed"
	if m.reviewCompleted {
		reviewHint = "R: unmark"
	}
	left := HelpStyle.Render("h/j/k/l nav · n/N section · " + reviewHint + " · ? help")
	right := MutedStyle.Render(fmt.Sprintf("%d tasks", totalCount))
	b.WriteString(renderFullscreenFooterBlock(m.statusMessage, m.err, left, right, m.width))

	return b.String()
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

	return renderWrappingTabBar(labels, m.tabs.Active, m.width)
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *DailyReviewModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	hints := PrefixHints{
		",": StatusPrefixHints,
		"d": DueDatePrefixHints,
		"g": DailyReviewNavigationPrefixHints,
		"s": SectionPrefixHints,
		"c": CreateLinkedPrefixHints,
		"m": SecondaryViewMovePrefixHints,
		"z": SectionFoldPrefixHints,
	}
	// Merge filter prefix hints (no DueDate or Status — sections group by these)
	for k, v := range buildCommonFilterPrefixPopupHints(&m.filter, m.config, false, false) {
		hints[k] = v
	}
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints:  hints,
	}
}

// DailyReviewNavigationPrefixHints contains "g" prefix hints for daily review.
var DailyReviewNavigationPrefixHints = []PrefixHint{
	{Key: "g", Description: "first", Detail: "Jump to first item"},
	{Key: "p", Description: "go to project", Detail: "Navigate to project"},
}

// renderAllSections renders all non-empty sections.
// Returns lines and the line index where the cursor is located.
func (m *DailyReviewModel) renderAllSections() ([]string, int) {
	var lines []string
	currentIndex := 0
	cursorLineIndex := 0
	separator := renderHorizontalDivider(m.width)

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
		lines = append(lines, m.renderSectionHeader(section, len(tasks), collapsed, headerSelected))
		currentIndex++

		// Section tasks (if not collapsed)
		if !collapsed {
			for _, dt := range tasks {
				isSelected := currentIndex == m.cursor
				if isSelected {
					cursorLineIndex = len(lines)
				}
				line := m.renderTaskLine(dt, section, isSelected, m.width-40)
				lines = append(lines, line)
				currentIndex++
			}
		}
	}

	return lines, cursorLineIndex
}

// renderSectionHeader renders a section header with collapse indicator.
func (m *DailyReviewModel) renderSectionHeader(section DailyReviewSection, count int, collapsed bool, isSelected bool) string {
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
	return renderCollapsibleHeader(getSectionName(section), count, collapsed, isSelected, m.cursorFlash, style)
}

// renderTaskLine renders a single task line.
func (m *DailyReviewModel) renderTaskLine(dt CrossProjectTask, section DailyReviewSection, isSelected bool, maxTitleLen int) string {
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

	return RenderCrossProjectTaskLine(task, dt.Project, nil, isSelected, m.width, maxTitleLen, CrossProjectTaskLineOpts{
		StatusBadge:     statusBadge,
		HideLowPriority: true,
		DueStr:          m.sectionContextStr(dt, section),
		ShowRecurrence:  true,
		Flash:           m.cursorFlash,
	})
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
			return m.formatDueDateContext(task.DueDate, section)
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
	}
	return ""
}

// formatDueDateContext formats due date with appropriate styling for context.
func (m *DailyReviewModel) formatDueDateContext(dueDate *time.Time, section DailyReviewSection) string {
	if dueDate == nil {
		return ""
	}

	// Future section uses MutedStyle with full date (always includes year).
	if section == SectionFuture {
		return MutedStyle.Render(dueDate.Format("Jan 2, 2006"))
	}

	return FormatRelativeDue(dueDate, true)
}
