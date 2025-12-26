package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// renderSidebar renders the detail sidebar for the currently selected task.
func (m *ProjectTreeModel) renderSidebar(task *models.Task, width, height int) string {
	if task == nil {
		return SidebarStyle.
			Width(width).
			Height(height).
			Render(MutedStyle.Render("No task selected"))
	}

	var b strings.Builder

	// Header: Task title, type, status, priority, short ID
	typeStyle := GetTypeStyle(string(task.Type))
	statusStyle := GetStatusStyle(string(task.Status))

	// Override status style if task has incomplete dependencies
	if m.project.HasIncompleteDependencies(task) {
		statusStyle = BlockedByDepsStyle
	}

	headerLine1 := fmt.Sprintf("%s %s",
		typeStyle.Render(task.Type.Icon()+" "+task.Type.HeadingPrefix()),
		statusStyle.Render(task.Status.Icon()+" "+string(task.Status)))

	priorityBadge := formatPriorityBadge(task.Priority)
	headerLine2 := fmt.Sprintf("#%d %s", task.ShortID, priorityBadge)

	b.WriteString(SidebarHeaderStyle.Render(task.Title))
	b.WriteString("\n")
	b.WriteString(headerLine1 + " " + headerLine2)
	b.WriteString("\n\n")

	// Due Date
	if task.DueDate != nil {
		dueAbsolute := task.DueDate.Format(util.DateFormat)
		dueRelative := formatRelativeDueDateVerbose(task.DueDate)

		dueLine := fmt.Sprintf("Due: %s (%s)", dueAbsolute, dueRelative)
		if isOverdue(task) && !task.Status.IsComplete() {
			b.WriteString(OverdueStyle.Render(dueLine))
		} else if isDueToday(task) {
			b.WriteString(DueTodayStyle.Render(dueLine))
		} else {
			b.WriteString(DueDateStyle.Render(dueLine))
		}
		b.WriteString("\n\n")
	}

	// Tags
	if len(task.Tags) > 0 {
		b.WriteString(SidebarSectionHeaderStyle.Render("Tags"))
		b.WriteString("\n")
		for _, tag := range task.Tags {
			b.WriteString(TagStyle.Render("#" + tag))
			b.WriteString(" ")
		}
		b.WriteString("\n\n")
	}

	// Section
	b.WriteString(SidebarSectionHeaderStyle.Render("Section"))
	b.WriteString("\n")
	sectionColor := m.project.GetSectionColor(task.Section)
	sectionStyle := GetSectionColorStyle(sectionColor)
	b.WriteString(sectionStyle.Render(task.Section))
	b.WriteString("\n\n")

	// Dependencies (with cursor if sidebar focused)
	if len(task.DependsOn) > 0 || len(m.project.GetBlockedByTasks(task)) > 0 {
		b.WriteString(SidebarSectionHeaderStyle.Render("Dependencies"))
		b.WriteString("\n")

		// Blocked by (tasks this depends on)
		if len(task.DependsOn) > 0 {
			blockedByHeader := "Blocked by:"
			if m.sidebarFocused && m.sidebarDependencySection == 0 {
				blockedByHeader = BoldStyle.Underline(true).Render(blockedByHeader)
			}
			b.WriteString(MutedStyle.Render(blockedByHeader))
			b.WriteString("\n")

			blockingTasks := m.project.GetBlockingTasks(task)
			for i, dep := range blockingTasks {
				depStatus := GetStatusStyle(string(dep.Status))

				cursor := "  "
				if m.sidebarFocused && m.sidebarDependencySection == 0 && i == m.sidebarDependencyCursor {
					cursor = "> "
				}

				depLine := fmt.Sprintf("%s#%-4d %s %s",
					cursor,
					dep.ShortID,
					depStatus.Render(dep.Status.Icon()),
					util.Truncate(dep.Title, width-15))

				if m.sidebarFocused && m.sidebarDependencySection == 0 && i == m.sidebarDependencyCursor {
					depLine = BoldStyle.Render(depLine)
				}

				b.WriteString(depLine)
				b.WriteString("\n")
			}
		}

		// Blocking (tasks that depend on this)
		blockedBy := m.project.GetBlockedByTasks(task)
		if len(blockedBy) > 0 {
			if len(task.DependsOn) > 0 {
				b.WriteString("\n")
			}

			blockingHeader := "Blocking:"
			if m.sidebarFocused && m.sidebarDependencySection == 1 {
				blockingHeader = BoldStyle.Underline(true).Render(blockingHeader)
			}
			b.WriteString(MutedStyle.Render(blockingHeader))
			b.WriteString("\n")

			for i, blocked := range blockedBy {
				blockStatus := GetStatusStyle(string(blocked.Status))

				cursor := "  "
				if m.sidebarFocused && m.sidebarDependencySection == 1 && i == m.sidebarDependencyCursor {
					cursor = "> "
				}

				blockLine := fmt.Sprintf("%s#%-4d %s %s",
					cursor,
					blocked.ShortID,
					blockStatus.Render(blocked.Status.Icon()),
					util.Truncate(blocked.Title, width-15))

				if m.sidebarFocused && m.sidebarDependencySection == 1 && i == m.sidebarDependencyCursor {
					blockLine = BoldStyle.Render(blockLine)
				}

				b.WriteString(blockLine)
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}

	// Activity History (derived from timestamps)
	b.WriteString(SidebarSectionHeaderStyle.Render("Activity"))
	b.WriteString("\n")
	b.WriteString(formatActivityEntry("Created", task.CreatedAt))
	b.WriteString("\n")

	if task.CompletedAt != nil {
		b.WriteString(formatActivityEntry("Completed", *task.CompletedAt))
		b.WriteString("\n")
	}

	if !task.UpdatedAt.Equal(task.CreatedAt) {
		b.WriteString(formatActivityEntry("Last modified", task.UpdatedAt))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Description (scrollable)
	if task.Description != "" {
		b.WriteString(SidebarSectionHeaderStyle.Render("Description"))
		b.WriteString("\n")

		// Apply scroll offset
		lines := strings.Split(task.Description, "\n")
		if m.sidebarScroll < len(lines) {
			visibleLines := lines[m.sidebarScroll:]
			b.WriteString(strings.Join(visibleLines, "\n"))
		}
	} else {
		b.WriteString(MutedStyle.Render("No description"))
	}

	// Sidebar focus help hint
	if m.sidebarFocused && m.hasDependencies(task) {
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("j/k: navigate | h/l: switch section | Enter: jump | Esc: back to tree"))
	} else if m.sidebarFocused {
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("j/k: scroll | Esc: back to tree"))
	}

	content := b.String()

	// Render without height constraint first to preserve all styling
	rendered := SidebarStyle.
		Width(width).
		Render(content)

	// Now manually truncate rendered lines to fit height
	// SidebarStyle has padding (2 lines total), border (2 lines total) = 4 lines overhead
	availableHeight := height - 4
	if availableHeight < 1 {
		availableHeight = 1
	}

	lines := strings.Split(rendered, "\n")
	if len(lines) > availableHeight {
		// Keep content from the start (header), truncate from the end
		lines = lines[:availableHeight]
	}

	// Pad to exact height if needed
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
