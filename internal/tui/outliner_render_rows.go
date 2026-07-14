package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

func (m *OutlinerModel) renderOutlinerRows(viewport outlinerViewport, width int) string {
	var colLayout *ColumnLayout
	if viewport.rowWidth >= MinColumnAlignWidth {
		colLayout = ComputeOutlinerColumnLayout(m.rows[viewport.startIdx:viewport.endIdx], m.columnConfig)
		// Augment with status/type widths from the same row set. Stage moved to
		// the selected-task metadata strip so it is no longer measured here.
		var visibleTasks []*models.Task
		for _, row := range m.rows[viewport.startIdx:viewport.endIdx] {
			if !row.isTaskRow() || row.task == nil {
				continue
			}
			visibleTasks = append(visibleTasks, row.task)
		}
		if badge := ComputeBadgeWidths(visibleTasks, BadgeWidthOpts{
			IncludeStatus: true,
			IncludeType:   true,
		}); badge != nil {
			if colLayout == nil {
				colLayout = badge
			} else {
				colLayout.HasStatus = badge.HasStatus
				colLayout.StatusWidth = badge.StatusWidth
				colLayout.HasType = badge.HasType
				colLayout.TypeWidth = badge.TypeWidth
			}
		}
	}

	var rowBuilder strings.Builder
	for i := viewport.startIdx; i < viewport.endIdx; i++ {
		row := m.rows[i]
		selected := i == m.cursor

		switch {
		case row.IsSeparator:
			if i < viewport.endIdx-1 {
				rowBuilder.WriteString("\n")
			}
			continue
		case row.IsSectionHeader:
			rowBuilder.WriteString(m.renderOutlinerSectionHeaderRow(row, selected, viewport.rowWidth, i))
		case row.IsProjectHeader:
			rowBuilder.WriteString(m.renderOutlinerProjectHeaderRow(row, selected, viewport.rowWidth, i))
		default:
			rowBuilder.WriteString(m.renderOutlinerTaskRow(row, selected, viewport.rowWidth, viewport.rowWidth, i, colLayout))
		}

		if i < viewport.endIdx-1 {
			rowBuilder.WriteString("\n")
		}
	}

	scrollbar := RenderScrollbar(len(m.rows), viewport.visibleRows, viewport.startIdx)
	return attachScrollbarAtWidth(rowBuilder.String(), scrollbar, viewport.rowWidth)
}

func (m *OutlinerModel) renderOutlinerSectionHeaderRow(row outlinerRow, selected bool, rowWidth, rowIndex int) string {
	displayName := row.SectionName
	if displayName == "" {
		displayName = "Active"
	}

	sectionStyle := GetViewModeStyleForSection(row.SectionName)

	line := renderSectionHeader(SectionHeaderOpts{
		Name:        displayName,
		Count:       row.SectionCount,
		Collapsed:   m.isSectionRowCollapsed(row),
		AccentColor: sectionStyle,
		Prefix:      "@ ",
	})

	if limit := m.runtime.Settings().GetSectionLimit(row.SectionName); limit > 0 && row.SectionCount > limit {
		line += "  " + PreviewWarningStyle.Render(sectionLimitWarning(row.SectionCount-limit, limit))
	}
	if row.ProjectID != "" {
		line += "  " + MutedStyle.Render("["+m.projectNameByID(row.ProjectID)+"]")
	}

	if selected {
		if m.mutationFlash {
			return MutationFlashLine(line, rowWidth, m.mutationFlashBg)
		}
		if m.cursorFlash {
			return SelectedFlashLine(line, rowWidth)
		}
		return SelectedLine(line, rowWidth)
	}

	return ZebraLine(line, rowWidth, rowIndex)
}

// renderOutlinerProjectHeaderRow renders a synthetic project sub-header row
// that visually clusters adjacent tasks from the same project in cross-project
// perspectives. In the All perspective these rows sit beneath shared section
// headers; other project-grouped perspectives emit them directly above each
// project's task group. The header reuses the per-project badge color and slug
// text so the inline `[slug]` badges on individual task rows can be suppressed.
// Sub-headers are skipped by keyboard navigation; the selected styling branches
// are kept defensive only.
func (m *OutlinerModel) renderOutlinerProjectHeaderRow(row outlinerRow, selected bool, rowWidth, rowIndex int) string {
	projName := m.projectNameByID(row.ProjectID)
	projSlug := m.projectSlugByID(row.ProjectID)
	badge := lipgloss.NewStyle().
		Foreground(lipgloss.Color(projectBadgeColor(projName))).
		Bold(true).
		Render("[" + projSlug + "]")
	line := "    " + badge

	if selected {
		if m.mutationFlash {
			return MutationFlashLine(line, rowWidth, m.mutationFlashBg)
		}
		if m.cursorFlash {
			return SelectedFlashLine(line, rowWidth)
		}
		return SelectedLine(line, rowWidth)
	}

	return ZebraLine(line, rowWidth, rowIndex)
}

