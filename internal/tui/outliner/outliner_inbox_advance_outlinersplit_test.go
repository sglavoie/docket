package outliner

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func inboxAdvanceRebuild(outliner *OutlinerModel) {
	outliner.Population.Invalidate()
	outliner.RebuildRows()
}

// departAfterCapture reproduces one synchronous triage keypress: capture the
// cursor's relocation, commit the state change, rebuild, and finish with
// forceFollow, exactly as the planning path does.
func departAfterCapture(t *testing.T, outliner *OutlinerModel, task *models.Task, project *models.Project, state models.WorkflowState) {
	t.Helper()
	identity := taskref.For(task, project)
	capture := outliner.CaptureOutlinerRelocation([]taskref.TaskIdentity{identity})
	if capture == nil {
		t.Fatalf("relocation capture for %s = nil", task.ID)
	}
	task.WorkflowState = state
	inboxAdvanceRebuild(outliner)
	if _, recorded := outliner.FinishOutlinerRelocation(capture, true); recorded {
		t.Fatal("a departure recorded a jump entry")
	}
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
	capture := outliner.CaptureOutlinerRelocation([]taskref.TaskIdentity{taskref.For(last, alpha)})
	if capture.Origin.Following.Valid() {
		t.Fatalf("last row recorded a successor %+v", capture.Origin.Following)
	}
	last.WorkflowState = models.StateBacklog
	inboxAdvanceRebuild(outliner)
	clamped := outliner.Cursor
	outliner.FinishOutlinerRelocation(capture, true)

	if outliner.Cursor != clamped {
		t.Fatalf("cursor = %d after a successorless departure, want the clamp's %d", outliner.Cursor, clamped)
	}
	requireCursorRow(t, outliner, first.ID)
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
	outliner.Filter.Tags = map[string]bool{"triage": true}
	adoptFilterCarriers(outliner)
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
	outliner.Folds.CollapsedSections[modeProjectCollapseKey(chrome.ModeInbox, beta.ID)] = true
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
			// show, so a passing assertion here is the gate holding shut. The
			// middle group is folded because its summary is the one row that now
			// separates the clamp from the successor: an expanded group spends no
			// row on a heading, so a departing task's row index would otherwise be
			// the successor's own and neither outcome would be observable.
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
			inboxAdvanceRebuild(outliner)
			if got := countTaskRows(outliner, betaOne.ID); got != 0 {
				t.Fatalf("%s rendered the folded group's %q %d time(s), want none", mode, betaOne.ID, got)
			}

			inboxCursorTo(t, outliner, alphaTwo.ID)
			capture := outliner.CaptureOutlinerRelocation([]taskref.TaskIdentity{taskref.For(alphaTwo, alpha)})
			if capture == nil {
				t.Fatal("relocation capture = nil")
			}
			successor := taskref.For(gammaOne, gamma)
			if capture.Origin.Following != successor {
				t.Fatalf("%s successor = %+v, want %+v", mode, capture.Origin.Following, successor)
			}
			alphaTwo.WorkflowState = models.StateDone
			inboxAdvanceRebuild(outliner)
			clamped := outliner.Cursor
			outliner.FinishOutlinerRelocation(capture, true)

			if outliner.Cursor != clamped {
				t.Fatalf("%s moved the cursor to %d on a departure, want the clamp's %d", mode, outliner.Cursor, clamped)
			}
			if got, ok := outliner.RowIndexForTask(successor); ok && outliner.Cursor == got {
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
