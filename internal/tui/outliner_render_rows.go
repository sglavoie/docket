package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
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
		case row.IsChecklistItem:
			rowBuilder.WriteString(m.renderOutlinerChecklistRow(row, selected, viewport.rowWidth, i))
		case selected && m.isQuickEditRow(row):
			rowBuilder.WriteString(m.renderOutlinerQuickEditRow(row, viewport.rowWidth))
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
	line = appendManualOrderMarker(line, row, m.sort.Field, rowWidth)

	if selected {
		if m.cursorFlash {
			return SelectedFlashLine(line, rowWidth)
		}
		return SelectedLine(line, rowWidth)
	}

	return ZebraLine(line, rowWidth, rowIndex)
}

// rowKeepsManualOrder reports whether the section a row belongs to is one the
// sort never touches. Pinned, Today and Next carry an order the user built by
// hand, and Review is ordered by the projection that promoted it.
func rowKeepsManualOrder(row outlinerRow) bool {
	return row.IsPinnedSection ||
		row.IsTodaySection ||
		row.IsNextSection ||
		row.PromotedTag == reviewTaskTag
}

// appendManualOrderMarker names an exempt section while a sort is active, so a
// lane that did not move says why instead of reading as a broken key. It is
// dropped rather than allowed to overflow, exactly as the pressure chips are.
func appendManualOrderMarker(line string, row outlinerRow, field SortField, rowWidth int) string {
	if field == SortByDefault || !rowKeepsManualOrder(row) {
		return line
	}
	suffix := MutedStyle.Render("  · manual")
	if lipgloss.Width(line)+lipgloss.Width(suffix) > rowWidth {
		return line
	}
	return line + suffix
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
	line = appendProjectHeaderPressure(line, row, rowWidth)
	line = truncateToVisualWidth(line, rowWidth)

	if selected {
		if m.cursorFlash {
			return SelectedFlashLine(line, rowWidth)
		}
		return SelectedLine(line, rowWidth)
	}

	return ZebraLine(line, rowWidth, rowIndex)
}

// appendProjectHeaderPressure names the overdue and Today-planned work a
// project group holds, so folding the group does not hide what is urgent in it.
// The suffix degrades before the row is truncated — today drops first, then
// overdue — because a clipped count reads as a different number. Each chip
// carries a glyph so it cannot be mistaken for the bare group size beside it.
func appendProjectHeaderPressure(line string, row outlinerRow, rowWidth int) string {
	var parts []string
	if row.OverdueCount > 0 {
		parts = append(parts, PreviewWarningStyle.Render(fmt.Sprintf("! overdue %d", row.OverdueCount)))
	}
	if row.TodayCount > 0 {
		parts = append(parts, MutedStyle.Render(fmt.Sprintf("◷ today %d", row.TodayCount)))
	}
	for ; len(parts) > 0; parts = parts[:len(parts)-1] {
		suffix := "  " + strings.Join(parts, MutedStyle.Render(" · "))
		if lipgloss.Width(line)+lipgloss.Width(suffix) <= rowWidth {
			return line + suffix
		}
	}
	return line
}

// renderOutlinerChecklistRow renders one expanded checklist item beneath its
// owner task. It reuses the overlay's `[ ]`/`[x]` box and muted done text so the
// same item reads the same way wherever it is shown.
func (m *OutlinerModel) renderOutlinerChecklistRow(row outlinerRow, selected bool, rowWidth, rowIndex int) string {
	text := ""
	if row.task != nil && row.ChecklistIndex < len(row.task.Checklist) {
		text = strings.ReplaceAll(row.task.Checklist[row.ChecklistIndex].Text, "\n", " ")
	}
	box := "[ ] "
	if row.ChecklistDone {
		box = "[x] "
		text = MutedStyle.Render(text)
	}
	prefixBudget := max(min(rowWidth/3, 12), 4)
	line := RenderCursorPrefix() +
		MutedStyle.Render(compactOutlinerTreePrefix(row.prefix, prefixBudget)) +
		MutedStyle.Render(box) +
		text
	line = truncateToVisualWidth(line, rowWidth)

	if selected {
		if m.cursorFlash {
			return SelectedFlashLine(line, rowWidth)
		}
		return SelectedLine(line, rowWidth)
	}
	return ZebraLine(line, rowWidth, rowIndex)
}

