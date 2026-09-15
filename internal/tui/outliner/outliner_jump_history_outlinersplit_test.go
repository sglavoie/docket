package outliner

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func TestOutlinerRelocationJumpBackwardAndForward(t *testing.T) {
	outliner, tasks := jumpHistoryFixture("before", "target", "after", "tail")
	target := taskref.TaskIdentity{ProjectID: "alpha", TaskID: "target"}
	outliner.Cursor = 1
	outliner.ViewScrollOffset = 7

	capture := outliner.CaptureOutlinerRelocation([]taskref.TaskIdentity{target})
	if capture == nil {
		t.Fatal("relocation capture = nil")
	}
	if got := capture.Origin.Following; got != (taskref.TaskIdentity{ProjectID: "alpha", TaskID: "after"}) {
		t.Fatalf("following anchor = %+v, want after", got)
	}
	if got := capture.Origin.Preceding; got != (taskref.TaskIdentity{ProjectID: "alpha", TaskID: "before"}) {
		t.Fatalf("preceding anchor = %+v, want before", got)
	}

	setJumpHistoryRows(outliner, tasks, "target", "before", "after", "tail")
	outliner.Cursor = 0 // The mutation refresh follows the relocated task.
	_, recorded := outliner.FinishOutlinerRelocation(capture, true)
	if !recorded {
		t.Fatal("relocation moved rows but did not report recorded origin history")
	}
	if len(outliner.RelocationHistory.Back) != 1 || len(outliner.RelocationHistory.forward) != 0 {
		t.Fatalf("history after relocation = back %d forward %d, want 1/0",
			len(outliner.RelocationHistory.Back), len(outliner.RelocationHistory.forward))
	}
	requireCurrentJumpTask(t, outliner, target)

	outliner.JumpBackward()
	requireCurrentJumpTask(t, outliner, taskref.TaskIdentity{ProjectID: "alpha", TaskID: "after"})
	if outliner.ViewScrollOffset != 7 {
		t.Fatalf("backward scroll offset = %d, want 7", outliner.ViewScrollOffset)
	}

	outliner.JumpForward()
	requireCurrentJumpTask(t, outliner, target)
}

func TestOutlinerRelocationReportsNoOriginWhenRowDoesNotMove(t *testing.T) {
	outliner, _ := jumpHistoryFixture("target", "after")
	target := taskref.TaskIdentity{ProjectID: "alpha", TaskID: "target"}
	outliner.Cursor = 0
	capture := outliner.CaptureOutlinerRelocation([]taskref.TaskIdentity{target})
	if _, recorded := outliner.FinishOutlinerRelocation(capture, true); recorded {
		t.Fatal("unchanged row advertised recorded origin history")
	}
}

func TestOutlinerRelocationOriginFallsBackToPrecedingThenNearestRow(t *testing.T) {
	t.Run("preceding", func(t *testing.T) {
		outliner, tasks := jumpHistoryFixture("before", "target", "after")
		target := taskref.TaskIdentity{ProjectID: "alpha", TaskID: "target"}
		outliner.Cursor = 1
		capture := outliner.CaptureOutlinerRelocation([]taskref.TaskIdentity{target})

		setJumpHistoryRows(outliner, tasks, "target", "before")
		outliner.FinishOutlinerRelocation(capture, true)
		outliner.JumpBackward()
		requireCurrentJumpTask(t, outliner, taskref.TaskIdentity{ProjectID: "alpha", TaskID: "before"})
	})

	t.Run("numeric", func(t *testing.T) {
		outliner, tasks := jumpHistoryFixture("before", "target", "after", "survivor")
		target := taskref.TaskIdentity{ProjectID: "alpha", TaskID: "target"}
		outliner.Cursor = 1
		capture := outliner.CaptureOutlinerRelocation([]taskref.TaskIdentity{target})

		setJumpHistoryRows(outliner, tasks, "target", "survivor")
		outliner.FinishOutlinerRelocation(capture, true)
		outliner.JumpBackward()
		requireCurrentJumpTask(t, outliner, taskref.TaskIdentity{ProjectID: "alpha", TaskID: "survivor"})
	})
}

