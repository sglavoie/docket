package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// sidebarLinkedTask represents a resolved task reference for sidebar display.
type sidebarLinkedTask struct {
	Task      *models.Task    // nil if not found (broken link)
	Project   *models.Project // nil for same-project links
	Ref       string          // Original reference string
	IsMissing bool            // True if the task/project couldn't be found
}

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
	// Prefer custom section color when available
	sectionColor := m.project.GetSectionColor(task.Section)
	if task.CustomSection != "" {
		sectionColor = m.project.GetSectionColor(task.CustomSection)
	}

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

	// Checklist card (only if has checklist items)
	if len(task.Checklist) > 0 {
		cards = append(cards, m.renderChecklistCard(task, width))
	}

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
		helpHint = HelpStyle.Render("j/k: nav | h/l: section | Enter: jump | x: unlink")
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
	priorityBadge := renderPriorityBadge(task.Priority)
	idLine := fmt.Sprintf("#%d %s", task.ShortID, priorityBadge)

	content := title + "\n" + typeLine + " " + statusLine + " " + idLine

	if task.IsSnoozed() {
		snoozeStr := task.SnoozedUntil.Format("Jan 2")
		if task.SnoozedUntil.Year() != time.Now().Year() {
			snoozeStr = task.SnoozedUntil.Format("Jan 2 '06")
		}
		content += "\n" + MutedStyle.Render("zzz "+snoozeStr)
	}

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

	// Section line (GTD section)
	sectionName := task.Section
	if sectionName == "" {
		sectionName = "None"
	}
	sectionStyle := GetSectionColorStyle(sectionColor)
	sectionLine := MutedStyle.Render("Section: ") + sectionStyle.Render(sectionName)
	lines = append(lines, sectionLine)

	// Custom section line
	if task.CustomSection != "" {
		csColor := m.project.GetSectionColor(task.CustomSection)
		csStyle := GetSectionColorStyle(csColor)
		csLine := MutedStyle.Render("Custom:  ") + csStyle.Render(task.CustomSection)
		lines = append(lines, csLine)
	}

	return renderSidebarCard("Info", strings.Join(lines, "\n"), width, false, sectionColor)
}

// renderChecklistCard renders the checklist items card.
func (m *ProjectTreeModel) renderChecklistCard(task *models.Task, width int) string {
	var lines []string

	completed := 0
	for _, item := range task.Checklist {
		if item.Done {
			completed++
		}
	}

	// Progress header
	header := fmt.Sprintf("%d/%d completed", completed, len(task.Checklist))
	lines = append(lines, MutedStyle.Render(header))

	// Show checklist items (limit to 6 visible)
	maxVisible := 6
	for i, item := range task.Checklist {
		if i >= maxVisible {
			remaining := len(task.Checklist) - maxVisible
			lines = append(lines, MutedStyle.Render(fmt.Sprintf("  ... +%d more", remaining)))
			break
		}
		marker := "☐"
		style := lipgloss.NewStyle()
		if item.Done {
			marker = "☑"
			style = MutedStyle
		}
		lines = append(lines, fmt.Sprintf("  %s %s", marker, style.Render(item.Text)))
	}

	return renderSidebarCard("Checklist", strings.Join(lines, "\n"), width, false, "")
}

