package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseTasksMarkdown_StripsLeadingBOM verifies that a UTF-8 BOM at the
// start of the file does not block the project-title regex from matching.
// Regression for a data-loss bug where files saved with a BOM (common on
// Windows) lost their project name silently because "# Project: ..."
// did not match `^# Project: ...`.
func TestParseTasksMarkdown_StripsLeadingBOM(t *testing.T) {
	//  is the UTF-8 byte-order mark.
	content := "\uFEFF# Project: BOM Test\n\n## Task: Hello\n<!-- id: 11111111-2222-3333-4444-555555555555, status: todo, priority: 3 -->\n\nA line of description.\n"

	result, err := ParseTasksMarkdownDocument(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ProjectName != "BOM Test" {
		t.Errorf("expected project name 'BOM Test', got %q", result.ProjectName)
	}

	if len(result.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(result.Tasks))
	}

	task := result.Tasks[0]
	if task.Title != "Hello" {
		t.Errorf("expected title 'Hello', got %q", task.Title)
	}
	if task.ID != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("expected ID preserved, got %q", task.ID)
	}
	if task.Description != "A line of description." {
		t.Errorf("expected description preserved, got %q", task.Description)
	}
}

// TestParseProjectNameFromFile_StripsLeadingBOM verifies the same BOM fix in
// the on-disk project-name reader path.
func TestParseProjectNameFromFile_StripsLeadingBOM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.md")
	content := "\uFEFF# Project: With BOM\n\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	name, err := ParseProjectNameFromFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "With BOM" {
		t.Errorf("expected 'With BOM', got %q", name)
	}
}

// TestParseProjectNameFromFile_HandlesLongLines verifies that
// ParseProjectNameFromFile uses the same large scanner buffer as the main
// parser. Regression for a bug where files containing any line above the
// default 64KB bufio limit before the project heading caused the scan to
// abort silently and the function to report "project name not found",
// effectively losing the project's name on load.
func TestParseProjectNameFromFile_HandlesLongLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.md")
	// A 200KB single-line HTML comment preceding the project heading would
	// blow the default 64KB scanner buffer. The buffer increase added to
	// ParseProjectNameFromFile must accept this gracefully.
	longComment := "<!-- " + strings.Repeat("x", 200*1024) + " -->\n"
	content := longComment + "# Project: Long Lines\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	name, err := ParseProjectNameFromFile(path)
	if err != nil {
		t.Fatalf("unexpected error parsing long-line file: %v", err)
	}
	if name != "Long Lines" {
		t.Errorf("expected 'Long Lines', got %q", name)
	}
}