func TestOutlinerRelocationUsesProjectQualifiedTaskAnchors(t *testing.T) {
	alpha := primaryRowsProject("alpha", &models.Task{ID: "same", Title: "alpha same"}, &models.Task{ID: "target", Title: "target"})
	beta := primaryRowsProject("beta", &models.Task{ID: "same", Title: "beta same"})
	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.PrimaryMode = chrome.ModeAll
	outliner.Rows = []OutlinerRow{
		{Task: alpha.Tasks[0], ProjectID: alpha.ID},
		{Task: alpha.Tasks[1], ProjectID: alpha.ID},
		{Task: beta.Tasks[0], ProjectID: beta.ID},
	}
	outliner.Cursor = 1
	target := taskref.TaskIdentity{ProjectID: alpha.ID, TaskID: "target"}
	capture := outliner.CaptureOutlinerRelocation([]taskref.TaskIdentity{target})

	outliner.Rows = []OutlinerRow{
		{Task: alpha.Tasks[1], ProjectID: alpha.ID},
		{Task: alpha.Tasks[0], ProjectID: alpha.ID},
		{Task: beta.Tasks[0], ProjectID: beta.ID},
	}
	outliner.FinishOutlinerRelocation(capture, true)
	outliner.JumpBackward()
	requireCurrentJumpTask(t, outliner, taskref.TaskIdentity{ProjectID: beta.ID, TaskID: "same"})
}

func TestOutlinerRelocationHistoryClearsOnProjectionChange(t *testing.T) {
	outliner, tasks := jumpHistoryFixture("before", "target", "after")
	target := taskref.TaskIdentity{ProjectID: "alpha", TaskID: "target"}
	outliner.Cursor = 1
	capture := outliner.CaptureOutlinerRelocation([]taskref.TaskIdentity{target})
	setJumpHistoryRows(outliner, tasks, "target", "before", "after")
	outliner.FinishOutlinerRelocation(capture, true)

	outliner.PrimaryMode = chrome.ModeInbox
	outliner.SyncJumpProjection()
	if len(outliner.RelocationHistory.Back) != 0 || len(outliner.RelocationHistory.forward) != 0 || outliner.RelocationHistory.Pending != nil {
		t.Fatalf("projection change retained history: %+v", outliner.RelocationHistory)
	}
}

func TestOutlinerOrdinaryMotionPreservesForwardHistory(t *testing.T) {
	outliner, tasks := jumpHistoryFixture("before", "target", "after", "tail")
	target := taskref.TaskIdentity{ProjectID: "alpha", TaskID: "target"}
	outliner.Cursor = 1
	capture := outliner.CaptureOutlinerRelocation([]taskref.TaskIdentity{target})
	setJumpHistoryRows(outliner, tasks, "target", "before", "after", "tail")
	outliner.FinishOutlinerRelocation(capture, true)
	outliner.JumpBackward()

	outliner.Cursor = 3 // Ordinary j/k-style movement must not clear Ctrl+I.
	outliner.JumpForward()
	requireCurrentJumpTask(t, outliner, target)
	outliner.JumpBackward()
	requireCurrentJumpTask(t, outliner, taskref.TaskIdentity{ProjectID: "alpha", TaskID: "tail"})
}

func TestOutlinerRelocationHistoryIgnoresNoOpAndHiddenDestinations(t *testing.T) {
	t.Run("same row", func(t *testing.T) {
		outliner, _ := jumpHistoryFixture("target", "after")
		target := taskref.TaskIdentity{ProjectID: "alpha", TaskID: "target"}
		capture := outliner.CaptureOutlinerRelocation([]taskref.TaskIdentity{target})
		outliner.FinishOutlinerRelocation(capture, true)
		if len(outliner.RelocationHistory.Back) != 0 {
			t.Fatalf("same-row mutation recorded %d entries", len(outliner.RelocationHistory.Back))
		}
	})

	t.Run("hidden", func(t *testing.T) {
		outliner, tasks := jumpHistoryFixture("before", "target", "after")
		target := taskref.TaskIdentity{ProjectID: "alpha", TaskID: "target"}
		outliner.Cursor = 1
		capture := outliner.CaptureOutlinerRelocation([]taskref.TaskIdentity{target})
		setJumpHistoryRows(outliner, tasks, "before", "after")
		outliner.FinishOutlinerRelocation(capture, true)
		if len(outliner.RelocationHistory.Back) != 0 {
			t.Fatalf("hidden destination recorded %d entries", len(outliner.RelocationHistory.Back))
		}
	})
}

