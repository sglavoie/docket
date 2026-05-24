package external

import (
	"os"
	"path/filepath"
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
		{"cursor-nightly", patternGoto},
		{"/usr/local/bin/cursor", patternGoto},
		{"/opt/editors/code-insiders", patternGoto},
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

// TestOpenCodeReference_UnknownEditorBlocks verifies that an unknown editor
// (classified as patternUnknown — e.g. helix/hx) is invoked with cmd.Run()
// (blocking) and with stdin attached, so terminal editors aren't spawned
// async into the void where they fail silently.
//
// Strategy: shadow PATH with a temp dir containing a fake "hx" that exits
// non-zero. OpenCodeReference should return that exit error synchronously —
// proving it used Run() rather than Start()-and-forget.
func TestOpenCodeReference_UnknownEditorBlocksAndReturnsExitError(t *testing.T) {
	// Confirm "hx" is still classified as patternUnknown.
	if detectEditorPattern("hx") != patternUnknown {
		t.Fatalf("expected hx to be patternUnknown")
	}

	dir := t.TempDir()
	fakeHx := filepath.Join(dir, "hx")
	// Script that exits 7 immediately — proves caller is blocking.
	if err := os.WriteFile(fakeHx, []byte("#!/bin/sh\nexit 7\n"), 0o755); err != nil {
		t.Fatalf("write fake hx: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	if err := os.Setenv("PATH", dir); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}

	err := OpenCodeReference("hx", "/tmp/file.go", 10)
	if err == nil {
		t.Fatal("expected error from blocking unknown editor that exited non-zero; got nil (fire-and-forget regression)")
	}
}
