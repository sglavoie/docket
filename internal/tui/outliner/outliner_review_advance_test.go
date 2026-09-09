package outliner

import (
	"slices"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// reviewFlagged is a candidate that answers because the user flagged it, which
// is the one kind of candidate whose placement is under the test's control:
// staleness and Waiting both move a row into a lane of their own in All.
func reviewFlagged(id string) *models.Task {
	task := primaryRowsTask(id, models.StateBacklog)
	task.Review = true
	return task
}

// reviewSettled is open work no axis admits: fresh, unflagged, not Waiting.
func reviewSettled(id string) *models.Task {
	return primaryRowsTask(id, models.StateBacklog)
}

// reviewAdvanceOutliner builds the real Backlog projection, whose
// [projectHeader][task...] groups and tree order are what the advance walks.
// Backlog is deliberate: it has no promoted lanes, so a flagged task stays where
// the fixture put it and row order is the fixture's own.
func reviewAdvanceOutliner(projects ...*models.Project) *OutlinerModel {
	outliner, _ := primaryRowsOutliner(projects...)
	outliner.SelectOutlinerMode(chrome.ModeBacklog)
	return outliner
}

// planReviewAcknowledgement runs the half of an R press that happens before the
// commit: capture the cursor, record the successors, and clear the flag the way
// the application layer's transaction does.
func planReviewAcknowledgement(
	t *testing.T,
	outliner *OutlinerModel,
	task *models.Task,
	project *models.Project,
) *OutlinerRelocationCapture {
	t.Helper()
	capture := outliner.CaptureOutlinerRelocation([]taskref.TaskIdentity{taskref.For(task, project)})
	if capture == nil {
		t.Fatalf("relocation capture for %s = nil", task.ID)
	}
	outliner.PlanReviewAdvance(capture)
	if !capture.ReviewAdvance {
		t.Fatal("PlanReviewAdvance left the capture an ordinary relocation")
	}
	task.AcknowledgeReview(time.Now())
	return capture
}

// settleReviewAcknowledgement runs the half after the commit: the rebuild the
// refreshed workspace produces, then the finish the reload path calls.
func settleReviewAcknowledgement(
	t *testing.T,
	outliner *OutlinerModel,
	capture *OutlinerRelocationCapture,
) tea.Cmd {
	t.Helper()
	inboxAdvanceRebuild(outliner)
	cmd, recorded := outliner.FinishOutlinerRelocation(capture, true)
	if recorded {
		t.Fatal("a review acknowledgement recorded a jump-history origin")
	}
	return cmd
}

func acknowledgeAtCursor(
	t *testing.T,
	outliner *OutlinerModel,
	task *models.Task,
	project *models.Project,
) tea.Cmd {
	t.Helper()
	return settleReviewAcknowledgement(t, outliner, planReviewAcknowledgement(t, outliner, task, project))
}

func requireExhaustionReported(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("an exhausted review pass reported nothing")
	}
	msg, ok := cmd().(status.InfoStatusMsg)
	if !ok {
		t.Fatalf("exhausted review pass emitted %T, want status.InfoStatusMsg", cmd())
	}
	if string(msg) != reviewAdvanceExhaustedStatus {
		t.Fatalf("exhaustion status = %q, want %q", string(msg), reviewAdvanceExhaustedStatus)
	}
}

func requireNoExhaustionReported(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd != nil {
		t.Fatalf("an advance that moved reported %#v", cmd())
	}
}

func removeProjectTask(project *models.Project, task *models.Task) {
	project.Tasks = slices.DeleteFunc(project.Tasks, func(candidate *models.Task) bool {
		return candidate.ID == task.ID
	})
	project.BuildTaskTree()
}

