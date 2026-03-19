package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// maxContentWidth aliases the layout constant for local use.
const maxContentWidth = MaxContentWidth

func (m *TaskDetailModel) View() string {
	if m.task == nil || m.project == nil {
		return "No task selected"
	}

	// Determine content width (capped, centered if wider)
	contentWidth := min(m.width, maxContentWidth)

	// Build all sections
	var sections []string

	sections = append(sections, m.renderHeaderSection(contentWidth))

	if m.task.DueDate != nil {
		sections = append(sections, m.renderDueSection(contentWidth))
	}

	if m.task.IsRecurring() {
		sections = append(sections, m.renderRecurrenceSection(contentWidth))
	}

	sections = append(sections, m.renderMetadataSection(contentWidth))

	if m.config != nil {
		if profile := m.config.GetProjectScoringProfile(m.project.ID); profile != nil {
			sections = append(sections, m.renderScoreSection(contentWidth, profile))
		}
	}

	if len(m.task.Checklist) > 0 {
		sections = append(sections, m.renderChecklistSection(contentWidth))
	}

	hasDeps := len(m.task.DependsOn) > 0 || len(m.project.GetBlockedByTasks(m.task)) > 0
	if hasDeps || len(m.task.RelatedTo) > 0 {
		sections = append(sections, m.renderDependenciesSection(contentWidth))
	}

	if m.task.Description != "" {
		sections = append(sections, m.renderDescriptionSection(contentWidth))
	}

	if len(m.task.Comments) > 0 {
		sections = append(sections, m.renderCommentsSection(contentWidth))
	}

	if m.task.HasAttachments() {
		sections = append(sections, m.renderAttachmentsSection(contentWidth))
	}

	sections = append(sections, m.renderActivitySection(contentWidth))

	// Join sections
	body := strings.Join(sections, "\n")

	// Split into lines for scrolling
	allLines := strings.Split(body, "\n")
	m.totalLines = len(allLines)
	m.clampScroll()

	vh := m.viewHeight()
	end := min(m.scrollOffset+vh, len(allLines))
	visibleLines := allLines[m.scrollOffset:end]

	// Pad to fill viewport
	for len(visibleLines) < vh {
		visibleLines = append(visibleLines, "")
	}

	// Center content if terminal is wider than maxContentWidth
	content := strings.Join(visibleLines, "\n")
	if m.width > maxContentWidth {
		padLeft := (m.width - maxContentWidth) / 2
		if padLeft > 0 {
			padStr := strings.Repeat(" ", padLeft)
			var centered []string
			for _, line := range visibleLines {
				centered = append(centered, padStr+line)
			}
			content = strings.Join(centered, "\n")
		}
	}

	// Title bar
	titleBar := m.renderTitleBar()

	// Footer with scroll position
	footer := m.renderFooter()

	return titleBar + "\n" + content + "\n" + footer
}

// renderTitleBar renders the top title bar.
func (m *TaskDetailModel) renderTitleBar() string {
	title := renderFullscreenTitle("Task Detail", fmt.Sprintf("#%d", m.task.ShortID))
	return title
}

// renderFooter renders the bottom footer with scroll position and help.
func (m *TaskDetailModel) renderFooter() string {
	var scrollInfo string
	if m.totalLines > m.viewHeight() {
		pct := 0
		maxScroll := m.totalLines - m.viewHeight()
		if maxScroll > 0 {
			pct = m.scrollOffset * 100 / maxScroll
		}
		scrollInfo = fmt.Sprintf("%d%% (%d/%d)", pct, m.scrollOffset+1, m.totalLines)
	} else {
		scrollInfo = "All"
	}

	left := StatusBarStyle.Render(fmt.Sprintf("#%d %s", m.task.ShortID, util.Truncate(m.task.Title, 40)))
	right := HelpStyle.Render("j/k: scroll · ,: status · !: priority · e: edit · c: comments · d: due · h: history · q: close  " + scrollInfo)
	return renderStatusBarFooter(left, right, m.width)
}

// renderHeaderSection renders the task header card.
func (m *TaskDetailModel) renderHeaderSection(width int) string {
	content := renderTaskHeaderContent(m.task, m.project.HasIncompleteDependencies(m.task), true)

	cfg := DefaultCardConfig()
	sectionColor := m.project.GetSectionColor(m.task.Section)
	if m.task.CustomSection != "" {
		sectionColor = m.project.GetSectionColor(m.task.CustomSection)
	}
	cfg.AccentColor = GetSectionColorValue(sectionColor)
	return renderCard("Task", content, width, cfg)
}

