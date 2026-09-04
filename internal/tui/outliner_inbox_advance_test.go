package tui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// inboxAdvanceOutliner builds the real Inbox row pipeline over the given
// projects. Every advance test runs against these rows rather than a hand-rolled
// slice, so the [projectHeader][task...] group shape is the one the product
// renders.
func inboxAdvanceOutliner(t *testing.T, projects ...*models.Project) *OutlinerModel {
	t.Helper()
	outliner, _ := primaryRowsOutliner(projects...)
	outliner.selectOutlinerMode(chrome.ModeInbox)
	return outliner
}

func inboxAdvanceRebuild(outliner *OutlinerModel) {
	outliner.population.invalidate()
	outliner.rebuildRows()
}

// departAfterCapture reproduces one synchronous triage keypress: capture the
// cursor's relocation, commit the state change, rebuild, and finish with
// forceFollow, exactly as the planning path does.
func departAfterCapture(t *testing.T, outliner *OutlinerModel, task *models.Task, project *models.Project, state models.WorkflowState) {
	t.Helper()
	identity := taskref.For(task, project)
	capture := outliner.captureOutlinerRelocation([]taskref.TaskIdentity{identity})
	if capture == nil {
		t.Fatalf("relocation capture for %s = nil", task.ID)
	}
	task.WorkflowState = state
	inboxAdvanceRebuild(outliner)
	if _, recorded := outliner.finishOutlinerRelocation(capture, true); recorded {
		t.Fatal("a departure recorded a jump entry")
	}
}

func requireCursorTask(t *testing.T, outliner *OutlinerModel, want taskref.TaskIdentity) {
	t.Helper()
	if got := outliner.CurrentTaskIdentity(); got != want {
		t.Fatalf("cursor task = %+v, want %+v (cursor=%d)", got, want, outliner.Cursor)
	}
}

func requireCursorRow(t *testing.T, outliner *OutlinerModel, taskID string) {
	t.Helper()
	if outliner.Cursor < 0 || outliner.Cursor >= len(outliner.rows) {
		t.Fatalf("cursor %d is outside %d rows", outliner.Cursor, len(outliner.rows))
	}
	row := outliner.rows[outliner.Cursor]
	if !row.isTaskRow() || row.task == nil {
		t.Fatalf("cursor landed on a non-task row %+v", row)
	}
	if row.task.ID != taskID {
		t.Fatalf("cursor task = %q, want %q", row.task.ID, taskID)
	}
}

func inboxCursorTo(t *testing.T, outliner *OutlinerModel, taskID string) {
	t.Helper()
	for i, row := range outliner.rows {
		if row.isTaskRow() && row.task != nil && row.task.ID == taskID {
			outliner.Cursor = i
			return
		}
	}
	t.Fatalf("task %q has no Inbox row", taskID)
}

// TestInboxAdvanceCrossesAProjectGroupHeader is the defect: the last Inbox task
// of a non-final project group must hand the cursor to the first task of the
// next group, never to that group's header.
func TestInboxAdvanceCrossesAProjectGroupHeader(t *testing.T) {
	alphaOne := primaryRowsTask("alpha-one", models.StateInbox)
	alphaTwo := primaryRowsTask("alpha-two", models.StateInbox)
	betaOne := primaryRowsTask("beta-one", models.StateInbox)
	betaTwo := primaryRowsTask("beta-two", models.StateInbox)
	alpha := primaryRowsProject("alpha", alphaOne, alphaTwo)
	beta := primaryRowsProject("beta", betaOne, betaTwo)
	outliner := inboxAdvanceOutliner(t, alpha, beta)

	inboxCursorTo(t, outliner, alphaTwo.ID)
	departAfterCapture(t, outliner, alphaTwo, alpha, models.StateBacklog)

	requireCursorRow(t, outliner, betaOne.ID)
	requireCursorTask(t, outliner, taskref.For(betaOne, beta))
}

// TestInboxAdvanceLandsOnAPromotedChild pins the case most likely to be
// "fixed" into a regression: a departing parent's surviving Inbox children are
// promoted into its place and must be landed on, not skipped.
func TestInboxAdvanceLandsOnAPromotedChild(t *testing.T) {
	lead := primaryRowsTask("lead", models.StateInbox)
	parent := primaryRowsTask("parent", models.StateInbox)
	child := primaryRowsTask("child", models.StateInbox)
	child.ParentID = &parent.ID
	trailing := primaryRowsTask("trailing", models.StateInbox)
	alpha := primaryRowsProject("alpha", lead, parent, child, trailing)
	outliner := inboxAdvanceOutliner(t, alpha)

	inboxCursorTo(t, outliner, parent.ID)
	departAfterCapture(t, outliner, parent, alpha, models.StateDoing)

	requireCursorRow(t, outliner, child.ID)
	requireCursorTask(t, outliner, taskref.For(child, alpha))
}

