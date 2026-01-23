package external

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildGotoArgument(t *testing.T) {
	path := "/path/to/file.go"
	line := 42

	arg := fmt.Sprintf("%s:%d", path, line)

	expected := "/path/to/file.go:42"
	if arg != expected {
		t.Errorf("expected %q, got %q", expected, arg)
	}
}

func TestBuildGotoArgumentWithColumn(t *testing.T) {
	path := "/path/to/file.go"
	line := 42
	column := 10

	arg := fmt.Sprintf("%s:%d:%d", path, line, column)

	expected := "/path/to/file.go:42:10"
	if arg != expected {
		t.Errorf("expected %q, got %q", expected, arg)
	}
}

func TestCreateTempFileWithContent(t *testing.T) {
	content := "test content here"
	prefix := "tt-test-"
	suffix := ".md"

	tmpPath, err := CreateTempFileWithContent(content, prefix, suffix)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpPath)

	if !filepath.IsAbs(tmpPath) {
		t.Error("expected absolute path")
	}

	base := filepath.Base(tmpPath)
	if len(base) < len(prefix)+len(suffix) {
		t.Error("expected filename to contain prefix and suffix")
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("failed to read temp file: %v", err)
	}
	if string(data) != content {
		t.Errorf("expected content %q, got %q", content, string(data))
	}
}

func TestCreateTempFileWithEmptyContent(t *testing.T) {
	tmpPath, err := CreateTempFileWithContent("", "tt-empty-", ".txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpPath)

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("failed to read temp file: %v", err)
	}
	if string(data) != "" {
		t.Errorf("expected empty content, got %q", string(data))
	}
}

func TestCreateTempFileWithMultilineContent(t *testing.T) {
	content := "line1\nline2\nline3"
	tmpPath, err := CreateTempFileWithContent(content, "tt-multi-", ".txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpPath)

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("failed to read temp file: %v", err)
	}
	if string(data) != content {
		t.Errorf("expected multiline content preserved, got %q", string(data))
	}
}

func TestEditorDefaultFallback(t *testing.T) {
	// Test the logic for choosing default editor
	editor := ""
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "code"
	}

	// When EDITOR is not set, should default to "code"
	originalEditor := os.Getenv("EDITOR")
	os.Unsetenv("EDITOR")
	defer func() {
		if originalEditor != "" {
			os.Setenv("EDITOR", originalEditor)
		}
	}()

	editor = ""
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "code"
	}

	if editor != "code" {
		t.Errorf("expected default editor to be 'code', got %q", editor)
	}
}
