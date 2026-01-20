package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// renderSidebarCard renders a card with a titled border for sidebar sections.
// Uses the shared renderCard utility with sidebar-specific configuration.
func renderSidebarCard(title, content string, width int, focused bool) string {
	config := DefaultCardConfig()
	config.Focused = focused
	return renderCard(title, content, width, config)
}

// renderSidebar renders the detail sidebar for the currently selected task.
func (m *ProjectTreeModel) renderSidebar(task *models.Task, width, height int) string {
	if task == nil {
		emptyCard := renderSidebarCard("Task", MutedStyle.Render("No task selected"), width, false)
		return m.padSidebarToHeight(emptyCard, width, height)
	}

	// Calculate inner width for card content (width minus card borders and padding)
	cardInnerWidth := width - 6 // 2 border + 2 padding + 2 margin

	var cards []string

	// Task card: title, type/status, ID/priority
	cards = append(cards, m.renderTaskCard(task, width))

	// Due card (only if due date set)
	if task.DueDate != nil {
		cards = append(cards, m.renderDueCard(task, width))
	}

	// Metadata card: tags + section
	cards = append(cards, m.renderMetadataCard(task, width))

	// Dependencies card (only if has dependencies)
	hasDeps := len(task.DependsOn) > 0 || len(m.project.GetBlockedByTasks(task)) > 0
	if hasDeps {
		cards = append(cards, m.renderDependenciesCard(task, width, cardInnerWidth))
	}

	// Activity card
	cards = append(cards, m.renderActivityCard(task, width))

	// Description card (only if has description)
	if task.Description != "" {
		cards = append(cards, m.renderDescriptionCard(task, width))
	}

	// Help hint at bottom if sidebar focused
	helpHint := ""
	if m.sidebarFocused && m.hasDependencies(task) {
		helpHint = HelpStyle.Render("j/k: nav | h/l: section | Enter: jump")
	} else if m.sidebarFocused {
		helpHint = HelpStyle.Render("j/k: scroll | Esc: back")
	}

	// Join cards with no spacing (cards have their own borders)
	content := strings.Join(cards, "\n")
	if helpHint != "" {
		content += "\n" + helpHint
	}

	return m.padSidebarToHeight(content, width, height)
}

// renderTaskCard renders the main task info card.
func (m *ProjectTreeModel) renderTaskCard(task *models.Task, width int) string {
	typeStyle := GetTypeStyle(string(task.Type))
	statusStyle := GetStatusStyle(string(task.Status))

	if m.project.HasIncompleteDependencies(task) {
		statusStyle = BlockedByDepsStyle
	}

	title := SidebarHeaderStyle.Render(task.Title)
	typeLine := GetTypeBadge(string(task.Type), task.Type.Icon()) + " " + typeStyle.Render(task.Type.HeadingPrefix())
	statusLine := statusStyle.Render(task.Status.Icon() + " " + string(task.Status))
	priorityBadge := formatPriorityBadge(task.Priority)
	idLine := fmt.Sprintf("#%d %s", task.ShortID, priorityBadge)

	content := title + "\n" + typeLine + " " + statusLine + " " + idLine
	return renderSidebarCard("Task", content, width, false)
}

// renderDueCard renders the due date card.
func (m *ProjectTreeModel) renderDueCard(task *models.Task, width int) string {
	dueAbsolute := task.DueDate.Format(util.DateFormat)
	dueRelative := formatRelativeDueDateVerbose(task.DueDate)
	dueLine := fmt.Sprintf("%s (%s)", dueAbsolute, dueRelative)

	if isOverdue(task) && !task.Status.IsComplete() {
		dueLine = OverdueStyle.Render(dueLine)
	} else if isDueToday(task) {
		dueLine = DueTodayStyle.Render(dueLine)
	} else {
		dueLine = DueDateStyle.Render(dueLine)
	}

	return renderSidebarCard("Due", dueLine, width, false)
}

// renderMetadataCard renders tags and section in a combined card.
func (m *ProjectTreeModel) renderMetadataCard(task *models.Task, width int) string {
	var lines []string

	// Tags line - use simple inline style without background/padding for alignment
	if len(task.Tags) > 0 {
		var tagParts []string
		for _, tag := range task.Tags {
			tagParts = append(tagParts, TagInlineStyle.Render("#"+tag))
		}
		lines = append(lines, strings.Join(tagParts, " "))
	}

	// Section line
	sectionName := task.Section
	if sectionName == "" {
		sectionName = "None"
	}
	sectionColor := m.project.GetSectionColor(task.Section)
	sectionStyle := GetSectionColorStyle(sectionColor)
	sectionLine := MutedStyle.Render("Section: ") + sectionStyle.Render(sectionName)
	lines = append(lines, sectionLine)

	return renderSidebarCard("Info", strings.Join(lines, "\n"), width, false)
}

