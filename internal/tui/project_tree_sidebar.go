package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
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

	// Score card (only if scoring profile assigned)
	if m.config != nil {
		if profile := m.config.GetProjectScoringProfile(m.project.ID); profile != nil {
			cards = append(cards, m.renderScoreCard(task, width, profile))
		}
	}

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
	content := renderTaskHeaderContent(task, m.project.HasIncompleteDependencies(task), true)
	return renderSidebarCard("Task", content, width, false, sectionColor)
}

// renderDueCard renders the due date card.
func (m *ProjectTreeModel) renderDueCard(task *models.Task, width int) string {
	return renderSidebarCard("Due", renderDueDateContent(task), width, false, "")
}

// renderMetadataCard renders tags and section in a combined card.
func (m *ProjectTreeModel) renderMetadataCard(task *models.Task, width int, sectionColor string) string {
	content := renderMetadataContent(task, m.project, width-6)
	return renderSidebarCard("Info", content, width, false, sectionColor)
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

// renderDepSectionItems renders the scrolled list portion of a dependency section:
// scroll offset calculation, ↑/↓ indicators, and visible items.
func (m *ProjectTreeModel) renderDepSectionItems(items []sidebarLinkedTask, sectionIndex, innerWidth int) []string {
	const maxVisiblePerSection = 5
	var lines []string

	sectionScroll := 0
	if m.sidebar.IsFocused() && m.sidebar.GetDependencySection() == sectionIndex {
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

	if sectionScroll > 0 {
		lines = append(lines, "  "+ScrollUpIndicator(sectionScroll))
	}

	visibleEnd := sectionScroll + maxVisiblePerSection
	if visibleEnd > len(items) {
		visibleEnd = len(items)
	}

	for i := sectionScroll; i < visibleEnd; i++ {
		isCursor := m.sidebar.IsFocused() && m.sidebar.GetDependencySection() == sectionIndex && i == m.sidebar.GetDependencyCursor()
		lines = append(lines, m.formatSidebarLinkedTask(items[i], innerWidth, isCursor, m.sidebar.IsFocused()))
	}

	if visibleEnd < len(items) {
		lines = append(lines, "  "+ScrollDownIndicator(len(items)-visibleEnd))
	}

	return lines
}

// depSectionHeader returns a styled section header, bold+underline when focused.
func (m *ProjectTreeModel) depSectionHeader(label string, sectionIndex int) string {
	if m.sidebar.IsFocused() && m.sidebar.GetDependencySection() == sectionIndex {
		return BoldStyle.Underline(true).Render(label)
	}
	return MutedStyle.Render(label)
}

// renderDependenciesCard renders the dependencies card with navigation support.
// Supports cross-project task references.
func (m *ProjectTreeModel) renderDependenciesCard(task *models.Task, width, innerWidth int) string {
	var lines []string

	// Blocked by (tasks this depends on) - may include cross-project refs
	if len(task.DependsOn) > 0 {
		lines = append(lines, m.depSectionHeader("Blocked by:", 0))

		var blockingTasks []sidebarLinkedTask
		for _, depRef := range task.DependsOn {
			blockingTasks = append(blockingTasks, m.resolveSidebarTaskRef(depRef))
		}
		lines = append(lines, m.renderDepSectionItems(blockingTasks, 0, innerWidth)...)
	}

	// Blocking (tasks that depend on this) - same-project only
	blockedBy := m.project.GetBlockedByTasks(task)
	if len(blockedBy) > 0 {
		if len(task.DependsOn) > 0 {
			lines = append(lines, "") // spacer
		}
		lines = append(lines, m.depSectionHeader("Blocking:", 1))

		var blockingItems []sidebarLinkedTask
		for _, blocked := range blockedBy {
			blockingItems = append(blockingItems, sidebarLinkedTask{Task: blocked, Project: nil, Ref: blocked.ID, IsMissing: false})
		}
		lines = append(lines, m.renderDepSectionItems(blockingItems, 1, innerWidth)...)
	}

	focused := m.sidebar.IsFocused() && m.hasDependencies(task)
	return renderSidebarCard("Deps", strings.Join(lines, "\n"), width, focused, "")
}

// renderRelatedCard renders the related tasks card with navigation support.
// Supports cross-project task references.
func (m *ProjectTreeModel) renderRelatedCard(task *models.Task, width, innerWidth int) string {
	var relatedTasks []sidebarLinkedTask
	for _, relRef := range task.RelatedTo {
		relatedTasks = append(relatedTasks, m.resolveSidebarTaskRef(relRef))
	}
	if len(relatedTasks) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, m.depSectionHeader("Related:", 2))
	lines = append(lines, m.renderDepSectionItems(relatedTasks, 2, innerWidth)...)

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
		timestampLine := relative + ":"
		if comment.Resolved {
			timestampLine += " [resolved]"
		}
		lines = append(lines, MutedStyle.Render(timestampLine))
		if comment.Resolved {
			lines = append(lines, MutedStyle.Render(comment.Text))
		} else {
			lines = append(lines, comment.Text)
		}
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

// renderScoreCard renders the score breakdown card for the sidebar.
func (m *ProjectTreeModel) renderScoreCard(task *models.Task, width int, profile *scoring.ScoringProfile) string {
	content := renderScoreCardContent(task, profile, width-6)
	return renderSidebarCard("Score", content, width, false, "")
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

// formatTaskAttributes formats type-specific attributes for sidebar display.
// Only non-empty attributes are included. Returns empty slice if no attributes set.
func formatTaskAttributes(task *models.Task, maxWidth int) []string {
	var lines []string

	switch task.Type {
	case models.TaskTypeBug:
		if task.BugAttrs != nil {
			if task.BugAttrs.Severity != "" {
				lines = append(lines, MutedStyle.Render("Severity: ")+string(task.BugAttrs.Severity))
			}
			if task.BugAttrs.Reproducible != "" {
				lines = append(lines, MutedStyle.Render("Reproducible: ")+string(task.BugAttrs.Reproducible))
			}
			if task.BugAttrs.Workaround != "" {
				preview := util.Truncate(task.BugAttrs.Workaround, maxWidth-12)
				preview = strings.ReplaceAll(preview, "\n", " ")
				lines = append(lines, MutedStyle.Render("Workaround: ")+preview)
			}
		}
	case models.TaskTypeIdea:
		if task.IdeaAttrs != nil {
			if task.IdeaAttrs.Category != "" {
				lines = append(lines, MutedStyle.Render("Category: ")+string(task.IdeaAttrs.Category))
			}
			if task.IdeaAttrs.Effort != "" {
				lines = append(lines, MutedStyle.Render("Effort: ")+string(task.IdeaAttrs.Effort))
			}
			if task.IdeaAttrs.Value != "" {
				lines = append(lines, MutedStyle.Render("Value: ")+string(task.IdeaAttrs.Value))
			}
			if task.IdeaAttrs.Validated {
				lines = append(lines, MutedStyle.Render("Validated: ")+"[x]")
			}
		}
	case models.TaskTypeTask:
		if task.TaskAttrs != nil {
			if task.TaskAttrs.Effort != "" {
				lines = append(lines, MutedStyle.Render("Effort: ")+string(task.TaskAttrs.Effort))
			}
			if task.TaskAttrs.WorkType != "" {
				lines = append(lines, MutedStyle.Render("WorkType: ")+string(task.TaskAttrs.WorkType))
			}
		}
	case models.TaskTypeStory:
		if task.StoryAttrs != nil {
			if task.StoryAttrs.StoryPoints > 0 {
				lines = append(lines, MutedStyle.Render("Points: ")+fmt.Sprintf("%d", task.StoryAttrs.StoryPoints))
			}
			if task.StoryAttrs.Persona != "" {
				preview := util.Truncate(task.StoryAttrs.Persona, maxWidth-9)
				preview = strings.ReplaceAll(preview, "\n", " ")
				lines = append(lines, MutedStyle.Render("Persona: ")+preview)
			}
		}
	case models.TaskTypeEpic:
		if task.EpicAttrs != nil {
			if task.EpicAttrs.Scope != "" {
				lines = append(lines, MutedStyle.Render("Scope: ")+string(task.EpicAttrs.Scope))
			}
			if task.EpicAttrs.Milestone != "" {
				preview := util.Truncate(task.EpicAttrs.Milestone, maxWidth-11)
				lines = append(lines, MutedStyle.Render("Milestone: ")+preview)
			}
			if task.EpicAttrs.SuccessMetrics != "" {
				preview := util.Truncate(task.EpicAttrs.SuccessMetrics, maxWidth-9)
				preview = strings.ReplaceAll(preview, "\n", " ")
				lines = append(lines, MutedStyle.Render("Metrics: ")+preview)
			}
		}
	}

	return lines
}