// taskRowSegments holds every rendered piece of a task row except the title, so
// the render path and the viewport measurement path derive the title budget from
// one formula instead of two that can drift apart.
type taskRowSegments struct {
	cursorPrefix, checkbox, prefixStr, foldIndicator string
	statusIcon, prioritySegment, contextSegment      string
	tagSegment, indicators, ancestorMarker           string
}

func (s taskRowSegments) titleWidth(rowWidth int) int {
	return max(rowWidth-outlinerNonTitleWidth(
		s.cursorPrefix, s.checkbox, s.prefixStr, s.foldIndicator,
		s.statusIcon, s.prioritySegment, s.contextSegment, s.tagSegment,
		s.indicators, s.ancestorMarker,
	), 1)
}

// promotedAncestorMarker is the fixed three-column mark a promoted section root
// wears in place of its ancestor path. It is reserved in taskRowSegments rather
// than offered the title's leftover columns, so it survives every width instead
// of vanishing exactly when the row is most crowded. The path itself is Peek's.
func promotedAncestorMarker(row outlinerRow) string {
	if row.Breadcrumb == "" {
		return ""
	}
	return MutedStyle.Render("  ‹")
}

func (m *OutlinerModel) buildTaskRowSegments(row outlinerRow, rowWidth int, colLayout *ColumnLayout) taskRowSegments {
	task := row.task
	foldIndicator := m.renderTaskFoldIndicator(row)
	statusIcon := m.renderTaskStatusIcon(task)
	if colLayout != nil && colLayout.HasStatus {
		statusIcon = PadBadge(statusIcon, colLayout.StatusWidth)
	}
	prioritySegment := m.renderTaskPrioritySegment(task, colLayout)
	contextSegment := ""
	if task.Context != "" {
		contextBudget := max(min(rowWidth/5, 12), 6)
		display := truncateToVisualWidth(task.Context, contextBudget)
		contextSegment = taskContextBadgeStyle(task).Render("["+display+"]") + " "
	}
	tagSegment := ""
	if m.columnConfig.IsVisible(ColTags) {
		if rendered := renderTruncatedTags(task.Tags, 2, 8); rendered != "" {
			tagSegment = " " + rendered
		}
	}
	indicatorSegments := m.renderTaskIndicatorSegments(row)

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
	prefixBudget := max(min(rowWidth/3, 12), 4)
	prefixStr, foldIndicator := renderTaskPrefix(compactOutlinerTreePrefix(row.prefix, prefixBudget), foldIndicator)

	segments := taskRowSegments{
		cursorPrefix:    RenderCursorPrefix(),
		checkbox:        m.renderTaskCheckbox(row),
		prefixStr:       prefixStr,
		foldIndicator:   foldIndicator,
		statusIcon:      statusIcon,
		prioritySegment: prioritySegment,
		contextSegment:  contextSegment,
		tagSegment:      tagSegment,
		indicators:      indicatorSegments.render(),
		ancestorMarker:  promotedAncestorMarker(row),
	}

	// Preserve a readable title before secondary metadata on narrow/deep rows.
	// Queue membership, multi-select state, tree position, and workflow state
	// remain; lower-priority badges are progressively removed as space tightens.
	titleTarget := min(8, max(rowWidth/4, 4))
	if segments.titleWidth(rowWidth) < titleTarget {
		segments.tagSegment = ""
	}
	for _, dropSegment := range indicatorDropLadder {
		if segments.titleWidth(rowWidth) >= titleTarget {
			break
		}
		dropSegment(&indicatorSegments)
		segments.indicators = indicatorSegments.render()
	}
	return segments
}

