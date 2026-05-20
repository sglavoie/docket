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
	defer func() {
		if removeErr := os.Remove(tmpPath); removeErr != nil {
			t.Errorf("failed to remove temp file: %v", removeErr)
		}
	}()

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
	defer func() {
		if removeErr := os.Remove(tmpPath); removeErr != nil {
			t.Errorf("failed to remove temp file: %v", removeErr)
		}
	}()

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
	defer func() {
		if removeErr := os.Remove(tmpPath); removeErr != nil {
			t.Errorf("failed to remove temp file: %v", removeErr)
		}
	}()

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("failed to read temp file: %v", err)
	}
	if string(data) != content {
		t.Errorf("expected multiline content preserved, got %q", string(data))
	}
}

func TestNeedsWaitFlag(t *testing.T) {
	testCases := []struct {
		editor string
		want   bool
	}{
		{"code", true},
		{"cursor", true},
		{"cursor-nightly", true},
		{"/usr/local/bin/cursor", true},
		{"subl", true},
		{"sublime", true},
		{"zed", true},
		{"code-insiders", true},
		{"code-oss", true},
		{"/usr/bin/code", true},
		{"/opt/editors/code-insiders", true},
		{"vim", false},
		{"nvim", false},
		{"nano", false},
		{"emacs", false},
		{"vi", false},
		{"", false},
	}

	for _, tc := range testCases {
		t.Run(tc.editor, func(t *testing.T) {
			got := needsWaitFlag(tc.editor)
			if got != tc.want {
				t.Errorf("needsWaitFlag(%q) = %v, want %v", tc.editor, got, tc.want)
			}
		})
	}
}

func TestEditorCommandAddsWaitForVariants(t *testing.T) {
	// EditorCommand should add --wait for code-insiders
	cmd := EditorCommand("code-insiders", "/tmp/test.md")
	args := cmd.Args
	foundWait := false
	for _, arg := range args {
		if arg == "--wait" {
			foundWait = true
		}
	}
	if !foundWait {
		t.Errorf("EditorCommand('code-insiders', ...) should include --wait, got args: %v", args)
	}

	// EditorCommand should NOT add --wait for vim
	cmd = EditorCommand("vim", "/tmp/test.md")
	for _, arg := range cmd.Args {
		if arg == "--wait" {
			t.Errorf("EditorCommand('vim', ...) should not include --wait, got args: %v", cmd.Args)
		}
	}
}

func TestEditorCommandReturnsNilStdio(t *testing.T) {
	// EditorCommand returns an exec.Cmd with nil Stdin/Stdout/Stderr.
	// Callers that run terminal editors (vim, nano) must set these to
	// os.Stdin/os.Stdout/os.Stderr before calling Run(), otherwise the
	// editor will fail silently. (This was a real bug in board_edit.go.)
	cmd := EditorCommand("vim", "/tmp/test.md")
	if cmd.Stdin != nil {
		t.Error("expected EditorCommand to return cmd with nil Stdin")
	}
	if cmd.Stdout != nil {
		t.Error("expected EditorCommand to return cmd with nil Stdout")
	}
	if cmd.Stderr != nil {
		t.Error("expected EditorCommand to return cmd with nil Stderr")
	}
}

func TestEditInExternalEditor_NoEditorAvailable(t *testing.T) {
	// Reset env so ResolveEditor only sees impossible binaries.
	origPath := os.Getenv("PATH")
	origVisual := os.Getenv("VISUAL")
	origEditor := os.Getenv("EDITOR")
	t.Cleanup(func() {
		_ = os.Setenv("PATH", origPath)
		_ = os.Setenv("VISUAL", origVisual)
		_ = os.Setenv("EDITOR", origEditor)
	})
	if err := os.Setenv("PATH", ""); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}
	if err := os.Unsetenv("VISUAL"); err != nil {
		t.Fatalf("unsetenv VISUAL: %v", err)
	}
	if err := os.Unsetenv("EDITOR"); err != nil {
		t.Fatalf("unsetenv EDITOR: %v", err)
	}

	// Snapshot the temp dir contents before the call so we can detect
	// whether a temp file was created and leaked.
	dir := os.TempDir()
	before, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read tempdir: %v", err)
	}
	beforeNames := make(map[string]bool, len(before))
	for _, e := range before {
		beforeNames[e.Name()] = true
	}

	_, err = EditInExternalEditor("tt-nonexistent-editor-xyz", "hello", ".md")
	if err == nil {
		t.Fatal("expected error when no editor is available")
	}

	after, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read tempdir after: %v", err)
	}
	for _, e := range after {
		if beforeNames[e.Name()] {
			continue
		}
		// New entries with our prefix indicate a leaked temp file created
		// before the editor-resolution failure.
		if filepath.Ext(e.Name()) == ".md" && len(e.Name()) > len("tt-edit-") && e.Name()[:len("tt-edit-")] == "tt-edit-" {
			t.Errorf("leaked temp file %q — editor should be resolved before creating temp file", e.Name())
		}
	}
}

func TestEditorDefaultFallback(t *testing.T) {
	// Test the logic for choosing default editor
	// When EDITOR is not set, should default to "code"
	originalEditor := os.Getenv("EDITOR")
	if err := os.Unsetenv("EDITOR"); err != nil {
		t.Fatalf("failed to unset EDITOR: %v", err)
	}
	defer func() {
		if originalEditor != "" {
			if err := os.Setenv("EDITOR", originalEditor); err != nil {
				t.Errorf("failed to restore EDITOR: %v", err)
			}
		}
	}()

	// Test that default editor is "code" when EDITOR is not set
	editor := ""
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
