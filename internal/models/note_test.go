package models

import (
	"strings"
	"testing"
	"time"
)

// TestGenerateFilenameCollision verifies that two calls with the same title
// and identical time.Time values produce different filenames due to the random suffix.
func TestGenerateFilenameCollision(t *testing.T) {
	now := time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC)
	title := "My Note"

	f1 := generateFilename(title, now)
	f2 := generateFilename(title, now)

	if f1 == f2 {
		t.Errorf("generateFilename produced identical filenames for same time: %q", f1)
	}
}

// TestGenerateFilenameFormat verifies the new filename format is valid.
func TestGenerateFilenameFormat(t *testing.T) {
	now := time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC)
	name := generateFilename("Hello World", now)

	if !strings.HasPrefix(name, "2024-01-15-143045_") {
		t.Errorf("filename %q does not start with expected date-time prefix", name)
	}
	if !strings.HasSuffix(name, ".md") {
		t.Errorf("filename %q does not end with .md", name)
	}
	// Verify 8-char hex suffix: "2024-01-15-143045_xxxxxxxx-slug.md"
	rest := strings.TrimPrefix(name, "2024-01-15-143045_")
	if len(rest) < 9 || rest[8] != '-' {
		t.Errorf("filename %q: expected 8-char hex suffix before slug dash, got %q", name, rest)
	}
}

// TestTitleFromFilenameNewFormat verifies TitleFromFilename handles the new
// format (with random suffix) correctly.
func TestTitleFromFilenameNewFormat(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		// New format (8-hex): 2006-01-02-150405_xxxxxxxx-slug.md
		{"2024-01-15-143045_ab12cd34-my-note.md", "my note"},
		{"2024-01-15-143045_ff001122-hello-world.md", "hello world"},
		// Old format (4-hex): 2006-01-02-150405_xxxx-slug.md
		{"2024-01-15-143045_ab12-my-note.md", "my note"},
		{"2024-01-15-143045_ff00-hello-world.md", "hello world"},
		// Legacy format: 2006-01-02-150405-slug.md
		{"2024-01-15-143045-my-note.md", "my note"},
		{"2024-01-15-143045-hello-world.md", "hello world"},
		// No date prefix
		{"just-a-file.md", "just a file"},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			got := TitleFromFilename(tc.filename)
			if got != tc.want {
				t.Errorf("TitleFromFilename(%q) = %q, want %q", tc.filename, got, tc.want)
			}
		})
	}
}

// TestSlugifyMultiByteUTF8Truncation verifies that slugify does not split
// multi-byte UTF-8 characters when truncating to 60 characters.
func TestSlugifyMultiByteUTF8Truncation(t *testing.T) {
	// Create a title with 70 CJK characters (each 3 bytes in UTF-8).
	// Byte-level truncation at 60 bytes would split the 20th character.
	// Rune-level truncation at 60 runes preserves all characters.
	title := strings.Repeat("\u4e16", 70) // 70 x "世" (U+4E16, 3 bytes each)
	slug := slugify(title)

	// The slug should be valid UTF-8 (no split runes)
	for i, r := range slug {
		if r == '\uFFFD' {
			t.Errorf("slugify produced invalid UTF-8 at byte %d (replacement character found)", i)
		}
	}

	// Should be truncated to 60 runes
	runes := []rune(slug)
	if len(runes) > 60 {
		t.Errorf("slug has %d runes, want <= 60", len(runes))
	}
}

// TestSlugifyASCIIStillTruncates verifies that ASCII slugs are still truncated.
func TestSlugifyASCIIStillTruncates(t *testing.T) {
	title := strings.Repeat("a", 100)
	slug := slugify(title)
	if len(slug) > 60 {
		t.Errorf("slug length = %d, want <= 60", len(slug))
	}
}

// TestNoteLinkRejectsEmptyTaskID verifies that AddLink/HasLink/RemoveLink all
// guard against empty task IDs. An empty ID would otherwise silently pollute
// LinkedTasks and cause every empty-string lookup to match (mirrors the
// AddDependency/AddRelatedTo empty-ID guards).
func TestNoteLinkRejectsEmptyTaskID(t *testing.T) {
	n := NewNote("notes")

	n.AddLink("")
	if len(n.LinkedTasks) != 0 {
		t.Errorf("AddLink(\"\") should be a no-op, LinkedTasks = %v", n.LinkedTasks)
	}

	// Simulate a corrupt entry sneaking in (e.g., from a malformed file).
	n.LinkedTasks = []string{"", "real-id"}
	if n.HasLink("") {
		t.Error("HasLink(\"\") should return false even if LinkedTasks contains an empty string")
	}
	if !n.HasLink("real-id") {
		t.Error("HasLink should still match real IDs")
	}

	beforeLen := len(n.LinkedTasks)
	n.RemoveLink("")
	if len(n.LinkedTasks) != beforeLen {
		t.Errorf("RemoveLink(\"\") should not modify LinkedTasks; before=%d after=%d", beforeLen, len(n.LinkedTasks))
	}
}

// TestNoteTagsStripLeadingHash verifies that Note.AddTag and Note.SetTags strip
// a leading '#' so callers passing "#backend" do not create a duplicate of the
// bare "backend" tag (rendering would otherwise produce "##backend").
func TestNoteTagsStripLeadingHash(t *testing.T) {
	n := NewNote("hash test")

	n.AddTag("#backend")
	n.AddTag("backend")
	if len(n.Tags) != 1 || n.Tags[0] != "backend" {
		t.Errorf("AddTag should normalize '#backend' and dedupe with 'backend'; got %v", n.Tags)
	}

	n.SetTags([]string{"#foo", "foo", "  #bar  ", "bar", ""})
	if len(n.Tags) != 2 || n.Tags[0] != "foo" || n.Tags[1] != "bar" {
		t.Errorf("SetTags should normalize leading '#' and dedupe; got %v", n.Tags)
	}
}
