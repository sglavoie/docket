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
// accentColor is optional - pass empty string for default styling.
func renderSidebarCard(title, content string, width int, focused bool, accentColor string) string {
	config := DefaultCardConfig()
	config.Focused = focused
	config.AccentColor = GetSectionColorValue(accentColor)
	return renderCard(title, content, width, config)
}

// renderSidebar renders the detail sidebar for the currently selected task.
func (m *ProjectTreeModel) renderSidebar(task *models.Task, width, height int) string {
	if task == nil {
		emptyCard := renderSidebarCard("Task", MutedStyle.Render("No task selected"), width, false, "")
		return m.padSidebarToHeight(emptyCard, width, height)
	}

	// Get section color for accent on Task and Info cards
	sectionColor := m.project.GetSectionColor(task.Section)

	// Calculate inner width for card content (width minus card borders and padding)
	cardInnerWidth := width - 6 // 2 border + 2 padding + 2 margin

	var cards []string

	// Task card: title, type/status, ID/priority (uses section color)
	cards = append(cards, m.renderTaskCard(task, width, sectionColor))

	// Due card (only if due date set)
	if task.DueDate != nil {
		cards = append(cards, m.renderDueCard(task, width))
	}

	// Metadata card: tags + section (uses section color)
	cards = append(cards, m.renderMetadataCard(task, width, sectionColor))

	// Dependencies card (only if has dependencies)
	hasDeps := len(task.DependsOn) > 0 || len(m.project.GetBlockedByTasks(task)) > 0
	if hasDeps {
		cards = append(cards, m.renderDependenciesCard(task, width, cardInnerWidth))
	}

	// Related tasks card (only if has related tasks)
	if len(task.RelatedTo) > 0 {
		cards = append(cards, m.renderRelatedCard(task, width, cardInnerWidth))
	}

	// Activity card
	cards = append(cards, m.renderActivityCard(task, width))

	// Comments card (only if has comments)
	if len(task.Comments) > 0 {
		cards = append(cards, m.renderCommentsCard(task, width))
	}

	// Attachments card (only if has attachments)
	if task.HasAttachments() {
		cards = append(cards, m.renderAttachmentsCard(task, width))
	}

	// Description card (only if has description)
	if task.Description != "" {
		cards = append(cards, m.renderDescriptionCard(task, width))
	}

	// Help hint at bottom if sidebar focused
	helpHint := ""
	if m.sidebar.IsFocused() && m.hasLinkedTasks(task) {
		helpHint = HelpStyle.Render("j/k: nav | h/l: section | Enter: jump")
	} else if m.sidebar.IsFocused() {
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
func (m *ProjectTreeModel) renderTaskCard(task *models.Task, width int, sectionColor string) string {
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
	return renderSidebarCard("Task", content, width, false, sectionColor)
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

	return renderSidebarCard("Due", dueLine, width, false, "")
}

// renderMetadataCard renders tags and section in a combined card.
func (m *ProjectTreeModel) renderMetadataCard(task *models.Task, width int, sectionColor string) string {
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
	sectionStyle := GetSectionColorStyle(sectionColor)
	sectionLine := MutedStyle.Render("Section: ") + sectionStyle.Render(sectionName)
	lines = append(lines, sectionLine)

	return renderSidebarCard("Info", strings.Join(lines, "\n"), width, false, sectionColor)
}

// renderDependenciesCard renders the dependencies card with navigation support.
func (m *ProjectTreeModel) renderDependenciesCard(task *models.Task, width, innerWidth int) string {
	var lines []string
	const maxVisiblePerSection = 5

	// Blocked by (tasks this depends on)
	if len(task.DependsOn) > 0 {
		header := "Blocked by:"
		if m.sidebar.IsFocused() && m.sidebar.GetDependencySection() == 0 {
			header = BoldStyle.Underline(true).Render(header)
		} else {
			header = MutedStyle.Render(header)
		}
		lines = append(lines, header)

		blockingTasks := m.project.GetBlockingTasks(task)

		// Calculate scroll offset for this section when it's active
		// Always recalculate to ensure cursor is visible
		sectionScroll := 0
		if m.sidebar.IsFocused() && m.sidebar.GetDependencySection() == 0 {
			cursor := m.sidebar.GetDependencyCursor()
			sectionScroll = m.sidebar.GetDepsScrollOffset()
			// Ensure cursor is within visible window
			if cursor >= sectionScroll+maxVisiblePerSection {
				sectionScroll = cursor - maxVisiblePerSection + 1
			}
			if cursor < sectionScroll {
				sectionScroll = cursor
			}
			// Update stored offset if it changed
			if sectionScroll != m.sidebar.GetDepsScrollOffset() {
				m.sidebar.SetDepsScrollOffset(sectionScroll)
			}
		}

		// Show indicator if items hidden above
		if sectionScroll > 0 {
			lines = append(lines, MutedStyle.Render(fmt.Sprintf("  ↑ %d more", sectionScroll)))
		}

		// Render visible items
		visibleEnd := sectionScroll + maxVisiblePerSection
		if visibleEnd > len(blockingTasks) {
			visibleEnd = len(blockingTasks)
		}

		for i := sectionScroll; i < visibleEnd; i++ {
			dep := blockingTasks[i]
			depStatus := GetStatusStyle(string(dep.Status))
			cursorStr := "  "
			if m.sidebar.IsFocused() && m.sidebar.GetDependencySection() == 0 && i == m.sidebar.GetDependencyCursor() {
				cursorStr = "> "
			}

			depLine := fmt.Sprintf("%s#%-3d %s %s",
				cursorStr, dep.ShortID, depStatus.Render(dep.Status.Icon()),
				util.Truncate(dep.Title, innerWidth-12))

			if m.sidebar.IsFocused() && m.sidebar.GetDependencySection() == 0 && i == m.sidebar.GetDependencyCursor() {
				depLine = BoldStyle.Render(depLine)
			}
			lines = append(lines, depLine)
		}

		// Show indicator if items hidden below
		if visibleEnd < len(blockingTasks) {
			lines = append(lines, MutedStyle.Render(fmt.Sprintf("  ↓ %d more", len(blockingTasks)-visibleEnd)))
		}
	}

	// Blocking (tasks that depend on this)
	blockedBy := m.project.GetBlockedByTasks(task)
	if len(blockedBy) > 0 {
		if len(task.DependsOn) > 0 {
			lines = append(lines, "") // spacer
		}

		header := "Blocking:"
		if m.sidebar.IsFocused() && m.sidebar.GetDependencySection() == 1 {
			header = BoldStyle.Underline(true).Render(header)
		} else {
			header = MutedStyle.Render(header)
		}
		lines = append(lines, header)

		// Calculate scroll offset for this section when it's active
		// Always recalculate to ensure cursor is visible
		sectionScroll := 0
		if m.sidebar.IsFocused() && m.sidebar.GetDependencySection() == 1 {
			cursor := m.sidebar.GetDependencyCursor()
			sectionScroll = m.sidebar.GetDepsScrollOffset()
			// Ensure cursor is within visible window
			if cursor >= sectionScroll+maxVisiblePerSection {
				sectionScroll = cursor - maxVisiblePerSection + 1
			}
			if cursor < sectionScroll {
				sectionScroll = cursor
			}
			// Update stored offset if it changed
			if sectionScroll != m.sidebar.GetDepsScrollOffset() {
				m.sidebar.SetDepsScrollOffset(sectionScroll)
			}
		}

		// Show indicator if items hidden above
		if sectionScroll > 0 {
			lines = append(lines, MutedStyle.Render(fmt.Sprintf("  ↑ %d more", sectionScroll)))
		}

		// Render visible items
		visibleEnd := sectionScroll + maxVisiblePerSection
		if visibleEnd > len(blockedBy) {
			visibleEnd = len(blockedBy)
		}

		for i := sectionScroll; i < visibleEnd; i++ {
			blocked := blockedBy[i]
			blockStatus := GetStatusStyle(string(blocked.Status))
			cursorStr := "  "
			if m.sidebar.IsFocused() && m.sidebar.GetDependencySection() == 1 && i == m.sidebar.GetDependencyCursor() {
				cursorStr = "> "
			}

			blockLine := fmt.Sprintf("%s#%-3d %s %s",
				cursorStr, blocked.ShortID, blockStatus.Render(blocked.Status.Icon()),
				util.Truncate(blocked.Title, innerWidth-12))

			if m.sidebar.IsFocused() && m.sidebar.GetDependencySection() == 1 && i == m.sidebar.GetDependencyCursor() {
				blockLine = BoldStyle.Render(blockLine)
			}
			lines = append(lines, blockLine)
		}

		// Show indicator if items hidden below
		if visibleEnd < len(blockedBy) {
			lines = append(lines, MutedStyle.Render(fmt.Sprintf("  ↓ %d more", len(blockedBy)-visibleEnd)))
		}
	}

	focused := m.sidebar.IsFocused() && m.hasDependencies(task)
	return renderSidebarCard("Deps", strings.Join(lines, "\n"), width, focused, "")
}

// renderRelatedCard renders the related tasks card with navigation support.
func (m *ProjectTreeModel) renderRelatedCard(task *models.Task, width, innerWidth int) string {
	const maxVisiblePerSection = 5
	var lines []string

	// Header with focus indicator
	header := "Related:"
	if m.sidebar.IsFocused() && m.sidebar.GetDependencySection() == 2 {
		header = BoldStyle.Underline(true).Render(header)
	} else {
		header = MutedStyle.Render(header)
	}
	lines = append(lines, header)

	relatedTasks := m.getRelatedTasks(task)
	if len(relatedTasks) == 0 {
		return ""
	}

	// Calculate scroll offset for this section when it's active
	// Always recalculate to ensure cursor is visible
	sectionScroll := 0
	if m.sidebar.IsFocused() && m.sidebar.GetDependencySection() == 2 {
		cursor := m.sidebar.GetDependencyCursor()
		sectionScroll = m.sidebar.GetDepsScrollOffset()
		// Ensure cursor is within visible window
		if cursor >= sectionScroll+maxVisiblePerSection {
			sectionScroll = cursor - maxVisiblePerSection + 1
		}
		if cursor < sectionScroll {
			sectionScroll = cursor
		}
		// Update stored offset if it changed
		if sectionScroll != m.sidebar.GetDepsScrollOffset() {
			m.sidebar.SetDepsScrollOffset(sectionScroll)
		}
	}

	// Show indicator if items hidden above
	if sectionScroll > 0 {
		lines = append(lines, MutedStyle.Render(fmt.Sprintf("  ↑ %d more", sectionScroll)))
	}

	// Render visible items
	visibleEnd := sectionScroll + maxVisiblePerSection
	if visibleEnd > len(relatedTasks) {
		visibleEnd = len(relatedTasks)
	}

	for i := sectionScroll; i < visibleEnd; i++ {
		relatedTask := relatedTasks[i]
		relatedStatus := GetStatusStyle(string(relatedTask.Status))
		cursorStr := "  "
		if m.sidebar.IsFocused() && m.sidebar.GetDependencySection() == 2 && i == m.sidebar.GetDependencyCursor() {
			cursorStr = "> "
		}

		relatedLine := fmt.Sprintf("%s#%-3d %s %s",
			cursorStr, relatedTask.ShortID, relatedStatus.Render(relatedTask.Status.Icon()),
			util.Truncate(relatedTask.Title, innerWidth-12))

		if m.sidebar.IsFocused() && m.sidebar.GetDependencySection() == 2 && i == m.sidebar.GetDependencyCursor() {
			relatedLine = BoldStyle.Render(relatedLine)
		}
		lines = append(lines, relatedLine)
	}

	// Show indicator if items hidden below
	if visibleEnd < len(relatedTasks) {
		lines = append(lines, MutedStyle.Render(fmt.Sprintf("  ↓ %d more", len(relatedTasks)-visibleEnd)))
	}

	focused := m.sidebar.IsFocused() && m.sidebar.GetDependencySection() == 2
	return renderSidebarCard("Related", strings.Join(lines, "\n"), width, focused, "")
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

	return renderSidebarCard("Activity", strings.Join(lines, "\n"), width, false, "")
}

// renderCommentsCard renders the comments card showing all comments.
func (m *ProjectTreeModel) renderCommentsCard(task *models.Task, width int) string {
	var lines []string

	for i, comment := range task.Comments {
		if i > 0 {
			lines = append(lines, "") // spacer between comments
		}
		relative := formatRelativeTime(comment.CreatedAt)
		lines = append(lines, MutedStyle.Render(relative+":"))
		lines = append(lines, comment.Text)
	}

	return renderSidebarCard("Comments", strings.Join(lines, "\n"), width, false, "")
}

// renderAttachmentsCard renders the attachments card showing filenames only.
func (m *ProjectTreeModel) renderAttachmentsCard(task *models.Task, width int) string {
	var lines []string

	// Calculate max length for filenames/URLs to fit on one line
	// width - 6 (borders/padding) - 4 (emoji + space, accounting for wide emoji)
	maxLen := width - 10
	if maxLen < 20 {
		maxLen = 20
	}

	for i, attachment := range task.Attachments {
		if i > 0 {
			lines = append(lines, "") // spacer between attachments
		}
		displayName := util.Truncate(attachment.Filename, maxLen)
		if attachment.IsURL {
			// Use OSC 8 hyperlink so truncated URL still links to full URL
			link := Hyperlink(attachment.Filename, displayName)
			lines = append(lines, MutedStyle.Render("🔗 ")+link)
		} else {
			lines = append(lines, MutedStyle.Render("📎 ")+displayName)
		}
	}

	return renderSidebarCard("Attachments", strings.Join(lines, "\n"), width, false, "")
}

// renderDescriptionCard renders the description card with scroll support.
func (m *ProjectTreeModel) renderDescriptionCard(task *models.Task, width int) string {
	lines := strings.Split(task.Description, "\n")
	if m.sidebar.GetScroll() < len(lines) {
		lines = lines[m.sidebar.GetScroll():]
	}
	// Limit visible lines to avoid overflow
	maxLines := 5
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines = append(lines, MutedStyle.Render("..."))
	}

	return renderSidebarCard("Description", strings.Join(lines, "\n"), width, false, "")
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
