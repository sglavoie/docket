package outliner

import (
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/query"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// TestReviewAdvanceReportsExhaustionAndHoldsTheSource is the end of a pass. The
// acknowledged task is still visible — clearing the flag does not remove a
// Backlog row — so the cursor stays on it and the status says why.
func TestReviewAdvanceReportsExhaustionAndHoldsTheSource(t *testing.T) {
	leading := reviewFlagged("leading")
	source := reviewFlagged("source")
	trailing := reviewSettled("trailing")
	alpha := primaryRowsProject("alpha", leading, source, trailing)
	outliner := reviewAdvanceOutliner(alpha)

	cursorToTaskRow(t, outliner, source.ID)
	capture := planReviewAcknowledgement(t, outliner, source, alpha)
	if len(capture.ReviewSuccessors) != 0 {
		t.Fatalf("successors = %+v, want none below the cursor", capture.ReviewSuccessors)
	}
	requireExhaustionReported(t, settleReviewAcknowledgement(t, outliner, capture))

	requireCursorRow(t, outliner, source.ID)
}

// TestReviewAdvanceNeverWraps is the exhaustion case's point: a candidate above
// the cursor is not a successor, so a pass that reaches the bottom is over
// rather than restarted from the top.
func TestReviewAdvanceNeverWraps(t *testing.T) {
	skipped := reviewFlagged("skipped")
	source := reviewFlagged("source")
	alpha := primaryRowsProject("alpha", skipped, source)
	outliner := reviewAdvanceOutliner(alpha)

	cursorToTaskRow(t, outliner, source.ID)
	requireExhaustionReported(t, acknowledgeAtCursor(t, outliner, source, alpha))

	requireCursorRow(t, outliner, source.ID)
	if !skipped.Review {
		t.Fatal("the fixture stopped proving a candidate remained above the cursor")
	}
}

// TestReviewAdvanceFollowsTheFilteredRows pins that the pass reads the
// projection the user is looking at: a candidate a filter hides is not visible
// and so is not a successor.
func TestReviewAdvanceFollowsTheFilteredRows(t *testing.T) {
	source := reviewFlagged("source")
	source.AddTag("triage")
	hidden := reviewFlagged("hidden")
	survivor := reviewFlagged("survivor")
	survivor.AddTag("triage")
	alpha := primaryRowsProject("alpha", source, hidden, survivor)
	outliner := reviewAdvanceOutliner(alpha)
	outliner.Filter.Tags = map[string]bool{"triage": true}
	adoptFilterCarriers(outliner)
	relocationRebuild(outliner)

	if got := countTaskRows(outliner, hidden.ID); got != 0 {
		t.Fatalf("filtered Backlog rendered %q %d time(s), want hidden", hidden.ID, got)
	}
	cursorToTaskRow(t, outliner, source.ID)
	requireNoExhaustionReported(t, acknowledgeAtCursor(t, outliner, source, alpha))

	requireCursorRow(t, outliner, survivor.ID)
	if !hidden.Review {
		t.Fatal("the hidden candidate lost its flag; the fixture no longer proves the skip")
	}
}

// TestReviewAdvanceFollowsTheSortedOrder is the same rule against the other
// axis: a sort reorders the rows, and "after the cursor" means after it in the
// order on screen rather than in stored order.
func TestReviewAdvanceFollowsTheSortedOrder(t *testing.T) {
	// Stored order is c, a, b; sorted by title it is a, b, c.
	stored := reviewFlagged("c-source")
	first := reviewFlagged("a-first")
	second := reviewFlagged("b-second")
	alpha := primaryRowsProject("alpha", stored, first, second)
	outliner := reviewAdvanceOutliner(alpha)
	outliner.Sort = query.SortState{Field: query.SortByTitle, Ascending: true}
	outliner.TreeSort = outliner.Sort
	relocationRebuild(outliner)

	cursorToTaskRow(t, outliner, first.ID)
	capture := planReviewAcknowledgement(t, outliner, first, alpha)
	want := []taskref.TaskIdentity{taskref.For(second, alpha), taskref.For(stored, alpha)}
	if !slices.Equal(capture.ReviewSuccessors, want) {
		t.Fatalf("successors = %+v, want the sorted order %+v", capture.ReviewSuccessors, want)
	}
	requireNoExhaustionReported(t, settleReviewAcknowledgement(t, outliner, capture))

	requireCursorRow(t, outliner, second.ID)
}

// TestReviewAdvanceStaysInsideFocus keeps the pass from widening the world to
// find work. Focus is part of the projection the sequence was recorded against,
// so a candidate outside the focused subtree does not exist to it.
func TestReviewAdvanceStaysInsideFocus(t *testing.T) {
	root := reviewSettled("root")
	inside := reviewFlagged("inside")
	inside.ParentID = &root.ID
	outside := reviewFlagged("outside")
	alpha := primaryRowsProject("alpha", root, inside, outside)
	outliner := reviewAdvanceOutliner(alpha)

	cursorToTaskRow(t, outliner, root.ID)
	if !outliner.FocusSubtreeAtCursor() {
		t.Fatal("Focus did not enter the root subtree")
	}
	if got := countTaskRows(outliner, outside.ID); got != 0 {
		t.Fatalf("focused projection rendered %q %d time(s), want none", outside.ID, got)
	}

	cursorToTaskRow(t, outliner, inside.ID)
	requireExhaustionReported(t, acknowledgeAtCursor(t, outliner, inside, alpha))

	requireCursorRow(t, outliner, inside.ID)
	if got := countTaskRows(outliner, outside.ID); got != 0 {
		t.Fatal("the advance widened past Focus to reach the outside candidate")
	}
}

// TestReviewAdvanceDoesNotEnterACollapsedGroup is the fold half of the same
// rule: the pass never expands anything to find a candidate, so a collapsed
// project group is walked past rather than opened.
func TestReviewAdvanceDoesNotEnterACollapsedGroup(t *testing.T) {
	alphaOne := reviewFlagged("alpha-one")
	betaOne := reviewFlagged("beta-one")
	gammaOne := reviewFlagged("gamma-one")
	alpha := primaryRowsProject("alpha", alphaOne)
	beta := primaryRowsProject("beta", betaOne)
	gamma := primaryRowsProject("gamma", gammaOne)
	outliner := reviewAdvanceOutliner(alpha, beta, gamma)
	outliner.Folds.CollapsedSections[modeProjectCollapseKey(chrome.ModeBacklog, beta.ID)] = true
	relocationRebuild(outliner)

	if got := countTaskRows(outliner, betaOne.ID); got != 0 {
		t.Fatalf("collapsed group rendered %q %d time(s), want none", betaOne.ID, got)
	}
	cursorToTaskRow(t, outliner, alphaOne.ID)
	requireNoExhaustionReported(t, acknowledgeAtCursor(t, outliner, alphaOne, alpha))

	requireCursorRow(t, outliner, gammaOne.ID)
	if !outliner.Folds.CollapsedSections[modeProjectCollapseKey(chrome.ModeBacklog, beta.ID)] {
		t.Fatal("the advance expanded the collapsed group it passed")
	}
}

// TestReviewAdvanceIsAbandonedWhenTheProjectionChanges rides the guard every
// relocation capture already sits behind. A user who changes mode while the
// commit is in flight is looking at a different list, and a sequence recorded
// against the old one has no meaning in it.
func TestReviewAdvanceIsAbandonedWhenTheProjectionChanges(t *testing.T) {
	source := reviewFlagged("source")
	successor := reviewFlagged("successor")
	alpha := primaryRowsProject("alpha", source, successor)
	outliner := reviewAdvanceOutliner(alpha)

	cursorToTaskRow(t, outliner, source.ID)
	capture := planReviewAcknowledgement(t, outliner, source, alpha)
	outliner.SelectOutlinerMode(chrome.ModeAll)
	relocationRebuild(outliner)
	before := outliner.Cursor

	cmd, recorded := outliner.FinishOutlinerRelocation(capture, true)
	if recorded {
		t.Fatal("an abandoned advance recorded a jump-history origin")
	}
	if cmd != nil {
		t.Fatalf("an abandoned advance reported %#v", cmd())
	}
	if outliner.Cursor != before {
		t.Fatalf("cursor = %d after a projection change, want the untouched %d", outliner.Cursor, before)
	}
}

// TestReviewAdvanceCarriesARepeatingPassWithoutRepeatingItself is the repeating
// candidate's own case, and it is the one a cadence could break: the schedule
// survives its acknowledgement, so the pass would circle if a surviving schedule
// still counted. It does not, because the answered occurrence moved to a day
// that has not come.
func TestReviewAdvanceCarriesARepeatingPassWithoutRepeatingItself(t *testing.T) {
	first := reviewScheduled("first", 7)
	second := reviewScheduled("second", 7)
	alpha := primaryRowsProject("alpha", first, second)
	outliner := reviewAdvanceOutliner(alpha)

	cursorToTaskRow(t, outliner, first.ID)
	requireNoExhaustionReported(t, acknowledgeAtCursor(t, outliner, first, alpha))

	requireCursorRow(t, outliner, second.ID)
	if first.NextCheckIn == nil || first.CheckInRepeatDays != 7 {
		t.Fatalf("acknowledged check-in = %v every %d days, want the cadence preserved",
			first.NextCheckIn, first.CheckInRepeatDays)
	}
	if models.TaskNeedsAttention(first, time.Now()) {
		t.Fatal("an answered repeating check-in is still asking for attention")
	}
	// And it is still reviewable by name, which is what an early review is; only
	// the automatic pass has moved on.
	if !models.TaskAllowsExplicitReview(first, time.Now()) {
		t.Fatal("a scheduled task stopped allowing an early review")
	}
}

// TestReviewAdvanceIgnoresAFutureCheckIn is the narrowing stated as navigation:
// scheduled work whose day has not come may be reviewed by name, but a pass does
// not stop on it, so the cursor crosses it to reach what is actually owed.
func TestReviewAdvanceIgnoresAFutureCheckIn(t *testing.T) {
	source := reviewFlagged("source")
	ahead := reviewScheduled("ahead", models.CheckInRepeatOff)
	scheduleCheckIn(ahead, 5, models.CheckInRepeatOff)
	owed := reviewScheduled("owed", models.CheckInRepeatOff)
	alpha := primaryRowsProject("alpha", source, ahead, owed)
	outliner := reviewAdvanceOutliner(alpha)

	cursorToTaskRow(t, outliner, source.ID)
	capture := planReviewAcknowledgement(t, outliner, source, alpha)
	want := []taskref.TaskIdentity{taskref.For(owed, alpha)}
	if !slices.Equal(capture.ReviewSuccessors, want) {
		t.Fatalf("successors = %+v, want the owed check-in alone %+v", capture.ReviewSuccessors, want)
	}
	requireNoExhaustionReported(t, settleReviewAcknowledgement(t, outliner, capture))

	requireCursorRow(t, outliner, owed.ID)
	if !models.TaskAllowsExplicitReview(ahead, time.Now()) {
		t.Fatal("the skipped row stopped being explicitly reviewable; the test proves nothing")
	}
}

// TestReviewAdvanceIgnoresStalenessAndWaiting pins the two retired routes as
// navigation refuses them too. Both rows are visible and both would have been
// successors under the old predicate, so a pass that stops on either has grown
// the automatic question back.
func TestReviewAdvanceIgnoresStalenessAndWaiting(t *testing.T) {
	source := reviewFlagged("source")
	stale := reviewSettled("stale")
	stale.UpdatedAt = time.Now().AddDate(0, 0, -(models.StaleTaskThresholdDays + 10))
	waiting := primaryRowsTask("waiting", models.StateWaiting)
	alpha := primaryRowsProject("alpha", source, stale, waiting)
	// All rather than Backlog, because All is the one projection that draws a
	// Waiting row, a stale row, and a flagged row at once.
	outliner, _ := primaryRowsOutliner(alpha)
	outliner.SelectOutlinerMode(chrome.ModeAll)

	if !models.QueryTaskIsStale(stale, time.Now()) {
		t.Fatal("the stale fixture is not stale; the boundary proves nothing")
	}
	for _, id := range []string{stale.ID, waiting.ID} {
		if countTaskRows(outliner, id) == 0 {
			t.Fatalf("%q is not visible; a skip over an invisible row proves nothing", id)
		}
	}
	cursorToTaskRow(t, outliner, source.ID)
	capture := planReviewAcknowledgement(t, outliner, source, alpha)
	if len(capture.ReviewSuccessors) != 0 {
		t.Fatalf("successors = %+v, want none: neither staleness nor Waiting is a candidate",
			capture.ReviewSuccessors)
	}
	requireExhaustionReported(t, settleReviewAcknowledgement(t, outliner, capture))

	requireCursorRow(t, outliner, source.ID)
}
