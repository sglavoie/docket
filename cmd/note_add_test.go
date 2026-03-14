package cmd

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestNewNote_Defaults(t *testing.T) {
	n := models.NewNote("My Test Note")

	if n.Title != "My Test Note" {
		t.Errorf("expected title %q, got %q", "My Test Note", n.Title)
	}
	if n.Tags == nil || len(n.Tags) != 0 {
		t.Errorf("expected empty non-nil Tags slice, got %v", n.Tags)
	}
	if n.LinkedTasks == nil || len(n.LinkedTasks) != 0 {
		t.Errorf("expected empty non-nil LinkedTasks slice, got %v", n.LinkedTasks)
	}
	if n.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if n.ModifiedAt.IsZero() {
		t.Error("expected non-zero ModifiedAt")
	}
	if n.Filename == "" {
		t.Error("expected non-empty Filename")
	}
	if len(n.Filename) < 4 || n.Filename[len(n.Filename)-3:] != ".md" {
		t.Errorf("expected Filename ending with .md, got %q", n.Filename)
	}
}

func TestNewNote_FilenameFormat(t *testing.T) {
	n := models.NewNote("Weekly Review")

	// Filename should start with a date pattern like "2025-01-28-150405-"
	if len(n.Filename) < 18 {
		t.Fatalf("filename too short: %q", n.Filename)
	}
	datePrefix := n.Filename[:10]
	if datePrefix[4] != '-' || datePrefix[7] != '-' {
		t.Errorf("expected date prefix in YYYY-MM-DD format, got %q", datePrefix)
	}

	// Should contain slugified title
	if !containsSubstring(n.Filename, "weekly-review") {
		t.Errorf("expected filename to contain slugified title, got %q", n.Filename)
	}
}

func TestNoteAddTag(t *testing.T) {
	n := models.NewNote("Tag Test")

	n.AddTag("urgent")
	if len(n.Tags) != 1 || n.Tags[0] != "urgent" {
		t.Errorf("expected [urgent], got %v", n.Tags)
	}

	// Duplicate should be ignored.
	n.AddTag("urgent")
	if len(n.Tags) != 1 {
		t.Errorf("expected 1 tag after duplicate add, got %d", len(n.Tags))
	}

	// Empty string should be ignored.
	n.AddTag("")
	if len(n.Tags) != 1 {
		t.Errorf("expected 1 tag after empty add, got %d", len(n.Tags))
	}

	// Whitespace-only should be ignored.
	n.AddTag("   ")
	if len(n.Tags) != 1 {
		t.Errorf("expected 1 tag after whitespace add, got %d", len(n.Tags))
	}
}

func TestNoteSetTags(t *testing.T) {
	n := models.NewNote("SetTags Test")

	n.AddTag("old")
	n.SetTags([]string{"alpha", "beta"})
	if len(n.Tags) != 2 || n.Tags[0] != "alpha" || n.Tags[1] != "beta" {
		t.Errorf("expected [alpha beta], got %v", n.Tags)
	}

	// Duplicates and empty strings should be filtered.
	n.SetTags([]string{"x", "", "x", "y", "  ", "y"})
	if len(n.Tags) != 2 || n.Tags[0] != "x" || n.Tags[1] != "y" {
		t.Errorf("expected [x y] after filtering, got %v", n.Tags)
	}

	// Setting empty list should result in empty non-nil slice.
	n.SetTags([]string{})
	if n.Tags == nil || len(n.Tags) != 0 {
		t.Errorf("expected empty non-nil slice, got %v", n.Tags)
	}
}

func TestNoteLinkedTasks(t *testing.T) {
	n := models.NewNote("Link Test")
	taskA := "aaaa-1111"
	taskB := "bbbb-2222"

	// AddLink and HasLink
	n.AddLink(taskA)
	if !n.HasLink(taskA) {
		t.Error("expected HasLink to return true for added task")
	}
	if n.HasLink(taskB) {
		t.Error("expected HasLink to return false for non-added task")
	}

	// Duplicate AddLink should be ignored.
	n.AddLink(taskA)
	if len(n.LinkedTasks) != 1 {
		t.Errorf("expected 1 linked task after duplicate, got %d", len(n.LinkedTasks))
	}

	// Add second link, then remove first.
	n.AddLink(taskB)
	if len(n.LinkedTasks) != 2 {
		t.Errorf("expected 2 linked tasks, got %d", len(n.LinkedTasks))
	}

	n.RemoveLink(taskA)
	if n.HasLink(taskA) {
		t.Error("expected taskA to be removed")
	}
	if !n.HasLink(taskB) {
		t.Error("expected taskB to still be present")
	}

	// Removing a non-existent link should be a no-op.
	n.RemoveLink("nonexistent")
	if len(n.LinkedTasks) != 1 {
		t.Errorf("expected 1 linked task after no-op remove, got %d", len(n.LinkedTasks))
	}
}

func TestTitleFromFilename(t *testing.T) {
	got := models.TitleFromFilename("2025-01-28-150405-my-note.md")
	if got != "my note" {
		t.Errorf("expected %q, got %q", "my note", got)
	}

	// Without date prefix should still strip .md and replace hyphens.
	got = models.TitleFromFilename("short.md")
	if got != "short" {
		t.Errorf("expected %q, got %q", "short", got)
	}
}

func TestGenerateNoteFilename(t *testing.T) {
	fixedTime := time.Date(2025, 3, 15, 9, 30, 45, 0, time.UTC)
	got := models.GenerateNoteFilename("My Great Note", fixedTime)
	expected := "2025-03-15-093045-my-great-note.md"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

// containsSubstring checks if s contains substr.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