func (m *OutlinerModel) renderOutlinerTaskRow(row outlinerRow, selected bool, rowWidth, width, rowIndex int, colLayout *ColumnLayout) string {
	task := row.task
	foldIndicator := m.renderTaskFoldIndicator(task)
	projBadge := m.renderTaskProjectBadge(row)
	proj := m.resolveProjectForRow(row)
	typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())
	statusIcon := m.renderTaskStatusIcon(task, proj)
	if colLayout != nil && colLayout.HasStatus {
		statusIcon = PadBadge(statusIcon, colLayout.StatusWidth)
	}
	if colLayout != nil && colLayout.HasType {
		typeIcon = PadBadge(typeIcon, colLayout.TypeWidth)
	}
	titleStyle := m.renderTaskTitleStyle(task, selected)
	prioritySegment := m.renderTaskPrioritySegment(task, colLayout)
	indicators := m.renderTaskIndicators(task, rowWidth)

	// Von Restorff: demote non-hero badges so the most urgent attribute stands
	// out. pickHeroAttribute already returns HeroNone for completed/dimmed rows
	// (session 1 contract), so no extra dimmed guard is needed here.
	hero := pickHeroAttribute(task, proj)
	if hero != HeroNone {
		if !statusIsHero(hero) {
			statusIcon = demoteBadge(statusIcon)
		}
		typeIcon = demoteBadge(typeIcon)
		if !priorityIsHero(hero) && prioritySegment != "" {
			if trimmed := strings.TrimRight(prioritySegment, " "); trimmed != "" {
				prioritySegment = demoteBadge(trimmed) + " "
			}
		}
	}
	cursorPrefix := RenderCursorPrefix(selected, m.queueIDs[task.ID], task.Pinned, task.Dimmed)
	queueIndicator := m.renderTaskQueueIndicator(task)
	checkbox := m.renderTaskCheckbox(task)
	prefixBudget := max(min(rowWidth/3, 12), 4)
	prefixStr, foldIndicator := renderTaskPrefix(compactOutlinerTreePrefix(row.prefix, prefixBudget), foldIndicator)

	// Preserve a readable title before secondary metadata on narrow/deep rows.
	// Queue membership, multi-select state, tree position, and workflow state
	// remain; lower-priority badges are progressively removed as space tightens.
	titleTarget := min(10, max(rowWidth/4, 4))
	availableTitleWidth := func() int {
		return rowWidth - outlinerNonTitleWidth(cursorPrefix, checkbox, prefixStr, foldIndicator, projBadge, typeIcon, statusIcon, prioritySegment, indicators)
	}
	if availableTitleWidth() < titleTarget {
		indicators = ""
	}
	if availableTitleWidth() < titleTarget {
		prioritySegment = ""
	}
	if availableTitleWidth() < titleTarget {
		projBadge = ""
	}
	if availableTitleWidth() < titleTarget {
		typeIcon = ""
	}
	title := m.renderTaskTitle(task, titleStyle, rowWidth, cursorPrefix, checkbox, prefixStr, foldIndicator, projBadge, typeIcon, statusIcon, prioritySegment, indicators)

	line := fmt.Sprintf("%s%s%s%s%s%s%s %s %s%s%s",
		cursorPrefix,
		queueIndicator,
		checkbox,
		prefixStr,
		foldIndicator,
		projBadge,
		typeIcon,
		statusIcon,
		prioritySegment,
		title,
		indicators,
	)

	// renderTaskTitle floors the title budget at 10 cols, so a deeply-nested row
	// (long tree prefix + multi-select checkbox) can assemble wider than rowWidth.
	// The row style below applies Width(rowWidth), which *wraps* an over-wide line
	// to a second terminal row — but computeOutlinerViewport budgets one line per
	// row, so the extra wrapped rows overflow terminal height. Clamp to rowWidth
	// (ANSI-aware) so every row stays exactly one line.
	line = truncateToVisualWidth(line, rowWidth)

	line = m.applyTaskRowLineStyle(line, task, proj, selected, rowWidth, rowIndex)

	if m.previewMode && rowIndex == m.cursor {
		var b strings.Builder
		b.WriteString(line)
		indent := row.prefix + "    "
		for _, previewLine := range inlinePreviewLines(task, indent, width) {
			b.WriteString("\n")
			b.WriteString(previewLine)
		}
		return b.String()
	}

	return line
}