// renderDependenciesCard renders the dependencies card with navigation support.
func (m *ProjectTreeModel) renderDependenciesCard(task *models.Task, width, innerWidth int) string {
	var lines []string

	// Blocked by (tasks this depends on)
	if len(task.DependsOn) > 0 {
		header := "Blocked by:"
		if m.sidebarFocused && m.sidebarDependencySection == 0 {
			header = BoldStyle.Underline(true).Render(header)
		} else {
			header = MutedStyle.Render(header)
		}
		lines = append(lines, header)

		blockingTasks := m.project.GetBlockingTasks(task)
		for i, dep := range blockingTasks {
			depStatus := GetStatusStyle(string(dep.Status))
			cursor := "  "
			if m.sidebarFocused && m.sidebarDependencySection == 0 && i == m.sidebarDependencyCursor {
				cursor = "> "
			}

			depLine := fmt.Sprintf("%s#%-3d %s %s",
				cursor, dep.ShortID, depStatus.Render(dep.Status.Icon()),
				util.Truncate(dep.Title, innerWidth-12))

			if m.sidebarFocused && m.sidebarDependencySection == 0 && i == m.sidebarDependencyCursor {
				depLine = BoldStyle.Render(depLine)
			}
			lines = append(lines, depLine)
		}
	}

	// Blocking (tasks that depend on this)
	blockedBy := m.project.GetBlockedByTasks(task)
	if len(blockedBy) > 0 {
		if len(task.DependsOn) > 0 {
			lines = append(lines, "") // spacer
		}

		header := "Blocking:"
		if m.sidebarFocused && m.sidebarDependencySection == 1 {
			header = BoldStyle.Underline(true).Render(header)
		} else {
			header = MutedStyle.Render(header)
		}
		lines = append(lines, header)

		for i, blocked := range blockedBy {
			blockStatus := GetStatusStyle(string(blocked.Status))
			cursor := "  "
			if m.sidebarFocused && m.sidebarDependencySection == 1 && i == m.sidebarDependencyCursor {
				cursor = "> "
			}

			blockLine := fmt.Sprintf("%s#%-3d %s %s",
				cursor, blocked.ShortID, blockStatus.Render(blocked.Status.Icon()),
				util.Truncate(blocked.Title, innerWidth-12))

			if m.sidebarFocused && m.sidebarDependencySection == 1 && i == m.sidebarDependencyCursor {
				blockLine = BoldStyle.Render(blockLine)
			}
			lines = append(lines, blockLine)
		}
	}

	focused := m.sidebarFocused && m.hasDependencies(task)
	return renderSidebarCard("Deps", strings.Join(lines, "\n"), width, focused)
}

// renderActivityCard renders the activity timestamps card.
func (m *ProjectTreeModel) renderActivityCard(task *models.Task, width int) string {
	var lines []string

	lines = append(lines, formatActivityEntry("Created", task.CreatedAt))

	if task.CompletedAt != nil {
		lines = append(lines, formatActivityEntry("Completed", *task.CompletedAt))
	}

	if !task.UpdatedAt.Equal(task.CreatedAt) {
		lines = append(lines, formatActivityEntry("Modified", task.UpdatedAt))
	}

	return renderSidebarCard("Activity", strings.Join(lines, "\n"), width, false)
}

// renderDescriptionCard renders the description card with scroll support.
func (m *ProjectTreeModel) renderDescriptionCard(task *models.Task, width int) string {
	lines := strings.Split(task.Description, "\n")
	if m.sidebarScroll < len(lines) {
		lines = lines[m.sidebarScroll:]
	}
	// Limit visible lines to avoid overflow
	maxLines := 5
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines = append(lines, MutedStyle.Render("..."))
	}

	return renderSidebarCard("Description", strings.Join(lines, "\n"), width, false)
}

// padSidebarToHeight ensures the sidebar content fills the exact height.
func (m *ProjectTreeModel) padSidebarToHeight(content string, width, height int) string {
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

// formatPriorityBadge returns a styled priority badge.
func formatPriorityBadge(priority int) string {
	badge := fmt.Sprintf("!%d", priority)
	switch priority {
	case 1:
		return PriorityHighStyle.Render(badge)
	case 2:
		return PriorityMediumStyle.Render(badge)
	default:
		return PriorityLowStyle.Render(badge)
	}
}

// formatRelativeDueDateVerbose returns a verbose relative due date string.
func formatRelativeDueDateVerbose(dueDate *time.Time) string {
	if dueDate == nil {
		return ""
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dueDay := time.Date(dueDate.Year(), dueDate.Month(), dueDate.Day(), 0, 0, 0, 0, now.Location())
	daysUntil := int(dueDay.Sub(today).Hours() / 24)

	switch {
	case daysUntil < -1:
		return fmt.Sprintf("%d days ago", -daysUntil)
	case daysUntil == -1:
		return "yesterday"
	case daysUntil == 0:
		return "today"
	case daysUntil == 1:
		return "tomorrow"
	case daysUntil < 7:
		return fmt.Sprintf("in %d days", daysUntil)
	case daysUntil < 14:
		return "next week"
	case daysUntil < 30:
		weeks := daysUntil / 7
		return fmt.Sprintf("in %d weeks", weeks)
	default:
		months := daysUntil / 30
		return fmt.Sprintf("in %d months", months)
	}
}

// formatActivityEntry formats a timestamp as an activity entry.
func formatActivityEntry(label string, timestamp time.Time) string {
	absolute := timestamp.Format(util.DateFormat)
	relative := formatRelativeTime(timestamp)
	return fmt.Sprintf("%s: %s (%s)",
		MutedStyle.Render(label),
		absolute,
		relative)
}

// formatRelativeTime returns a human-readable relative time string.
func formatRelativeTime(t time.Time) string {
	duration := time.Since(t)

	switch {
	case duration < time.Minute:
		return "just now"
	case duration < time.Hour:
		mins := int(duration.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case duration < 24*time.Hour:
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case duration < 7*24*time.Hour:
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case duration < 30*24*time.Hour:
		weeks := int(duration.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	case duration < 365*24*time.Hour:
		months := int(duration.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	default:
		years := int(duration.Hours() / 24 / 365)
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}
