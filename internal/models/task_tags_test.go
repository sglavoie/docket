package models

import (
	"strings"
	"testing"
)

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

func TestValidateTag(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		wantErr bool
	}{
		{"valid tag", "backend", false},
		{"valid with hyphens", "high-priority", false},
		{"empty string", "", true},
		{"whitespace only", "   ", true},
		{"leading whitespace", " backend", true},
		{"trailing whitespace", "backend ", true},
		{"leading and trailing whitespace", " backend ", true},
		{"contains comma", "a,b", true},
		{"contains semicolon", "a;b", true},
		{"contains newline", "a\nb", true},
		{"too long", strings.Repeat("a", MaxTagLength+1), true},
		{"at max length", strings.Repeat("a", MaxTagLength), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTag(tt.tag)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTag(%q) error = %v, wantErr %v", tt.tag, err, tt.wantErr)
			}
		})
	}
}

func TestAddTagRejectsInvalid(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		tag  string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
		{"too long", strings.Repeat("a", MaxTagLength+1)},
		{"contains comma", "has,comma"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := len(task.Tags)
			task.AddTag(tt.tag)
			if len(task.Tags) != before {
				t.Errorf("AddTag(%q) should have been rejected but tags changed: %v", tt.tag, task.Tags)
			}
		})
	}
}

func TestAddTagNormalizesWhitespace(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	// Adding a tag with leading/trailing whitespace should store trimmed version
	task.AddTag("  backend  ")
	if !task.HasTag("backend") {
		t.Error("expected AddTag to normalize whitespace and store 'backend'")
	}
	if len(task.Tags) != 1 {
		t.Errorf("expected 1 tag, got %d", len(task.Tags))
	}
	if task.Tags[0] != "backend" {
		t.Errorf("expected stored tag to be 'backend', got %q", task.Tags[0])
	}

	// Adding trimmed version of same tag should be a no-op (dedup)
	task.AddTag("backend")
	if len(task.Tags) != 1 {
		t.Errorf("expected 1 tag after adding trimmed duplicate, got %d", len(task.Tags))
	}

	// Adding another whitespace-padded version should also be a no-op
	task.AddTag(" backend ")
	if len(task.Tags) != 1 {
		t.Errorf("expected 1 tag after adding padded duplicate, got %d", len(task.Tags))
	}
}

func TestRemoveTagNormalizesWhitespace(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	task.AddTag("backend")
	// Removing with whitespace should still find and remove the trimmed version
	task.RemoveTag("  backend  ")
	if task.HasTag("backend") {
		t.Error("expected RemoveTag to normalize whitespace and remove 'backend'")
	}
	if len(task.Tags) != 0 {
		t.Errorf("expected 0 tags, got %d", len(task.Tags))
	}
}

func TestHasTagNormalizesWhitespace(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	task.AddTag("backend")
	if !task.HasTag("  backend  ") {
		t.Error("expected HasTag to normalize whitespace and find 'backend'")
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