// taskRowTitleOverflow returns the title text the row itself could not show, so
// Peek can complete it rather than restate it.
func (m *OutlinerModel) taskRowTitleOverflow(row outlinerRow, rowWidth int, colLayout *ColumnLayout) string {
	if m == nil || row.task == nil {
		return ""
	}
	segments := m.buildTaskRowSegments(row, rowWidth, colLayout)
	return visualWidthOverflow(row.task.Title, segments.titleWidth(rowWidth))
}

func (m *OutlinerModel) renderOutlinerTaskRow(row outlinerRow, selected bool, rowWidth, width, rowIndex int, colLayout *ColumnLayout) string {
	task := row.task
	titleStyle := m.renderTaskTitleStyle(task, selected)
	segments := m.buildTaskRowSegments(row, rowWidth, colLayout)
	title := m.renderTaskTitle(task, titleStyle, rowWidth, segments)

	line := fmt.Sprintf("%s%s%s%s%s %s%s%s%s%s%s",
		segments.cursorPrefix,
		segments.checkbox,
		segments.prefixStr,
		segments.foldIndicator,
		segments.statusIcon,
		segments.prioritySegment,
		segments.contextSegment,
		title,
		segments.ancestorMarker,
		segments.tagSegment,
		segments.indicators,
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
		for _, previewLine := range m.inlinePeekLinesForRow(row, indent, width, colLayout) {
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

// renderTaskFoldIndicator reports the fold `h`/`l` would act on next. Children
// answer first because the ladder collapses the checklist before them, so a task
// whose only foldable content is its checklist is the one that reports the
// checklist's own state.
func (m *OutlinerModel) renderTaskFoldIndicator(row outlinerRow) string {
	task := row.task
	if task == nil {
		return ""
	}
	identity := m.rowIdentity(row)
	var expanded bool
	switch {
	case row.hasVisibleChildren:
		expanded = m.isExpanded(identity)
	case len(task.Checklist) > 0:
		expanded = m.isChecklistExpanded(identity)
	default:
		return ""
	}
	if expanded {
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
	waiting     string
	check       string
	due         string
	deferred    string
	checklist   string
	comments    string
	attachments string
}

func (segments taskIndicatorSegments) render() string {
	rendered := JoinGroupParts(
		segments.recency,
		segments.waiting,
		segments.check,
		segments.due,
		segments.deferred,
		segments.checklist,
		segments.comments,
		segments.attachments,
	)
	if rendered == "" {
		return ""
	}
	return " " + rendered
}

// indicatorDropLadder orders which indicators a narrow row sheds, shallowest
// signal first. Counts go early because the row's own badges and Peek still
// carry them; the check-back date goes last because it is the one indicator
// naming work that has already gone quiet on someone else's desk.
var indicatorDropLadder = []func(*taskIndicatorSegments){
	func(segments *taskIndicatorSegments) { segments.checklist = "" },
	func(segments *taskIndicatorSegments) { segments.comments = "" },
	func(segments *taskIndicatorSegments) { segments.attachments = "" },
	func(segments *taskIndicatorSegments) { segments.recency = "" },
	func(segments *taskIndicatorSegments) { segments.deferred = "" },
	func(segments *taskIndicatorSegments) { segments.waiting = "" },
	func(segments *taskIndicatorSegments) { segments.due = "" },
	func(segments *taskIndicatorSegments) { segments.check = "" },
}

// renderTaskIndicatorSegments promotes operational task state and concise
// activity counts into the row. Rich content stays in Peek. The caller drops
// segments along indicatorDropLadder when preserving all signals would leave an
// unreadably small title.
func (m *OutlinerModel) renderTaskIndicatorSegments(row outlinerRow) taskIndicatorSegments {
	var segments taskIndicatorSegments
	task := row.task
	if task == nil {
		return segments
	}
	now := time.Now()
	if m.primaryMode == ModeRecent && m.columnConfig.IsVisible(ColRecency) {
		segments.recency = formatRecencyChip(m.rowRecencyKey(row), now)
	}
	if task.IsInState(models.StateWaiting) {
		segments.waiting = formatWaitingAgeIndicator(task, now)
		if m.columnConfig.IsVisible(ColDueDate) {
			segments.check = formatCheckBackIndicator(task, now)
		}
	}
	// A waiting task's due date is its check-back date, so the countdown replaces
	// the plain date rather than restating it beside itself.
	if segments.check == "" && m.columnConfig.IsVisible(ColDueDate) {
		segments.due = formatDueDate(task)
	}
	if task.DeferredUntil != nil && m.columnConfig.IsVisible(ColDefer) {
		segments.deferred = formatDeferIndicator(task, now)
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

// formatWaitingAgeIndicator reads the state clock rather than UpdatedAt, so
// commenting on a blocked task does not reset how long it has been blocked.
func formatWaitingAgeIndicator(task *models.Task, now time.Time) string {
	if !task.IsInState(models.StateWaiting) {
		return ""
	}
	age := formatRecencyChip(task.StateChangedAt, now)
	if age == "" {
		return ""
	}
	return MutedStyle.Render("waiting") + " " + age
}

// formatCheckBackIndicator renders a waiting task's due date as the countdown to
// chasing it, which is the actionable reading of that date once the work is out
// of your hands. Comparison is by calendar day because a due date is a day, not
// an instant.
func formatCheckBackIndicator(task *models.Task, now time.Time) string {
	if task.DueDate == nil || task.IsComplete() {
		return ""
	}
	today := util.StartOfDay(now)
	dueDay := util.StartOfDay(*task.DueDate)
	switch {
	case dueDay.Before(today):
		return OverdueStyle.Render("check +" + formatRecencyAge(dueDay, today))
	case dueDay.Equal(today):
		return DueTodayStyle.Render("check today")
	default:
		return MutedStyle.Render("check " + formatRecencyAge(today, dueDay))
	}
}

// formatDeferIndicator separates a pending deferral from one that has lapsed. A
// woken task still wearing a past-dated `defer` reads as deferred, so the row
// says it came back and how long ago, then falls silent once the wake window
// closes and the task is ordinary work again.
func formatDeferIndicator(task *models.Task, now time.Time) string {
	if task.IsDeferredAt(now) {
		return MutedStyle.Render("defer " + formatDeferDate(*task.DeferredUntil, now))
	}
	if !taskWokeRecently(task, now) {
		return ""
	}
	return MutedStyle.Render("woke") + " " + formatRecencyChip(*task.DeferredUntil, now)
}

// taskWokeRecently reports whether a lapsed deferral is still inside the wake
// window. The row indicator and the Peek dates line both ask before announcing
// that a task came back, and a second copy of the predicate would let one
// surface keep saying `woke` after the other went quiet.
func taskWokeRecently(task *models.Task, now time.Time) bool {
	if task == nil || task.DeferredUntil == nil || task.IsDeferredAt(now) {
		return false
	}
	return now.Sub(*task.DeferredUntil) < deferWakeWindow
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

func (m *OutlinerModel) renderTaskTitle(task *models.Task, titleStyle lipgloss.Style, rowWidth int, segments taskRowSegments) string {
	availableWidth := segments.titleWidth(rowWidth)
	title := truncateToVisualWidth(task.Title, availableWidth)
	query := strings.TrimSpace(m.SearchQuery())
	if query == "" {
		return titleStyle.Render(title)
	}
	positions := searchSubstringPositions(title, query, m.searchCaseSensitive())
	if len(positions) == 0 {
		return titleStyle.Render(title)
	}
	return HighlightText(title, positions, titleStyle)
}

func outlinerNonTitleWidth(cursorPrefix, checkbox, prefixStr, foldIndicator, statusIcon, prioritySegment, contextSegment, tagSegment, indicators, ancestorMarker string) int {
	return lipgloss.Width(cursorPrefix) + lipgloss.Width(checkbox) +
		lipgloss.Width(prefixStr) + lipgloss.Width(foldIndicator) +
		lipgloss.Width(statusIcon) + 1 +
		lipgloss.Width(prioritySegment) + lipgloss.Width(contextSegment) +
		lipgloss.Width(tagSegment) + lipgloss.Width(indicators) +
		lipgloss.Width(ancestorMarker)
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
