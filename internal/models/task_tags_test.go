package models

import "testing"

func TestAddTag(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	task.AddTag("backend")
	if !task.HasTag("backend") {
		t.Error("expected task to have tag 'backend'")
	}
	if len(task.Tags) != 1 {
		t.Errorf("expected 1 tag, got %d", len(task.Tags))
	}

	// Adding duplicate should be a no-op
	task.AddTag("backend")
	if len(task.Tags) != 1 {
		t.Errorf("expected 1 tag after duplicate add, got %d", len(task.Tags))
	}
}

func TestAddMultipleTags(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.AddTag("backend")
	task.AddTag("frontend")
	task.AddTag("urgent")

	if len(task.Tags) != 3 {
		t.Errorf("expected 3 tags, got %d", len(task.Tags))
	}
	for _, tag := range []string{"backend", "frontend", "urgent"} {
		if !task.HasTag(tag) {
			t.Errorf("expected task to have tag %q", tag)
		}
	}
}

func TestRemoveTag(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.AddTag("backend")
	task.AddTag("frontend")

	task.RemoveTag("backend")
	if task.HasTag("backend") {
		t.Error("expected 'backend' tag to be removed")
	}
	if !task.HasTag("frontend") {
		t.Error("expected 'frontend' tag to remain")
	}
	if len(task.Tags) != 1 {
		t.Errorf("expected 1 tag, got %d", len(task.Tags))
	}
}

func TestRemoveNonexistentTag(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.AddTag("backend")
	initialUpdatedAt := task.UpdatedAt

	task.RemoveTag("nonexistent")
	if len(task.Tags) != 1 {
		t.Errorf("expected 1 tag, got %d", len(task.Tags))
	}
	// UpdatedAt should not change when removing a tag that doesn't exist
	if !task.UpdatedAt.Equal(initialUpdatedAt) {
		t.Error("expected UpdatedAt to not change when removing nonexistent tag")
	}
}

func TestHasTag(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	if task.HasTag("anything") {
		t.Error("expected HasTag to return false for empty tags")
	}

	task.AddTag("backend")
	if !task.HasTag("backend") {
		t.Error("expected HasTag to return true")
	}
	if task.HasTag("frontend") {
		t.Error("expected HasTag to return false for missing tag")
	}
}