func (m *OutlinerModel) renderTaskFoldIndicator(task *models.Task) string {
	if len(task.Children) == 0 {
		return ""
	}
	if m.isExpanded(task.ID) {
		return MutedStyle.Render(IconExpanded) + " "
	}
	return MutedStyle.Render(IconCollapsed) + " "
}

func (m *OutlinerModel) renderTaskProjectBadge(row outlinerRow) string {
	if row.ProjectID == "" {
		return ""
	}
	// Suppress the per-task `[slug]` badge in every normal cross-project
	// perspective: each task already appears beneath a project sub-header that
	// carries the same colored badge, so repeating it on every task adds
	// visual noise. Cross-project Priority mode is the exception — it buckets
	// tasks individually with no per-project visual grouping, so the inline
	// badge remains the only project identity cue.
	if m.project == nil && !m.isActiveCrossPriority() {
		return ""
	}

	projName := m.projectNameByID(row.ProjectID)
	projSlug := m.projectSlugByID(row.ProjectID)
	badgeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(projectBadgeColor(projName)))
	return badgeStyle.Render("["+projSlug+"]") + " "
}

func (m *OutlinerModel) renderTaskStatusIcon(task *models.Task, proj *models.Project) string {
	statusIcon := GetStateBadge(task.State())
	if proj != nil && proj.HasIncompleteDependencies(task) {
		return BlockedByDepsStyle.Render(task.State().Icon())
	}
	return statusIcon
}

func (m *OutlinerModel) renderTaskTitleStyle(task *models.Task, selected bool) lipgloss.Style {
	return TaskTitleStyleFor(task, TaskTitleStyleOptions{
		CursorSelected: selected,
	})
}

// renderTaskIndicators returns the dense-row inline metadata. Per the dense-row
// hierarchy, normal outliner rows keep only the primary scan anchors inline:
// the short ID (so keyboard nav targets stay visible) and the overdue/due-today
// text (the urgency hero for date-driven attention). Every other secondary
// indicator — future due dates, snooze, recurrence, progress, dependencies,
// score, tags, comments, description, attachments, checklist, decisions —
// moves to the selected-task metadata strip rendered beneath the row.
func (m *OutlinerModel) renderTaskIndicators(task *models.Task, rowWidth int) string {
	var parts []string
	if m.columnConfig.IsVisible(ColShortID) {
		parts = append(parts, MutedStyle.Render(fmt.Sprintf("#%d", task.ShortID)))
	}
	if parked := parkedRowSnippet(task, rowWidth); parked != "" && !m.sidebar.IsEnabled() {
		parts = append(parts, parked)
	}
	if m.columnConfig.IsVisible(ColDueDate) && dueIsInlineHero(task) {
		if dueStr := formatDueDate(task, m.runtime.Settings().GetTimeFormat()); dueStr != "" {
			parts = append(parts, dueStr)
		}
	}
	indicatorStr := JoinGroupParts(parts...)
	if indicatorStr == "" {
		return ""
	}
	return " " + indicatorStr
}

func (m *OutlinerModel) renderTaskPrioritySegment(task *models.Task, colLayout *ColumnLayout) string {
	if colLayout != nil && colLayout.HasPriority {
		priorityBadge := ""
		if m.columnConfig.ShouldShowPriority(task.Priority) {
			priorityBadge = renderPriorityBadge(task.Priority)
		}
		return PadBadge(priorityBadge, colLayout.PriorityWidth) + " "
	}
	if m.columnConfig.ShouldShowPriority(task.Priority) {
		return renderPriorityBadge(task.Priority) + " "
	}
	return ""
}

func (m *OutlinerModel) renderTaskContentGroup(task *models.Task, proj *models.Project) string {
	var contentParts []string
	if m.columnConfig.IsVisible(ColScore) && m.runtime != nil && proj != nil {
		if profile := m.runtime.Automation().GetProjectScoringProfile(proj.ID); profile != nil {
			if score := RenderScoreBadge(task, profile); score != "" {
				contentParts = append(contentParts, score)
			}
		}
	}
	if m.columnConfig.IsVisible(ColTags) {
		if tags := renderTruncatedTags(task.Tags, 2, 8); tags != "" {
			contentParts = append(contentParts, tags)
		}
	}
	if m.columnConfig.IsVisible(ColComments) {
		if comments := renderCommentIndicator(task); comments != "" {
			contentParts = append(contentParts, comments)
		}
	}
	if m.columnConfig.IsVisible(ColDescription) {
		if d := renderDescriptionIndicator(task); d != "" {
			contentParts = append(contentParts, d)
		}
	}
	if m.columnConfig.IsVisible(ColAttachments) {
		if issue := renderGitHubIssueIndicator(task); issue != "" {
			contentParts = append(contentParts, issue)
		}
		if attachments := renderAttachmentIndicator(task); attachments != "" {
			contentParts = append(contentParts, attachments)
		}
	}
	if m.columnConfig.IsVisible(ColChecklist) {
		if checklist := renderChecklistIndicator(task); checklist != "" {
			contentParts = append(contentParts, checklist)
		}
	}
	if m.columnConfig.IsVisible(ColDecisions) {
		if decisions := renderDecisionIndicator(task); decisions != "" {
			contentParts = append(contentParts, decisions)
		}
	}
	return JoinGroupParts(contentParts...)
}

