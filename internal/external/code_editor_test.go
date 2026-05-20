package external

import (
	"os"
	"strings"
	"testing"
)

func TestDetectEditorPattern(t *testing.T) {
	cases := []struct {
		editor string
		want   editorPattern
	}{
		{"code", patternGoto},
		{"cursor", patternGoto},
		{"code-insiders", patternGoto},
		{"/usr/bin/code", patternGoto},
		{"vim", patternPlusLine},
		{"nvim", patternPlusLine},
		{"vi", patternPlusLine},
		{"nano", patternPlusLine},
		{"emacs", patternPlusLine},
		{"emacsclient", patternPlusLine},
		{"pico", patternPlusLine},
		{"/usr/local/bin/nvim", patternPlusLine},
		{"subl", patternColonLine},
		{"sublime", patternColonLine},
		{"zed", patternColonLine},
		{"idea", patternLineFlag},
		{"goland", patternLineFlag},
		{"pycharm", patternLineFlag},
		{"webstorm", patternLineFlag},
		{"helix", patternUnknown},
		{"hx", patternUnknown},
		{"unknown-tool", patternUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.editor, func(t *testing.T) {
			got := detectEditorPattern(tc.editor)
			if got != tc.want {
				t.Errorf("detectEditorPattern(%q) = %v, want %v", tc.editor, got, tc.want)
			}
		})
	}
}

func TestOpenCodeReference_EmptyEditor(t *testing.T) {
	err := OpenCodeReference("", "/tmp/file.go", 10)
	if err == nil {
		t.Fatal("expected error when editor is empty")
	}
	if !strings.Contains(err.Error(), "no editor configured") {
		t.Errorf("expected 'no editor configured' message, got %v", err)
	}
}

func TestOpenCodeReference_MissingEditor(t *testing.T) {
	// Force PATH to empty so the editor lookup fails deterministically.
	origPath := os.Getenv("PATH")
	t.Cleanup(func() {
		_ = os.Setenv("PATH", origPath)
	})
	if err := os.Setenv("PATH", ""); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}

	err := OpenCodeReference("tt-nonexistent-editor-xyz", "/tmp/file.go", 10)
	if err == nil {
		t.Fatal("expected error when editor is missing from PATH")
	}
	if !strings.Contains(err.Error(), "not found in PATH") {
		t.Errorf("expected 'not found in PATH' message, got %v", err)
	}
}
