package models

import "testing"

// TestTypeChangeClearsAttrs verifies that Promote, Demote, and CycleLeafType
// each clear type-specific attribute pointers from the previous type.
func TestTypeChangeClearsAttrs(t *testing.T) {
	t.Run("Promote clears leaf attrs", func(t *testing.T) {
		task, err := NewTask("t1", 1, TaskTypeTask, "Task")
		if err != nil {
			t.Fatal(err)
		}
		task.TaskAttrs = &TaskAttrs{Effort: EffortSmall}

		if err := task.Promote(); err != nil {
			t.Fatal(err)
		}
		if task.Type != TaskTypeStory {
			t.Fatalf("expected story after Promote, got %s", task.Type)
		}
		if task.TaskAttrs != nil {
			t.Error("TaskAttrs should be nil after Promote to story")
		}
	})

	t.Run("Demote clears epic attrs", func(t *testing.T) {
		epic, err := NewTask("e1", 1, TaskTypeEpic, "Epic")
		if err != nil {
			t.Fatal(err)
		}
		epic.EpicAttrs = &EpicAttrs{Milestone: "v2"}

		if err := epic.Demote(); err != nil {
			t.Fatal(err)
		}
		if epic.Type != TaskTypeStory {
			t.Fatalf("expected story after Demote, got %s", epic.Type)
		}
		if epic.EpicAttrs != nil {
			t.Error("EpicAttrs should be nil after Demote to story")
		}
	})

	t.Run("CycleLeafType clears previous leaf attrs", func(t *testing.T) {
		task, err := NewTask("t1", 1, TaskTypeTask, "Task")
		if err != nil {
			t.Fatal(err)
		}
		task.TaskAttrs = &TaskAttrs{WorkType: WorkTypeDevelopment}

		if err := task.CycleLeafType(false); err != nil {
			t.Fatal(err)
		}
		if task.Type != TaskTypeBug {
			t.Fatalf("expected bug after CycleLeafType, got %s", task.Type)
		}
		if task.TaskAttrs != nil {
			t.Error("TaskAttrs should be nil after cycling to bug")
		}
	})
}
