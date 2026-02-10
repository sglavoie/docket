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

// maxContentWidth is the maximum width for the content area.
const maxContentWidth = 100

func (m *TaskDetailModel) View() string {
	if m.task == nil {
		return "No task selected"
	}

	// Determine content width (capped, centered if wider)
	contentWidth := m.width
	if contentWidth > maxContentWidth {
		contentWidth = maxContentWidth
	}

	// Build all sections
	var sections []string

	sections = append(sections, m.renderHeaderSection(contentWidth))

	if m.task.DueDate != nil {
		sections = append(sections, m.renderDueSection(contentWidth))
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
	end := m.scrollOffset + vh
	if end > len(allLines) {
		end = len(allLines)
	}
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
	right := HelpStyle.Render("j/k: scroll | ,: status | !: priority | q: close  " + scrollInfo)
	return renderStatusBarFooter(left, right, m.width)
}

// renderHeaderSection renders the task header card.
func (m *TaskDetailModel) renderHeaderSection(width int) string {
	typeStyle := GetTypeStyle(string(m.task.Type))
	statusStyle := GetStatusStyle(string(m.task.Status))

	if m.project.HasIncompleteDependencies(m.task) {
		statusStyle = BlockedByDepsStyle
	}

	title := SidebarHeaderStyle.Render(m.task.Title)
	typeLine := GetTypeBadge(string(m.task.Type), m.task.Type.Icon()) + " " + typeStyle.Render(m.task.Type.HeadingPrefix())
	statusLine := statusStyle.Render(m.task.Status.Icon() + " " + string(m.task.Status))
	priorityBadge := renderPriorityBadge(m.task.Priority)
	idLine := fmt.Sprintf("#%d %s", m.task.ShortID, priorityBadge)

	content := title + "\n" + typeLine + " " + statusLine + " " + idLine

	if m.task.IsSnoozed() {
		snoozeStr := m.task.SnoozedUntil.Format("Jan 2")
		if m.task.SnoozedUntil.Year() != currentYear() {
			snoozeStr = m.task.SnoozedUntil.Format("Jan 2 '06")
		}
		content += "\n" + MutedStyle.Render("zzz "+snoozeStr)
	}

	cfg := DefaultCardConfig()
	sectionColor := m.project.GetSectionColor(m.task.Section)
	if m.task.CustomSection != "" {
		sectionColor = m.project.GetSectionColor(m.task.CustomSection)
	}
	cfg.AccentColor = GetSectionColorValue(sectionColor)
	return renderCard("Task", content, width, cfg)
}

// renderDueSection renders the due date card.
func (m *TaskDetailModel) renderDueSection(width int) string {
	dueAbsolute := m.task.DueDate.Format(util.DateFormat)
	dueRelative := formatRelativeDueDateVerbose(m.task.DueDate)
	dueLine := fmt.Sprintf("%s (%s)", dueAbsolute, dueRelative)

	if isOverdue(m.task) && !m.task.Status.IsComplete() {
		dueLine = OverdueStyle.Render(dueLine)
	} else if isDueToday(m.task) {
		dueLine = DueTodayStyle.Render(dueLine)
	} else {
		dueLine = DueDateStyle.Render(dueLine)
	}

	if m.task.Recurrence != nil {
		dueLine += "\n" + MutedStyle.Render("Recurs: "+m.task.Recurrence.String())
	}

	return renderCard("Due", dueLine, width, DefaultCardConfig())
}

// renderMetadataSection renders tags, section, and type-specific attributes.
func (m *TaskDetailModel) renderMetadataSection(width int) string {
	var lines []string

	if len(m.task.Tags) > 0 {
		var tagParts []string
		for _, tag := range m.task.Tags {
			tagParts = append(tagParts, TagInlineStyle.Render("#"+tag))
		}
		lines = append(lines, strings.Join(tagParts, " "))
	}

	sectionName := m.task.Section
	if sectionName == "" {
		sectionName = "None"
	}
	sectionColor := m.project.GetSectionColor(m.task.Section)
	sectionStyle := GetSectionColorStyle(sectionColor)
	sectionLine := MutedStyle.Render("Section: ") + sectionStyle.Render(sectionName)
	lines = append(lines, sectionLine)

	if m.task.CustomSection != "" {
		csColor := m.project.GetSectionColor(m.task.CustomSection)
		csStyle := GetSectionColorStyle(csColor)
		csLine := MutedStyle.Render("Custom:  ") + csStyle.Render(m.task.CustomSection)
		lines = append(lines, csLine)
	}

	// Type-specific attributes
	attrLines := formatTaskAttributes(m.task, width-6)
	if len(attrLines) > 0 {
		lines = append(lines, "")
		lines = append(lines, attrLines...)
	}

	cfg := DefaultCardConfig()
	cfg.AccentColor = GetSectionColorValue(sectionColor)
	return renderCard("Info", strings.Join(lines, "\n"), width, cfg)
}

// renderScoreSection renders the score breakdown card.
func (m *TaskDetailModel) renderScoreSection(width int, profile *scoring.ScoringProfile) string {
	results := scoring.ScoreBreakdown(m.task, profile)
	totalScore := scoring.ComputeScore(m.task, profile)
	_ = scoring.ScorePercent(totalScore, profile)

	var lines []string

	bar := RenderScoreBar(m.task, profile)
	badge := RenderScoreBadge(m.task, profile)
	formulaStr := MutedStyle.Render(fmt.Sprintf("(%s)", profile.Formula))
	lines = append(lines, fmt.Sprintf("%s %s  %s", bar, badge, formulaStr))
	lines = append(lines, "")

	manualStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	derivedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("44"))

	maxNameWidth := 0
	for _, r := range results {
		if len(r.DimensionName) > maxNameWidth {
			maxNameWidth = len(r.DimensionName)
		}
	}
	cardInner := width - 6
	if maxNameWidth > cardInner-16 {
		maxNameWidth = cardInner - 16
	}

	for _, r := range results {
		name := r.DimensionName
		if len(name) > maxNameWidth {
			name = name[:maxNameWidth]
		}

		valStr := fmt.Sprintf("%d", r.Value)

		var sourceLabel string
		switch r.Source {
		case "manual":
			sourceLabel = manualStyle.Render("manual")
		case "derived":
			sourceLabel = derivedStyle.Render("derived")
		default:
			sourceLabel = MutedStyle.Render("default")
		}

		roleStr := ""
		switch r.Role {
		case scoring.RoleNumerator:
			roleStr = MutedStyle.Render(" [n]")
		case scoring.RoleDenominator:
			roleStr = MutedStyle.Render(" [d]")
		}

		padding := maxNameWidth - len(name)
		if padding < 0 {
			padding = 0
		}
		line := fmt.Sprintf("%s%s %s %s%s",
			MutedStyle.Render(name), strings.Repeat(" ", padding),
			valStr, sourceLabel, roleStr)
		lines = append(lines, line)
	}

	return renderCard("Score", strings.Join(lines, "\n"), width, DefaultCardConfig())
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
		style := lipgloss.NewStyle()
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
func (m *TaskDetailModel) formatLinkedTask(ref string, maxWidth int) string {
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
		_ = storage.LoadProject(targetProject)
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

// renderDescriptionSection renders the full task description.
func (m *TaskDetailModel) renderDescriptionSection(width int) string {
	return renderCard("Description", m.task.Description, width, DefaultCardConfig())
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
		if comment.Resolved {
			lines = append(lines, MutedStyle.Render(comment.Text))
		} else {
			lines = append(lines, comment.Text)
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

// currentYear returns the current year for date formatting.
func currentYear() int {
	return time.Now().Year()
}
