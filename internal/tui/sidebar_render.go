package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/storage"
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

// renderSidebarContent renders the detail sidebar for the currently selected task.
func renderSidebarContent(host SidebarHost, task *models.Task, width, height int) string {
	if task == nil {
		emptyCard := renderSidebarCard("Task", MutedStyle.Render("No task selected"), width, false, "")
		return padSidebarToHeight(emptyCard, nil, width, height)
	}

	project := host.SidebarProject()
	cfg := host.SidebarConfig()
	sidebar := host.SidebarGetSidebar()

	// Get section color for accent on Task and Info cards
	sectionColor := project.GetSectionColor(task.Section)
	if task.CustomSection != "" {
		sectionColor = project.GetSectionColor(task.CustomSection)
	}

	// Calculate inner width for card content (width minus card borders and padding)
	cardInnerWidth := width - 6 // 2 border + 2 padding + 2 margin

	var cards []string

	// Task card: title, type/status, ID/priority (uses section color)
	cards = append(cards, renderSidebarTaskCard(project, task, width, sectionColor))

	// Due card (only if due date set)
	if task.DueDate != nil {
		cards = append(cards, renderSidebarDueCard(task, width))
	}

	// Metadata card: tags + section (uses section color)
	cards = append(cards, renderSidebarMetadataCard(project, task, width, sectionColor))

	// Score card (only if scoring profile assigned)
	if cfg != nil {
		if profile := cfg.GetProjectScoringProfile(project.ID); profile != nil {
			cards = append(cards, renderSidebarScoreCard(task, width, profile))
		}
	}

	// Checklist card (only if has checklist items)
	if len(task.Checklist) > 0 {
		cards = append(cards, renderSidebarChecklistCard(task, width))
	}

	// Dependencies card (only if has dependencies)
	hasDeps := len(task.DependsOn) > 0 || len(project.GetBlockedByTasks(task)) > 0
	if hasDeps {
		cards = append(cards, renderSidebarDependenciesCard(host, task, width, cardInnerWidth))
	}

	// Related tasks card (only if has related tasks)
	if len(task.RelatedTo) > 0 {
		cards = append(cards, renderSidebarRelatedCard(host, task, width, cardInnerWidth))
	}

	// Activity card
	cards = append(cards, renderSidebarActivityCard(task, width))

	// Comments card (only if has comments)
	if len(task.Comments) > 0 {
		cards = append(cards, renderSidebarCommentsCard(task, width))
	}

	// Attachments card (only if has attachments)
	if task.HasAttachments() {
		cards = append(cards, renderSidebarAttachmentsCard(task, width))
	}

	// Description card (only if has description)
	if task.Description != "" {
		cards = append(cards, renderSidebarDescriptionCard(task, width))
	}

	// Help hint at bottom if sidebar focused
	helpHint := ""
	if sidebar.IsFocused() && sidebarHasLinkedTasks(project, task) {
		helpHint = HelpStyle.Render("j/k: nav · h/l: section · Enter: jump · x: unlink")
	} else if sidebar.IsFocused() {
		helpHint = HelpStyle.Render("j/k: scroll · Esc: back")
	}

	// Join cards with no spacing (cards have their own borders)
	content := strings.Join(cards, "\n")
	if helpHint != "" {
		content += "\n" + helpHint
	}

	return padSidebarToHeight(content, sidebar, width, height)
}

// renderSidebarTaskCard renders the main task info card.
func renderSidebarTaskCard(project *models.Project, task *models.Task, width int, sectionColor string) string {
	content := renderTaskHeaderContent(task, project.HasIncompleteDependencies(task), true)
	return renderSidebarCard("Task", content, width, false, sectionColor)
}

// renderSidebarDueCard renders the due date card.
func renderSidebarDueCard(task *models.Task, width int) string {
	return renderSidebarCard("Due", renderDueDateContent(task), width, false, "")
}

// renderSidebarMetadataCard renders tags and section in a combined card.
func renderSidebarMetadataCard(project *models.Project, task *models.Task, width int, sectionColor string) string {
	content := renderMetadataContent(task, project, width-6)
	return renderSidebarCard("Info", content, width, false, sectionColor)
}

