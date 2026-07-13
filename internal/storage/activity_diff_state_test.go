package storage

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// A lifecycle move records exactly one canonical "state" entry — no parallel
// legacy status/section/stage diffs.
func TestDiffTaskRecordsStateChange(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask.Complete()

	diffTask(old, newTask)

	found := findActivity(newTask, "state")
	if found == nil {
		t.Fatal("expected state activity entry")
	}
	if found.OldValue != string(models.StateReady) || found.NewValue != string(models.StateDone) {
		t.Errorf("expected ready→done, got %s→%s", found.OldValue, found.NewValue)
	}
	for _, legacy := range []string{"status", "section", "stage", "active"} {
		if findActivity(newTask, legacy) != nil {
			t.Errorf("expected no legacy %q activity entry", legacy)
		}
	}
}

// Focus Queue membership is planning bookkeeping tracked outside the task (the
// legacy Active flag is retired), so it is not part of the diff surface: two
// otherwise-identical tasks must not produce any activity entry.
func TestDiffTaskIgnoresActiveFlag(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	diffTask(old, newTask)

	if len(newTask.Activity) != 0 {
		t.Errorf("expected no activity entries when Focus Queue membership is the only difference, got %d", len(newTask.Activity))
	}
}
