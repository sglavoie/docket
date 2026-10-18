package outliner

import (
	"fmt"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
	"github.com/sglavoie/tt/internal/util"
)

// taskIndicatorSegments is the whole of what a row says beside its title,
// badges, and structural marks. It is three segments rather than eleven because
// a row is scanned, not read: the deadline and the owed review are the two
// facts that change what a reader does next, and the reference census is the
// one cue that work the row cannot show may be unreadable or unfinished.
// Everything a row used to carry beside them — tags, context, elapsed ages,
// deferral dates, and the checklist, comment and attachment counts — is spelled
// in full in Peek, one keypress below the cursor.
type taskIndicatorSegments struct {
	review string
	due    string
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
	// The deadline and the review are chips carrying a column of padding each
	// side; the census is bare text, so it is given the leading column a chip
	// would have brought and sits the same distance from whatever precedes it.
	census := segments.dependencies
	if census != "" {
		census = " " + census
	}
	rendered := listsurface.JoinGroupParts(
		segments.review,
		segments.due,
		census,
	)
	if rendered == "" {
		return ""
	}
	return " " + rendered
}

// indicatorDropLadder orders which indicators a narrow row sheds, shallowest
// signal first. The reference census goes first because Peek names every target
// in full and no deadline or owed review depends on it; the owed review follows,
// because it says a task wants looking at while the deadline says when something
// is actually due. The deadline goes last: it is the final commitment width
// pressure may hide.
var indicatorDropLadder = []func(*taskIndicatorSegments){
	func(segments *taskIndicatorSegments) { segments.dependencies = "" },
	func(segments *taskIndicatorSegments) { segments.review = "" },
	func(segments *taskIndicatorSegments) { segments.due = "" },
}

// previewIndicatorDropLadder is the ladder less its last rung: Select Project's
// preview sheds the census and the review for a whole title, never the deadline.
var previewIndicatorDropLadder = indicatorDropLadder[:len(indicatorDropLadder)-1]

// renderTaskIndicatorSegments promotes the row's three surviving operational
// signals: the deadline, the owed review, and the outgoing-reference census.
// Every other detail the row once abbreviated is Peek's, in full. The caller
// drops segments along indicatorDropLadder when preserving all three would leave
// an unreadably small title.
func (m *OutlinerModel) renderTaskIndicatorSegments(row OutlinerRow, now time.Time) taskIndicatorSegments {
	var segments taskIndicatorSegments
	task := row.Task
	if task == nil {
		return segments
	}
	segments.review = formatReviewIndicator(task, now)
	// The Review lane already names a flag-only review, so a row there drops the
	// bare word and keeps the tiers that carry a day.
	if row.PromotedTag == reviewLaneKey && !task.CheckInIsDueAt(now) {
		segments.review = ""
	}
	// A deadline remains a deadline in every lifecycle state. Check-ins have
	// their own schedule and indicator, so Waiting no longer reinterprets a due
	// date as a check-back or suppresses the ordinary due chip.
	if m.columnConfig.IsVisible(listsurface.ColDueDate) {
		segments.due = query.FormatDueDate(task)
	}
	// Deliberately ungated by a column: a reference count is not a column the
	// user can hide, and naming one would put a dependency back in the column
	// vocabulary the retired hard-dependency panels owned.
	if len(task.Dependencies) > 0 {
		segments.dependencies = renderDependencyIndicator(task, m.dependencyIndex())
	}
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

// formatReviewIndicator announces a task that is asking to be looked at, which
// models.TaskNeedsAttention answers and this only spells: the stored review flag,
// or a scheduled check-in whose calendar day has arrived and whose task is
// eligible to raise it. A future schedule belongs in Peek, while a terminal task
// stays silent and a deferred one suppresses its schedule. The row and the
// attention ladder read one predicate, so a row can never demote its priority
// badge for a review it does not name.
//
// The three tiers share deadline emphasis without borrowing deadline meaning:
// the word review keeps the signals distinct even when both occur on one row,
// and a late one is amber rather than the teal of a deadline.
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
	return theme.ReviewLateStyle.Render(fmt.Sprintf("review +%dd", daysLate))
}

const repeatedDeferralThreshold = 3

func taskHasRepeatedDeferrals(task *models.Task) bool {
	return task != nil && task.DeferCount >= repeatedDeferralThreshold
}

// taskWokeRecently reports whether a lapsed deferral is still inside the wake
// window. Peek's dates line is its one reader now that the row carries no
// deferral of its own, and it stays a named predicate because the pending and
// lapsed halves of a deferral are one ladder: `defer <date>` until the instant
// passes, then `woke <age>` for the window, then nothing.
func taskWokeRecently(task *models.Task, now time.Time) bool {
	if task == nil || task.DeferredUntil == nil || task.IsDeferredAt(now) {
		return false
	}
	return now.Sub(*task.DeferredUntil) < query.DeferWakeWindow
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
// It is returned rather than applied because the row the cursor is on wears the
// cursor's band instead: one row may carry one background, and on the row you
// are standing on the answer is where you are. Nothing about the deadline is
// lost to that — the row keeps its bolded due badge, its `!` hero emphasis and
// its overdue text color, which are the operational indicators urgency was
// always read from and the only ones that survive --no-color anyway. Move off
// the row and the tint is back.
func taskRowTint(task *models.Task) (lipgloss.Style, bool) {
	if task.IsOverdue() {
		return theme.OverdueRowStyle, true
	}
	return lipgloss.Style{}, false
}

func applyTaskRowLineStyle(gutter, content string, task *models.Task, paint cursorPaint, rowWidth, rowIndex int) string {
	if paint.marked {
		// Background precedence, stated once: the cursor's band owns the marked row
		// and neither the overdue tint nor the zebra stripe is applied over or under
		// it. outlinerCursorRow lays it under the content it was handed, so every
		// foreground that content carries reads on top of it unchanged.
		return outlinerCursorRow(gutter, content, rowWidth, paint)
	}
	tint, tinted := taskRowTint(task)

	// renderTaskTitle floors the title budget at 10 cols, so a deeply-nested row
	// (a long tree prefix) can assemble wider than rowWidth.
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
