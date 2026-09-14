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
	MatchSourceResume:      "resume",
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
	// One instant for the whole row. The ladder and the review segment ask the
	// same calendar-day question, so reading the clock twice could demote a badge
	// for a review the row does not name.
	now := time.Now()
	indicatorSegments := m.renderTaskIndicatorSegments(row, now)

	// Von Restorff: one row asks for attention with one badge. The ladder in
	// chrome.PickHeroAttribute decides whether priority is that badge, and the
	// priority badge quietens when something more time-sensitive is on the row.
	// The state badge is not in this: it says what the row is rather than how
	// loudly it is asking, so it keeps its own colors on every row, which is what
	// leaves an ordinary Waiting row readable without making it urgent.
	hero := chrome.PickHeroAttribute(task, now)
	if hero != chrome.HeroNone && !chrome.PriorityIsHero(hero) && prioritySegment != "" {
		if trimmed := strings.TrimRight(prioritySegment, " "); trimmed != "" {
			prioritySegment = chrome.DemoteBadge(trimmed) + " "
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

func (m *OutlinerModel) RenderOutlinerTaskRow(row OutlinerRow, selected bool, rowWidth, width, rowIndex int, colLayout *chrome.ColumnLayout) string {
	task := row.Task
	titleStyle := m.renderTaskTitleStyle(task)
	paint := m.cursorPaintFor(selected)
	segments := m.buildTaskRowSegments(row, rowWidth, colLayout, paint)
	title := m.renderTaskTitle(task, titleStyle, rowWidth, segments)

	line := applyTaskRowLineStyle(segments.gutter(), segments.content(title), task, paint, rowWidth, rowIndex)

	return m.appendInlinePeek(line, row, rowIndex, width)
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

// renderTaskTitleStyle resolves an Outliner title's color, which for every
// non-terminal task is ordinary text: attention travels on the badges and
// indicators beside the title, never on the title itself. It passes no
// CursorSelected either, because the cursor is marked by the gutter and the
// trailing fill, so the row under it reads exactly like the rows around it.
func (m *OutlinerModel) renderTaskTitleStyle(task *models.Task) lipgloss.Style {
	return theme.TaskTitleStyleFor(task, theme.TaskTitleStyleOptions{})
}

type taskIndicatorSegments struct {
	deferCount  string
	Recency     string
	waiting     string
	doing       string
	review      string
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
		segments.review,
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
// carry them; elapsed ages leave before a due check-in, and the task's deadline
// goes last because it is the final commitment width pressure may hide.
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
	func(segments *taskIndicatorSegments) { segments.doing = "" },
	func(segments *taskIndicatorSegments) { segments.review = "" },
	func(segments *taskIndicatorSegments) { segments.due = "" },
}

// renderTaskIndicatorSegments promotes operational task state and concise
// activity counts into the row. Rich content stays in Peek. The caller drops
// segments along indicatorDropLadder when preserving all signals would leave an
// unreadably small title.
func (m *OutlinerModel) renderTaskIndicatorSegments(row OutlinerRow, now time.Time) taskIndicatorSegments {
	var segments taskIndicatorSegments
	task := row.Task
	if task == nil {
		return segments
	}
	if m.PrimaryMode == chrome.ModeRecent && m.columnConfig.IsVisible(listsurface.ColRecency) {
		segments.Recency = query.FormatRecencyChip(m.rowRecencyKey(row), now)
	}
	if task.IsInState(models.StateWaiting) {
		segments.waiting = formatWaitingAgeIndicator(task, now)
	}
	if task.IsInState(models.StateDoing) {
		segments.doing = formatDoingAgeIndicator(task, now)
	}
	segments.review = formatReviewIndicator(task, now)
	// A deadline remains a deadline in every lifecycle state. Check-ins have
	// their own schedule and indicator, so Waiting no longer reinterprets a due
	// date as a check-back or suppresses the ordinary due chip.
	if m.columnConfig.IsVisible(listsurface.ColDueDate) {
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

// formatReviewIndicator announces a task that is asking to be looked at, which
// models.TaskNeedsAttention answers and this only spells: the stored review flag,
// or a scheduled check-in whose calendar day has arrived and whose task is
// eligible to raise it. A future schedule belongs in Peek, while a terminal task
// stays silent and a deferred one suppresses its schedule. The row and the
// attention ladder read one predicate, so a row can never demote its priority
// badge for a review it does not name.
//
// The three tiers share deadline emphasis without borrowing deadline meaning:
// the word review keeps the signals distinct even when both occur on one row.
// The bare `review` is the flag alone — someone marked this task, so there is no
// day to be late against — and it stays as long as the flag does, since `=` set
// it and `R` is what takes it off.
func formatReviewIndicator(task *models.Task, now time.Time) string {
	if !models.TaskNeedsAttention(task, now) {
		return ""
	}
	if !task.CheckInIsDueAt(now) {
		return theme.DueTodayStyle.Render("review")
	}
	daysLate := util.DaysBetween(*task.NextCheckIn, now)
	if daysLate == 0 {
		return theme.DueTodayStyle.Render("review today")
	}
	return theme.OverdueStyle.Render(fmt.Sprintf("review +%dd", daysLate))
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

// taskRowTint names the row-level background a task earns, and a missed deadline
// is the only thing that earns one. It is the loudest mark the list has — a
// whole row repainted — so it is spent on the top of the attention ladder alone.
// Waiting held one until the ladder was consolidated: every Waiting row wore an
// alert background whether or not anything was owed on it, which made the
// quietest work in the workspace look like the most urgent. Waiting now reads
// through its state badge and its `waiting <age>` segment, and a Waiting task
// with a missed deadline is tinted for the deadline like any other task.
//
// It is returned rather than applied so the cursor row can wear it on the
// content region alone, leaving the gutter and the trailing fill to say where
// the cursor is. An overdue row therefore stays visibly overdue while you stand
// on it.
func taskRowTint(task *models.Task) (lipgloss.Style, bool) {
	if task.IsOverdue() {
		return theme.OverdueRowStyle, true
	}
	return lipgloss.Style{}, false
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

// InlinePeekLinesForRow is the window onto the row's detail document that is on
// screen. The document itself, its window geometry, and the remembered offset
// live in inline_peek_document.go and inline_peek_paging.go; this is the seam the
// renderer and the viewport measurement below it share.
func (m *OutlinerModel) InlinePeekLinesForRow(row OutlinerRow, indent string, rowWidth int) []string {
	if m == nil || !rowSupportsPeek(row) {
		return nil
	}
	doc := m.inlinePeekDocumentForRow(row, indent, rowWidth)
	return doc.Lines(m.peekOffsetForRow(row, doc))
}

// appendInlinePeek draws a row's Peek window beneath the row itself. Both row
// kinds that own a document come through here, so the pane attaches on the same
// terms wherever it appears: only under the cursor, only while Peek is open, and
// only when the document has something to say.
func (m *OutlinerModel) appendInlinePeek(line string, row OutlinerRow, rowIndex, width int) string {
	if m == nil || !m.PeekMode || rowIndex != m.Cursor {
		return line
	}
	// Peek belongs to the selected row but is not another tree node. Preserve the
	// nesting width for alignment without repeating connector glyphs.
	peekLines := m.InlinePeekLinesForRow(row, inlinePeekIndentForRow(row), width)
	if len(peekLines) == 0 {
		return line
	}
	var b strings.Builder
	b.WriteString(line)
	for _, peekLine := range peekLines {
		b.WriteString("\n")
		b.WriteString(peekLine)
	}
	return b.String()
}

// inlinePeekLineCount measures the Peek height ComputeOutlinerViewport must
// reserve. It reconstructs the row's real indent, which is what decides how wide
// the document wrapped and therefore how many of its rows a counter row has to
// account for. It counts the same window the render draws: both call Lines, so a
// height that is reserved and a window that is painted cannot disagree.
func (m *OutlinerModel) inlinePeekLineCount(row OutlinerRow, width int) int {
	width = max(width, 1)
	return len(m.InlinePeekLinesForRow(row, inlinePeekIndentForRow(row), width))
}