// TestReviewAdvanceRecordsTheEligibleRowsBelowTheCursor is the planning contract
// on its own: the sequence is the visible eligible rows after the cursor, in row
// order, and it holds neither headers, nor settled work, nor anything above.
func TestReviewAdvanceRecordsTheEligibleRowsBelowTheCursor(t *testing.T) {
	above := reviewFlagged("above")
	source := reviewFlagged("source")
	settled := reviewSettled("settled")
	nextUp := reviewFlagged("next-up")
	alpha := primaryRowsProject("alpha", above, source, settled, nextUp)
	betaOne := reviewSettled("beta-one")
	betaTwo := reviewFlagged("beta-two")
	beta := primaryRowsProject("beta", betaOne, betaTwo)
	outliner := reviewAdvanceOutliner(alpha, beta)

	inboxCursorTo(t, outliner, source.ID)
	capture := outliner.CaptureOutlinerRelocation([]taskref.TaskIdentity{taskref.For(source, alpha)})
	if capture == nil {
		t.Fatal("relocation capture = nil")
	}
	outliner.PlanReviewAdvance(capture)

	want := []taskref.TaskIdentity{taskref.For(nextUp, alpha), taskref.For(betaTwo, beta)}
	if !slices.Equal(capture.ReviewSuccessors, want) {
		t.Fatalf("successors = %+v, want %+v", capture.ReviewSuccessors, want)
	}
}

// TestReviewAdvanceCrossesAProjectGroupHeader is the advancement contract's
// headline: the pass skips the settled row and the header between the two
// candidates and lands on the next project's flagged task.
func TestReviewAdvanceCrossesAProjectGroupHeader(t *testing.T) {
	source := reviewFlagged("source")
	trailing := reviewSettled("trailing")
	alpha := primaryRowsProject("alpha", source, trailing)
	betaOne := reviewSettled("beta-one")
	betaTwo := reviewFlagged("beta-two")
	beta := primaryRowsProject("beta", betaOne, betaTwo)
	outliner := reviewAdvanceOutliner(alpha, beta)

	inboxCursorTo(t, outliner, source.ID)
	requireNoExhaustionReported(t, acknowledgeAtCursor(t, outliner, source, alpha))

	requireCursorRow(t, outliner, betaTwo.ID)
	requireCursorTask(t, outliner, taskref.For(betaTwo, beta))
}

// TestReviewAdvanceLandsOnANestedCandidate keeps the pass inside the tree it is
// reading: a flagged child is an ordinary visible row and is landed on rather
// than skipped in favor of the next root.
func TestReviewAdvanceLandsOnANestedCandidate(t *testing.T) {
	source := reviewFlagged("source")
	parent := reviewSettled("parent")
	child := reviewFlagged("child")
	child.ParentID = &parent.ID
	trailing := reviewFlagged("trailing")
	alpha := primaryRowsProject("alpha", source, parent, child, trailing)
	outliner := reviewAdvanceOutliner(alpha)

	inboxCursorTo(t, outliner, source.ID)
	requireNoExhaustionReported(t, acknowledgeAtCursor(t, outliner, source, alpha))

	requireCursorRow(t, outliner, child.ID)
}

// TestReviewAdvanceSkipsAVanishedSuccessor is the reason eligibility is read
// twice. The recorded sequence describes rows that existed before the commit,
// and one of them can be gone by the time the cursor is placed.
func TestReviewAdvanceSkipsAVanishedSuccessor(t *testing.T) {
	source := reviewFlagged("source")
	vanishing := reviewFlagged("vanishing")
	survivor := reviewFlagged("survivor")
	alpha := primaryRowsProject("alpha", source, vanishing, survivor)
	outliner := reviewAdvanceOutliner(alpha)

	inboxCursorTo(t, outliner, source.ID)
	capture := planReviewAcknowledgement(t, outliner, source, alpha)
	if len(capture.ReviewSuccessors) != 2 {
		t.Fatalf("successors = %+v, want both candidates", capture.ReviewSuccessors)
	}
	removeProjectTask(alpha, vanishing)
	requireNoExhaustionReported(t, settleReviewAcknowledgement(t, outliner, capture))

	requireCursorRow(t, outliner, survivor.ID)
}

