package models

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeTaskTagsTreatsFormerMarkerSpellingsAsIndependentTags(t *testing.T) {
	got, err := NormalizeTaskTags([]string{"backend", "#type:BUG", "bug", "idea", "bug"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"backend", "type-bug", "bug", "idea"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tags = %v, want %v", got, want)
	}

	task, err := NewGenericTask("t", 1, "Ordinary tags")
	if err != nil {
		t.Fatal(err)
	}
	for _, tag := range []string{"epic", "story", "bug", "feature", "idea", "type:bug"} {
		task.AddTag(tag)
	}
	if !reflect.DeepEqual(task.Tags, []string{"epic", "story", "bug", "feature", "idea", "type-bug"}) {
		t.Fatalf("ordinary tags = %v", task.Tags)
	}
}

func TestAddTag(t *testing.T) {
	task, err := NewGenericTask("t1", 1, "Test")
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
	task, err := NewGenericTask("t1", 1, "Test")
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
	task, err := NewGenericTask("t1", 1, "Test")
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
	task, err := NewGenericTask("t1", 1, "Test")
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
	task, err := NewGenericTask("t1", 1, "Test")
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
		{"no ascii alphanumeric", "!!!"},
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
	task, err := NewGenericTask("t1", 1, "Test")
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
	task, err := NewGenericTask("t1", 1, "Test")
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
	task, err := NewGenericTask("t1", 1, "Test")
	if err != nil {
		t.Fatal(err)
	}

	task.AddTag("backend")
	if !task.HasTag("  backend  ") {
		t.Error("expected HasTag to normalize whitespace and find 'backend'")
	}
}

func TestHasTag(t *testing.T) {
	task, err := NewGenericTask("t1", 1, "Test")
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

// TestAddTagStripsLeadingHash verifies that a leading '#' is removed during
// normalization. Tag rendering always prepends '#' itself; without this strip,
// CLI input like `--tag "#backend"` would render as "##backend" — diverging
// from the TUI tag adder (commit 086f1321) which already strips the prefix.
func TestAddTagStripsLeadingHash(t *testing.T) {
	task, err := NewGenericTask("t1", 1, "Test")
	if err != nil {
		t.Fatal(err)
	}

	task.AddTag("#backend")
	if !task.HasTag("backend") {
		t.Error("expected AddTag to strip leading '#' and store 'backend'")
	}
	if len(task.Tags) != 1 || task.Tags[0] != "backend" {
		t.Errorf("expected stored tag 'backend', got %v", task.Tags)
	}

	// Adding the unprefixed version should be a no-op (dedup).
	task.AddTag("backend")
	if len(task.Tags) != 1 {
		t.Errorf("expected 1 tag after adding unprefixed duplicate, got %d", len(task.Tags))
	}

	// Whitespace around the '#' should also be normalized.
	task.AddTag("  #frontend  ")
	if !task.HasTag("frontend") {
		t.Error("expected AddTag to normalize whitespace + leading '#'")
	}

	// HasTag and RemoveTag should accept either form.
	if !task.HasTag("#backend") {
		t.Error("expected HasTag to normalize leading '#'")
	}
	task.RemoveTag("#backend")
	if task.HasTag("backend") {
		t.Error("expected RemoveTag to normalize leading '#' and remove 'backend'")
	}
}

// TestAddTagRejectsHashOnly verifies that a tag consisting only of '#' (or
// whitespace + '#') is rejected, since stripping the prefix leaves an empty
// string.
func TestAddTagRejectsHashOnly(t *testing.T) {
	task, err := NewGenericTask("t1", 1, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.AddTag("#")
	task.AddTag("  #  ")
	if len(task.Tags) != 0 {
		t.Errorf("expected no tags, got %v", task.Tags)
	}
}

// TestHasTagEmptyReturnsFalse verifies that HasTag rejects empty/whitespace
// inputs even if a malformed empty tag somehow ended up in Tags (e.g. via
// direct field assignment bypassing AddTag validation). Without this guard,
// HasTag("") could match an empty entry and a missing/unset filter argument
// would treat every task as tagged.
func TestHasTagEmptyReturnsFalse(t *testing.T) {
	task, err := NewGenericTask("t", 1, "Test")
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a corrupt empty tag that bypassed AddTag.
	task.Tags = []string{"", "backend"}
	if task.HasTag("") {
		t.Error("HasTag(\"\") should return false even if Tags contains an empty string")
	}
	if task.HasTag("   ") {
		t.Error("HasTag(\"   \") should normalize to empty and return false")
	}
	if !task.HasTag("backend") {
		t.Error("HasTag should still match real tags when an empty entry is present")
	}
}
