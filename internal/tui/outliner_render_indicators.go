package tui

import (
	"fmt"
	"image/color"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
	"github.com/sglavoie/tt/internal/util"
)

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
	recency     string
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
	if m.primaryMode == chrome.ModeRecent && m.columnConfig.IsVisible(ColRecency) {
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
	// deferCount is deliberately outside that branch. Repeated deferral is its
	// own behavioral signal, and docs/decisions/behavioral-signals-contracts.md
	// locks the split: hiding the defer column hides the pending instant and the
	// wake marker, while the count survives at or above the threshold.
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
	census := dependencyCensusFor(task, index)
	count := "~" + census.counts()
	if census.unreadable > 0 {
		count += "?"
	}
	return census.outlook().style().Render(count)
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
	return theme.MutedStyle.Render("waiting") + " " + age
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
		return theme.OverdueStyle.Render("check +" + formatRecencyAge(dueDay, today))
	case dueDay.Equal(today):
		return theme.DueTodayStyle.Render("check today")
	default:
		return theme.DueDateStyle.Render("check " + formatRecencyAge(today, dueDay))
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
	return theme.MutedStyle.Render("woke") + " " + formatRecencyChip(*task.DeferredUntil, now)
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
	return now.Sub(*task.DeferredUntil) < deferWakeWindow
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

func (m *OutlinerModel) renderTaskCheckbox(row outlinerRow, bg color.Color) string {
	if !m.multiSelectMode {
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
	query, caseSensitive := m.titleHighlightQuery()
	if query == "" {
		return titleStyle.Render(title)
	}
	positions := searchSubstringPositions(title, query, caseSensitive)
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
	if !m.hasActiveLens() {
		return "", false
	}
	return m.lensTextQuery(), m.lensCaseSensitive()
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