// renderSidebarChecklistCard renders the checklist items card.
func renderSidebarChecklistCard(task *models.Task, width int) string {
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

// renderSidebarDepSectionItems renders the scrolled list portion of a dependency section.
func renderSidebarDepSectionItems(sidebar *SidebarComponent, items []sidebarLinkedTask, sectionIndex, innerWidth int) []string {
	const maxVisiblePerSection = 5
	var lines []string

	sectionScroll := 0
	if sidebar.IsFocused() && sidebar.GetDependencySection() == sectionIndex {
		cursor := sidebar.GetDependencyCursor()
		sectionScroll = sidebar.GetDepsScrollOffset()
		if cursor >= sectionScroll+maxVisiblePerSection {
			sectionScroll = cursor - maxVisiblePerSection + 1
		}
		if cursor < sectionScroll {
			sectionScroll = cursor
		}
		if sectionScroll != sidebar.GetDepsScrollOffset() {
			sidebar.SetDepsScrollOffset(sectionScroll)
		}
	}

	if sectionScroll > 0 {
		lines = append(lines, "  "+ScrollUpIndicator(sectionScroll))
	}

	visibleEnd := min(sectionScroll+maxVisiblePerSection, len(items))

	for i := sectionScroll; i < visibleEnd; i++ {
		isCursor := sidebar.IsFocused() && sidebar.GetDependencySection() == sectionIndex && i == sidebar.GetDependencyCursor()
		lines = append(lines, formatSidebarLinkedTask(items[i], innerWidth, isCursor, sidebar.IsFocused()))
	}

	if visibleEnd < len(items) {
		lines = append(lines, "  "+ScrollDownIndicator(len(items)-visibleEnd))
	}

	return lines
}

// sidebarDepSectionHeader returns a styled section header, bold+underline when focused.
func sidebarDepSectionHeader(sidebar *SidebarComponent, label string, sectionIndex int) string {
	if sidebar.IsFocused() && sidebar.GetDependencySection() == sectionIndex {
		return BoldStyle.Underline(true).Render(label)
	}
	return MutedStyle.Render(label)
}

// renderSidebarDependenciesCard renders the dependencies card with navigation support.
func renderSidebarDependenciesCard(host SidebarHost, task *models.Task, width, innerWidth int) string {
	project := host.SidebarProject()
	cfg := host.SidebarConfig()
	sidebar := host.SidebarGetSidebar()
	var lines []string

	// Blocked by (tasks this depends on)
	if len(task.DependsOn) > 0 {
		lines = append(lines, sidebarDepSectionHeader(sidebar, "Blocked by:", 0))

		var blockingTasks []sidebarLinkedTask
		for _, depRef := range task.DependsOn {
			blockingTasks = append(blockingTasks, sidebarResolveSidebarTaskRef(project, cfg, depRef))
		}
		lines = append(lines, renderSidebarDepSectionItems(sidebar, blockingTasks, 0, innerWidth)...)
	}

	// Blocking (tasks that depend on this) - same-project only
	blockedBy := project.GetBlockedByTasks(task)
	if len(blockedBy) > 0 {
		if len(task.DependsOn) > 0 {
			lines = append(lines, "") // spacer
		}
		lines = append(lines, sidebarDepSectionHeader(sidebar, "Blocking:", 1))

		var blockingItems []sidebarLinkedTask
		for _, blocked := range blockedBy {
			blockingItems = append(blockingItems, sidebarLinkedTask{Task: blocked, Project: nil, Ref: blocked.ID, IsMissing: false})
		}
		lines = append(lines, renderSidebarDepSectionItems(sidebar, blockingItems, 1, innerWidth)...)
	}

	focused := sidebar.IsFocused() && sidebarHasDependencies(project, task)
	return renderSidebarCard("Deps", strings.Join(lines, "\n"), width, focused, "")
}

// renderSidebarRelatedCard renders the related tasks card with navigation support.
func renderSidebarRelatedCard(host SidebarHost, task *models.Task, width, innerWidth int) string {
	project := host.SidebarProject()
	cfg := host.SidebarConfig()
	sidebar := host.SidebarGetSidebar()

	var relatedTasks []sidebarLinkedTask
	for _, relRef := range task.RelatedTo {
		relatedTasks = append(relatedTasks, sidebarResolveSidebarTaskRef(project, cfg, relRef))
	}
	if len(relatedTasks) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, sidebarDepSectionHeader(sidebar, "Related:", 2))
	lines = append(lines, renderSidebarDepSectionItems(sidebar, relatedTasks, 2, innerWidth)...)

	focused := sidebar.IsFocused() && sidebar.GetDependencySection() == 2
	return renderSidebarCard("Related", strings.Join(lines, "\n"), width, focused, "")
}

// renderSidebarActivityCard renders the activity timestamps card.
func renderSidebarActivityCard(task *models.Task, width int) string {
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

// renderSidebarCommentsCard renders the comments card showing all comments.
func renderSidebarCommentsCard(task *models.Task, width int) string {
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
		innerWidth := max(width-CardBorderOverhead, 20)
		rendered := renderMarkdown(comment.Text, innerWidth)
		if comment.Resolved {
			lines = append(lines, MutedStyle.Render(rendered))
		} else {
			lines = append(lines, rendered)
		}
	}

	return renderSidebarCard("Comments", strings.Join(lines, "\n"), width, false, "")
}

// renderSidebarAttachmentsCard renders the attachments card showing filenames only.
func renderSidebarAttachmentsCard(task *models.Task, width int) string {
	var lines []string

	maxLen := max(width-10, 20)

	for i, attachment := range task.Attachments {
		if i > 0 {
			lines = append(lines, "") // spacer between attachments
		}
		displayName := util.Truncate(attachment.Filename, maxLen)
		if attachment.IsURL {
			link := Hyperlink(attachment.Filename, displayName)
			lines = append(lines, MutedStyle.Render("🔗 ")+link)
		} else {
			lines = append(lines, MutedStyle.Render("📎 ")+displayName)
		}
	}

	return renderSidebarCard("Attachments", strings.Join(lines, "\n"), width, false, "")
}

