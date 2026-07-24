package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// installCrossOutlinerRoot rebuilds a ledger-backed AppModel so the cross-project
// Outliner is the live sole-root view with a single on-disk task under the
// cursor. It returns the owning project so keyflow assertions can read committed
// state straight from disk.
func installCrossOutlinerRoot(t *testing.T, m *AppModel, task *models.Task) *models.Project {
	t.Helper()
	project := m.currentProject
	cross := NewOutlinerModel(nil, m.configManager, m.runtime)
	cross.loadedProjects = []*models.Project{project}
	cross.rows = []outlinerRow{{task: task, ProjectID: project.ID}}
	cross.cursor = 0
	m.crossOutliner = cross
	m.views.Register(ViewCrossProjectOutliner, cross)
	m.viewStack = []PresentationID{ViewCrossProjectOutliner}
	return project
}

// TestOutlinerRootPlanningAndReviewKeys locks FR-001: from the sole Outliner
// root, q toggles Next membership and w toggles Today membership through the
// durable planning transaction path, and R marks/resets the persisted daily
// review timestamp in Daily Review mode while staying non-conflicting outside it.
// Every case drives a real KeyPressMsg through the shell so the routing — not
// just the underlying handlers — is exercised end to end.
func TestOutlinerRootPlanningAndReviewKeys(t *testing.T) {
	t.Run("q promotes the cursor task to Next through the durable ledger", func(t *testing.T) {
		m, _, task, dataDir := newPlanningLedgerApp(t, models.StateReady)
		project := installCrossOutlinerRoot(t, m, task)

		m.Update(syntheticKey('q'))

		if st := diskTaskState(t, project, task.ID); st != models.StateNext {
			t.Fatalf("state after q = %s, want next", st)
		}
		if got := diskFocusQueueLen(t, dataDir); got != 1 {
			t.Fatalf("Next order after q = %d, want 1 (appended in the same txn)", got)
		}
	})

	t.Run("q removes a Next task, dropping its order entry in the same txn", func(t *testing.T) {
		m, _, task, dataDir := newPlanningLedgerApp(t, models.StateNext)
		m.seedDiskFocusQueue(t, task.ID, m.currentProject.ID)
		project := installCrossOutlinerRoot(t, m, task)

		m.Update(syntheticKey('q'))

		if st := diskTaskState(t, project, task.ID); st != models.StateReady {
			t.Fatalf("state after q = %s, want ready", st)
		}
		if got := diskFocusQueueLen(t, dataDir); got != 0 {
			t.Fatalf("Next order after q = %d, want 0 (dropped by reconciliation)", got)
		}
	})

	t.Run("w adds the cursor task to Today through the durable planning path", func(t *testing.T) {
		m, _, task, dataDir := newPlanningLedgerApp(t, models.StateReady)
		installCrossOutlinerRoot(t, m, task)

		m.Update(syntheticKey('w'))

		if got := diskDailyPlanLen(t, dataDir); got != 1 {
			t.Fatalf("Daily Plan after w = %d, want 1", got)
		}
	})

	t.Run("R marks then resets the persisted review timestamp in Daily Review mode", func(t *testing.T) {
		m, _, task, _ := newPlanningLedgerApp(t, models.StateReady)
		installCrossOutlinerRoot(t, m, task)
		m.crossOutliner.reviewMode = reviewDaily

		_, cmd := m.crossOutliner.handleKeyMsg(syntheticKey('R'))
		if cmd == nil {
			t.Fatal("R in Daily Review mode returned no command")
		}
		if msg := cmd(); !isType[MarkDailyReviewDoneMsg](msg) {
			t.Fatalf("R (unmarked) emitted %T, want MarkDailyReviewDoneMsg", msg)
		} else {
			m.Update(msg)
		}
		if m.runtime.Planning().GetLastDailyReviewAt() == nil {
			t.Fatal("review timestamp not persisted after first R")
		}

		_, cmd = m.crossOutliner.handleKeyMsg(syntheticKey('R'))
		if cmd == nil {
			t.Fatal("second R in Daily Review mode returned no command")
		}
		if msg := cmd(); !isType[ResetDailyReviewMsg](msg) {
			t.Fatalf("R (marked) emitted %T, want ResetDailyReviewMsg", msg)
		} else {
			m.Update(msg)
		}
		if m.runtime.Planning().GetLastDailyReviewAt() != nil {
			t.Fatal("review timestamp not cleared after second R")
		}
	})

	t.Run("R outside Daily Review mode leaves the review timestamp untouched", func(t *testing.T) {
		m, _, task, _ := newPlanningLedgerApp(t, models.StateReady)
		installCrossOutlinerRoot(t, m, task)
		m.crossOutliner.reviewMode = reviewNone

		if before := m.runtime.Planning().GetLastDailyReviewAt(); before != nil {
			t.Fatalf("precondition review timestamp = %v, want nil", before)
		}

		m.crossOutliner.handleKeyMsg(syntheticKey('R'))

		if after := m.runtime.Planning().GetLastDailyReviewAt(); after != nil {
			t.Fatalf("R outside Daily Review set review timestamp to %v, want nil (reload branch)", after)
		}
	})
}

// isType reports whether msg is exactly of type T.
func isType[T any](msg any) bool {
	_, ok := msg.(T)
	return ok
}
