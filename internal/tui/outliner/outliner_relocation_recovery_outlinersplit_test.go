package outliner

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// The contract these tests hold is one rule for every mutation path: a task that
// is still in the block the keypress was aimed from is followed there, and one
// that left it is not followed at all — the cursor continues the sweep from the
// position the departure vacated. No mode opts in or out.

func relocationRebuild(outliner *OutlinerModel) {
	outliner.Population.Invalidate()
	outliner.RebuildRows()
}

// captureRelocation issues the keypress half of a mutation: the cursor's
// relocation capture over the given target set, which must name the cursor.
func captureRelocation(t *testing.T, outliner *OutlinerModel, targets ...taskref.TaskIdentity) *OutlinerRelocationCapture {
	t.Helper()
	capture := outliner.CaptureOutlinerRelocation(targets)
	if capture == nil {
		t.Fatalf("relocation capture over %+v = nil", targets)
	}
	return capture
}

// departAfterCapture reproduces one synchronous triage keypress: capture the
// cursor's relocation, commit the state change, rebuild, and finish with
// forceFollow, exactly as the planning path does.
func departAfterCapture(t *testing.T, outliner *OutlinerModel, task *models.Task, project *models.Project, state models.WorkflowState) {
	t.Helper()
	capture := captureRelocation(t, outliner, taskref.For(task, project))
	task.WorkflowState = state
	relocationRebuild(outliner)
	if _, recorded := outliner.FinishOutlinerRelocation(capture, true); recorded {
		t.Fatal("a departure recorded a jump entry")
	}
}

// TestRelocationRecoveryCrossesAProjectGroupHeader is the defect: the last Inbox
// task of a non-final project group must hand the cursor to the first task of
// the next group, never to that group's header.
func TestRelocationRecoveryCrossesAProjectGroupHeader(t *testing.T) {
	alphaOne := primaryRowsTask("alpha-one", models.StateInbox)
	alphaTwo := primaryRowsTask("alpha-two", models.StateInbox)
	betaOne := primaryRowsTask("beta-one", models.StateInbox)
	betaTwo := primaryRowsTask("beta-two", models.StateInbox)
	alpha := primaryRowsProject("alpha", alphaOne, alphaTwo)
	beta := primaryRowsProject("beta", betaOne, betaTwo)
	outliner := inboxModeOutliner(t, alpha, beta)

	cursorToTaskRow(t, outliner, alphaTwo.ID)
	departAfterCapture(t, outliner, alphaTwo, alpha, models.StateBacklog)

	requireCursorRow(t, outliner, betaOne.ID)
	requireCursorTask(t, outliner, taskref.For(betaOne, beta))
}

// TestRelocationRecoveryEmptiesTheGroupItLeaves is the boundary of the case
// above: the departing task was its group's only row, so the block it vacated no
// longer renders at all and the sweep continues in the next one.
func TestRelocationRecoveryEmptiesTheGroupItLeaves(t *testing.T) {
	alphaOne := primaryRowsTask("alpha-one", models.StateInbox)
	betaOne := primaryRowsTask("beta-one", models.StateInbox)
	alpha := primaryRowsProject("alpha", alphaOne)
	beta := primaryRowsProject("beta", betaOne)
	outliner := inboxModeOutliner(t, alpha, beta)

	cursorToTaskRow(t, outliner, alphaOne.ID)
	departAfterCapture(t, outliner, alphaOne, alpha, models.StateBacklog)

	if got := countTaskRows(outliner, alphaOne.ID); got != 0 {
		t.Fatalf("emptied group still rendered %q %d time(s), want none", alphaOne.ID, got)
	}
	requireCursorRow(t, outliner, betaOne.ID)
}

// TestRelocationRecoveryLandsOnAPromotedChild pins the case most likely to be
// "fixed" into a regression: a departing parent's surviving children are
// promoted into its place and must be landed on, not skipped.
func TestRelocationRecoveryLandsOnAPromotedChild(t *testing.T) {
	lead := primaryRowsTask("lead", models.StateInbox)
	parent := primaryRowsTask("parent", models.StateInbox)
	child := primaryRowsTask("child", models.StateInbox)
	child.ParentID = &parent.ID
	trailing := primaryRowsTask("trailing", models.StateInbox)
	alpha := primaryRowsProject("alpha", lead, parent, child, trailing)
	outliner := inboxModeOutliner(t, alpha)

	cursorToTaskRow(t, outliner, parent.ID)
	departAfterCapture(t, outliner, parent, alpha, models.StateDoing)

	requireCursorRow(t, outliner, child.ID)
	requireCursorTask(t, outliner, taskref.For(child, alpha))
}