// renderDueSection renders the due date card.
// Omits the recurrence line since the task detail view has a dedicated Recurrence card.
func (m *TaskDetailModel) renderDueSection(width int) string {
	return renderCard("Due", renderDueDateContentOpts(m.task, m.task.IsRecurring()), width, DefaultCardConfig())
}

// renderRecurrenceSection renders the recurrence card with pattern and upcoming dates.
func (m *TaskDetailModel) renderRecurrenceSection(width int) string {
	var lines []string
	lines = append(lines, MutedStyle.Render("Pattern: ")+m.task.Recurrence.String())

	if m.task.Recurrence.Streak > 0 || m.task.Recurrence.MaxStreak > 0 {
		lines = append(lines, MutedStyle.Render(fmt.Sprintf("Streak:  %d (best: %d)", m.task.Recurrence.Streak, m.task.Recurrence.MaxStreak)))
	}
	if m.task.Recurrence.Skipped > 0 {
		lines = append(lines, MutedStyle.Render(fmt.Sprintf("Skipped: %d times", m.task.Recurrence.Skipped)))
	}

	upcoming := m.task.NextNDueDates(3)
	if len(upcoming) > 0 {
		lines = append(lines, "")
		lines = append(lines, MutedStyle.Render("Upcoming:"))
		for _, d := range upcoming {
			lines = append(lines, "  "+MutedStyle.Render(d.Format("Mon Jan 2, 2006")))
		}
	}

	return renderCard("Recurrence", strings.Join(lines, "\n"), width, DefaultCardConfig())
}

// renderMetadataSection renders tags, section, and type-specific attributes.
func (m *TaskDetailModel) renderMetadataSection(width int) string {
	content := renderMetadataContent(m.task, m.project, width-6)
	sectionColor := m.project.GetSectionColor(m.task.Section)
	cfg := DefaultCardConfig()
	cfg.AccentColor = GetSectionColorValue(sectionColor)
	return renderCard("Info", content, width, cfg)
}

// renderScoreSection renders the score breakdown card.
func (m *TaskDetailModel) renderScoreSection(width int, profile *scoring.ScoringProfile) string {
	content := renderScoreCardContent(m.task, profile, width-6)
	return renderCard("Score", content, width, DefaultCardConfig())
}

// renderChecklistSection renders all checklist items.
func (m *TaskDetailModel) renderChecklistSection(width int) string {
	var lines []string

	completed := 0
	for _, item := range m.task.Checklist {
		if item.Done {
			completed++
		}
	}

	header := fmt.Sprintf("%d/%d completed", completed, len(m.task.Checklist))
	lines = append(lines, MutedStyle.Render(header))

	for _, item := range m.task.Checklist {
		marker := "☐"
		style := NormalStyle
		if item.Done {
			marker = "☑"
			style = MutedStyle
		}
		lines = append(lines, fmt.Sprintf("  %s %s", marker, style.Render(item.Text)))
	}

	return renderCard("Checklist", strings.Join(lines, "\n"), width, DefaultCardConfig())
}

// renderDependenciesSection renders blocked-by, blocking, and related tasks.
func (m *TaskDetailModel) renderDependenciesSection(width int) string {
	var lines []string

	// Blocked by (tasks this depends on)
	if len(m.task.DependsOn) > 0 {
		lines = append(lines, MutedStyle.Render("Blocked by:"))
		for _, depRef := range m.task.DependsOn {
			lines = append(lines, m.formatLinkedTask(depRef, width-6))
		}
	}

	// Blocking (tasks that depend on this)
	blockedBy := m.project.GetBlockedByTasks(m.task)
	if len(blockedBy) > 0 {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, MutedStyle.Render("Blocking:"))
		for _, blocked := range blockedBy {
			depStatus := GetStatusStyle(string(blocked.Status))
			line := fmt.Sprintf("  #%-3d %s %s",
				blocked.ShortID, depStatus.Render(blocked.Status.Icon()), blocked.Title)
			lines = append(lines, line)
		}
	}

	// Related tasks
	if len(m.task.RelatedTo) > 0 {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, MutedStyle.Render("Related:"))
		for _, relRef := range m.task.RelatedTo {
			lines = append(lines, m.formatLinkedTask(relRef, width-6))
		}
	}

	return renderCard("Dependencies", strings.Join(lines, "\n"), width, DefaultCardConfig())
}

