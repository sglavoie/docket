package external

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDetectEditorPattern(t *testing.T) {
	cases := []struct {
		editor string
		want   editorPattern
	}{
		{"code", patternGoto},
		{"codium", patternGoto},
		{"cursor", patternGoto},
		{"code-insiders", patternGoto},
		{"codium-insiders", patternGoto},
		{"cursor-nightly", patternGoto},
		{"/usr/local/bin/codium", patternGoto},
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
		{"helix", patternPlusLine},
		{"hx", patternPlusLine},
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

func TestOpenCodeReference_UnknownEditorBlocksAndReturnsExitError(t *testing.T) {
	// Confirm "hx" is now classified as patternPlusLine (terminal editor).
	if detectEditorPattern("hx") != patternPlusLine {
		t.Fatalf("expected hx to be patternPlusLine")
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

// TestDetectEditorPattern_FlaggedString verifies that detectEditorPattern
// correctly identifies editors even when the editor string includes flags
// (e.g. "nvim -u NORC" should still be patternPlusLine).
func TestDetectEditorPattern_FlaggedString(t *testing.T) {
	cases := []struct {
		editor string
		want   editorPattern
	}{
		{"nvim -u NORC", patternPlusLine},
		{"vim --noplugin", patternPlusLine},
		{"code --wait", patternGoto},
		{"codium --wait", patternGoto},
		{"/usr/bin/nvim -u NORC", patternPlusLine},
		{"subl --new-window", patternColonLine},
		{"goland --line 1", patternLineFlag},
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

// TestOpenCodeReference_FlaggedEditorPassesArgs verifies that when an editor
// string like "nvim -u NORC" is given, OpenCodeReference correctly invokes
// the executable (nvim) with the extra flags (-u NORC) prepended before the
// line/file arguments.
func TestOpenCodeReference_FlaggedEditorPassesArgs(t *testing.T) {
	dir := t.TempDir()
	// Write a fake "nvim" that records its arguments to a file.
	argFile := filepath.Join(dir, "args.txt")
	fakeNvim := filepath.Join(dir, "nvim")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argFile + "\nexit 0\n"
	if err := os.WriteFile(fakeNvim, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake nvim: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	if err := os.Setenv("PATH", dir); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}

	filePath := filepath.Join(dir, "test.go")
	if err := os.WriteFile(filePath, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	if err := OpenCodeReference("nvim -u NORC", filePath, 5); err != nil {
		t.Fatalf("OpenCodeReference('nvim -u NORC', ...) failed: %v", err)
	}

	recorded, err := os.ReadFile(argFile)
	if err != nil {
		t.Fatalf("read arg file: %v", err)
	}
	args := strings.Split(strings.TrimSpace(string(recorded)), "\n")
	// Expected args for patternPlusLine: -u NORC +5 <file>
	if len(args) < 3 {
		t.Fatalf("expected at least 3 args, got %v", args)
	}
	if args[0] != "-u" {
		t.Errorf("expected args[0]='-u', got %q (all: %v)", args[0], args)
	}
	if args[1] != "NORC" {
		t.Errorf("expected args[1]='NORC', got %q (all: %v)", args[1], args)
	}
	if args[2] != "+5" {
		t.Errorf("expected args[2]='+5', got %q (all: %v)", args[2], args)
	}
}

func TestOpenCodeReference_NonPositiveLineOmitsLineArgForTerminalEditor(t *testing.T) {
	dir := t.TempDir()
	argFile := filepath.Join(dir, "args.txt")
	fakeNvim := filepath.Join(dir, "nvim")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argFile + "\nexit 0\n"
	if err := os.WriteFile(fakeNvim, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake nvim: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	if err := os.Setenv("PATH", dir); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}

	filePath := filepath.Join(dir, "test.go")
	if err := os.WriteFile(filePath, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	if err := OpenCodeReference("nvim", filePath, 0); err != nil {
		t.Fatalf("OpenCodeReference('nvim', ..., 0) failed: %v", err)
	}

	recorded, err := os.ReadFile(argFile)
	if err != nil {
		t.Fatalf("read arg file: %v", err)
	}
	args := strings.Split(strings.TrimSpace(string(recorded)), "\n")
	// patternPlusLine always emits "--" before the file path to guard
	// against file names beginning with '-'. With line=0 (omit +N), the
	// expected arg list is exactly ["--", filePath].
	if len(args) != 2 || args[0] != "--" || args[1] != filePath {
		t.Fatalf("expected ['--', filePath], got %v", args)
	}
}

// TestOpenCodeReference_TerminalEditorGuardsLeadingDashPath verifies that
// patternPlusLine inserts "--" between the line/flags and the file path, so a
// file whose name begins with '-' is not parsed as a vim/nvim option.
func TestOpenCodeReference_TerminalEditorGuardsLeadingDashPath(t *testing.T) {
	dir := t.TempDir()
	argFile := filepath.Join(dir, "args.txt")
	fakeNvim := filepath.Join(dir, "nvim")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argFile + "\nexit 0\n"
	if err := os.WriteFile(fakeNvim, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake nvim: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	if err := os.Setenv("PATH", dir); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}

	dashPath := filepath.Join(dir, "-leading-dash.go")
	if err := os.WriteFile(dashPath, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	if err := OpenCodeReference("nvim", dashPath, 5); err != nil {
		t.Fatalf("OpenCodeReference('nvim', dashPath, 5) failed: %v", err)
	}

	recorded, err := os.ReadFile(argFile)
	if err != nil {
		t.Fatalf("read arg file: %v", err)
	}
	args := strings.Split(strings.TrimSpace(string(recorded)), "\n")
	// Expected: +5 -- <dashPath>
	if len(args) != 3 {
		t.Fatalf("expected 3 args, got %v", args)
	}
	if args[0] != "+5" {
		t.Errorf("args[0] = %q, want '+5'", args[0])
	}
	if args[1] != "--" {
		t.Errorf("args[1] = %q, want '--' (option terminator)", args[1])
	}
	if args[2] != dashPath {
		t.Errorf("args[2] = %q, want %q", args[2], dashPath)
	}
}

func TestOpenCodeReference_NonPositiveLineOmitsLineArgForGotoEditor(t *testing.T) {
	dir := t.TempDir()
	argFile := filepath.Join(dir, "args.txt")
	fakeCode := filepath.Join(dir, "code")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argFile + "\nexit 0\n"
	if err := os.WriteFile(fakeCode, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake code: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	if err := os.Setenv("PATH", dir); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}

	filePath := filepath.Join(dir, "test.go")
	if err := os.WriteFile(filePath, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	if err := OpenCodeReference("code", filePath, -3); err != nil {
		t.Fatalf("OpenCodeReference('code', ..., -3) failed: %v", err)
	}

	recorded, err := readFileEventually(argFile, 5*time.Second)
	if err != nil {
		t.Fatalf("read arg file: %v", err)
	}
	args := strings.Split(strings.TrimSpace(string(recorded)), "\n")
	if len(args) != 1 || args[0] != filePath {
		t.Fatalf("expected only file path argument, got %v", args)
	}
}

func TestOpenCodeReference_CodiumUsesGotoPattern(t *testing.T) {
	dir := t.TempDir()
	argFile := filepath.Join(dir, "args.txt")
	fakeCodium := filepath.Join(dir, "codium")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argFile + "\nexit 0\n"
	if err := os.WriteFile(fakeCodium, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codium: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	if err := os.Setenv("PATH", dir); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}

	filePath := filepath.Join(dir, "test.go")
	if err := os.WriteFile(filePath, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	if err := OpenCodeReference("codium", filePath, 12); err != nil {
		t.Fatalf("OpenCodeReference('codium', ..., 12) failed: %v", err)
	}

	recorded, err := readFileEventually(argFile, 5*time.Second)
	if err != nil {
		t.Fatalf("read arg file: %v", err)
	}
	args := strings.Split(strings.TrimSpace(string(recorded)), "\n")
	if len(args) != 2 || args[0] != "--goto" || args[1] != filePath+":12" {
		t.Fatalf("expected ['--goto', filePath:12], got %v", args)
	}
}

func readFileEventually(path string, timeout time.Duration) ([]byte, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		b, err := os.ReadFile(path)
		if err == nil && len(strings.TrimSpace(string(b))) > 0 {
			return b, nil
		}
		if err != nil {
			lastErr = err
		}
		time.Sleep(10 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = os.ErrNotExist
	}
	return nil, lastErr
}