// TestRelocationRecoveryLandsOnAChildPromotedFromAFoldedParent is why the
// capture records the children rather than reading them back off the rebuilt
// rows: a folded parent renders none of them, so the row that followed it was
// the next sibling and the promoted child can only be named from before the
// mutation.
func TestRelocationRecoveryLandsOnAChildPromotedFromAFoldedParent(t *testing.T) {
	parent := primaryRowsTask("parent", models.StateInbox)
	child := primaryRowsTask("child", models.StateInbox)
	child.ParentID = &parent.ID
	trailing := primaryRowsTask("trailing", models.StateInbox)
	alpha := primaryRowsProject("alpha", parent, child, trailing)
	outliner := inboxModeOutliner(t, alpha)
	outliner.Folds.setNode(taskref.For(parent, alpha), false)
	relocationRebuild(outliner)

	if got := countTaskRows(outliner, child.ID); got != 0 {
		t.Fatalf("folded parent rendered %q %d time(s), want none", child.ID, got)
	}
	cursorToTaskRow(t, outliner, parent.ID)
	departAfterCapture(t, outliner, parent, alpha, models.StateDoing)

	requireCursorRow(t, outliner, child.ID)
}

// TestRelocationRecoverySkipsTheRestOfABulkSelection pins the partially
// surviving bulk case: the selected tasks are not candidates for the cursor, so
// a sweep that removed two of four lands on the first task nobody selected.
func TestRelocationRecoverySkipsTheRestOfABulkSelection(t *testing.T) {
	first := primaryRowsTask("first", models.StateBacklog)
	second := primaryRowsTask("second", models.StateBacklog)
	survivor := primaryRowsTask("survivor", models.StateBacklog)
	trailing := primaryRowsTask("trailing", models.StateBacklog)
	alpha := primaryRowsProject("alpha", first, second, survivor, trailing)
	outliner, _ := primaryRowsOutliner(alpha)
	outliner.SelectOutlinerMode(chrome.ModeBacklog)

	cursorToTaskRow(t, outliner, first.ID)
	capture := captureRelocation(t, outliner, taskref.For(first, alpha), taskref.For(second, alpha))
	if want := taskref.For(survivor, alpha); capture.Origin.Following != want {
		t.Fatalf("successor = %+v, want the first unselected task %+v", capture.Origin.Following, want)
	}
	first.WorkflowState = models.StateDone
	second.WorkflowState = models.StateDone
	relocationRebuild(outliner)
	outliner.FinishOutlinerRelocation(capture, true)

	requireCursorRow(t, outliner, survivor.ID)
}

// TestRelocationRecoveryStopsAtTheLastRow leaves the cursor to the existing
// clamp when the departing task had no successor at all.
func TestRelocationRecoveryStopsAtTheLastRow(t *testing.T) {
	first := primaryRowsTask("first", models.StateInbox)
	last := primaryRowsTask("last", models.StateInbox)
	alpha := primaryRowsProject("alpha", first, last)
	outliner := inboxModeOutliner(t, alpha)

	cursorToTaskRow(t, outliner, last.ID)
	capture := captureRelocation(t, outliner, taskref.For(last, alpha))
	if capture.Origin.Following.Valid() {
		t.Fatalf("last row recorded a successor %+v", capture.Origin.Following)
	}
	last.WorkflowState = models.StateBacklog
	relocationRebuild(outliner)
	outliner.FinishOutlinerRelocation(capture, true)

	requireCursorRow(t, outliner, first.ID)
}

// TestRelocationRecoveryNeverFollowsAnUndo is a guarantee by absence:
// handleLedgerTaskUndoRedo never calls captureOutlinerRelocation, so u and
// Ctrl+R have no capture to advance from. Nothing arms the relocation history,
// so finishing a pending relocation is a no-op and the cursor holds.
func TestRelocationRecoveryNeverFollowsAnUndo(t *testing.T) {
	one := primaryRowsTask("one", models.StateInbox)
	two := primaryRowsTask("two", models.StateInbox)
	alpha := primaryRowsProject("alpha", one, two)
	outliner := inboxModeOutliner(t, alpha)

	cursorToTaskRow(t, outliner, one.ID)
	before := outliner.Cursor
	if outliner.RelocationHistory.Pending != nil {
		t.Fatal("undo path armed a relocation capture")
	}
	if cmd := outliner.FinishPendingOutlinerRelocation(); cmd != nil {
		t.Fatal("finishing an unarmed relocation produced a command")
	}
	if outliner.Cursor != before {
		t.Fatalf("cursor = %d without a capture, want %d", outliner.Cursor, before)
	}
	requireCursorRow(t, outliner, one.ID)
}

