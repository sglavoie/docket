package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/storage"
)

// maxContentWidth aliases the layout constant for local use.
const maxContentWidth = MaxContentWidth

func (m *TaskDetailModel) View() tea.View {
	if m.task == nil || m.project == nil {
		return tea.NewView("No task selected")
	}

	// Determine content width (capped, centered if wider)
	contentWidth := min(m.width, maxContentWidth)
	content := m.renderDetailContent(contentWidth)

	// Title bar
	titleBar := m.renderTitleBar()

	// Footer with scroll position
	footer := m.renderFooter()

	return tea.NewView(titleBar + "\n" + content + "\n" + footer)
}

func (m *TaskDetailModel) renderDetailContent(contentWidth int) string {
	body := strings.Join(m.detailSections(contentWidth), "\n")
	allLines := strings.Split(body, "\n")
	m.totalLines = len(allLines)
	m.clampScroll()

	visibleLines := m.visibleDetailLines(allLines)
	return m.centerDetailLines(visibleLines)
}

func (m *TaskDetailModel) detailSections(contentWidth int) []string {
	sections := append([]string{}, m.overviewSections(contentWidth)...)
	planningSections, planningHasFrame := m.planningSections(contentWidth)
	if len(planningSections) > 0 {
		if !planningHasFrame {
			sections = append(sections, renderDetailSectionDivider("Planning", contentWidth))
		}
		sections = append(sections, planningSections...)
	}

	history := m.historySections(contentWidth)
	if history.hasHistory {
		if !history.hasFrame {
			sections = append(sections, renderHistoryClusterDivider(history.commentsN, history.decisionsN, history.attachmentsN, m.historyExpanded, contentWidth))
		}
		sections = append(sections, history.sections...)
	}
	return sections
}

func (m *TaskDetailModel) overviewSections(contentWidth int) []string {
	sections := []string{m.renderHeaderSection(contentWidth)}
	if m.task.DueDate != nil {
		sections = append(sections, m.renderDueSection(contentWidth))
	}
	if m.task.IsRecurring() {
		sections = append(sections, m.renderRecurrenceSection(contentWidth))
	}
	if m.task.Description != "" {
		sections = append(sections, m.renderDescriptionSection(contentWidth))
	}
	if metaCard := m.renderMetadataSection(contentWidth); metaCard != "" {
		sections = append(sections, metaCard)
	}
	return sections
}

func (m *TaskDetailModel) planningSections(contentWidth int) ([]string, bool) {
	var sections []string
	planningHasFrame := false
	if scoreProfile := m.taskDetailScoreProfile(); scoreProfile != nil {
		sections = append(sections, m.renderPlanningCluster(scoreProfile, contentWidth))
		planningHasFrame = true
	} else {
		sections = append(sections, m.renderActivitySection(contentWidth))
	}
	if len(m.task.Checklist) > 0 {
		sections = append(sections, m.renderChecklistSection(contentWidth))
	}
	if m.hasDetailDependencies() || len(m.task.RelatedTo) > 0 {
		sections = append(sections, m.renderDependenciesSection(contentWidth))
	}
	return sections, planningHasFrame
}

func (m *TaskDetailModel) taskDetailScoreProfile() *scoring.ScoringProfile {
	if m.config == nil {
		return nil
	}
	return m.config.GetProjectScoringProfile(m.project.ID)
}

func (m *TaskDetailModel) renderPlanningCluster(scoreProfile *scoring.ScoringProfile, contentWidth int) string {
	innerW := max(contentWidth-CardBorderOverhead, 4)
	leftCol := innerW / 2
	rightCol := innerW - leftCol
	scoreCol := renderClusterColumn("Score", renderScoreCardContent(m.task, scoreProfile, leftCol-2), leftCol)
	activityCol := renderClusterColumn("Activity", buildActivityContent(m.task), rightCol)
	return renderClusterFrame("Planning", []string{scoreCol, activityCol}, contentWidth)
}

func (m *TaskDetailModel) hasDetailDependencies() bool {
	return len(m.task.DependsOn) > 0 || len(m.project.GetBlockedByTasks(m.task)) > 0
}

type taskDetailHistorySections struct {
	sections     []string
	hasHistory   bool
	hasFrame     bool
	commentsN    int
	decisionsN   int
	attachmentsN int
}