// formatLinkedTask formats a task reference for display.
func (m *TaskDetailModel) formatLinkedTask(ref string, _ int) string {
	taskRef := models.ParseTaskRef(ref)

	if !taskRef.IsCrossProject() {
		task := m.project.GetTaskByID(taskRef.TaskID)
		if task != nil {
			depStatus := GetStatusStyle(string(task.Status))
			return fmt.Sprintf("  #%-3d %s %s",
				task.ShortID, depStatus.Render(task.Status.Icon()), task.Title)
		}
		return "  " + ValidationErrorStyle.Render("(missing)")
	}

	// Cross-project reference
	if m.config == nil {
		return "  " + ValidationErrorStyle.Render("(missing)")
	}
	targetProject := m.config.GetProject(taskRef.ProjectID)
	if targetProject == nil {
		return "  " + ValidationErrorStyle.Render(fmt.Sprintf("[%s] (missing)", taskRef.ProjectID))
	}
	if len(targetProject.Tasks) == 0 {
		if err := storage.LoadProject(targetProject); err != nil {
			logging.Warn("failed to load project %s: %v", targetProject.Name, err)
		}
	}
	task := targetProject.GetTaskByID(taskRef.TaskID)
	if task == nil {
		return "  " + ValidationErrorStyle.Render(fmt.Sprintf("[%s] (missing)", targetProject.Name))
	}
	depStatus := GetStatusStyle(string(task.Status))
	projectLabel := MutedStyle.Render(fmt.Sprintf("[%s] ", targetProject.Name))
	return fmt.Sprintf("  %s#%-3d %s %s",
		projectLabel, task.ShortID, depStatus.Render(task.Status.Icon()), task.Title)
}

// renderMarkdownDescription renders the task description as styled markdown.
func (m *TaskDetailModel) renderMarkdownDescription(contentWidth int) string {
	if m.task.Description == "" {
		return ""
	}

	innerWidth := max(contentWidth-CardBorderOverhead, 10)

	// Return cached result if width hasn't changed
	if m.renderedDesc != "" && m.renderedDescWidth == innerWidth {
		return m.renderedDesc
	}

	m.renderedDesc = renderMarkdown(m.task.Description, innerWidth)
	m.renderedDescWidth = innerWidth
	return m.renderedDesc
}

// renderDescriptionSection renders the full task description.
func (m *TaskDetailModel) renderDescriptionSection(width int) string {
	content := m.renderMarkdownDescription(width)
	return renderCard("Description", content, width, DefaultCardConfig())
}

// renderCommentsSection renders all comments with timestamps.
func (m *TaskDetailModel) renderCommentsSection(width int) string {
	var lines []string

	for i, comment := range m.task.Comments {
		if i > 0 {
			lines = append(lines, "")
		}
		relative := formatRelativeTime(comment.CreatedAt)
		timestampLine := relative + ":"
		if comment.Resolved {
			timestampLine += " [resolved]"
		}
		lines = append(lines, MutedStyle.Render(timestampLine))
		rendered := renderMarkdown(comment.Text, width-CardBorderOverhead)
		if comment.Resolved {
			lines = append(lines, MutedStyle.Render(rendered))
		} else {
			lines = append(lines, rendered)
		}
	}

	return renderCard("Comments", strings.Join(lines, "\n"), width, DefaultCardConfig())
}

// renderAttachmentsSection renders all attachments.
func (m *TaskDetailModel) renderAttachmentsSection(width int) string {
	var lines []string

	for i, attachment := range m.task.Attachments {
		if i > 0 {
			lines = append(lines, "")
		}
		if attachment.IsURL {
			link := Hyperlink(attachment.Filename, attachment.Filename)
			lines = append(lines, MutedStyle.Render("🔗 ")+link)
		} else {
			lines = append(lines, MutedStyle.Render("📎 ")+attachment.Filename)
		}
	}

	return renderCard("Attachments", strings.Join(lines, "\n"), width, DefaultCardConfig())
}

// renderActivitySection renders activity timestamps.
func (m *TaskDetailModel) renderActivitySection(width int) string {
	var lines []string

	if !m.task.CreatedAt.IsZero() {
		lines = append(lines, formatActivityEntry("Created", m.task.CreatedAt))
	}
	if m.task.CompletedAt != nil {
		lines = append(lines, formatActivityEntry("Completed", *m.task.CompletedAt))
	}
	if !m.task.UpdatedAt.IsZero() && !m.task.UpdatedAt.Equal(m.task.CreatedAt) {
		lines = append(lines, formatActivityEntry("Modified", m.task.UpdatedAt))
	}

	return renderCard("Activity", strings.Join(lines, "\n"), width, DefaultCardConfig())
}