// TestRelocationRecoveryFollowsTheFilteredRows pins that the successor is read
// from the projection the user is looking at, not from the unfiltered
// population.
func TestRelocationRecoveryFollowsTheFilteredRows(t *testing.T) {
	keep := primaryRowsTask("keep", models.StateInbox)
	keep.AddTag("triage")
	hidden := primaryRowsTask("hidden", models.StateInbox)
	survivor := primaryRowsTask("survivor", models.StateInbox)
	survivor.AddTag("triage")
	alpha := primaryRowsProject("alpha", keep, hidden, survivor)
	outliner := inboxModeOutliner(t, alpha)
	outliner.Filter.Tags = map[string]bool{"triage": true}
	adoptFilterCarriers(outliner)
	relocationRebuild(outliner)

	if got := countTaskRows(outliner, hidden.ID); got != 0 {
		t.Fatalf("filtered Inbox rendered %q %d time(s), want hidden", hidden.ID, got)
	}
	cursorToTaskRow(t, outliner, keep.ID)
	departAfterCapture(t, outliner, keep, alpha, models.StateBacklog)

	requireCursorRow(t, outliner, survivor.ID)
}

// TestRelocationRecoveryPastACollapsedProjectGroup pins that a collapsed
// successor group is not entered: the recovery lands on the first task of the
// next group that actually renders one.
func TestRelocationRecoveryPastACollapsedProjectGroup(t *testing.T) {
	alphaOne := primaryRowsTask("alpha-one", models.StateInbox)
	betaOne := primaryRowsTask("beta-one", models.StateInbox)
	gammaOne := primaryRowsTask("gamma-one", models.StateInbox)
	alpha := primaryRowsProject("alpha", alphaOne)
	beta := primaryRowsProject("beta", betaOne)
	gamma := primaryRowsProject("gamma", gammaOne)
	outliner := inboxModeOutliner(t, alpha, beta, gamma)
	outliner.Folds.CollapsedSections[modeProjectCollapseKey(chrome.ModeInbox, beta.ID)] = true
	relocationRebuild(outliner)

	if got := countTaskRows(outliner, betaOne.ID); got != 0 {
		t.Fatalf("collapsed group rendered %q %d time(s), want none", betaOne.ID, got)
	}
	cursorToTaskRow(t, outliner, alphaOne.ID)
	departAfterCapture(t, outliner, alphaOne, alpha, models.StateBacklog)

	requireCursorRow(t, outliner, gammaOne.ID)
}

// TestRelocationRecoveryContinuesTheSweepInEveryMode is what replaced the Inbox
// gate: the identical sequence in a mode a user reads recovers exactly the same
// way. The middle group is folded because its summary is the one row that
// separates a clamp from a recovery — an expanded group spends no row on a
// heading, so the departing task's row index would otherwise be the successor's
// own and neither outcome would be observable.
func TestRelocationRecoveryContinuesTheSweepInEveryMode(t *testing.T) {
	for _, mode := range []chrome.OutlinerModeID{chrome.ModeAll, chrome.ModeBacklog} {
		t.Run(string(mode), func(t *testing.T) {
			alphaOne := primaryRowsTask("alpha-one", models.StateBacklog)
			alphaTwo := primaryRowsTask("alpha-two", models.StateBacklog)
			betaOne := primaryRowsTask("beta-one", models.StateBacklog)
			gammaOne := primaryRowsTask("gamma-one", models.StateBacklog)
			alpha := primaryRowsProject("alpha", alphaOne, alphaTwo)
			beta := primaryRowsProject("beta", betaOne)
			gamma := primaryRowsProject("gamma", gammaOne)
			outliner, _ := primaryRowsOutliner(alpha, beta, gamma)
			outliner.SelectOutlinerMode(mode)
			outliner.Folds.CollapsedSections[projectGroupCollapseKey(outliner, beta.ID)] = true
			relocationRebuild(outliner)
			if got := countTaskRows(outliner, betaOne.ID); got != 0 {
				t.Fatalf("%s rendered the folded group's %q %d time(s), want none", mode, betaOne.ID, got)
			}

			cursorToTaskRow(t, outliner, alphaTwo.ID)
			departAfterCapture(t, outliner, alphaTwo, alpha, models.StateDone)

			requireCursorRow(t, outliner, gammaOne.ID)
			requireCursorTask(t, outliner, taskref.For(gammaOne, gamma))
		})
	}
}

// TestRelocationDoesNotFollowATaskIntoAnotherLane is the group half of the rule
// stated without a mode: All still draws the task, in the Waiting lane, and
// following it there would carry the cursor out of the block the keypress was
// aimed from. Bare visibility is therefore not the retention test.
func TestRelocationDoesNotFollowATaskIntoAnotherLane(t *testing.T) {
	moved := primaryRowsTask("moved", models.StateBacklog)
	survivor := primaryRowsTask("survivor", models.StateBacklog)
	alpha := primaryRowsProject("alpha", moved, survivor)
	outliner, _ := primaryRowsOutliner(alpha)
	outliner.SelectOutlinerMode(chrome.ModeAll)

	cursorToTaskRow(t, outliner, moved.ID)
	departAfterCapture(t, outliner, moved, alpha, models.StateWaiting)

	if got := countTaskRows(outliner, moved.ID); got != 1 {
		t.Fatalf("waiting task rendered %d time(s), want its Waiting lane row", got)
	}
	requireCursorRow(t, outliner, survivor.ID)
}