func TestOutlinerRelocationHistoryIsBounded(t *testing.T) {
	var stack []OutlinerJumpPosition
	for i := 0; i < outlinerJumpHistoryLimit+25; i++ {
		stack = appendBoundedJump(stack, OutlinerJumpPosition{RowIndex: i})
	}
	if len(stack) != outlinerJumpHistoryLimit {
		t.Fatalf("history length = %d, want %d", len(stack), outlinerJumpHistoryLimit)
	}
	if got := stack[0].RowIndex; got != 25 {
		t.Fatalf("oldest retained row = %d, want 25", got)
	}
}

func TestRefocusRestoredTaskSeatsTheCursorAndRecordsTheVacatedOrigin(t *testing.T) {
	outliner, _ := jumpHistoryFixture("before", "restored", "after")
	outliner.Cursor = 0
	restored := taskref.TaskIdentity{ProjectID: "alpha", TaskID: "restored"}

	_, seated, originRecorded := outliner.RefocusRestoredTask(restored)
	if !seated || !originRecorded {
		t.Fatalf("refocus = seated %v origin %v, want true/true", seated, originRecorded)
	}
	requireCurrentJumpTask(t, outliner, restored)
	if len(outliner.RelocationHistory.Back) != 1 {
		t.Fatalf("jump history = %d entries, want 1", len(outliner.RelocationHistory.Back))
	}

	outliner.JumpBackward()
	requireCurrentJumpTask(t, outliner, taskref.TaskIdentity{ProjectID: "alpha", TaskID: "before"})
}

func TestRefocusRestoredTaskAdvertisesNoOriginWhenTheCursorIsAlreadyThere(t *testing.T) {
	outliner, _ := jumpHistoryFixture("restored", "after")
	outliner.Cursor = 0

	_, seated, originRecorded := outliner.RefocusRestoredTask(taskref.TaskIdentity{ProjectID: "alpha", TaskID: "restored"})
	if !seated || originRecorded {
		t.Fatalf("refocus = seated %v origin %v, want true/false", seated, originRecorded)
	}
	if len(outliner.RelocationHistory.Back) != 0 {
		t.Fatalf("unmoved cursor recorded %d jump entries", len(outliner.RelocationHistory.Back))
	}
}

func TestRefocusRestoredTaskLeavesAnUndrawnTaskAndAVisualSelectionAlone(t *testing.T) {
	t.Run("not drawn", func(t *testing.T) {
		outliner, _ := jumpHistoryFixture("before", "after")
		outliner.Cursor = 1
		_, seated, originRecorded := outliner.RefocusRestoredTask(taskref.TaskIdentity{ProjectID: "alpha", TaskID: "restored"})
		if seated || originRecorded {
			t.Fatalf("refocus = seated %v origin %v, want false/false", seated, originRecorded)
		}
		if outliner.Cursor != 1 {
			t.Fatalf("cursor = %d, want 1", outliner.Cursor)
		}
	})

	t.Run("visual selection", func(t *testing.T) {
		outliner, _ := jumpHistoryFixture("before", "restored")
		outliner.Cursor = 0
		outliner.StartRangeSelection(taskref.TaskIdentity{ProjectID: "alpha", TaskID: "before"})
		_, seated, _ := outliner.RefocusRestoredTask(taskref.TaskIdentity{ProjectID: "alpha", TaskID: "restored"})
		if seated {
			t.Fatal("refocus moved the cursor out of a standing visual selection")
		}
		if outliner.Cursor != 0 {
			t.Fatalf("cursor = %d, want 0", outliner.Cursor)
		}
	})

	t.Run("unaddressed", func(t *testing.T) {
		outliner, _ := jumpHistoryFixture("before", "after")
		outliner.Cursor = 1
		if _, seated, _ := outliner.RefocusRestoredTask(taskref.TaskIdentity{}); seated {
			t.Fatal("an unaddressed identity seated the cursor")
		}
	})
}