// TestInboxAdvanceStopsAtTheLastRow leaves the cursor to the existing clamp when
// the departing task had no successor at all.
func TestInboxAdvanceStopsAtTheLastRow(t *testing.T) {
	first := primaryRowsTask("first", models.StateInbox)
	last := primaryRowsTask("last", models.StateInbox)
	alpha := primaryRowsProject("alpha", first, last)
	outliner := inboxAdvanceOutliner(t, alpha)

	inboxCursorTo(t, outliner, last.ID)
	capture := outliner.captureOutlinerRelocation([]taskref.TaskIdentity{taskref.For(last, alpha)})
	if capture.Origin.Following.Valid() {
		t.Fatalf("last row recorded a successor %+v", capture.Origin.Following)
	}
	last.WorkflowState = models.StateBacklog
	inboxAdvanceRebuild(outliner)
	clamped := outliner.Cursor
	outliner.finishOutlinerRelocation(capture, true)

	if outliner.Cursor != clamped {
		t.Fatalf("cursor = %d after a successorless departure, want the clamp's %d", outliner.Cursor, clamped)
	}
	requireCursorRow(t, outliner, first.ID)
}

// TestInboxAdvanceIgnoresFailedMutations records that a failed mutation cannot
// advance because withOutlinerRelocationCapture attaches the capture to
// successful results only; a TransactionErrorMsg passes through bare.
func TestInboxAdvanceIgnoresFailedMutations(t *testing.T) {
	one := primaryRowsTask("one", models.StateInbox)
	two := primaryRowsTask("two", models.StateInbox)
	alpha := primaryRowsProject("alpha", one, two)
	outliner := inboxAdvanceOutliner(t, alpha)

	inboxCursorTo(t, outliner, one.ID)
	before := outliner.Cursor
	capture := outliner.captureOutlinerRelocation([]taskref.TaskIdentity{taskref.For(one, alpha)})
	failure := coordinator.TransactionErrorMsg{Err: errors.New("commit refused")}
	wrapped := withOutlinerRelocationCapture(func() tea.Msg { return failure }, capture)
	if got, ok := wrapped().(coordinator.TransactionErrorMsg); !ok || got != failure {
		t.Fatalf("failed mutation = %#v, want the bare TransactionErrorMsg", wrapped())
	}
	if outliner.Cursor != before {
		t.Fatalf("cursor = %d after a failed mutation, want %d", outliner.Cursor, before)
	}
	requireCursorRow(t, outliner, one.ID)
}

// TestInboxAdvanceNeverFollowsAnUndo is a guarantee by absence:
// handleLedgerTaskUndoRedo never calls captureOutlinerRelocation, so u and
// Ctrl+R have no capture to advance from. Nothing arms the relocation history,
// so finishing a pending relocation is a no-op and the cursor holds.
func TestInboxAdvanceNeverFollowsAnUndo(t *testing.T) {
	one := primaryRowsTask("one", models.StateInbox)
	two := primaryRowsTask("two", models.StateInbox)
	alpha := primaryRowsProject("alpha", one, two)
	outliner := inboxAdvanceOutliner(t, alpha)

	inboxCursorTo(t, outliner, one.ID)
	before := outliner.Cursor
	if outliner.relocationHistory.pending != nil {
		t.Fatal("undo path armed a relocation capture")
	}
	if cmd := outliner.finishPendingOutlinerRelocation(); cmd != nil {
		t.Fatal("finishing an unarmed relocation produced a command")
	}
	if outliner.Cursor != before {
		t.Fatalf("cursor = %d without a capture, want %d", outliner.Cursor, before)
	}
	requireCursorRow(t, outliner, one.ID)
}

// TestInboxAdvanceFollowsTheFilteredRows pins that the successor is read from
// the projection the user is looking at, not from the unfiltered population.
func TestInboxAdvanceFollowsTheFilteredRows(t *testing.T) {
	keep := primaryRowsTask("keep", models.StateInbox)
	keep.AddTag("triage")
	hidden := primaryRowsTask("hidden", models.StateInbox)
	survivor := primaryRowsTask("survivor", models.StateInbox)
	survivor.AddTag("triage")
	alpha := primaryRowsProject("alpha", keep, hidden, survivor)
	outliner := inboxAdvanceOutliner(t, alpha)
	outliner.filter.Tags = map[string]bool{"triage": true}
	outliner.adoptFilterCarriers()
	inboxAdvanceRebuild(outliner)

	if got := countTaskRows(outliner, hidden.ID); got != 0 {
		t.Fatalf("filtered Inbox rendered %q %d time(s), want hidden", hidden.ID, got)
	}
	inboxCursorTo(t, outliner, keep.ID)
	departAfterCapture(t, outliner, keep, alpha, models.StateBacklog)

	requireCursorRow(t, outliner, survivor.ID)
}