// renderSidebarDescriptionCard renders the description card.
func renderSidebarDescriptionCard(task *models.Task, width int) string {
	innerWidth := max(width-CardBorderOverhead, 20)
	rendered := renderMarkdown(task.Description, innerWidth)
	lines := strings.Split(rendered, "\n")
	maxLines := 5
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines = append(lines, MutedStyle.Render("..."))
	}

	return renderSidebarCard("Description", strings.Join(lines, "\n"), width, false, "")
}

// sidebarResolveSidebarTaskRef resolves a task reference (local or cross-project) for sidebar display.
func sidebarResolveSidebarTaskRef(project *models.Project, cfg *config.Config, ref string) sidebarLinkedTask {
	taskRef := models.ParseTaskRef(ref)

	if !taskRef.IsCrossProject() {
		task := project.GetTaskByID(taskRef.TaskID)
		if task != nil {
			return sidebarLinkedTask{Task: task, Project: nil, Ref: ref, IsMissing: false}
		}
		return sidebarLinkedTask{Task: nil, Project: nil, Ref: ref, IsMissing: true}
	}

	// Cross-project reference
	if cfg == nil {
		return sidebarLinkedTask{Task: nil, Project: nil, Ref: ref, IsMissing: true}
	}

	targetProject := cfg.GetProject(taskRef.ProjectID)
	if targetProject == nil {
		return sidebarLinkedTask{Task: nil, Project: nil, Ref: ref, IsMissing: true}
	}

	if len(targetProject.Tasks) == 0 {
		_ = storage.LoadProject(targetProject)
	}

	task := targetProject.GetTaskByID(taskRef.TaskID)
	if task == nil {
		return sidebarLinkedTask{Task: nil, Project: targetProject, Ref: ref, IsMissing: true}
	}

	return sidebarLinkedTask{Task: task, Project: targetProject, Ref: ref, IsMissing: false}
}

// formatSidebarLinkedTask formats a linked task line for sidebar display.
func formatSidebarLinkedTask(linked sidebarLinkedTask, innerWidth int, cursor, isFocused bool) string {
	cursorStr := "  "
	if cursor {
		cursorStr = "> "
	}

	if linked.IsMissing {
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

	depStatus := GetStatusStyle(string(linked.Task.Status))

	titleWidth := innerWidth - 12
	if linked.Project != nil {
		titleWidth -= len(linked.Project.Name) + 3
	}
	if titleWidth < 10 {
		titleWidth = 10
	}

	var line string
	if linked.Project != nil {
		projectLabel := MutedStyle.Render(fmt.Sprintf("[%s] ", linked.Project.Name))
		line = fmt.Sprintf("%s%s#%-3d %s %s",
			cursorStr, projectLabel, linked.Task.ShortID, depStatus.Render(linked.Task.Status.Icon()),
			util.Truncate(linked.Task.Title, titleWidth))
	} else {
		line = fmt.Sprintf("%s#%-3d %s %s",
			cursorStr, linked.Task.ShortID, depStatus.Render(linked.Task.Status.Icon()),
			util.Truncate(linked.Task.Title, titleWidth))
	}

	if cursor && isFocused {
		line = BoldStyle.Render(line)
	}
	return line
}

// renderSidebarScoreCard renders the score breakdown card for the sidebar.
func renderSidebarScoreCard(task *models.Task, width int, profile *scoring.ScoringProfile) string {
	content := renderScoreCardContent(task, profile, width-6)
	return renderSidebarCard("Score", content, width, false, "")
}

// padSidebarToHeight ensures the sidebar content fills the exact height.
// When a sidebar component is provided, applies and clamps its scroll offset.
// Attaches a scrollbar when content overflows.
func padSidebarToHeight(content string, sidebar *SidebarComponent, width, height int) string {
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	// Apply and clamp scroll offset from sidebar
	scrollOffset := 0
	if sidebar != nil {
		maxScroll := max(totalLines-height, 0)
		if sidebar.scroll > maxScroll {
			sidebar.scroll = maxScroll
		}
		scrollOffset = sidebar.scroll
		if scrollOffset > 0 {
			lines = lines[scrollOffset:]
		}
	}

	if len(lines) > height {
		lines = lines[:height]
	}

	// Attach scrollbar when content overflows
	scrollbar := RenderScrollbar(totalLines, height, scrollOffset)
	if scrollbar != "" {
		result := AttachScrollbar(strings.Join(lines, "\n"), scrollbar)
		lines = strings.Split(result, "\n")
	}

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

// renderVerticalDivider renders a vertical line for split-pane separation.
// When focused is true, the divider uses ColorPrimary; otherwise ColorBorder.
func renderVerticalDivider(height int, focused bool) string {
	if height <= 0 {
		return ""
	}
	var b strings.Builder
	for i := range height {
		b.WriteString("┃")
		if i < height-1 {
			b.WriteString("\n")
		}
	}
	color := ColorBorder
	if focused {
		color = ColorPrimary
	}
	return lipgloss.NewStyle().Foreground(color).Render(b.String())
}
