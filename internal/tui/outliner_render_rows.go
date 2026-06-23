package tui

import (
	"fmt"
	"strings"
	"time"

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
			if row.IsSectionHeader || row.IsSeparator || row.task == nil {
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

	if limit := m.config.GetSectionLimit(row.SectionName); limit > 0 && row.SectionCount > limit {
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
	indicators := m.renderTaskIndicators(task, proj)

	// Von Restorff: demote non-hero badges so the most urgent attribute stands out.
	hero := pickHeroAttribute(task, proj)
	if hero != HeroNone && !task.Dimmed {
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
	cursorPrefix := RenderCursorPrefix(selected, task.Active, task.Pinned, task.Dimmed)
	queueIndicator := m.renderTaskQueueIndicator(task)
	checkbox := m.renderTaskCheckbox(task)
	prefixStr, foldIndicator := renderTaskPrefix(row.prefix, foldIndicator)
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

	projName := m.projectNameByID(row.ProjectID)
	projSlug := m.projectSlugByID(row.ProjectID)
	badgeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(projectBadgeColor(projName)))
	return badgeStyle.Render("["+projSlug+"]") + " "
}

func (m *OutlinerModel) renderTaskStatusIcon(task *models.Task, proj *models.Project) string {
	statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())
	if proj != nil && proj.HasIncompleteDependencies(task) {
		return BlockedByDepsStyle.Render(task.Status.Icon())
	}
	return statusIcon
}

func (m *OutlinerModel) renderTaskTitleStyle(task *models.Task, selected bool) lipgloss.Style {
	return TaskTitleStyleFor(task, TaskTitleStyleOptions{
		CursorSelected: selected,
	})
}

func (m *OutlinerModel) renderTaskIndicators(task *models.Task, proj *models.Project) string {
	orgGroup := m.renderTaskOrganizationGroup(task, proj)
	lifecycleGroup := m.renderTaskLifecycleGroup(task)
	structureGroup := m.renderTaskStructureGroup(task, proj)
	contentGroup := m.renderTaskContentGroup(task, proj)
	indicatorStr := JoinIndicatorGroups(orgGroup, lifecycleGroup, structureGroup, contentGroup)
	if indicatorStr == "" {
		return ""
	}
	return " " + indicatorStr
}

func (m *OutlinerModel) renderTaskOrganizationGroup(task *models.Task, proj *models.Project) string {
	var orgParts []string
	if m.columnConfig.IsVisible(ColShortID) {
		orgParts = append(orgParts, MutedStyle.Render(fmt.Sprintf("#%d", task.ShortID)))
	}
	if perspective := m.currentPerspective(); perspective.Name != "All" && m.project != nil {
		sectionName := task.Section
		if sectionName == "" {
			sectionName = "Active"
		}
		effectiveSection := task.Section
		orgParts = append(orgParts, GetViewModeStyleForSection(effectiveSection).Render("@ "+sectionName))
	}
	return JoinGroupParts(orgParts...)
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

func (m *OutlinerModel) renderTaskLifecycleGroup(task *models.Task) string {
	var lifecycleParts []string
	if m.columnConfig.IsVisible(ColDueDate) {
		if dueStr := formatDueDate(task, m.config.GetTimeFormat()); dueStr != "" {
			lifecycleParts = append(lifecycleParts, dueStr)
		}
	}
	if m.columnConfig.IsVisible(ColSnooze) && task.SnoozedUntil != nil {
		snoozeStr := task.SnoozedUntil.Format("Jan 2")
		if task.SnoozedUntil.Year() != time.Now().Year() {
			snoozeStr = task.SnoozedUntil.Format("Jan 2 '06")
		}
		lifecycleParts = append(lifecycleParts, MutedStyle.Render("zzz "+snoozeStr))
	}
	if m.columnConfig.IsVisible(ColRecurrence) && task.IsRecurring() {
		recurStr := "↻"
		if task.Recurrence.Streak > 0 {
			recurStr = fmt.Sprintf("↻%d", task.Recurrence.Streak)
		}
		lifecycleParts = append(lifecycleParts, MutedStyle.Render(recurStr))
	}
	return JoinGroupParts(lifecycleParts...)
}

func (m *OutlinerModel) renderTaskStructureGroup(task *models.Task, proj *models.Project) string {
	var structureParts []string
	if m.columnConfig.IsVisible(ColProgress) {
		if frac := renderProgressFraction(task); frac != "" {
			structureParts = append(structureParts, frac)
		}
	}
	if m.columnConfig.IsVisible(ColDependencies) && !m.sidebar.IsEnabled() && proj != nil {
		if indicators := renderDependencyIndicators(task, proj); indicators != "" {
			structureParts = append(structureParts, indicators)
		}
	}
	return JoinGroupParts(structureParts...)
}

func (m *OutlinerModel) renderTaskContentGroup(task *models.Task, proj *models.Project) string {
	var contentParts []string
	if m.columnConfig.IsVisible(ColScore) && m.config != nil && proj != nil {
		if profile := m.config.GetProjectScoringProfile(proj.ID); profile != nil {
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
	nonTitleWidth := lipgloss.Width(cursorPrefix) + 2 + lipgloss.Width(checkbox) +
		lipgloss.Width(prefixStr) + lipgloss.Width(foldIndicator) +
		lipgloss.Width(projBadge) + lipgloss.Width(typeIcon) + 1 +
		lipgloss.Width(statusIcon) + 1 +
		lipgloss.Width(prioritySegment) + lipgloss.Width(indicators)
	availableWidth := max(rowWidth-nonTitleWidth, 10)
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
	if task.Status == models.StatusBlocked || (proj != nil && proj.HasIncompleteDependencies(task)) {
		return BlockedRowStyle.Width(rowWidth).Render(line)
	}
	if task.IsOverdue() {
		return OverdueRowStyle.Width(rowWidth).Render(line)
	}
	return ZebraLine(line, rowWidth, rowIndex)
}
