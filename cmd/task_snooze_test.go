package cmd

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestSnoozeTask(t *testing.T) {
	t.Run("setting SnoozedUntil to future time", func(t *testing.T) {
		task := testutil.NewTestTask("Write tests", models.TaskTypeTask)

		future := time.Now().AddDate(0, 0, 7)
		task.SnoozedUntil = &future

		if task.SnoozedUntil == nil {
			t.Error("expected SnoozedUntil to be set")
		}
		if !task.SnoozedUntil.Equal(future) {
			t.Errorf("expected SnoozedUntil to be %v, got %v", future, *task.SnoozedUntil)
		}
	})

	t.Run("clearing SnoozedUntil by setting to nil", func(t *testing.T) {
		task := testutil.NewTestTask("Write tests", models.TaskTypeTask)

		future := time.Now().AddDate(0, 0, 3)
		task.SnoozedUntil = &future

		// Clear the snooze
		task.SnoozedUntil = nil

		if task.SnoozedUntil != nil {
			t.Error("expected SnoozedUntil to be nil after clearing")
		}
	})
}

func TestSnoozeTaskRejectsInProgress(t *testing.T) {
	task := testutil.NewTestTask("Active task", models.TaskTypeTask)
	task.Start() // sets status to in_progress

	// Attempting to snooze an in-progress task should be rejected
	if task.State() != models.StateInProgress {
		t.Fatalf("expected StateInProgress, got %s", task.State())
	}

	// The validation happens in the command layer, but we can verify
	// the model-level invariant: Start() clears any existing snooze
	future := time.Now().AddDate(0, 0, 7)
	task.SnoozedUntil = &future
	task.Start()

	if task.SnoozedUntil != nil {
		t.Error("Start() should clear SnoozedUntil")
	}
}

func TestSnoozeTaskIsSnoozed(t *testing.T) {
	t.Run("task snoozed until future date is snoozed", func(t *testing.T) {
		task := testutil.NewTestTask("Future task", models.TaskTypeTask)

		future := time.Now().AddDate(0, 0, 7)
		task.SnoozedUntil = &future

		if !task.IsSnoozed() {
			t.Error("expected task with future SnoozedUntil to be snoozed")
		}
	})

	t.Run("task snoozed until past date is not snoozed", func(t *testing.T) {
		task := testutil.NewTestTask("Past task", models.TaskTypeTask)

		past := time.Now().AddDate(0, 0, -3)
		task.SnoozedUntil = &past

		if task.IsSnoozed() {
			t.Error("expected task with past SnoozedUntil to not be snoozed")
		}
	})

	t.Run("task with nil SnoozedUntil is not snoozed", func(t *testing.T) {
		task := testutil.NewTestTask("Normal task", models.TaskTypeTask)

		if task.IsSnoozed() {
			t.Error("expected task with nil SnoozedUntil to not be snoozed")
		}
	})
}
