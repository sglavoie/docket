package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/util"
)

func (m *DailyReviewModel) View() string {
	totalCount := m.getTotalTaskCount()

	if totalCount == 0 {
		var b strings.Builder
		b.WriteString(MutedStyle.Render("No tasks to review."))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Your inbox is clear. Press q or Esc to close."))

		return renderOverlayCard(b.String(), m.width, m.height, OverlayCardConfig{
			Title:    "Daily Review",
			Subtitle: "all caught up",
			Centered: true,
		})
	}

	var b strings.Builder

	// Calculate available height for content
	contentHeight := m.height - 12

	// Render all sections
	lines, cursorLineIndex := m.renderAllSections()

	// Apply scrolling based on actual line index of cursor
	scrollOffset := 0
	if cursorLineIndex >= contentHeight {
		scrollOffset = cursorLineIndex - contentHeight + 1
	}

	visibleLines := lines
	if scrollOffset > 0 && scrollOffset < len(lines) {
		visibleLines = lines[scrollOffset:]
	}
	if len(visibleLines) > contentHeight {
		visibleLines = visibleLines[:contentHeight]
	}

	b.WriteString(strings.Join(visibleLines, "\n"))
	b.WriteString("\n\n")

	// Prefix popup
	if popup := m.renderPrefixPopup(); popup != "" {
		b.WriteString(popup)
		b.WriteString("\n")
	}

	// Status message
	if m.statusMessage != "" {
		b.WriteString(SuccessStyle.Render(m.statusMessage))
		b.WriteString("\n")
	}

	// Error message
	if m.err != nil {
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
		b.WriteString("\n")
	}

	// Footer
	footer := HelpStyle.Render("j/k nav | n/N section | Tab collapse | ,s/d/b/x status | 1-5 priority | dt/dm due | ? help")
	b.WriteString(footer)

	return renderOverlayCard(b.String(), m.width, m.height, OverlayCardConfig{
		Title:    "Daily Review",
		Subtitle: fmt.Sprintf("%d tasks across %d sections", totalCount, len(m.sectionOrder)),
		Centered: true,
	})
}

// renderPrefixPopup renders the which-key style popup for prefix key sequences.
func (m *DailyReviewModel) renderPrefixPopup() string {
	return RenderPrefixPopup(m.pendingPrefix, PrefixHints{
		",": StatusPrefixHints,
		"d": DueDatePrefixHints,
		"g": DailyReviewNavigationPrefixHints,
	}, m.width)
}

// DailyReviewNavigationPrefixHints contains "g" prefix hints for daily review.
var DailyReviewNavigationPrefixHints = []PrefixHint{
	{Key: "g", Description: "first"},
	{Key: "p", Description: "go to project"},
}