// renderDependenciesCard renders the dependencies card with navigation support.
// Supports cross-project task references.
func (m *ProjectTreeModel) renderDependenciesCard(task *models.Task, width, innerWidth int) string {
	var lines []string
	const maxVisiblePerSection = 5

	// Blocked by (tasks this depends on) - may include cross-project refs
	if len(task.DependsOn) > 0 {
		header := "Blocked by:"
		if m.sidebar.IsFocused() && m.sidebar.GetDependencySection() == 0 {
			header = BoldStyle.Underline(true).Render(header)
		} else {
			header = MutedStyle.Render(header)
		}
		lines = append(lines, header)

		// Resolve all blocking task references (including cross-project)
		var blockingTasks []sidebarLinkedTask
		for _, depRef := range task.DependsOn {
			blockingTasks = append(blockingTasks, m.resolveSidebarTaskRef(depRef))
		}

		// Calculate scroll offset for this section when it's active
		sectionScroll := 0
		if m.sidebar.IsFocused() && m.sidebar.GetDependencySection() == 0 {
			cursor := m.sidebar.GetDependencyCursor()
			sectionScroll = m.sidebar.GetDepsScrollOffset()
			if cursor >= sectionScroll+maxVisiblePerSection {
				sectionScroll = cursor - maxVisiblePerSection + 1
			}
			if cursor < sectionScroll {
				sectionScroll = cursor
			}
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
			isCursor := m.sidebar.IsFocused() && m.sidebar.GetDependencySection() == 0 && i == m.sidebar.GetDependencyCursor()
			lines = append(lines, m.formatSidebarLinkedTask(blockingTasks[i], innerWidth, isCursor, m.sidebar.IsFocused()))
		}

		// Show indicator if items hidden below
		if visibleEnd < len(blockingTasks) {
			lines = append(lines, MutedStyle.Render(fmt.Sprintf("  ↓ %d more", len(blockingTasks)-visibleEnd)))
		}
	}

	// Blocking (tasks that depend on this) - same-project only
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
		sectionScroll := 0
		if m.sidebar.IsFocused() && m.sidebar.GetDependencySection() == 1 {
			cursor := m.sidebar.GetDependencyCursor()
			sectionScroll = m.sidebar.GetDepsScrollOffset()
			if cursor >= sectionScroll+maxVisiblePerSection {
				sectionScroll = cursor - maxVisiblePerSection + 1
			}
			if cursor < sectionScroll {
				sectionScroll = cursor
			}
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
			// Wrap as sidebarLinkedTask for consistent formatting
			linked := sidebarLinkedTask{Task: blocked, Project: nil, Ref: blocked.ID, IsMissing: false}
			isCursor := m.sidebar.IsFocused() && m.sidebar.GetDependencySection() == 1 && i == m.sidebar.GetDependencyCursor()
			lines = append(lines, m.formatSidebarLinkedTask(linked, innerWidth, isCursor, m.sidebar.IsFocused()))
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
// Supports cross-project task references.
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

	// Resolve all related task references (including cross-project)
	var relatedTasks []sidebarLinkedTask
	for _, relRef := range task.RelatedTo {
		relatedTasks = append(relatedTasks, m.resolveSidebarTaskRef(relRef))
	}
	if len(relatedTasks) == 0 {
		return ""
	}

	// Calculate scroll offset for this section when it's active
	sectionScroll := 0
	if m.sidebar.IsFocused() && m.sidebar.GetDependencySection() == 2 {
		cursor := m.sidebar.GetDependencyCursor()
		sectionScroll = m.sidebar.GetDepsScrollOffset()
		if cursor >= sectionScroll+maxVisiblePerSection {
			sectionScroll = cursor - maxVisiblePerSection + 1
		}
		if cursor < sectionScroll {
			sectionScroll = cursor
		}
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
		isCursor := m.sidebar.IsFocused() && m.sidebar.GetDependencySection() == 2 && i == m.sidebar.GetDependencyCursor()
		lines = append(lines, m.formatSidebarLinkedTask(relatedTasks[i], innerWidth, isCursor, m.sidebar.IsFocused()))
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

	if !task.CreatedAt.IsZero() {
		lines = append(lines, formatActivityEntry("Created", task.CreatedAt))
	}

	if task.CompletedAt != nil {
		lines = append(lines, formatActivityEntry("Completed", *task.CompletedAt))
	}

	if !task.UpdatedAt.IsZero() && !task.UpdatedAt.Equal(task.CreatedAt) {
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

// resolveSidebarTaskRef resolves a task reference (local or cross-project) for sidebar display.
func (m *ProjectTreeModel) resolveSidebarTaskRef(ref string) sidebarLinkedTask {
	taskRef := models.ParseTaskRef(ref)

	if !taskRef.IsCrossProject() {
		// Same-project reference
		task := m.project.GetTaskByID(taskRef.TaskID)
		if task != nil {
			return sidebarLinkedTask{
				Task:      task,
				Project:   nil,
				Ref:       ref,
				IsMissing: false,
			}
		}
		return sidebarLinkedTask{
			Task:      nil,
			Project:   nil,
			Ref:       ref,
			IsMissing: true,
		}
	}

	// Cross-project reference
	if m.config == nil {
		return sidebarLinkedTask{
			Task:      nil,
			Project:   nil,
			Ref:       ref,
			IsMissing: true,
		}
	}

	targetProject := m.config.GetProject(taskRef.ProjectID)
	if targetProject == nil {
		return sidebarLinkedTask{
			Task:      nil,
			Project:   nil,
			Ref:       ref,
			IsMissing: true,
		}
	}

	// Load the project if not already loaded
	if len(targetProject.Tasks) == 0 {
		_ = storage.LoadProject(targetProject)
	}

	task := targetProject.GetTaskByID(taskRef.TaskID)
	if task == nil {
		return sidebarLinkedTask{
			Task:      nil,
			Project:   targetProject,
			Ref:       ref,
			IsMissing: true,
		}
	}

	return sidebarLinkedTask{
		Task:      task,
		Project:   targetProject,
		Ref:       ref,
		IsMissing: false,
	}
}

// formatSidebarLinkedTask formats a linked task line for sidebar display.
func (m *ProjectTreeModel) formatSidebarLinkedTask(linked sidebarLinkedTask, innerWidth int, cursor, isFocused bool) string {
	cursorStr := "  "
	if cursor {
		cursorStr = "> "
	}

	if linked.IsMissing {
		// Show broken link indicator
		ref := models.ParseTaskRef(linked.Ref)
		var label string
		if ref.IsCrossProject() {
			if linked.Project != nil {
				label = fmt.Sprintf("[%s] (missing)", linked.Project.Name)
			} else {
				label = fmt.Sprintf("[%s] (missing)", ref.ProjectID)
			}
		} else {
			label = "(missing)"
		}
		line := cursorStr + ValidationErrorStyle.Render(label)
		if cursor && isFocused {
			line = BoldStyle.Render(line)
		}
		return line
	}

	// Format found task
	depStatus := GetStatusStyle(string(linked.Task.Status))

	// Calculate available width for title
	titleWidth := innerWidth - 12
	if linked.Project != nil {
		titleWidth -= len(linked.Project.Name) + 3 // "[ProjectName] "
	}
	if titleWidth < 10 {
		titleWidth = 10
	}

	var line string
	if linked.Project != nil {
		// Cross-project: show project name
		projectLabel := MutedStyle.Render(fmt.Sprintf("[%s] ", linked.Project.Name))
		line = fmt.Sprintf("%s%s#%-3d %s %s",
			cursorStr, projectLabel, linked.Task.ShortID, depStatus.Render(linked.Task.Status.Icon()),
			util.Truncate(linked.Task.Title, titleWidth))
	} else {
		// Same-project
		line = fmt.Sprintf("%s#%-3d %s %s",
			cursorStr, linked.Task.ShortID, depStatus.Render(linked.Task.Status.Icon()),
			util.Truncate(linked.Task.Title, titleWidth))
	}

	if cursor && isFocused {
		line = BoldStyle.Render(line)
	}
	return line
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
	case duration < util.Day:
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case duration < util.Week:
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case duration < util.Month:
		weeks := int(duration.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	case duration < util.Year:
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
