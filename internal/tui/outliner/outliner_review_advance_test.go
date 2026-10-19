package outliner

import (
	"slices"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
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

// reviewScheduled is the other kind of candidate: unflagged work carrying a
// check-in that has already come due, which is what a pass navigates by once the
// flag is not the only way to ask.
func reviewScheduled(id string, repeatDays int) *models.Task {
	task := primaryRowsTask(id, models.StateBacklog)
	scheduleCheckIn(task, 0, repeatDays)
	return task
}

// scheduleCheckIn writes a schedule dayOffset calendar days from today through
// the model's own gate. The offset is relative because a check-in is judged
// against the wall clock these row tests run at, and it is written rather than
// assigned so a fixture the production path would refuse cannot be built.
func scheduleCheckIn(task *models.Task, dayOffset, repeatDays int) {
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).
		AddDate(0, 0, dayOffset)
	if err := task.SetCheckIn(
		models.CheckInSchedule{Next: &next, RepeatDays: repeatDays}, now,
	); err != nil {
		panic("fixture check-in refused: " + err.Error())
	}
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
	if ackErr := task.AcknowledgeReview(time.Now()); ackErr != nil {
		t.Fatalf("AcknowledgeReview: %v", ackErr)
	}
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
	relocationRebuild(outliner)
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

	cursorToTaskRow(t, outliner, source.ID)
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

	cursorToTaskRow(t, outliner, source.ID)
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

	cursorToTaskRow(t, outliner, source.ID)
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

	cursorToTaskRow(t, outliner, source.ID)
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

	cursorToTaskRow(t, outliner, source.ID)
	capture := planReviewAcknowledgement(t, outliner, source, alpha)
	if ackErr := answered.AcknowledgeReview(time.Now()); ackErr != nil {
		t.Fatalf("AcknowledgeReview: %v", ackErr)
	}
	requireNoExhaustionReported(t, settleReviewAcknowledgement(t, outliner, capture))

	requireCursorRow(t, outliner, survivor.ID)
}
