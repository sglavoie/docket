package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *OutlinerModel) renderOutlinerRows(viewport outlinerViewport) string {
	var colLayout *ColumnLayout
	if viewport.rowWidth >= MinColumnAlignWidth {
		colLayout = ComputeOutlinerColumnLayout(m.rows[viewport.startIdx:viewport.endIdx], m.columnConfig)
		// Augment with status widths from the same row set.
		var visibleTasks []*models.Task
		for _, row := range m.rows[viewport.startIdx:viewport.endIdx] {
			if !row.isTaskRow() || row.task == nil {
				continue
			}
			visibleTasks = append(visibleTasks, row.task)
		}
		if badge := ComputeBadgeWidths(visibleTasks, BadgeWidthOpts{
			IncludeStatus: true,
		}); badge != nil {
			if colLayout == nil {
				colLayout = badge
			} else {
				colLayout.HasStatus = badge.HasStatus
				colLayout.StatusWidth = badge.StatusWidth
			}
		}
	}

	var rowBuilder strings.Builder
	for i := viewport.startIdx; i < viewport.endIdx; i++ {
		row := m.rows[i]
		selected := i == m.cursor

		switch {
		case row.IsSeparator:
			rowBuilder.WriteString(renderHorizontalDivider(viewport.rowWidth))
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
		displayName = models.StateGroupBacklog.Label()
	}

	sectionStyle := GetWorkflowStateGroupStyle(models.StateGroupBacklog)
	if row.StateGroup.IsValid() {
		sectionStyle = GetWorkflowStateGroupStyle(row.StateGroup)
	}
	if row.PromotedTag == reviewTaskTag {
		sectionStyle = PurpleStyle
	}
	prefix := "@ "
	if row.IsComputedHeader {
		prefix = "◷ "
	}

	line := renderSectionHeader(SectionHeaderOpts{
		Name:        displayName,
		Count:       row.SectionCount,
		Collapsed:   m.isSectionRowCollapsed(row),
		AccentColor: sectionStyle,
		Prefix:      prefix,
	})

	if row.ProjectID != "" {
		project := m.resolveProjectForRow(row)
		projectName := row.ProjectID
		if project != nil {
			projectName = project.Name
		}
		line += "  " + projectColorStyle(project).Render("["+projectName+"]")
	}

	if selected {
		if m.cursorFlash {
			return SelectedFlashLine(line, rowWidth)
		}
		return SelectedLine(line, rowWidth)
	}

	return ZebraLine(line, rowWidth, rowIndex)
}

// renderOutlinerProjectHeaderRow renders a synthetic project sub-header row
// that visually clusters adjacent tasks from the same project in cross-project
// modes. In All these rows sit beneath shared state-group headers. The header
// reuses the persisted project color and slug
// text so the inline `[slug]` badges on individual task rows can be suppressed.
// They use the same fold indicator as built-in sections, while Enter and
// Shift+Enter retain their project scope and administration behavior.
func (m *OutlinerModel) renderOutlinerProjectHeaderRow(row outlinerRow, selected bool, rowWidth, rowIndex int) string {
	project := m.resolveProjectForRow(row)
	projectSlug := row.ProjectID
	if project != nil {
		projectSlug = project.Slug
		if projectSlug == "" {
			projectSlug = project.Name
		}
	}
	badge := projectColorStyle(project).
		Bold(true).
		Render("[" + projectSlug + "]")
	indicator := IconExpanded
	if m.isSectionRowCollapsed(row) {
		indicator = IconCollapsed
	}
	line := " " + MutedStyle.Render(indicator) + " " + badge
	if row.SectionCount > 0 {
		line += " " + MutedStyle.Render(fmt.Sprintf("(%d)", row.SectionCount))
	}
	line = truncateToVisualWidth(line, rowWidth)

	if selected {
		if m.cursorFlash {
			return SelectedFlashLine(line, rowWidth)
		}
		return SelectedLine(line, rowWidth)
	}

	return ZebraLine(line, rowWidth, rowIndex)
}

func (m *OutlinerModel) renderOutlinerTaskRow(row outlinerRow, selected bool, rowWidth, width, rowIndex int, colLayout *ColumnLayout) string {
	task := row.task
	foldIndicator := m.renderTaskFoldIndicator(row)
	statusIcon := m.renderTaskStatusIcon(task)
	if colLayout != nil && colLayout.HasStatus {
		statusIcon = PadBadge(statusIcon, colLayout.StatusWidth)
	}
	titleStyle := m.renderTaskTitleStyle(task, selected)
	prioritySegment := m.renderTaskPrioritySegment(task, colLayout)
	tagSegment := ""
	if m.columnConfig.IsVisible(ColTags) {
		if rendered := renderTruncatedTags(task.Tags, 2, 8); rendered != "" {
			tagSegment = " " + rendered
		}
	}
	indicatorSegments := m.renderTaskIndicatorSegments(row)
	indicators := indicatorSegments.render()

	// Von Restorff: demote non-hero badges so the most urgent attribute stands
	// out.
	hero := pickHeroAttribute(task)
	if hero != HeroNone {
		if !statusIsHero(hero) {
			statusIcon = demoteBadge(statusIcon)
		}
		if !priorityIsHero(hero) && prioritySegment != "" {
			if trimmed := strings.TrimRight(prioritySegment, " "); trimmed != "" {
				prioritySegment = demoteBadge(trimmed) + " "
			}
		}
	}
	cursorPrefix := RenderCursorPrefix()
	checkbox := m.renderTaskCheckbox(row)
	prefixBudget := max(min(rowWidth/3, 12), 4)
	prefixStr, foldIndicator := renderTaskPrefix(compactOutlinerTreePrefix(row.prefix, prefixBudget), foldIndicator)

	// Preserve a readable title before secondary metadata on narrow/deep rows.
	// Queue membership, multi-select state, tree position, and workflow state
	// remain; lower-priority badges are progressively removed as space tightens.
	titleTarget := min(8, max(rowWidth/4, 4))
	availableTitleWidth := func() int {
		return rowWidth - outlinerNonTitleWidth(cursorPrefix, checkbox, prefixStr, foldIndicator, statusIcon, prioritySegment, tagSegment, indicators)
	}
	if availableTitleWidth() < titleTarget {
		tagSegment = ""
	}
	if availableTitleWidth() < titleTarget {
		indicatorSegments.checklist = ""
		indicators = indicatorSegments.render()
	}
	title := m.renderTaskTitle(task, row.Breadcrumb, titleStyle, rowWidth, cursorPrefix, checkbox, prefixStr, foldIndicator, statusIcon, prioritySegment, tagSegment, indicators)

	line := fmt.Sprintf("%s%s%s%s%s %s%s%s%s",
		cursorPrefix,
		checkbox,
		prefixStr,
		foldIndicator,
		statusIcon,
		prioritySegment,
		title,
		tagSegment,
		indicators,
	)

	// renderTaskTitle floors the title budget at 10 cols, so a deeply-nested row
	// (long tree prefix + multi-select checkbox) can assemble wider than rowWidth.
	// The row style below applies Width(rowWidth), which *wraps* an over-wide line
	// to a second terminal row — but computeOutlinerViewport budgets one line per
	// row, so the extra wrapped rows overflow terminal height. Clamp to rowWidth
	// (ANSI-aware) so every row stays exactly one line.
	line = truncateToVisualWidth(line, rowWidth)

	line = m.applyTaskRowLineStyle(line, task, selected, rowWidth, rowIndex)

	if m.peekMode && rowIndex == m.cursor {
		var b strings.Builder
		b.WriteString(line)
		// Peek belongs to the selected task but is not another tree node. Preserve
		// the nesting width for alignment without repeating connector glyphs.
		indent := strings.Repeat(" ", lipgloss.Width(row.prefix)+4)
		for _, previewLine := range m.inlinePeekLinesForRow(row, indent, width) {
			b.WriteString("\n")
			b.WriteString(previewLine)
		}
		return b.String()
	}

	return line
}

// rowIdentity resolves a row's project-qualified task identity, supplying the
// scoped project ID that single-project rows omit.
func (m *OutlinerModel) rowIdentity(row outlinerRow) TaskIdentity {
	identity := TaskIdentity{ProjectID: row.ProjectID}
	if row.task != nil {
		identity.TaskID = row.task.ID
	}
	if identity.ProjectID == "" && m.project != nil {
		identity.ProjectID = m.project.ID
	}
	return identity
}

// rowRecencyKey returns the frozen Recent ordering timestamp so the chip agrees
// with the bucket the row was placed in, falling back to live activity for a
// task the snapshot does not name.
func (m *OutlinerModel) rowRecencyKey(row outlinerRow) time.Time {
	if frozen, ok := m.recencyKeys[m.rowIdentity(row)]; ok {
		return frozen
	}
	if row.task == nil {
		return time.Time{}
	}
	return row.task.UpdatedAt
}

func (m *OutlinerModel) renderTaskFoldIndicator(row outlinerRow) string {
	task := row.task
	if task == nil || !row.hasVisibleChildren {
		return ""
	}
	identity := m.rowIdentity(row)
	if m.isExpanded(identity) {
		return MutedStyle.Render(IconExpanded) + " "
	}
	return MutedStyle.Render(IconCollapsed) + " "
}

func (m *OutlinerModel) renderTaskStatusIcon(task *models.Task) string {
	return GetStateBadge(task.State())
}

func (m *OutlinerModel) renderTaskTitleStyle(task *models.Task, selected bool) lipgloss.Style {
	return TaskTitleStyleFor(task, TaskTitleStyleOptions{
		CursorSelected: selected,
	})
}

type taskIndicatorSegments struct {
	recency     string
	due         string
	snooze      string
	checklist   string
	comments    string
	attachments string
}

func (segments taskIndicatorSegments) render() string {
	rendered := JoinGroupParts(
		segments.recency,
		segments.due,
		segments.snooze,
		segments.checklist,
		segments.comments,
		segments.attachments,
	)
	if rendered == "" {
		return ""
	}
	return " " + rendered
}

// renderTaskIndicatorSegments promotes operational task state and concise
// activity counts into the row. Rich content stays in Peek. The caller
// progressively drops checklist only when
// preserving all signals would leave an unreadably small title.
func (m *OutlinerModel) renderTaskIndicatorSegments(row outlinerRow) taskIndicatorSegments {
	var segments taskIndicatorSegments
	task := row.task
	if task == nil {
		return segments
	}
	if m.primaryMode == ModeRecent && m.columnConfig.IsVisible(ColRecency) {
		segments.recency = formatRecencyChip(m.rowRecencyKey(row), time.Now())
	}
	if m.columnConfig.IsVisible(ColDueDate) {
		segments.due = formatDueDate(task)
	}
	if task.SnoozedUntil != nil && m.columnConfig.IsVisible(ColSnooze) {
		segments.snooze = MutedStyle.Render("zzz " + formatSnoozeDate(*task.SnoozedUntil))
	}
	if m.columnConfig.IsVisible(ColChecklist) {
		segments.checklist = renderChecklistIndicator(task)
	}
	if m.columnConfig.IsVisible(ColComments) {
		segments.comments = renderCommentIndicator(task)
	}
	if m.columnConfig.IsVisible(ColAttachments) {
		segments.attachments = renderAttachmentIndicator(task)
	}
	return segments
}

func (m *OutlinerModel) renderTaskPrioritySegment(task *models.Task, colLayout *ColumnLayout) string {
	priorityBadge := ""
	if m.columnConfig.ShouldShowPriority(task.Priority) {
		priorityBadge = renderPriorityBadge(task.Priority)
	}
	if colLayout != nil && colLayout.HasPriority {
		return PadBadge(priorityBadge, colLayout.PriorityWidth) + " "
	}
	if priorityBadge == "" {
		return ""
	}
	return priorityBadge + " "
}

func (m *OutlinerModel) renderTaskCheckbox(row outlinerRow) string {
	if !m.multiSelectMode {
		return ""
	}
	if m.isRowSelected(row) {
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
	// A leaf connector already communicates its tree position with ├──/└──.
	// Do not extend it into the task-content gutter.
	return MutedStyle.Render(prefix), foldIndicator
}

func (m *OutlinerModel) renderTaskTitle(task *models.Task, breadcrumb string, titleStyle lipgloss.Style, rowWidth int, cursorPrefix, checkbox, prefixStr, foldIndicator, statusIcon, prioritySegment, tagSegment, indicators string) string {
	nonTitleWidth := outlinerNonTitleWidth(cursorPrefix, checkbox, prefixStr, foldIndicator, statusIcon, prioritySegment, tagSegment, indicators)
	availableWidth := max(rowWidth-nonTitleWidth, 1)
	title := task.Title
	remainingWidth := availableWidth - lipgloss.Width(title)
	if remainingWidth < 0 {
		title = truncateToVisualWidth(title, availableWidth)
		remainingWidth = 0
	}
	query := strings.TrimSpace(m.SearchQuery())
	var titleRendered string
	if query == "" {
		titleRendered = titleStyle.Render(title)
	} else {
		positions := searchSubstringPositions(title, query)
		if len(positions) == 0 {
			titleRendered = titleStyle.Render(title)
		} else {
			titleRendered = HighlightText(title, positions, titleStyle)
		}
	}

	suffix := compactPromotedAncestorSuffix(breadcrumb, remainingWidth)
	if suffix == "" {
		return titleRendered
	}
	return titleRendered + MutedStyle.Render(suffix)
}

func compactPromotedAncestorSuffix(breadcrumb string, maxWidth int) string {
	if breadcrumb == "" || maxWidth < 5 {
		return ""
	}

	trimmed := strings.TrimSuffix(strings.TrimSpace(breadcrumb), " ›")
	if trimmed == "" {
		return ""
	}
	ancestors := strings.Split(trimmed, " › ")
	full := "  ‹ " + strings.Join(ancestors, " › ")
	if lipgloss.Width(full) <= maxWidth {
		return full
	}

	compacted := full
	if len(ancestors) > 1 {
		compacted = "  ‹ … › " + ancestors[len(ancestors)-1]
	}
	return truncateToVisualWidth(compacted, maxWidth)
}

func outlinerNonTitleWidth(cursorPrefix, checkbox, prefixStr, foldIndicator, statusIcon, prioritySegment, tagSegment, indicators string) int {
	return lipgloss.Width(cursorPrefix) + lipgloss.Width(checkbox) +
		lipgloss.Width(prefixStr) + lipgloss.Width(foldIndicator) +
		lipgloss.Width(statusIcon) + 1 +
		lipgloss.Width(prioritySegment) +
		lipgloss.Width(tagSegment) + lipgloss.Width(indicators)
}

func (m *OutlinerModel) applyTaskRowLineStyle(line string, task *models.Task, selected bool, rowWidth, rowIndex int) string {
	if selected {
		if m.cursorFlash {
			return SelectedFlashLine(line, rowWidth)
		}
		return SelectedLine(line, rowWidth)
	}
	if task.IsInState(models.StateWaiting) {
		return WaitingRowStyle.Width(rowWidth).Render(line)
	}
	if task.IsOverdue() {
		return OverdueRowStyle.Width(rowWidth).Render(line)
	}
	return ZebraLine(line, rowWidth, rowIndex)
}