func (m *TaskDetailModel) historySections(contentWidth int) taskDetailHistorySections {
	history := taskDetailHistorySections{
		hasHistory:   len(m.task.Comments) > 0 || len(m.task.Decisions) > 0 || m.task.HasAttachments(),
		commentsN:    len(m.task.Comments),
		decisionsN:   len(m.task.Decisions),
		attachmentsN: len(m.task.Attachments),
	}
	if !history.hasHistory || !m.historyExpanded {
		return history
	}

	switch {
	case history.commentsN > 0 && history.decisionsN > 0:
		history.sections = append(history.sections, m.renderHistoryClusterFrame(contentWidth))
		history.hasFrame = true
	case history.commentsN > 0:
		history.sections = append(history.sections, m.renderCommentsSection(contentWidth))
	case history.decisionsN > 0:
		history.sections = append(history.sections, m.renderDecisionsSection(contentWidth))
	}
	if history.attachmentsN > 0 {
		history.sections = append(history.sections, m.renderAttachmentsSection(contentWidth))
	}
	return history
}

func (m *TaskDetailModel) renderHistoryClusterFrame(contentWidth int) string {
	innerW := max(contentWidth-CardBorderOverhead, 4)
	leftCol := innerW / 2
	rightCol := innerW - leftCol
	commentsCol := renderClusterColumn("Comments", buildCommentsContent(m.task, leftCol-2), leftCol)
	decisionsCol := renderClusterColumn("Decisions", buildDecisionsContent(m.task, rightCol-2), rightCol)
	frameLabel := "History"
	if summary := historyCountsSummary(len(m.task.Comments), len(m.task.Decisions), 0); summary != "" {
		frameLabel = "History (" + summary + ")"
	}
	return renderClusterFrame(frameLabel, []string{commentsCol, decisionsCol}, contentWidth)
}

func (m *TaskDetailModel) visibleDetailLines(allLines []string) []string {
	vh := m.viewHeight()
	end := min(m.scrollOffset+vh, len(allLines))
	visibleLines := allLines[m.scrollOffset:end]
	for len(visibleLines) < vh {
		visibleLines = append(visibleLines, "")
	}
	return visibleLines
}

func (m *TaskDetailModel) centerDetailLines(visibleLines []string) string {
	if m.width <= maxContentWidth {
		return strings.Join(visibleLines, "\n")
	}

	padLeft := (m.width - maxContentWidth) / 2
	if padLeft <= 0 {
		return strings.Join(visibleLines, "\n")
	}

	padStr := strings.Repeat(" ", padLeft)
	centered := make([]string, 0, len(visibleLines))
	for _, line := range visibleLines {
		centered = append(centered, padStr+line)
	}
	return strings.Join(centered, "\n")
}

// renderDetailSectionDivider renders a labeled cluster divider line.
func renderDetailSectionDivider(label string, width int) string {
	prefix := "── " + label + " "
	remaining := max(width-len(prefix), 0)
	return MutedStyle.Render(prefix + strings.Repeat("─", remaining))
}

// renderHistoryClusterDivider renders the History cluster divider with an
// inline summary of counts and an expand/collapse hint. When collapsed, the
// summary acts as the only content; when expanded, it labels the sections
// rendered below.
func renderHistoryClusterDivider(commentsN, decisionsN, attachmentsN int, expanded bool, width int) string {
	summary := historyCountsSummary(commentsN, decisionsN, attachmentsN)
	var hint string
	if expanded {
		hint = "⏷ press H to collapse"
	} else {
		hint = "⏵ press H to expand"
	}
	prefix := "── History "
	if summary != "" {
		prefix += "(" + summary + ") "
	}
	prefix += hint + " "
	remaining := max(width-len(prefix), 0)
	return MutedStyle.Render(prefix + strings.Repeat("─", remaining))
}

// historyCountsSummary builds a "3 comments · 1 decision · 2 attachments" string,
// omitting zero entries and pluralizing correctly.
func historyCountsSummary(commentsN, decisionsN, attachmentsN int) string {
	var parts []string
	if commentsN > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", commentsN, pluralize(commentsN, "comment", "comments")))
	}
	if decisionsN > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", decisionsN, pluralize(decisionsN, "decision", "decisions")))
	}
	if attachmentsN > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", attachmentsN, pluralize(attachmentsN, "attachment", "attachments")))
	}
	return strings.Join(parts, " · ")
}

