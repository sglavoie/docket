package outliner

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
	"github.com/sglavoie/tt/internal/util"
)

// taskRowSegments holds every rendered piece of a task row except the title, so
// the render path and the viewport measurement path derive the title budget from
// one formula instead of two that can drift apart.
type taskRowSegments struct {
	cursorPrefix, checkbox, prefixStr, foldIndicator string
	statusIcon, prioritySegment, pinMarker           string
	contextSegment, tagSegment                       string
	indicators, relationMarker, matchCue             string
}

func (s taskRowSegments) titleWidth(rowWidth int) int {
	return max(rowWidth-outlinerNonTitleWidth(
		s.cursorPrefix, s.checkbox, s.prefixStr, s.foldIndicator,
		s.statusIcon, s.prioritySegment, s.pinMarker, s.contextSegment,
		s.tagSegment, s.indicators, s.relationMarker, s.matchCue,
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
	return fmt.Sprintf("%s %s%s%s%s%s%s%s%s",
		s.statusIcon,
		s.prioritySegment,
		s.pinMarker,
		s.contextSegment,
		title,
		s.relationMarker,
		s.matchCue,
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
func (m *OutlinerModel) taskRelationMarker(row OutlinerRow) string {
	marker := ""
	if row.Breadcrumb != "" {
		marker = "‹"
	}
	// A row showing any child already says it is a parent through its connectors
	// and fold indicator; the count would only restate them.
	if !row.HasVisibleChildren {
		if open := m.openChildCounts[m.rowIdentity(row)]; open > 0 {
			marker += fmt.Sprintf("›%d", open)
		}
	}
	if marker == "" {
		return ""
	}
	return theme.MutedStyle.Render("  " + marker)
}

// taskPinnedMarker names a pin the surrounding rows cannot: the Pinned lane's
// own rows carry the header instead, exactly as a row showing its child carries
// no child count.
func (m *OutlinerModel) taskPinnedMarker(row OutlinerRow) string {
	if row.Task == nil || !row.Task.Pinned || row.IsPinnedSection {
		return ""
	}
	return listsurface.PinnedMarker()
}

// matchSourceLabels is the one presentation vocabulary for search attribution:
// each source the projection can report maps to exactly one word. A title hit is
// absent deliberately — the highlighted title is already the explanation — as is
// matchSourceNone, which every ordinary row carries.
var matchSourceLabels = map[MatchSource]string{
	MatchSourceDescription: "description",
	MatchSourceChecklist:   "checklist",
	MatchSourceComment:     "comment",
	MatchSourceTag:         "tag",
	MatchSourceContext:     "context",
	MatchSourceAttachment:  "attachment",
	MatchSourceProject:     "project",
	MatchSourceDetails:     "details",
	MatchSourceNote:        "note",
}

// taskMatchSourceCue explains why a row whose title shows no match is in the
// results. It is one segment so ANSI-aware measurement charges it once, and it
// is muted so it reads as an annotation rather than competing with the semantic
// title color beside it.
func taskMatchSourceCue(row OutlinerRow) string {
	label, ok := matchSourceLabels[row.MatchSource]
	if !ok {
		return ""
	}
	return theme.MutedStyle.Render("  · " + label)
}

// buildTaskRowSegments assembles every rendered piece of a task row except the
// title. paint is how the row wears the cursor, and the zero value on an
// ordinary row; it changes no piece's width, so a caller measuring the title
// budget can pass the zero value.
func (m *OutlinerModel) buildTaskRowSegments(row OutlinerRow, rowWidth int, colLayout *chrome.ColumnLayout, paint cursorPaint) taskRowSegments {
	task := row.Task
	bg := paint.gutterBg()
	foldIndicator := m.renderTaskFoldIndicator(row, bg)
	statusIcon := m.renderTaskStatusIcon(task)
	if colLayout != nil && colLayout.HasStatus {
		statusIcon = theme.PadBadge(statusIcon, colLayout.StatusWidth)
	}
	prioritySegment := m.renderTaskPrioritySegment(task, colLayout)
	contextSegment := ""
	if task.Context != "" {
		contextBudget := max(min(rowWidth/5, 12), 6)
		display := textfit.TruncateToVisualWidth(task.Context, contextBudget)
		contextSegment = theme.TaskContextBadgeStyle(task).Render("["+display+"]") + " "
	}
	tagSegment := ""
	if m.columnConfig.IsVisible(listsurface.ColTags) {
		if rendered := listsurface.RenderTruncatedTags(task.Tags, 2, 8); rendered != "" {
			tagSegment = " " + rendered
		}
	}
	indicatorSegments := m.renderTaskIndicatorSegments(row)

	// Von Restorff: demote non-hero badges so the most urgent attribute stands
	// out.
	hero := chrome.PickHeroAttribute(task)
	if hero != chrome.HeroNone {
		if !chrome.StatusIsHero(hero) {
			statusIcon = chrome.DemoteBadge(statusIcon)
		}
		if !chrome.PriorityIsHero(hero) && prioritySegment != "" {
			if trimmed := strings.TrimRight(prioritySegment, " "); trimmed != "" {
				prioritySegment = chrome.DemoteBadge(trimmed) + " "
			}
		}
	}
	prefixBudget := max(min(rowWidth/3, 12), 4)
	prefixStr, foldIndicator := renderTaskPrefix(compactOutlinerTreePrefix(row.Prefix, prefixBudget), foldIndicator, bg)

	segments := taskRowSegments{
		cursorPrefix:    paint.bar(lipgloss.NewStyle(), theme.RenderCursorPrefix()),
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
		matchCue:        taskMatchSourceCue(row),
	}

	// Preserve a readable title before secondary metadata on narrow/deep rows.
	// Queue membership, multi-select state, tree position, and workflow state
	// remain; lower-priority badges are progressively removed as space tightens.
	titleTarget := min(8, max(rowWidth/4, 4))
	// The match cue goes first: it explains a result the title itself cannot
	// show, which is worth nothing once that title is unreadable. It is also the
	// only segment a row wears solely because a search is live, so shedding it
	// leaves an ordinary row's degradation exactly as it was.
	if segments.titleWidth(rowWidth) < titleTarget {
		segments.matchCue = ""
	}
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
func (m *OutlinerModel) TaskRowTitleOverflow(row OutlinerRow, rowWidth int, colLayout *chrome.ColumnLayout) string {
	if m == nil || row.Task == nil {
		return ""
	}
	segments := m.buildTaskRowSegments(row, rowWidth, colLayout, cursorPaint{})
	return textfit.VisualWidthOverflow(row.Task.Title, segments.titleWidth(rowWidth))
}

func (m *OutlinerModel) RenderOutlinerTaskRow(row OutlinerRow, selected bool, rowWidth, width, rowIndex int, colLayout *chrome.ColumnLayout) string {
	task := row.Task
	titleStyle := m.renderTaskTitleStyle(task)
	paint := m.cursorPaintFor(selected)
	segments := m.buildTaskRowSegments(row, rowWidth, colLayout, paint)
	title := m.renderTaskTitle(task, titleStyle, rowWidth, segments)

	line := applyTaskRowLineStyle(segments.gutter(), segments.content(title), task, paint, rowWidth, rowIndex)

	if m.PeekMode && rowIndex == m.Cursor {
		var b strings.Builder
		b.WriteString(line)
		// Peek belongs to the selected task but is not another tree node. Preserve
		// the nesting width for alignment without repeating connector glyphs.
		indent := strings.Repeat(" ", lipgloss.Width(row.Prefix)+4)
		for _, previewLine := range m.InlinePeekLinesForRow(row, indent, width, colLayout) {
			b.WriteString("\n")
			b.WriteString(previewLine)
		}
		return b.String()
	}

	return line
}

// rowIdentity resolves a row's project-qualified task identity, supplying the
// scoped project ID that single-project rows omit.
func (m *OutlinerModel) rowIdentity(row OutlinerRow) taskref.TaskIdentity {
	identity := taskref.TaskIdentity{ProjectID: row.ProjectID}
	if row.Task != nil {
		identity.TaskID = row.Task.ID
	}
	if identity.ProjectID == "" && m.Project != nil {
		identity.ProjectID = m.Project.ID
	}
	return identity
}

// rowRecencyKey returns the frozen Recent ordering timestamp so the chip agrees
// with the bucket the row was placed in, falling back to live activity for a
// task the snapshot does not name.
func (m *OutlinerModel) rowRecencyKey(row OutlinerRow) time.Time {
	if frozen, ok := m.Recency.frozenAt(m.rowIdentity(row)); ok {
		return frozen
	}
	if row.Task == nil {
		return time.Time{}
	}
	return row.Task.UpdatedAt
}

// renderTaskFoldIndicator reports the fold `h`/`l` would act on next. Children
// answer first because the ladder collapses the checklist before them, so a task
// whose only foldable content is its checklist is the one that reports the
// checklist's own state.
func (m *OutlinerModel) renderTaskFoldIndicator(row OutlinerRow, bg color.Color) string {
	task := row.Task
	if task == nil {
		return ""
	}
	identity := m.rowIdentity(row)
	var expanded bool
	switch {
	case row.HasVisibleChildren:
		expanded = m.IsExpanded(identity)
	case len(task.Checklist) > 0:
		expanded = m.IsChecklistExpanded(identity)
	default:
		return ""
	}
	if expanded {
		return gutterPiece(bg, theme.MutedStyle, theme.IconExpanded) + gutterSpace(bg)
	}
	return gutterPiece(bg, theme.MutedStyle, theme.IconCollapsed) + gutterSpace(bg)
}

func (m *OutlinerModel) renderTaskStatusIcon(task *models.Task) string {
	return theme.GetStateBadge(task.State())
}

// renderTaskTitleStyle resolves an Outliner title's color. It passes no
// CursorSelected: the cursor is marked by the gutter and the trailing fill, so
// the title under it keeps the priority or semantic color it earned.
func (m *OutlinerModel) renderTaskTitleStyle(task *models.Task) lipgloss.Style {
	return theme.TaskTitleStyleFor(task, theme.TaskTitleStyleOptions{})
}

type taskIndicatorSegments struct {
	deferCount  string
	Recency     string
	waiting     string
	doing       string
	check       string
	due         string
	deferred    string
	checklist   string
	comments    string
	attachments string
	// dependencies is the trailing `~S/N` reading of outgoing soft references.
	// N is a count of what the task stores, not a reading of what those targets
	// are: a reference to closed, deleted, or unloaded work counts exactly like a
	// reference to open work, because nothing about the subject changes either
	// way. S and the color are the aggregate reading of the targets themselves,
	// one hop and never further; Peek remains the one place those targets are
	// named.
	dependencies string
}

func (segments taskIndicatorSegments) render() string {
	rendered := listsurface.JoinGroupParts(
		segments.Recency,
		segments.waiting,
		segments.doing,
		segments.check,
		segments.due,
		segments.deferred,
		segments.deferCount,
		segments.checklist,
		segments.comments,
		segments.attachments,
		segments.dependencies,
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
	// The reference count sheds with the other optional content counts and after
	// them, because Peek is the only surface that names the targets at all while
	// checklist rows, the Comments panel, and the Attachments panel each restate
	// their own. It stays ahead of every operational signal below: a reference
	// changes nothing about when the task is due or how long it has been waiting.
	func(segments *taskIndicatorSegments) { segments.dependencies = "" },
	func(segments *taskIndicatorSegments) { segments.Recency = "" },
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
func (m *OutlinerModel) renderTaskIndicatorSegments(row OutlinerRow) taskIndicatorSegments {
	var segments taskIndicatorSegments
	task := row.Task
	if task == nil {
		return segments
	}
	now := time.Now()
	if m.PrimaryMode == chrome.ModeRecent && m.columnConfig.IsVisible(listsurface.ColRecency) {
		segments.Recency = query.FormatRecencyChip(m.rowRecencyKey(row), now)
	}
	if task.IsInState(models.StateWaiting) {
		segments.waiting = formatWaitingAgeIndicator(task, now)
		if m.columnConfig.IsVisible(listsurface.ColDueDate) {
			segments.check = formatCheckBackIndicator(task, now)
		}
	}
	if task.IsInState(models.StateDoing) {
		segments.doing = formatDoingAgeIndicator(task, now)
	}
	// A waiting task's due date is its check-back date, so the countdown replaces
	// the plain date rather than restating it beside itself.
	if segments.check == "" && m.columnConfig.IsVisible(listsurface.ColDueDate) {
		segments.due = query.FormatDueDate(task)
	}
	if m.columnConfig.IsVisible(listsurface.ColDefer) {
		if task.DeferredUntil != nil {
			segments.deferred = formatDeferIndicator(task, now)
		}
	}
	// deferCount is deliberately outside that branch. Repeated deferral is its
	// own behavioral signal, and docs/decisions/behavioral-signals-contracts.md
	// locks the split: hiding the defer column hides the pending instant and the
	// wake marker, while the count survives at or above the threshold.
	segments.deferCount = formatDeferCountIndicator(task)
	if m.columnConfig.IsVisible(listsurface.ColChecklist) {
		segments.checklist = listsurface.RenderChecklistIndicator(task)
	}
	if m.columnConfig.IsVisible(listsurface.ColComments) {
		segments.comments = listsurface.RenderCommentIndicator(task)
	}
	if m.columnConfig.IsVisible(listsurface.ColAttachments) {
		segments.attachments = listsurface.RenderAttachmentIndicator(task)
	}
	// Deliberately ungated by a column: a reference count is not a column the
	// user can hide, and naming one would put a dependency back in the column
	// vocabulary the retired hard-dependency panels owned.
	segments.dependencies = renderDependencyIndicator(task, m.dependencyIndex())
	return segments
}

// renderDependencyIndicator renders the trailing outgoing-reference count as
// `~S/N`: N is every stored locator regardless of what it resolves to, so the
// number a row shows is exactly the number of tokens the edit grammar would
// round-trip and is independent of the task's own workflow state, and S is how
// many of them are settled. A wholly settled set collapses to the bare `~N`,
// and a trailing `?` says at least one locator points into a project this view
// did not load, so the shape now carries what only the color used to. The color
// is the census's own outlook, over the same index Peek resolves its detail
// lines against, so the row and Peek can never disagree. Nothing here adds or
// drops a reference.
//
// The index behind this is built once per row-rebuild pass rather than per
// reference per row per frame, which is strictly cheaper than the linear scan it
// replaced. It cannot go stale under a target's state change: changing a
// target's state is a mutation, and a mutation sets the force-rebuild flag that
// drops the index.
func renderDependencyIndicator(task *models.Task, index *app.DependencyIndex) string {
	if task == nil || len(task.Dependencies) == 0 {
		return ""
	}
	census := DependencyCensusFor(task, index)
	count := "~" + census.counts()
	if census.unreadable > 0 {
		count += "?"
	}
	return census.Outlook().style().Render(count)
}

// formatWaitingAgeIndicator reads the state clock rather than UpdatedAt, so
// commenting on a blocked task does not reset how long it has been blocked.
func formatWaitingAgeIndicator(task *models.Task, now time.Time) string {
	if !task.IsInState(models.StateWaiting) {
		return ""
	}
	age := query.FormatRecencyChip(task.StateChangedAt, now)
	if age == "" {
		return ""
	}
	return theme.MutedStyle.Render("waiting") + " " + age
}

// formatDoingAgeIndicator reads the state clock rather than UpdatedAt, so
// touching active work does not reset how long it has remained in Doing.
func formatDoingAgeIndicator(task *models.Task, now time.Time) string {
	if !task.IsInState(models.StateDoing) {
		return ""
	}
	age := query.FormatRecencyChip(task.StateChangedAt, now)
	if age == "" {
		return ""
	}
	return theme.MutedStyle.Render("doing") + " " + age
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
		return theme.OverdueStyle.Render("check +" + query.FormatRecencyAge(dueDay, today))
	case dueDay.Equal(today):
		return theme.DueTodayStyle.Render("check today")
	default:
		return theme.DueDateStyle.Render("check " + query.FormatRecencyAge(today, dueDay))
	}
}

// formatDeferIndicator separates a pending deferral from one that has lapsed. A
// woken task still wearing a past-dated `defer` reads as deferred, so the row
// says it came back and how long ago, then falls silent once the wake window
// closes and the task is ordinary work again.
func formatDeferIndicator(task *models.Task, now time.Time) string {
	if task.IsDeferredAt(now) {
		return theme.MutedStyle.Render("defer " + chrome.FormatDeferDate(*task.DeferredUntil, now))
	}
	if !taskWokeRecently(task, now) {
		return ""
	}
	return theme.MutedStyle.Render("woke") + " " + query.FormatRecencyChip(*task.DeferredUntil, now)
}

const repeatedDeferralThreshold = 3

func taskHasRepeatedDeferrals(task *models.Task) bool {
	return task != nil && task.DeferCount >= repeatedDeferralThreshold
}

func formatDeferCountIndicator(task *models.Task) string {
	if !taskHasRepeatedDeferrals(task) {
		return ""
	}
	return theme.MutedStyle.Render(fmt.Sprintf("deferred ×%d", task.DeferCount))
}

// taskWokeRecently reports whether a lapsed deferral is still inside the wake
// window. The row indicator and the Peek dates line both ask before announcing
// that a task came back, and a second copy of the predicate would let one
// surface keep saying `woke` after the other went quiet.
func taskWokeRecently(task *models.Task, now time.Time) bool {
	if task == nil || task.DeferredUntil == nil || task.IsDeferredAt(now) {
		return false
	}
	return now.Sub(*task.DeferredUntil) < query.DeferWakeWindow
}

func (m *OutlinerModel) renderTaskPrioritySegment(task *models.Task, colLayout *chrome.ColumnLayout) string {
	priorityBadge := chrome.RenderPriorityBadge(task.Priority)
	if colLayout != nil && colLayout.HasPriority {
		return theme.PadBadge(priorityBadge, colLayout.PriorityWidth) + " "
	}
	if priorityBadge == "" {
		return ""
	}
	return priorityBadge + " "
}

func (m *OutlinerModel) renderTaskCheckbox(row OutlinerRow, bg color.Color) string {
	if !m.MultiSelectMode {
		return ""
	}
	if m.isRowSelected(row) {
		return gutterPiece(bg, theme.SelectionCheckStyle, "[x]") + gutterSpace(bg)
	}
	return gutterPiece(bg, theme.MutedStyle, "[ ]") + gutterSpace(bg)
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
	return gutterPiece(bg, theme.MutedStyle, prefix), foldIndicator
}

func (m *OutlinerModel) renderTaskTitle(task *models.Task, titleStyle lipgloss.Style, rowWidth int, segments taskRowSegments) string {
	availableWidth := segments.titleWidth(rowWidth)
	title := textfit.TruncateToVisualWidth(task.Title, availableWidth)
	highlight, caseSensitive := m.titleHighlightQuery()
	if highlight == "" {
		return titleStyle.Render(title)
	}
	positions := query.SearchSubstringPositions(title, highlight, caseSensitive)
	if len(positions) == 0 {
		return titleStyle.Render(title)
	}
	return textfit.HighlightText(title, positions, titleStyle)
}

// titleHighlightQuery names the free text the visible rows were narrowed by,
// with the case mode they were actually produced under. Ordinary search and a
// lens narrowing are the same act to a title: whichever one owns the current
// projection is what a highlight must read, so the two can never disagree.
func (m *OutlinerModel) titleHighlightQuery() (string, bool) {
	if query := m.searchTextQuery(); query != "" {
		return query, m.searchCaseSensitive()
	}
	if !m.HasActiveLens() {
		return "", false
	}
	return m.LensTextQuery(), m.LensCaseSensitive()
}

func outlinerNonTitleWidth(cursorPrefix, checkbox, prefixStr, foldIndicator, statusIcon, prioritySegment, pinMarker, contextSegment, tagSegment, indicators, relationMarker, matchCue string) int {
	return lipgloss.Width(cursorPrefix) + lipgloss.Width(checkbox) +
		lipgloss.Width(prefixStr) + lipgloss.Width(foldIndicator) +
		lipgloss.Width(statusIcon) + 1 +
		lipgloss.Width(prioritySegment) + lipgloss.Width(pinMarker) +
		lipgloss.Width(contextSegment) +
		lipgloss.Width(tagSegment) + lipgloss.Width(indicators) +
		lipgloss.Width(relationMarker) + lipgloss.Width(matchCue)
}

// taskRowTint names the row-level background a task earns from its own state. It
// is returned rather than applied so the cursor row can wear it on the content
// region alone, leaving the gutter and the trailing fill to say where the cursor
// is. A waiting row therefore stays visibly waiting while you stand on it.
func taskRowTint(task *models.Task) (lipgloss.Style, bool) {
	switch {
	case task.IsInState(models.StateWaiting):
		return theme.WaitingRowStyle, true
	case task.IsOverdue():
		return theme.OverdueRowStyle, true
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
	line := textfit.TruncateToVisualWidth(gutter+content, rowWidth)
	if tinted {
		return tint.Width(rowWidth).Render(line)
	}
	return chrome.ZebraLine(line, rowWidth, rowIndex)
}

const (
	inlinePeekMaxLines        = 5
	inlinePeekDescriptionRows = 3
	inlinePeekTitleRows       = 3
)

type inlinePeekOptions struct {
	Task          *models.Task
	Indent        string
	Width         int
	TitleOverflow string
	Breadcrumb    string
	// Dependencies and Dependents are the two read-only reference details. They
	// arrive resolved because Peek is a pure renderer and neither direction can
	// be read off the task alone: an outgoing entry needs the target project to
	// name it, and an incoming one exists nowhere but in the other tasks.
	Dependencies []DependencyDetail
	Dependents   []DependencyDetail
	// DependencyCensus is the same reading the row's `~S/N` painted, carried in
	// so Peek's Dependencies label repeats those numbers rather than recounting
	// them against a population of its own.
	DependencyCensus dependencyCensus
}

func (m *OutlinerModel) InlinePeekLinesForRow(row OutlinerRow, indent string, rowWidth int, colLayout *chrome.ColumnLayout) []string {
	if m == nil || !row.IsTaskRow() {
		return nil
	}
	index := m.dependencyIndex()
	return inlinePeekLines(inlinePeekOptions{
		Task:             row.Task,
		Indent:           indent,
		Width:            rowWidth,
		TitleOverflow:    m.TaskRowTitleOverflow(row, rowWidth, colLayout),
		Breadcrumb:       row.Breadcrumb,
		Dependencies:     outgoingDependencyDetails(row.Task, index),
		Dependents:       incomingDependencyDetails(m.rowIdentity(row), index),
		DependencyCensus: DependencyCensusFor(row.Task, index),
	})
}

// inlinePeekLineCount measures the Peek height computeOutlinerViewport must
// reserve. It reconstructs the row's real indent and substitutes the worst-case
// column layout, because the title continuation now sits outside the additive
// cap and a measurement that assumed neither would diverge without bound on a
// deep row.
func (m *OutlinerModel) inlinePeekLineCount(row OutlinerRow, width int) int {
	width = max(width, 1)
	indent := strings.Repeat(" ", lipgloss.Width(row.Prefix)+4)
	return len(m.InlinePeekLinesForRow(row, indent, width, chrome.WorstCaseColumnLayout(width)))
}

// inlinePeekLines renders the additive, cursor-following task preview. The
// selected row already owns task identity and operational metadata, so Peek
// contains only richer content plus tags: description, dates, attachment names,
// comments, and tags. A title the row had to clip is completed above them and a
// promoted root's ancestor chain follows it; both raise the cap rather than
// displacing anything.
func inlinePeekLines(opts inlinePeekOptions) []string {
	if opts.Task == nil || opts.Width <= 0 {
		return nil
	}

	available := max(opts.Width-lipgloss.Width(opts.Indent), 1)
	titleLines := inlinePeekTitleLines(opts.Task, opts.TitleOverflow, opts.Indent, available)
	parentLine := inlinePeekParentLine(opts.Breadcrumb, available)
	lines := make([]string, 0, inlinePeekMaxLines+len(titleLines)+1)
	lines = append(lines, titleLines...)
	if parentLine != "" {
		lines = append(lines, opts.Indent+parentLine)
	}
	lines = append(lines, inlinePeekDescriptionLines(opts.Task.Description, opts.Indent, available)...)

	if dates := inlinePeekDatesLine(opts.Task, time.Now(), available); dates != "" {
		lines = append(lines, opts.Indent+dates)
	}

	if attachments := inlinePeekAttachmentsLine(opts.Task, available); attachments != "" {
		lines = append(lines, opts.Indent+theme.MutedStyle.Render(attachments))
	}

	if comments := inlinePeekCommentsLine(opts.Task, available); comments != "" {
		lines = append(lines, opts.Indent+theme.MutedStyle.Render(comments))
	}

	// The two reference lines sit above tags because Peek is the only surface
	// that names a reference at all: the row carries `~S/N`, and there is no
	// panel behind it the way `;` and `&` stand behind comments and attachments.
	// Tags are already on the row, so they are the cheaper line to lose here.
	// Each label carries its own count — the outgoing one the row's very numbers
	// — so a reader sees the whole size of a set the entry cap may have elided.
	// No `?` follows the fraction here: every entry below names its own state in
	// full, so the row's shorthand for "something could not be read" would be
	// repeating what the list already says.
	dependencyLabel := "Dependencies " + opts.DependencyCensus.counts()
	if dependencies := inlinePeekReferenceLine(dependencyLabel, opts.Dependencies, available); dependencies != "" {
		lines = append(lines, opts.Indent+theme.MutedStyle.Render(dependencies))
	}

	dependentLabel := fmt.Sprintf("Dependents %d", len(opts.Dependents))
	if dependents := inlinePeekReferenceLine(dependentLabel, opts.Dependents, available); dependents != "" {
		lines = append(lines, opts.Indent+theme.MutedStyle.Render(dependents))
	}

	if tags := inlinePeekTagsLine(opts.Task, available); tags != "" {
		lines = append(lines, opts.Indent+tags)
	}

	if len(lines) == 0 {
		lines = append(lines, opts.Indent+theme.MutedStyle.Render("No additional details"))
	}
	maxLines := inlinePeekMaxLines + len(titleLines)
	if parentLine != "" {
		maxLines++
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return lines
}

// inlinePeekParentLine names the ancestor chain the row reduced to a bare `‹`.
// It raises the cap like the title continuation and for the same reason: the row
// deliberately elided it, and Peek is the surface that shows what the row had no
// width for. Root-first and uncompacted — Peek has the width the row did not.
func inlinePeekParentLine(breadcrumb string, width int) string {
	if width <= 0 {
		return ""
	}
	chain := strings.TrimSuffix(strings.TrimSpace(breadcrumb), " ›")
	if chain == "" {
		return ""
	}
	return textfit.TruncateIntentional(theme.MutedStyle.Render("Parent: ")+chain, width, "…")
}

// inlinePeekTitleLines completes a title the row clipped rather than restating
// it: the row keeps its ellipsis, which now reads as "continues below", and Peek
// carries only the remainder. No label and the task's own title styling, so it
// reads as the same title instead of as another Label: value line.
func inlinePeekTitleLines(task *models.Task, overflow, indent string, width int) []string {
	if task == nil || overflow == "" || width <= 0 {
		return nil
	}

	wrapped := textfit.WrapText(overflow, width)
	if len(wrapped) == 0 {
		return nil
	}
	if len(wrapped) > inlinePeekTitleRows {
		wrapped = wrapped[:inlinePeekTitleRows]
		wrapped[len(wrapped)-1] = textfit.TruncateIntentional(wrapped[len(wrapped)-1]+"…", width, "…")
	}

	style := theme.TaskTitleStyleFor(task, theme.TaskTitleStyleOptions{})
	lines := make([]string, 0, len(wrapped))
	for _, line := range wrapped {
		lines = append(lines, indent+style.Render(textfit.TruncateIntentional(line, width, "…")))
	}
	return lines
}

// inlinePeekDatesLine names the task clock the row has no width for: how long
// ago it was created and last touched, when it is due, and whether it recently
// came back from a deferral. Labels stay muted and values take the footer's key
// color, so the durations read as data rather than as prose.
func inlinePeekDatesLine(task *models.Task, now time.Time, width int) string {
	if task == nil || width <= 0 {
		return ""
	}

	parts := make([]string, 0, 5)
	field := func(label, value string) {
		parts = append(parts, theme.HelpStyle.Render(label+" ")+theme.HelpKeyStyle.Render(value))
	}
	if age := query.FormatRecencyAge(task.CreatedAt, now); age != "" {
		field("created", age)
	}
	if age := query.FormatRecencyAge(task.UpdatedAt, now); age != "" {
		field("updated", age)
	}
	if task.DueDate != nil {
		field("due", chrome.DueGrammarValue(task))
	}
	if taskWokeRecently(task, now) {
		field("woke", query.FormatRecencyAge(*task.DeferredUntil, now))
	}
	if taskHasRepeatedDeferrals(task) {
		field("deferred", fmt.Sprintf("×%d", task.DeferCount))
	}
	if len(parts) == 0 {
		return ""
	}
	return textfit.TruncateIntentional(strings.Join(parts, theme.HelpStyle.Render(" · ")), width, "…")
}

func inlinePeekAttachmentsLine(task *models.Task, width int) string {
	if task == nil || len(task.Attachments) == 0 || width <= 0 {
		return ""
	}

	const maxAttachments = 2
	parts := make([]string, 0, min(len(task.Attachments), maxAttachments+1))
	for i, attachment := range task.Attachments {
		if i >= maxAttachments {
			parts = append(parts, fmt.Sprintf("+%d more", len(task.Attachments)-maxAttachments))
			break
		}
		if name := strings.TrimSpace(listsurface.AttachmentDisplayName(attachment)); name != "" {
			parts = append(parts, name)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return textfit.TruncateIntentional("Attachments: "+strings.Join(parts, " · "), width, "…")
}

// inlinePeekReferenceLine renders one read-only reference section. It names the
// first few entries and counts the rest the way the attachments line does, so a
// task with many references costs one line rather than many. The list is
// read-only and non-navigable: there is no cursor in it and no key acts on an
// entry, because a reference is information about other work, not a way to
// reach it.
func inlinePeekReferenceLine(label string, details []DependencyDetail, width int) string {
	if len(details) == 0 || width <= 0 {
		return ""
	}
	const maxDetails = 2
	parts := make([]string, 0, min(len(details), maxDetails+1))
	for i, detail := range details {
		if i >= maxDetails {
			parts = append(parts, fmt.Sprintf("+%d more", len(details)-maxDetails))
			break
		}
		parts = append(parts, detail.String())
	}
	return textfit.TruncateIntentional(label+": "+strings.Join(parts, " · "), width, "…")
}

func inlinePeekCommentsLine(task *models.Task, width int) string {
	if task == nil || len(task.Comments) == 0 || width <= 0 {
		return ""
	}
	return textfit.TruncateIntentional(fmt.Sprintf("Comments: %d", len(task.Comments)), width, "…")
}

func inlinePeekTagsLine(task *models.Task, width int) string {
	if task == nil || width <= 0 {
		return ""
	}

	chips := make([]string, 0, len(task.Tags))
	for _, tag := range listsurface.OrderedTaskTags(task.Tags) {
		if tag = strings.TrimSpace(tag); tag != "" {
			chips = append(chips, theme.TagChipStyle(tag).Render("#"+tag))
		}
	}
	showFullContext := lipgloss.Width(task.Context) > 12
	if len(chips) == 0 && !showFullContext {
		return ""
	}
	parts := make([]string, 0, 2)
	if showFullContext {
		parts = append(parts, theme.TaskContextBadgeStyle(task).Render("["+task.Context+"]"))
	}
	if len(chips) > 0 {
		parts = append(parts, strings.Join(chips, " "))
	}
	line := theme.MutedStyle.Render("Tags: ") + strings.Join(parts, " ")
	return textfit.TruncateIntentional(line, width, "…")
}

func inlinePeekDescriptionLines(description, indent string, width int) []string {
	description = strings.TrimSpace(strings.ReplaceAll(description, "\r\n", "\n"))
	if description == "" || width <= 0 {
		return nil
	}

	label := "Description: "
	bodyWidth := max(width-lipgloss.Width(label), 1)
	wrapped := textfit.WrapText(description, bodyWidth)
	if len(wrapped) == 0 {
		return nil
	}
	if len(wrapped) > inlinePeekDescriptionRows {
		wrapped = wrapped[:inlinePeekDescriptionRows]
		wrapped[len(wrapped)-1] = textfit.TruncateIntentional(wrapped[len(wrapped)-1]+"…", bodyWidth, "…")
	}

	lines := make([]string, 0, len(wrapped))
	for i, line := range wrapped {
		prefix := strings.Repeat(" ", lipgloss.Width(label))
		if i == 0 {
			prefix = label
		}
		lines = append(lines, indent+theme.MutedStyle.Render(
			textfit.TruncateIntentional(prefix+line, width, "…"),
		))
	}
	return lines
}