// TestRelocationFollowsATaskThatStaysInItsGroup is the retention half: a
// reparent inside the same block moves the row without leaving the group, so the
// cursor goes with it and the vacated origin is recorded for Ctrl+O.
func TestRelocationFollowsATaskThatStaysInItsGroup(t *testing.T) {
	first := primaryRowsTask("first", models.StateBacklog)
	second := primaryRowsTask("second", models.StateBacklog)
	moved := primaryRowsTask("moved", models.StateBacklog)
	alpha := primaryRowsProject("alpha", first, second, moved)
	outliner, _ := primaryRowsOutliner(alpha)
	outliner.SelectOutlinerMode(chrome.ModeAll)

	cursorToTaskRow(t, outliner, moved.ID)
	capture := captureRelocation(t, outliner, taskref.For(moved, alpha))
	origin := capture.Origin.RowIndex
	moved.ParentID = &first.ID
	alpha.BuildTaskTree()
	relocationRebuild(outliner)
	_, recorded := outliner.FinishOutlinerRelocation(capture, true)

	requireCursorRow(t, outliner, moved.ID)
	if outliner.Cursor == origin {
		t.Fatalf("cursor stayed at %d, so the reparent did not move the row", origin)
	}
	if !recorded {
		t.Fatal("an in-group relocation recorded no jump origin")
	}
	if len(outliner.RelocationHistory.Back) != 1 {
		t.Fatalf("jump history = %+v, want the one vacated origin", outliner.RelocationHistory.Back)
	}
}

// TestTaskLineEditFollowsAnInGroupEdit and its companion below are the inline
// editor's half of the same rule. An edit that rearranged the task inside its
// own block is followed, and Ctrl+O returns to the row it was typed from.
func TestTaskLineEditFollowsAnInGroupEdit(t *testing.T) {
	first := primaryRowsTask("first", models.StateBacklog)
	second := primaryRowsTask("second", models.StateBacklog)
	edited := primaryRowsTask("edited", models.StateBacklog)
	alpha := primaryRowsProject("alpha", first, second, edited)
	outliner, _ := primaryRowsOutliner(alpha)
	outliner.SelectOutlinerMode(chrome.ModeAll)

	cursorToTaskRow(t, outliner, edited.ID)
	capture := captureRelocation(t, outliner, taskref.For(edited, alpha))
	origin := capture.Origin.RowIndex
	edited.ParentID = &first.ID
	alpha.BuildTaskTree()
	relocationRebuild(outliner)
	_, recorded := outliner.FinishTaskLineEditRelocation(capture, taskref.For(edited, alpha))

	requireCursorRow(t, outliner, edited.ID)
	if outliner.Cursor == origin {
		t.Fatalf("cursor stayed at %d, so the edit did not move the row", origin)
	}
	if !recorded {
		t.Fatal("an in-group line edit recorded no jump origin")
	}
}

// TestTaskLineEditRecoversFromACrossProjectMove is the departure the editor can
// spell that no other key can: `%project` moves the task out of the block
// entirely, and the cursor stays with the work the user was reading.
func TestTaskLineEditRecoversFromACrossProjectMove(t *testing.T) {
	moved := primaryRowsTask("moved", models.StateBacklog)
	survivor := primaryRowsTask("survivor", models.StateBacklog)
	alpha := primaryRowsProject("alpha", moved, survivor)
	beta := primaryRowsProject("beta", primaryRowsTask("beta-one", models.StateBacklog))
	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.SelectOutlinerMode(chrome.ModeAll)

	cursorToTaskRow(t, outliner, moved.ID)
	capture := captureRelocation(t, outliner, taskref.For(moved, alpha))
	alpha.Tasks = []*models.Task{survivor}
	alpha.BuildTaskTree()
	beta.Tasks = append(beta.Tasks, moved)
	beta.BuildTaskTree()
	relocationRebuild(outliner)
	_, recorded := outliner.FinishTaskLineEditRelocation(capture, taskref.For(moved, beta))

	if recorded {
		t.Fatal("a cross-project edit recorded a jump entry")
	}
	requireCursorRow(t, outliner, survivor.ID)
	if len(outliner.RelocationHistory.Back) != 0 {
		t.Fatalf("jump history = %+v, want nothing vacated", outliner.RelocationHistory.Back)
	}
}