// renderAllSections renders all non-empty sections.
// Returns lines and the line index where the cursor is located.
func (m *DailyReviewModel) renderAllSections() ([]string, int) {
	var lines []string
	currentIndex := 0
	cursorLineIndex := 0
	separatorWidth := min(m.width-10, 50)

	for i, section := range m.sectionOrder {
		if i > 0 {
			lines = append(lines, MutedStyle.Render(strings.Repeat("─", separatorWidth)))
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
	indicator := "▼"
	if collapsed {
		indicator = "▶"
	}

	cursor := "  "
	if isSelected {
		cursor = "> "
	}

	name := getSectionName(section)
	header := fmt.Sprintf("%s%s %s (%d)", cursor, indicator, name, count)

	// Style based on section type
	var styled string
	switch section {
	case SectionOverdue:
		styled = OverdueStyle.Bold(true).Render(header)
	case SectionDueToday:
		styled = DueTodayStyle.Bold(true).Render(header)
	case SectionCompletedRecently:
		styled = SuccessStyle.Bold(true).Render(header)
	case SectionBlocked:
		styled = BlockedStyle.Bold(true).Render(header)
	default:
		styled = DailyReviewSectionHeaderStyle.Render(header)
	}

	if isSelected {
		return SelectedStyle.Render(styled)
	}
	return styled
}

// renderTaskLine renders a single task line.
func (m *DailyReviewModel) renderTaskLine(dt DailyReviewTask, section DailyReviewSection, isSelected bool, maxTitleLen int) string {
	task := dt.Task

	// Cursor
	cursor := "  "
	if isSelected {
		cursor = "> "
	}

	// Project name
	projectName := util.Truncate(dt.Project.Name, 12)
	projectStr := MutedStyle.Render(fmt.Sprintf("[%s]", projectName))

	// Type icon
	typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())

	// Status icon
	var statusIcon string
	if section == SectionBlocked {
		if dt.Project.HasIncompleteDependencies(task) {
			statusIcon = BlockedByDepsStyle.Render(task.Status.Icon())
		} else {
			statusIcon = BlockedStyle.Render(task.Status.Icon())
		}
	} else if task.Status.IsComplete() {
		statusIcon = GetStatusStyle(string(task.Status)).Render(task.Status.Icon())
	} else {
		statusIcon = GetStatusBadge(string(task.Status), task.Status.Icon())
	}

	// Priority (only show high priority)
	priorityStr := ""
	if task.Priority <= 2 {
		priorityStr = formatPriorityBadge(task.Priority) + " "
	}

	// Title
	if maxTitleLen < 10 {
		maxTitleLen = 10
	}
	title := util.Truncate(task.Title, maxTitleLen)

	// Additional context based on section
	contextStr := ""
	switch section {
	case SectionCompletedRecently:
		if task.CompletedAt != nil {
			contextStr = MutedStyle.Render(formatRelativeTime(*task.CompletedAt))
		}
	case SectionOverdue, SectionDueToday, SectionThisWeek:
		if task.DueDate != nil {
			contextStr = m.formatDueDateContext(task.DueDate, section)
		}
	case SectionBlocked:
		// Show blocking reason
		if dt.Project.HasIncompleteDependencies(task) {
			deps := dt.Project.GetIncompleteDependencies(task)
			if len(deps) > 0 {
				contextStr = MutedStyle.Render(fmt.Sprintf("waiting on #%d", deps[0].ShortID))
			}
		} else {
			contextStr = MutedStyle.Render("blocked")
		}
	}

	// Build line
	line := fmt.Sprintf("%s%s %s %s%s%s", cursor, projectStr, statusIcon, typeIcon, priorityStr, title)
	if contextStr != "" {
		// Right-align context info
		lineWidth := len(line) // Approximate, ignoring ANSI
		padding := m.width - lineWidth - len(contextStr) - 5
		if padding > 0 {
			line = fmt.Sprintf("%-*s %s", m.width-len(contextStr)-5, line, contextStr)
		} else {
			line = line + " " + contextStr
		}
	}

	if isSelected {
		line = SelectedStyle.Render(line)
	}

	return line
}

// formatDueDateContext formats due date with appropriate styling for context.
func (m *DailyReviewModel) formatDueDateContext(dueDate *time.Time, section DailyReviewSection) string {
	if dueDate == nil {
		return ""
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dueDay := time.Date(dueDate.Year(), dueDate.Month(), dueDate.Day(), 0, 0, 0, 0, now.Location())

	var dateStr string

	switch section {
	case SectionOverdue:
		daysAgo := int(today.Sub(dueDay).Hours() / 24)
		if daysAgo == 1 {
			dateStr = "yesterday"
		} else {
			dateStr = fmt.Sprintf("%d days ago", daysAgo)
		}
		return OverdueStyle.Render(dateStr)

	case SectionDueToday:
		return DueTodayStyle.Render("today")

	case SectionThisWeek:
		if dueDay.Equal(today.AddDate(0, 0, 1)) {
			dateStr = "tomorrow"
		} else {
			dateStr = dueDate.Format("Mon")
		}
		return DueDateStyle.Render(dateStr)
	}

	return DueDateStyle.Render(dueDate.Format("Jan 2"))
}

// DailyReviewSectionHeaderStyle is the style for section headers.
var DailyReviewSectionHeaderStyle = SubtitleStyle.Bold(true)