// TestInboxAdvancePastACollapsedProjectGroup pins that a collapsed successor
// group is not entered: the advance lands on the first task of the next group
// that actually renders one, and otherwise does not move.
func TestInboxAdvancePastACollapsedProjectGroup(t *testing.T) {
	alphaOne := primaryRowsTask("alpha-one", models.StateInbox)
	betaOne := primaryRowsTask("beta-one", models.StateInbox)
	gammaOne := primaryRowsTask("gamma-one", models.StateInbox)
	alpha := primaryRowsProject("alpha", alphaOne)
	beta := primaryRowsProject("beta", betaOne)
	gamma := primaryRowsProject("gamma", gammaOne)
	outliner := inboxAdvanceOutliner(t, alpha, beta, gamma)
	outliner.collapsedSections[modeProjectCollapseKey(chrome.ModeInbox, beta.ID)] = true
	inboxAdvanceRebuild(outliner)

	if got := countTaskRows(outliner, betaOne.ID); got != 0 {
		t.Fatalf("collapsed group rendered %q %d time(s), want none", betaOne.ID, got)
	}
	inboxCursorTo(t, outliner, alphaOne.ID)
	departAfterCapture(t, outliner, alphaOne, alpha, models.StateBacklog)

	requireCursorRow(t, outliner, gammaOne.ID)
}

// TestOutlinerDepartureDoesNotAdvanceOutsideInbox is the gate's contract: the
// identical sequence in a mode a user reads must leave the cursor alone.
func TestOutlinerDepartureDoesNotAdvanceOutsideInbox(t *testing.T) {
	for _, mode := range []chrome.OutlinerModeID{chrome.ModeAll, chrome.ModeBacklog} {
		t.Run(string(mode), func(t *testing.T) {
			// The same multi-project shape the Inbox defect needs: departing the
			// last task of a non-final group is exactly where an advance would
			// show, so a passing assertion here is the gate holding shut.
			alphaOne := primaryRowsTask("alpha-one", models.StateBacklog)
			alphaTwo := primaryRowsTask("alpha-two", models.StateBacklog)
			betaOne := primaryRowsTask("beta-one", models.StateBacklog)
			betaTwo := primaryRowsTask("beta-two", models.StateBacklog)
			alpha := primaryRowsProject("alpha", alphaOne, alphaTwo)
			beta := primaryRowsProject("beta", betaOne, betaTwo)
			outliner, _ := primaryRowsOutliner(alpha, beta)
			outliner.selectOutlinerMode(mode)

			inboxCursorTo(t, outliner, alphaTwo.ID)
			capture := outliner.captureOutlinerRelocation([]taskref.TaskIdentity{taskref.For(alphaTwo, alpha)})
			if capture == nil {
				t.Fatal("relocation capture = nil")
			}
			successor := taskref.For(betaOne, beta)
			if capture.Origin.Following != successor {
				t.Fatalf("%s successor = %+v, want %+v", mode, capture.Origin.Following, successor)
			}
			alphaTwo.WorkflowState = models.StateDone
			inboxAdvanceRebuild(outliner)
			clamped := outliner.Cursor
			outliner.finishOutlinerRelocation(capture, true)

			if outliner.Cursor != clamped {
				t.Fatalf("%s moved the cursor to %d on a departure, want the clamp's %d", mode, outliner.Cursor, clamped)
			}
			if got, ok := outliner.rowIndexForTask(successor); ok && outliner.Cursor == got {
				t.Fatalf("%s advanced onto the successor at row %d", mode, got)
			}
		})
	}
}

// TestOutlinerModeCatalogGrantsCursorAdvanceToInboxAlone is the companion to
// TestOutlinerModeCatalogIsExactSevenModeCycle, which compares IDs only, and
// mirrors how ShowsPinned is constrained.
func TestOutlinerModeCatalogGrantsCursorAdvanceToInboxAlone(t *testing.T) {
	for _, mode := range chrome.OutlinerModeCatalog() {
		want := mode.ID == chrome.ModeInbox
		if got := mode.SupportsCursorAdvance(); got != want {
			t.Errorf("%s SupportsCursorAdvance() = %t, want %t", mode.ID, got, want)
		}
		if mode.AdvancesOnDeparture != mode.SupportsCursorAdvance() {
			t.Errorf("%s predicate disagrees with its catalog field", mode.ID)
		}
	}
}
