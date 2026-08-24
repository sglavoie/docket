package tui

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

func (m *OutlinerModel) renderOutlinerRows(viewport outlinerViewport) string {
	var colLayout *ColumnLayout
	if viewport.rowWidth >= MinColumnAlignWidth {
		colLayout = ComputeOutlinerColumnLayout(m.rows[viewport.startIdx:viewport.endIdx])
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

// cursorPaint decides how one row wears the cursor. One call answers both
// whether a row is the cursor and whether it is mid-flash, so no renderer reads
// m.cursorFlash on its own.
type cursorPaint struct {
	marked bool
	flash  bool
}

func (m *OutlinerModel) cursorPaintFor(selected bool) cursorPaint {
	return cursorPaint{marked: selected, flash: selected && m.cursorFlash}
}

// bar renders the row's leftmost column, the whole of the resting mark. It is
// inverse video rather than a background merged into the glyph's own style
// because one column has to carry the mark alone, so it takes the loudest
// treatment available and gives up the muted gray it would otherwise wear. Every
// column it can cover is structural margin: a blank, or a fold arrow that stays
// legible reversed.
func (p cursorPaint) bar(style lipgloss.Style, text string) string {
	switch {
	case !p.marked:
		return style.Render(text)
	case p.flash:
		return CursorFlashBarStyle.Render(text)
	default:
		return CursorBarStyle.Render(text)
	}
}

// gutterBg names the background the gutter columns after the bar carry. At rest
// there is none — the connectors, the fold arrow and the multi-select box stay
// ordinary, because widening the mark is what made it compete with the row. A
// flash spends exactly that: it floods the whole gutter, which reads as more of
// the same mark rather than a different one.
func (p cursorPaint) gutterBg() color.Color {
	if !p.flash {
		return nil
	}
	return ColorFlashBg
}

// trail styles the columns past the row's content, dim enough to read as the
// cursor's reach rather than compete with the bar for the eye.
func (p cursorPaint) trail() lipgloss.Style {
	if p.flash {
		return CursorFlashTrailStyle
	}
	return CursorTrailStyle
}

// gutterPiece renders one leading structural glyph in its own style, carrying
// the cursor background when bg is non-nil. The background belongs on each piece
// rather than on the assembled gutter: a style applied over already-styled text
// ends at that text's own reset, which would leave the rest of the block bare.
func gutterPiece(bg color.Color, style lipgloss.Style, text string) string {
	return withOptionalBg(style, bg).Render(text)
}

// gutterSpace paints one of the gutter's structural blank columns, which carry
// no color of their own to preserve.
func gutterSpace(bg color.Color) string {
	return gutterPiece(bg, lipgloss.NewStyle(), " ")
}

// outlinerCursorRow marks the cursor without repainting it. The gutter arrives
// already painted — its first column bright, the rest of it ordinary — and the
// columns past the content take a dim fill, so every color the row earned
// survives under the cursor and no drawn column is covered. The result is always
// exactly rowWidth wide, which is what attachScrollbarAtWidth expects.
// Truncation only ever removes from the end, so clamping the content to the
// columns the gutter leaves cuts where a clamp of the whole line would.
func outlinerCursorRow(gutter, content string, rowWidth int, paint cursorPaint) string {
	gutterWidth := lipgloss.Width(gutter)
	if rowWidth <= gutterWidth {
		return truncateToVisualWidth(gutter, rowWidth)
	}
	content = truncateToVisualWidth(content, rowWidth-gutterWidth)
	fill := rowWidth - gutterWidth - lipgloss.Width(content)
	if fill <= 0 {
		return gutter + content
	}
	return gutter + content + paint.trail().Render(strings.Repeat(" ", fill))
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

	paint := m.cursorPaintFor(selected)
	gutter, content := renderSectionHeader(SectionHeaderOpts{
		Name:        displayName,
		Count:       row.SectionCount,
		Collapsed:   m.isSectionRowCollapsed(row),
		AccentColor: sectionStyle,
		Prefix:      prefix,
		Cursor:      paint,
	})

	if row.ProjectID != "" {
		project := m.resolveProjectForRow(row)
		projectName := row.ProjectID
		if project != nil {
			projectName = project.Name
		}
		content += "  " + projectColorStyle(project).Render("["+projectName+"]")
	}
	content = appendManualOrderMarker(content, row, m.sort.Field, rowWidth-lipgloss.Width(gutter))

	if selected {
		return outlinerCursorRow(gutter, content, rowWidth, paint)
	}

	return ZebraLine(gutter+content, rowWidth, rowIndex)
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
// budget is the columns the content region has, which is the row width less the
// gutter the cursor highlight owns.
func appendManualOrderMarker(content string, row outlinerRow, field SortField, budget int) string {
	if field == SortByDefault || !rowKeepsManualOrder(row) {
		return content
	}
	suffix := MutedStyle.Render("  · manual")
	if lipgloss.Width(content)+lipgloss.Width(suffix) > budget {
		return content
	}
	return content + suffix
}

// renderOutlinerProjectHeaderRow renders a synthetic project sub-header row
// that visually clusters adjacent tasks from the same project in cross-project
// modes. In All these rows sit beneath shared state-group headers. The header
// reuses the persisted project color and slug
// text so the inline `[slug]` badges on individual task rows can be suppressed.
// They use the same fold indicator as built-in sections, while Enter retains
// its project scope behavior.
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
	paint := m.cursorPaintFor(selected)
	bg := paint.gutterBg()
	gutter := paint.bar(lipgloss.NewStyle(), " ") +
		gutterPiece(bg, MutedStyle, indicator) +
		gutterSpace(bg)
	content := badge
	if row.SectionCount > 0 {
		content += " " + MutedStyle.Render(fmt.Sprintf("(%d)", row.SectionCount))
	}
	content = appendProjectHeaderPressure(content, row, rowWidth-lipgloss.Width(gutter))

	if selected {
		return outlinerCursorRow(gutter, content, rowWidth, paint)
	}

	return ZebraLine(truncateToVisualWidth(gutter+content, rowWidth), rowWidth, rowIndex)
}

// appendProjectHeaderPressure names the overdue and Today-planned work a
// project group holds, so folding the group does not hide what is urgent in it.
// The suffix degrades before the row is truncated — today drops first, then
// overdue — because a clipped count reads as a different number. Each chip
// carries a glyph so it cannot be mistaken for the bare group size beside it.
// budget is the columns the content region has, which is the row width less the
// gutter the cursor highlight owns.
func appendProjectHeaderPressure(content string, row outlinerRow, budget int) string {
	var parts []string
	if row.OverdueCount > 0 {
		parts = append(parts, PreviewWarningStyle.Render(fmt.Sprintf("! overdue %d", row.OverdueCount)))
	}
	if row.TodayCount > 0 {
		parts = append(parts, MutedStyle.Render(fmt.Sprintf("◷ today %d", row.TodayCount)))
	}
	for ; len(parts) > 0; parts = parts[:len(parts)-1] {
		suffix := "  " + strings.Join(parts, MutedStyle.Render(" · "))
		if lipgloss.Width(content)+lipgloss.Width(suffix) <= budget {
			return content + suffix
		}
	}
	return content
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
	paint := m.cursorPaintFor(selected)
	prefixBudget := max(min(rowWidth/3, 12), 4)
	gutter := paint.bar(lipgloss.NewStyle(), RenderCursorPrefix()) +
		gutterPiece(paint.gutterBg(), MutedStyle, compactOutlinerTreePrefix(row.prefix, prefixBudget))
	content := MutedStyle.Render(box) + text

	if selected {
		return outlinerCursorRow(gutter, content, rowWidth, paint)
	}
	return ZebraLine(truncateToVisualWidth(gutter+content, rowWidth), rowWidth, rowIndex)
}

// taskRowSegments holds every rendered piece of a task row except the title, so
// the render path and the viewport measurement path derive the title budget from
// one formula instead of two that can drift apart.
type taskRowSegments struct {
	cursorPrefix, checkbox, prefixStr, foldIndicator string
	statusIcon, prioritySegment, pinMarker           string
	contextSegment, tagSegment                       string
	indicators, relationMarker                       string
}

func (s taskRowSegments) titleWidth(rowWidth int) int {
	return max(rowWidth-outlinerNonTitleWidth(
		s.cursorPrefix, s.checkbox, s.prefixStr, s.foldIndicator,
		s.statusIcon, s.prioritySegment, s.pinMarker, s.contextSegment,
		s.tagSegment, s.indicators, s.relationMarker,
	), 1)
}

// gutter is the leading structural region the cursor highlight owns: the left
// margin, the multi-select box, the tree connectors and the fold arrow. It is a
// concatenation only — the seam changes nothing about how wide each piece is, so
// titleWidth stays the single title-budget authority.
func (s taskRowSegments) gutter() string {
	return s.cursorPrefix + s.checkbox + s.prefixStr + s.foldIndicator
}

// content is everything from the state badge on, the region that keeps its own
// colors under the cursor. The space after the badge is the one outlinerNonTitleWidth
// counts.
func (s taskRowSegments) content(title string) string {
	return fmt.Sprintf("%s %s%s%s%s%s%s%s",
		s.statusIcon,
		s.prioritySegment,
		s.pinMarker,
		s.contextSegment,
		title,
		s.relationMarker,
		s.tagSegment,
		s.indicators,
	)
}

// taskRelationMarker names the family the outline placed elsewhere: `‹` for a
// promoted root whose parent is not shown above it, `›N` for a row whose N open
// children are all seated somewhere else. It is reserved in taskRowSegments
// rather than offered the title's leftover columns, so it survives every width
// instead of vanishing exactly when the row is most crowded. Both marks share
// one segment so a row that is both keeps the reserve near three columns. The
// ancestor path itself, and the children, stay Peek's.
func (m *OutlinerModel) taskRelationMarker(row outlinerRow) string {
	marker := ""
	if row.Breadcrumb != "" {
		marker = "‹"
	}
	// A row showing any child already says it is a parent through its connectors
	// and fold indicator; the count would only restate them.
	if !row.hasVisibleChildren {
		if open := m.openChildCounts[m.rowIdentity(row)]; open > 0 {
			marker += fmt.Sprintf("›%d", open)
		}
	}
	if marker == "" {
		return ""
	}
	return MutedStyle.Render("  " + marker)
}

// taskPinnedMarker names a pin the surrounding rows cannot: the Pinned lane's
// own rows carry the header instead, exactly as a row showing its child carries
// no child count.
func (m *OutlinerModel) taskPinnedMarker(row outlinerRow) string {
	if row.task == nil || !row.task.Pinned || row.IsPinnedSection {
		return ""
	}
	return PinnedMarkerStyle.Render("*") + " "
}

// buildTaskRowSegments assembles every rendered piece of a task row except the
// title. paint is how the row wears the cursor, and the zero value on an
// ordinary row; it changes no piece's width, so a caller measuring the title
// budget can pass the zero value.
func (m *OutlinerModel) buildTaskRowSegments(row outlinerRow, rowWidth int, colLayout *ColumnLayout, paint cursorPaint) taskRowSegments {
	task := row.task
	bg := paint.gutterBg()
	foldIndicator := m.renderTaskFoldIndicator(row, bg)
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
	prefixStr, foldIndicator := renderTaskPrefix(compactOutlinerTreePrefix(row.prefix, prefixBudget), foldIndicator, bg)

	segments := taskRowSegments{
		cursorPrefix:    paint.bar(lipgloss.NewStyle(), RenderCursorPrefix()),
		checkbox:        m.renderTaskCheckbox(row, bg),
		prefixStr:       prefixStr,
		foldIndicator:   foldIndicator,
		statusIcon:      statusIcon,
		prioritySegment: prioritySegment,
		pinMarker:       m.taskPinnedMarker(row),
		contextSegment:  contextSegment,
		tagSegment:      tagSegment,
		indicators:      indicatorSegments.render(),
		relationMarker:  m.taskRelationMarker(row),
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
	segments := m.buildTaskRowSegments(row, rowWidth, colLayout, cursorPaint{})
	return visualWidthOverflow(row.task.Title, segments.titleWidth(rowWidth))
}

func (m *OutlinerModel) renderOutlinerTaskRow(row outlinerRow, selected bool, rowWidth, width, rowIndex int, colLayout *ColumnLayout) string {
	task := row.task
	titleStyle := m.renderTaskTitleStyle(task)
	paint := m.cursorPaintFor(selected)
	segments := m.buildTaskRowSegments(row, rowWidth, colLayout, paint)
	title := m.renderTaskTitle(task, titleStyle, rowWidth, segments)

	line := applyTaskRowLineStyle(segments.gutter(), segments.content(title), task, paint, rowWidth, rowIndex)

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
func (m *OutlinerModel) renderTaskFoldIndicator(row outlinerRow, bg color.Color) string {
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
		return gutterPiece(bg, MutedStyle, IconExpanded) + gutterSpace(bg)
	}
	return gutterPiece(bg, MutedStyle, IconCollapsed) + gutterSpace(bg)
}

func (m *OutlinerModel) renderTaskStatusIcon(task *models.Task) string {
	return GetStateBadge(task.State())
}

// renderTaskTitleStyle resolves an Outliner title's color. It passes no
// CursorSelected: the cursor is marked by the gutter and the trailing fill, so
// the title under it keeps the priority or semantic color it earned.
func (m *OutlinerModel) renderTaskTitleStyle(task *models.Task) lipgloss.Style {
	return TaskTitleStyleFor(task, TaskTitleStyleOptions{})
}

type taskIndicatorSegments struct {
	deferCount  string
	recency     string
	waiting     string
	doing       string
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
		segments.doing,
		segments.check,
		segments.due,
		segments.deferred,
		segments.deferCount,
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
	func(segments *taskIndicatorSegments) { segments.deferCount = "" },
	func(segments *taskIndicatorSegments) { segments.checklist = "" },
	func(segments *taskIndicatorSegments) { segments.comments = "" },
	func(segments *taskIndicatorSegments) { segments.attachments = "" },
	func(segments *taskIndicatorSegments) { segments.recency = "" },
	func(segments *taskIndicatorSegments) { segments.deferred = "" },
	func(segments *taskIndicatorSegments) { segments.waiting = "" },
	func(segments *taskIndicatorSegments) { segments.due = "" },
	func(segments *taskIndicatorSegments) { segments.doing = "" },
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
	if task.IsInState(models.StateDoing) {
		segments.doing = formatDoingAgeIndicator(task, now)
	}
	// A waiting task's due date is its check-back date, so the countdown replaces
	// the plain date rather than restating it beside itself.
	if segments.check == "" && m.columnConfig.IsVisible(ColDueDate) {
		segments.due = formatDueDate(task)
	}
	if m.columnConfig.IsVisible(ColDefer) {
		if task.DeferredUntil != nil {
			segments.deferred = formatDeferIndicator(task, now)
		}
	}
	segments.deferCount = formatDeferCountIndicator(task)
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

// formatDoingAgeIndicator reads the state clock rather than UpdatedAt, so
// touching active work does not reset how long it has remained in Doing.
func formatDoingAgeIndicator(task *models.Task, now time.Time) string {
	if !task.IsInState(models.StateDoing) {
		return ""
	}
	age := formatRecencyChip(task.StateChangedAt, now)
	if age == "" {
		return ""
	}
	return MutedStyle.Render("doing") + " " + age
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
		return DueDateStyle.Render("check " + formatRecencyAge(today, dueDay))
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

const repeatedDeferralThreshold = 3

func taskHasRepeatedDeferrals(task *models.Task) bool {
	return task != nil && task.DeferCount >= repeatedDeferralThreshold
}

func formatDeferCountIndicator(task *models.Task) string {
	if !taskHasRepeatedDeferrals(task) {
		return ""
	}
	return MutedStyle.Render(fmt.Sprintf("deferred ×%d", task.DeferCount))
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
	priorityBadge := renderPriorityBadge(task.Priority)
	if colLayout != nil && colLayout.HasPriority {
		return PadBadge(priorityBadge, colLayout.PriorityWidth) + " "
	}
	if priorityBadge == "" {
		return ""
	}
	return priorityBadge + " "
}

func (m *OutlinerModel) renderTaskCheckbox(row outlinerRow, bg color.Color) string {
	if !m.multiSelectMode {
		return ""
	}
	if m.isRowSelected(row) {
		return gutterPiece(bg, SelectionCheckStyle, "[x]") + gutterSpace(bg)
	}
	return gutterPiece(bg, MutedStyle, "[ ]") + gutterSpace(bg)
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

func renderTaskPrefix(prefix, foldIndicator string, bg color.Color) (string, string) {
	// A leaf connector already communicates its tree position with ├──/└──.
	// Do not extend it into the task-content gutter.
	return gutterPiece(bg, MutedStyle, prefix), foldIndicator
}

func (m *OutlinerModel) renderTaskTitle(task *models.Task, titleStyle lipgloss.Style, rowWidth int, segments taskRowSegments) string {
	availableWidth := segments.titleWidth(rowWidth)
	title := truncateToVisualWidth(task.Title, availableWidth)
	query := m.searchTextQuery()
	if query == "" {
		return titleStyle.Render(title)
	}
	positions := searchSubstringPositions(title, query, m.searchCaseSensitive())
	if len(positions) == 0 {
		return titleStyle.Render(title)
	}
	return HighlightText(title, positions, titleStyle)
}

func outlinerNonTitleWidth(cursorPrefix, checkbox, prefixStr, foldIndicator, statusIcon, prioritySegment, pinMarker, contextSegment, tagSegment, indicators, relationMarker string) int {
	return lipgloss.Width(cursorPrefix) + lipgloss.Width(checkbox) +
		lipgloss.Width(prefixStr) + lipgloss.Width(foldIndicator) +
		lipgloss.Width(statusIcon) + 1 +
		lipgloss.Width(prioritySegment) + lipgloss.Width(pinMarker) +
		lipgloss.Width(contextSegment) +
		lipgloss.Width(tagSegment) + lipgloss.Width(indicators) +
		lipgloss.Width(relationMarker)
}

// taskRowTint names the row-level background a task earns from its own state. It
// is returned rather than applied so the cursor row can wear it on the content
// region alone, leaving the gutter and the trailing fill to say where the cursor
// is. A waiting row therefore stays visibly waiting while you stand on it.
func taskRowTint(task *models.Task) (lipgloss.Style, bool) {
	switch {
	case task.IsInState(models.StateWaiting):
		return WaitingRowStyle, true
	case task.IsOverdue():
		return OverdueRowStyle, true
	default:
		return lipgloss.Style{}, false
	}
}

func applyTaskRowLineStyle(gutter, content string, task *models.Task, paint cursorPaint, rowWidth, rowIndex int) string {
	tint, tinted := taskRowTint(task)
	if paint.marked {
		if tinted {
			content = tint.Render(content)
		}
		return outlinerCursorRow(gutter, content, rowWidth, paint)
	}

	// renderTaskTitle floors the title budget at 10 cols, so a deeply-nested row
	// (long tree prefix + multi-select checkbox) can assemble wider than rowWidth.
	// The row style below applies Width(rowWidth), which *wraps* an over-wide line
	// to a second terminal row — but computeOutlinerViewport budgets one line per
	// row, so the extra wrapped rows overflow terminal height. Clamp to rowWidth
	// (ANSI-aware) so every row stays exactly one line. outlinerCursorRow clamps
	// the cursor row the same way, against the columns its gutter leaves.
	line := truncateToVisualWidth(gutter+content, rowWidth)
	if tinted {
		return tint.Width(rowWidth).Render(line)
	}
	return ZebraLine(line, rowWidth, rowIndex)
}