func (m *OutlinerModel) renderTaskQueueIndicator(task *models.Task) string {
	if m.dailyPlanIDs[task.ID] {
		return ActiveStyle.Render("w") + " "
	}
	if m.queueIDs[task.ID] {
		return ActiveStyle.Render("q") + " "
	}
	return "  "
}

func (m *OutlinerModel) renderTaskCheckbox(task *models.Task) string {
	if !m.multiSelectMode {
		return ""
	}
	if m.IsSelected(task.ID) {
		return SelectionCheckStyle.Render("[x]") + " "
	}
	return MutedStyle.Render("[ ]") + " "
}

func compactOutlinerTreePrefix(prefix string, maxWidth int) string {
	if maxWidth <= 0 || lipgloss.Width(prefix) <= maxWidth {
		return prefix
	}
	runes := []rune(prefix)
	tailWidth := max(maxWidth-1, 0)
	if tailWidth == 0 {
		return "…"
	}
	if tailWidth > len(runes) {
		tailWidth = len(runes)
	}
	return "…" + string(runes[len(runes)-tailWidth:])
}

func renderTaskPrefix(prefix, foldIndicator string) (string, string) {
	if foldIndicator == "" && prefix != "" {
		runes := []rune(prefix)
		if runes[len(runes)-1] == ' ' {
			runes[len(runes)-1] = '─'
		}
		return MutedStyle.Render(string(runes)), MutedStyle.Render("─") + " "
	}
	return MutedStyle.Render(prefix), foldIndicator
}

func (m *OutlinerModel) renderTaskTitle(task *models.Task, titleStyle lipgloss.Style, rowWidth int, cursorPrefix, checkbox, prefixStr, foldIndicator, projBadge, typeIcon, statusIcon, prioritySegment, indicators string) string {
	nonTitleWidth := outlinerNonTitleWidth(cursorPrefix, checkbox, prefixStr, foldIndicator, projBadge, typeIcon, statusIcon, prioritySegment, indicators)
	availableWidth := max(rowWidth-nonTitleWidth, 1)
	truncatedTitle := truncateToVisualWidth(task.Title, availableWidth)
	query := strings.TrimSpace(ExtractSearchTextQuery(m.SearchQuery()))
	if query == "" {
		return titleStyle.Render(truncatedTitle)
	}
	_, positions := fuzzy.ScoreTermsWithPositions(truncatedTitle, query)
	if len(positions) == 0 {
		return titleStyle.Render(truncatedTitle)
	}
	return HighlightText(truncatedTitle, positions, titleStyle)
}

func outlinerNonTitleWidth(cursorPrefix, checkbox, prefixStr, foldIndicator, projBadge, typeIcon, statusIcon, prioritySegment, indicators string) int {
	return lipgloss.Width(cursorPrefix) + 2 + lipgloss.Width(checkbox) +
		lipgloss.Width(prefixStr) + lipgloss.Width(foldIndicator) +
		lipgloss.Width(projBadge) + lipgloss.Width(typeIcon) + 1 +
		lipgloss.Width(statusIcon) + 1 +
		lipgloss.Width(prioritySegment) + lipgloss.Width(indicators)
}

func (m *OutlinerModel) applyTaskRowLineStyle(line string, task *models.Task, proj *models.Project, selected bool, rowWidth, rowIndex int) string {
	if task.Dimmed {
		if selected {
			return DimSelectedLine(line, rowWidth)
		}
		return DimLine(line)
	}
	if selected {
		if m.mutationFlash {
			return MutationFlashLine(line, rowWidth, m.mutationFlashBg)
		}
		if m.cursorFlash {
			return SelectedFlashLine(line, rowWidth)
		}
		return SelectedLine(line, rowWidth)
	}
	if task.IsInState(models.StateWaiting) || (proj != nil && proj.HasIncompleteDependencies(task)) {
		return BlockedRowStyle.Width(rowWidth).Render(line)
	}
	if task.IsOverdue() {
		return OverdueRowStyle.Width(rowWidth).Render(line)
	}
	return ZebraLine(line, rowWidth, rowIndex)
}
