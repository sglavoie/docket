package workflow

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestSnapshotTaskForEvent_DeepCopiesSlices(t *testing.T) {
	t.Run("DependsOn mutation does not affect snapshot", func(t *testing.T) {
		task := &models.Task{
			ID:        "task-1",
			DependsOn: []string{"dep-1", "dep-2", "dep-3"},
		}

		snap := CopyTaskState(task)

		// Verify initial state
		if len(snap.DependsOn) != 3 {
			t.Fatalf("snapshot DependsOn length = %d, want 3", len(snap.DependsOn))
		}

		// Mutate the original task's DependsOn in-place (simulating RemoveDependency
		// which uses slices.Delete under the hood)
		task.DependsOn = append(task.DependsOn[:1], task.DependsOn[2:]...)

		// Snapshot must be unaffected
		if len(snap.DependsOn) != 3 {
			t.Errorf("after mutation: snapshot DependsOn length = %d, want 3", len(snap.DependsOn))
		}
		if snap.DependsOn[0] != "dep-1" || snap.DependsOn[1] != "dep-2" || snap.DependsOn[2] != "dep-3" {
			t.Errorf("after mutation: snapshot DependsOn = %v, want [dep-1 dep-2 dep-3]", snap.DependsOn)
		}
	})

	t.Run("Weekdays mutation does not affect snapshot", func(t *testing.T) {
		task := &models.Task{
			ID: "task-2",
			Recurrence: &models.RecurrencePattern{
				Interval: 1,
				Unit:     models.RecurrenceWeekly,
				Weekdays: []int{1, 3, 5}, // Mon, Wed, Fri
			},
		}

		snap := CopyTaskState(task)

		// Verify initial state
		if len(snap.Recurrence.Weekdays) != 3 {
			t.Fatalf("snapshot Weekdays length = %d, want 3", len(snap.Recurrence.Weekdays))
		}

		// Mutate the original task's Weekdays in-place
		task.Recurrence.Weekdays = append(task.Recurrence.Weekdays[:1], task.Recurrence.Weekdays[2:]...)

		// Snapshot must be unaffected
		if len(snap.Recurrence.Weekdays) != 3 {
			t.Errorf("after mutation: snapshot Weekdays length = %d, want 3", len(snap.Recurrence.Weekdays))
		}
		if snap.Recurrence.Weekdays[0] != 1 || snap.Recurrence.Weekdays[1] != 3 || snap.Recurrence.Weekdays[2] != 5 {
			t.Errorf("after mutation: snapshot Weekdays = %v, want [1 3 5]", snap.Recurrence.Weekdays)
		}
	})

	t.Run("Tags mutation does not affect snapshot", func(t *testing.T) {
		task := &models.Task{
			ID:   "task-3",
			Tags: []string{"urgent", "backend", "api"},
		}

		snap := CopyTaskState(task)

		// Mutate the original
		task.Tags[1] = "frontend"

		if snap.Tags[1] != "backend" {
			t.Errorf("snapshot Tags[1] = %q, want %q", snap.Tags[1], "backend")
		}
	})

	t.Run("DueDate mutation does not affect snapshot", func(t *testing.T) {
		due := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
		task := &models.Task{
			ID:      "task-4",
			DueDate: &due,
		}

		snap := CopyTaskState(task)

		// Replace original due date
		newDue := time.Date(2025, 12, 25, 0, 0, 0, 0, time.UTC)
		task.DueDate = &newDue

		if !snap.DueDate.Equal(due) {
			t.Errorf("snapshot DueDate = %v, want %v", snap.DueDate, due)
		}
	})

	t.Run("nil DependsOn produces empty snapshot", func(t *testing.T) {
		task := &models.Task{
			ID:        "task-5",
			DependsOn: nil,
		}

		snap := CopyTaskState(task)

		// append([]string(nil), nil...) returns nil, which is fine
		if snap.DependsOn != nil {
			t.Errorf("snapshot DependsOn = %v, want nil", snap.DependsOn)
		}
	})

	t.Run("empty DependsOn produces empty snapshot", func(t *testing.T) {
		task := &models.Task{
			ID:        "task-6",
			DependsOn: []string{},
		}

		snap := CopyTaskState(task)

		// append([]string(nil), []string{}...) returns nil
		if len(snap.DependsOn) != 0 {
			t.Errorf("snapshot DependsOn length = %d, want 0", len(snap.DependsOn))
		}
	})
}

func TestTaskStateChanged_DependsOn(t *testing.T) {
	t.Run("different DependsOn length returns true", func(t *testing.T) {
		prev := &models.Task{ID: "t", DependsOn: []string{"a"}}
		current := &models.Task{ID: "t", DependsOn: []string{"a", "b"}}

		if !TaskStateChanged(current, prev) {
			t.Errorf("TaskStateChanged with differing DependsOn length = false, want true")
		}
	})

	t.Run("same length but different IDs returns true", func(t *testing.T) {
		prev := &models.Task{ID: "t", DependsOn: []string{"a", "b"}}
		current := &models.Task{ID: "t", DependsOn: []string{"a", "c"}}

		if !TaskStateChanged(current, prev) {
			t.Errorf("TaskStateChanged with differing DependsOn IDs = false, want true")
		}
	})

	t.Run("identical DependsOn returns false", func(t *testing.T) {
		prev := &models.Task{ID: "t", DependsOn: []string{"a", "b"}}
		current := &models.Task{ID: "t", DependsOn: []string{"a", "b"}}

		if TaskStateChanged(current, prev) {
			t.Errorf("TaskStateChanged with identical DependsOn = true, want false")
		}
	})
}