// renderTitleBar renders the top title bar.
func (m *TaskDetailModel) renderTitleBar() string {
	return renderFullscreenTitleWithCounter("Task Detail", fmt.Sprintf("#%d", m.task.ShortID), FormatOpenLoops(m.openLoops), m.width, m.AccentColor())
}

// renderFooter renders the bottom footer with scroll position and help.
func (m *TaskDetailModel) renderFooter() string {
	var right string
	if m.totalLines > m.viewHeight() {
		pct := 0
		maxScroll := m.totalLines - m.viewHeight()
		if maxScroll > 0 {
			pct = m.scrollOffset * 100 / maxScroll
		}
		right = HelpStyle.Render(fmt.Sprintf("%d%% (%d/%d)", pct, m.scrollOffset+1, m.totalLines))
	}

	left := HelpStyle.Render("j/k: scroll · ,: status · !: priority · e: edit · c: comments · D: decisions · d: due · h: history · H: toggle history · Esc: close")
	return renderStatusBarFooter(left, right, m.width, m.AccentColor())
}

// renderHeaderSection renders the task header card with section badge.
func (m *TaskDetailModel) renderHeaderSection(width int) string {
	content := renderTaskHeaderContent(m.task, m.project.HasIncompleteDependencies(m.task), true)

	effectiveSection := m.task.Section
	if effectiveSection == "" && m.task.CustomSection != "" {
		effectiveSection = m.task.CustomSection
	}
	sectionColor := m.project.GetSectionColor(effectiveSection)

	// Append section badge to the type/status/ID line (line index 1)
	sectionName := effectiveSection
	if sectionName == "" {
		sectionName = "Active"
	}
	sectionStyle := GetSectionColorStyle(sectionColor)
	badge := sectionStyle.Render("[" + sectionName + "]")
	lines := strings.Split(content, "\n")
	if len(lines) > 1 {
		lines[1] = lines[1] + " " + badge
	}
	content = strings.Join(lines, "\n")

	cfg := DefaultCardConfig()
	cfg.AccentColor = GetSectionColorValue(sectionColor)
	return renderCard("Task", content, width, cfg)
}

// renderDueSection renders the due date card.
// Omits the recurrence line since the task detail view has a dedicated Recurrence card.
func (m *TaskDetailModel) renderDueSection(width int) string {
	return renderCard("Due", renderDueDateContentOpts(m.task, m.task.IsRecurring(), m.config.GetTimeFormat()), width, DefaultCardConfig())
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

// renderMetadataSection renders tags, source, URL, and type-specific attributes.
// Section lines are hidden here since they appear in the header badge.
// Returns "" if no content remains after hiding sections.
func (m *TaskDetailModel) renderMetadataSection(width int) string {
	content := renderMetadataContentOpts(m.task, m.project, width-6, true)
	if strings.TrimSpace(content) == "" {
		return ""
	}
	cfg := DefaultCardConfig()
	return renderCard("Info", content, width, cfg)
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
		marker := icons.Get("checkbox.unchecked")
		style := NormalStyle
		if item.Done {
			marker = icons.Get("checkbox.checked")
			style = MutedStyle
		}
		lines = append(lines, fmt.Sprintf("  %s %s", marker, style.Render(item.Text)))
	}

	return renderCard(IconChecklist()+" Checklist", strings.Join(lines, "\n"), width, DefaultCardConfig())
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

	// Return cached result if width and theme haven't changed
	if m.renderedDesc != "" && m.renderedDescWidth == innerWidth && m.renderedDescTheme == activeThemeName {
		return m.renderedDesc
	}

	m.renderedDesc = renderMarkdown(m.task.Description, innerWidth)
	m.renderedDescWidth = innerWidth
	m.renderedDescTheme = activeThemeName
	return m.renderedDesc
}

// renderDescriptionSection renders the full task description.
func (m *TaskDetailModel) renderDescriptionSection(width int) string {
	content := m.renderMarkdownDescription(width)
	return renderCard("Description", content, width, DefaultCardConfig())
}
