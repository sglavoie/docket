package workflow

import (
	"sync"
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

// TestCopyTaskState_DoesNotAliasParentOrChildren verifies that the snapshot
// does not retain pointers to the live Parent or Children of the source task.
// Aliasing Parent/Children means a later mutation to the hierarchy (re-parenting
// or rewriting Children via index) silently rewrites the snapshot's view of
// those fields, defeating the purpose of capturing a point-in-time state.
// Hierarchical triggers operate on the live task, never on the snapshot, so the
// snapshot has no business carrying these pointers at all.
func TestCopyTaskState_DoesNotAliasParentOrChildren(t *testing.T) {
	t.Run("Parent pointer is nil in snapshot", func(t *testing.T) {
		parent := &models.Task{ID: "p", WorkflowState: models.StateReady}
		child := &models.Task{ID: "c", WorkflowState: models.StateInProgress, Parent: parent}

		snap := CopyTaskState(child)
		if snap.Parent != nil {
			t.Errorf("snapshot.Parent = %p, want nil (snapshot must not retain live parent)", snap.Parent)
		}
	})

	t.Run("Children slice is nil in snapshot", func(t *testing.T) {
		c1 := &models.Task{ID: "c1"}
		c2 := &models.Task{ID: "c2"}
		parent := &models.Task{ID: "p", Children: []*models.Task{c1, c2}}

		snap := CopyTaskState(parent)
		if snap.Children != nil {
			t.Errorf("snapshot.Children = %v, want nil (snapshot must not retain live children)", snap.Children)
		}
	})

	t.Run("indexed Children mutation on live task does not bleed into snapshot", func(t *testing.T) {
		c1 := &models.Task{ID: "c1"}
		c2 := &models.Task{ID: "c2"}
		parent := &models.Task{ID: "p", Children: []*models.Task{c1, c2}}

		snap := CopyTaskState(parent)

		// Indexed mutation of the live children slice. With the old aliasing
		// behavior this would also rewrite snap.Children[0].
		parent.Children[0] = &models.Task{ID: "replaced"}

		// snapshot.Children should be nil now (deliberately not copied);
		// the important property is that no live-task mutation can corrupt
		// any aliased state on the snapshot.
		if snap.Children != nil {
			t.Errorf("snapshot.Children should remain nil regardless of live mutation, got %v", snap.Children)
		}
	})
}

// TestCopyTaskState_ConcurrentReadersGetIndependentSnapshots verifies that
// concurrent calls to CopyTaskState produce independent snapshots whose
// mutable fields can be safely mutated in parallel without corrupting each
// other. This is the "parallel guard" property: each observer reads a fully
// independent state slice, so downstream mutations (e.g. via slices.Delete
// inside RemoveTag/RemoveDependency) do not race against another observer's
// view.
func TestCopyTaskState_ConcurrentReadersGetIndependentSnapshots(t *testing.T) {
	source := &models.Task{
		ID:        "t1",
		Tags:      []string{"a", "b", "c"},
		DependsOn: []string{"d1", "d2"},
		Recurrence: &models.RecurrencePattern{
			Interval: 1,
			Unit:     models.RecurrenceWeekly,
			Weekdays: []int{1, 3, 5},
		},
	}

	const N = 20
	snaps := make([]*models.Task, N)
	var wg sync.WaitGroup
	for i := range N {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			snaps[idx] = CopyTaskState(source)
		}(i)
	}
	wg.Wait()

	// Mutate each snapshot in parallel. If any two snapshots aliased the
	// same backing array for Tags / DependsOn / Weekdays, this would race
	// or produce inconsistent results.
	wg = sync.WaitGroup{}
	for i := range N {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			s := snaps[idx]
			s.Tags[0] = "mutated"
			s.DependsOn[0] = "mutated"
			s.Recurrence.Weekdays[0] = 99
		}(i)
	}
	wg.Wait()

	// The source must remain untouched.
	if source.Tags[0] != "a" {
		t.Errorf("source Tags[0] = %q, want %q (snapshot mutation bled into source)", source.Tags[0], "a")
	}
	if source.DependsOn[0] != "d1" {
		t.Errorf("source DependsOn[0] = %q, want %q", source.DependsOn[0], "d1")
	}
	if source.Recurrence.Weekdays[0] != 1 {
		t.Errorf("source Weekdays[0] = %d, want 1", source.Recurrence.Weekdays[0])
	}
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
