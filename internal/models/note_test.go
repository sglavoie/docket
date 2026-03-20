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
}

// TestTitleFromFilenameNewFormat verifies TitleFromFilename handles the new
// format (with random suffix) correctly.
func TestTitleFromFilenameNewFormat(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		// New format: 2006-01-02-150405_xxxx-slug.md
		{"2024-01-15-143045_ab12-my-note.md", "my note"},
		{"2024-01-15-143045_ff00-hello-world.md", "hello world"},
		// Old format: 2006-01-02-150405-slug.md
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