// TestReviewAdvanceSkipsASuccessorThatStoppedQualifying is the same recheck
// against the other outcome: the row is still there, but something answered it
// while the transaction was in flight, so the pass moves past it.
func TestReviewAdvanceSkipsASuccessorThatStoppedQualifying(t *testing.T) {
	source := reviewFlagged("source")
	answered := reviewFlagged("answered")
	survivor := reviewFlagged("survivor")
	alpha := primaryRowsProject("alpha", source, answered, survivor)
	outliner := reviewAdvanceOutliner(alpha)

	inboxCursorTo(t, outliner, source.ID)
	capture := planReviewAcknowledgement(t, outliner, source, alpha)
	answered.AcknowledgeReview(time.Now())
	requireNoExhaustionReported(t, settleReviewAcknowledgement(t, outliner, capture))

	requireCursorRow(t, outliner, survivor.ID)
}

// TestReviewAdvanceReportsExhaustionAndHoldsTheSource is the end of a pass. The
// acknowledged task is still visible — clearing the flag does not remove a
// Backlog row — so the cursor stays on it and the status says why.
func TestReviewAdvanceReportsExhaustionAndHoldsTheSource(t *testing.T) {
	leading := reviewFlagged("leading")
	source := reviewFlagged("source")
	trailing := reviewSettled("trailing")
	alpha := primaryRowsProject("alpha", leading, source, trailing)
	outliner := reviewAdvanceOutliner(alpha)

	inboxCursorTo(t, outliner, source.ID)
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

	inboxCursorTo(t, outliner, source.ID)
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
	inboxAdvanceRebuild(outliner)

	if got := countTaskRows(outliner, hidden.ID); got != 0 {
		t.Fatalf("filtered Backlog rendered %q %d time(s), want hidden", hidden.ID, got)
	}
	inboxCursorTo(t, outliner, source.ID)
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
	inboxAdvanceRebuild(outliner)

	inboxCursorTo(t, outliner, first.ID)
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

	inboxCursorTo(t, outliner, root.ID)
	if !outliner.FocusSubtreeAtCursor() {
		t.Fatal("Focus did not enter the root subtree")
	}
	if got := countTaskRows(outliner, outside.ID); got != 0 {
		t.Fatalf("focused projection rendered %q %d time(s), want none", outside.ID, got)
	}

	inboxCursorTo(t, outliner, inside.ID)
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
	inboxAdvanceRebuild(outliner)

	if got := countTaskRows(outliner, betaOne.ID); got != 0 {
		t.Fatalf("collapsed group rendered %q %d time(s), want none", betaOne.ID, got)
	}
	inboxCursorTo(t, outliner, alphaOne.ID)
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

	inboxCursorTo(t, outliner, source.ID)
	capture := planReviewAcknowledgement(t, outliner, source, alpha)
	outliner.SelectOutlinerMode(chrome.ModeAll)
	inboxAdvanceRebuild(outliner)
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

// TestReviewAdvanceCarriesAWaitingPassWithoutRepeatingItself is the Waiting
// candidate's own case. Acknowledging one changes nothing about its eligibility,
// so the pass has to move on by position rather than by predicate — otherwise
// the very next candidate it finds is the row it just answered.
func TestReviewAdvanceCarriesAWaitingPassWithoutRepeatingItself(t *testing.T) {
	first := primaryRowsTask("first", models.StateWaiting)
	second := primaryRowsTask("second", models.StateWaiting)
	alpha := primaryRowsProject("alpha", first, second)
	outliner, _ := primaryRowsOutliner(alpha)
	outliner.SelectOutlinerMode(chrome.ModeAll)

	inboxCursorTo(t, outliner, first.ID)
	requireNoExhaustionReported(t, acknowledgeAtCursor(t, outliner, first, alpha))

	requireCursorRow(t, outliner, second.ID)
	if !first.IsInState(models.StateWaiting) {
		t.Fatalf("acknowledged task state = %s, want the preserved Waiting", first.State())
	}
	if !models.TaskAwaitsReview(first, time.Now()) {
		t.Fatal("acknowledging a Waiting task stopped it being a candidate")
	}
}
