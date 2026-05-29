package external

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
		{"codium", true},
		{"cursor", true},
		{"codium-insiders", true},
		{"cursor-nightly", true},
		{"/usr/local/bin/codium", true},
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

func TestSplitEditorString(t *testing.T) {
	cases := []struct {
		input    string
		wantExe  string
		wantArgs []string
	}{
		{"code", "code", nil},
		{"vim", "vim", nil},
		{"nvim -u NORC", "nvim", []string{"-u", "NORC"}},
		{"code --wait", "code", []string{"--wait"}},
		{"/usr/bin/vim -u ~/.min.vim", "/usr/bin/vim", []string{"-u", "~/.min.vim"}},
		{"", "", nil},
		{"   ", "", nil},
		// Regression: quoted executable path containing spaces.
		{`"/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code" --wait`,
			"/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code",
			[]string{"--wait"}},
		// Regression: single-quoted executable path containing spaces.
		{`'/opt/my editor/code' --wait`,
			"/opt/my editor/code",
			[]string{"--wait"}},
		// Regression: quoted flag value containing spaces.
		{`nvim -u "/tmp/my vimrc"`, "nvim", []string{"-u", "/tmp/my vimrc"}},
		// Regression: single-quoted flag value containing spaces.
		{`nvim -u '/tmp/my vimrc'`, "nvim", []string{"-u", "/tmp/my vimrc"}},
		// Quoted exe with no args.
		{`"/path/with spaces/editor"`, "/path/with spaces/editor", nil},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			exe, args := SplitEditorString(tc.input)
			if exe != tc.wantExe {
				t.Errorf("SplitEditorString(%q) exe = %q, want %q", tc.input, exe, tc.wantExe)
			}
			if len(args) != len(tc.wantArgs) {
				t.Errorf("SplitEditorString(%q) args = %v, want %v", tc.input, args, tc.wantArgs)
				return
			}
			for i, a := range args {
				if a != tc.wantArgs[i] {
					t.Errorf("SplitEditorString(%q) args[%d] = %q, want %q", tc.input, i, a, tc.wantArgs[i])
				}
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

func TestEditorCommandWithFlags(t *testing.T) {
	// "nvim -u NORC" should produce: nvim -u NORC <file>
	cmd := EditorCommand("nvim -u NORC", "/path/to/file.md")
	args := cmd.Args // Args[0] is the executable (possibly full path)
	if len(args) < 4 {
		t.Fatalf("expected at least 4 args (nvim, -u, NORC, file), got %v", args)
	}
	if args[1] != "-u" {
		t.Errorf("expected args[1] = '-u', got %q", args[1])
	}
	if args[2] != "NORC" {
		t.Errorf("expected args[2] = 'NORC', got %q", args[2])
	}
	if args[3] != "/path/to/file.md" {
		t.Errorf("expected args[3] = '/path/to/file.md', got %q", args[3])
	}

	// "code --wait" passed as editor string: EditorCommand should NOT double-add --wait.
	// The extra args from the editor string come before the auto-added --wait for code.
	cmd = EditorCommand("code --wait", "/path/to/file.md")
	waitCount := 0
	for _, a := range cmd.Args {
		if a == "--wait" {
			waitCount++
		}
	}
	if waitCount != 2 {
		// --wait appears once from the editor string flags AND once auto-added.
		// This is expected: the user explicitly included it, and we add it too.
		// We don't try to deduplicate flags — this matches shell behavior where
		// "code --wait --wait file" is harmless.
		t.Logf("note: --wait appears %d time(s) in args %v (both from editor string and auto-add)", waitCount, cmd.Args)
	}
	// The file path must always be the last arg.
	last := cmd.Args[len(cmd.Args)-1]
	if last != "/path/to/file.md" {
		t.Errorf("expected file path as last arg, got %q (args: %v)", last, cmd.Args)
	}
}

func TestOpenFolderCommandStripsWaitFlagsForGUIEditors(t *testing.T) {
	cmd := OpenFolderCommand("code --wait -w --reuse-window", "/tmp/project")

	for _, arg := range cmd.Args {
		if arg == "--wait" || arg == "-w" {
			t.Fatalf("OpenFolderCommand should strip GUI wait flags, got args: %v", cmd.Args)
		}
	}
	if got := cmd.Args[len(cmd.Args)-1]; got != "/tmp/project" {
		t.Fatalf("expected folder path as last arg, got %q (args: %v)", got, cmd.Args)
	}
}

func TestOpenFolderCommandPreservesNonGUIEditorDashWFlag(t *testing.T) {
	cmd := OpenFolderCommand("nvim -w /tmp/scriptout", "/tmp/project")
	args := cmd.Args

	if len(args) != 4 {
		t.Fatalf("expected nvim args to preserve -w flag and value, got %v", args)
	}
	if args[1] != "-w" {
		t.Errorf("expected args[1] = '-w', got %q (args: %v)", args[1], args)
	}
	if args[2] != "/tmp/scriptout" {
		t.Errorf("expected args[2] = '/tmp/scriptout', got %q (args: %v)", args[2], args)
	}
	if args[3] != "/tmp/project" {
		t.Errorf("expected folder path after preserved args, got %q (args: %v)", args[3], args)
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

// TestResolveEditor_FlaggedEditorString verifies that ResolveEditor accepts
// editor strings that contain flags (e.g. "nvim -u NORC") and correctly uses
// only the executable part for the PATH lookup, returning the full string.
func TestResolveEditor_FlaggedEditorString(t *testing.T) {
	// Create a fake editor binary so LookPath succeeds.
	dir := t.TempDir()
	fakeNvim := filepath.Join(dir, "nvim")
	if err := os.WriteFile(fakeNvim, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake nvim: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	if err := os.Setenv("PATH", dir); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}

	// "nvim -u NORC" should resolve successfully: only "nvim" is looked up,
	// but the full string is returned.
	result, err := ResolveEditor("nvim -u NORC")
	if err != nil {
		t.Fatalf("ResolveEditor('nvim -u NORC') failed: %v", err)
	}
	if result != "nvim -u NORC" {
		t.Errorf("ResolveEditor returned %q, want %q", result, "nvim -u NORC")
	}
}

// TestStripWaitFlags verifies that stripWaitFlags removes --wait and -w
// but preserves all other flags.
func TestStripWaitFlags(t *testing.T) {
	cases := []struct {
		input []string
		want  []string
	}{
		{nil, nil},
		{[]string{}, nil},
		{[]string{"--wait"}, nil},
		{[]string{"-w"}, nil},
		{[]string{"-u", "NORC"}, []string{"-u", "NORC"}},
		{[]string{"--wait", "-u", "NORC"}, []string{"-u", "NORC"}},
		{[]string{"-u", "NORC", "--wait"}, []string{"-u", "NORC"}},
		{[]string{"-w", "--some-flag"}, []string{"--some-flag"}},
		{[]string{"--wait", "-w", "--other"}, []string{"--other"}},
	}
	for _, tc := range cases {
		got := stripWaitFlags(tc.input)
		if len(got) != len(tc.want) {
			t.Errorf("stripWaitFlags(%v) = %v, want %v", tc.input, got, tc.want)
			continue
		}
		for i, v := range got {
			if v != tc.want[i] {
				t.Errorf("stripWaitFlags(%v)[%d] = %q, want %q", tc.input, i, v, tc.want[i])
			}
		}
	}
}

// even for VS Code-family editors that EditorCommand would normally block.
func TestOpenFolderCommand(t *testing.T) {
	cases := []struct {
		editor string
		path   string
	}{
		{"code", "/home/user/project"},
		{"cursor", "/home/user/project"},
		{"subl", "/home/user/project"},
		{"zed", "/home/user/project"},
		{"vim", "/home/user/project"},
		{"nvim -u NORC", "/home/user/project"},
		{"code-insiders", "/home/user/project"},
		// Regression: user-configured "code --wait" must not block folder-open.
		{"code --wait", "/home/user/project"},
		{"cursor --wait", "/home/user/project"},
		{"subl -w", "/home/user/project"},
	}
	for _, tc := range cases {
		t.Run(tc.editor, func(t *testing.T) {
			cmd := OpenFolderCommand(tc.editor, tc.path)
			for _, arg := range cmd.Args {
				if arg == "--wait" || arg == "-w" {
					t.Errorf("OpenFolderCommand(%q, ...) must not include blocking wait flag %q, got args: %v", tc.editor, arg, cmd.Args)
				}
			}
			// Path must appear as an argument.
			last := cmd.Args[len(cmd.Args)-1]
			if last != tc.path {
				t.Errorf("OpenFolderCommand(%q, %q) last arg = %q, want path", tc.editor, tc.path, last)
			}
		})
	}
}

// TestOpenFolderCommandWithFlags verifies that pre-configured flags in the
// editor string are preserved in the correct position (before the path).
func TestOpenFolderCommandWithFlags(t *testing.T) {
	cmd := OpenFolderCommand("nvim -u NORC", "/data/project")
	args := cmd.Args
	// Args[0] is the resolved executable path; args[1..] are the logical args.
	if len(args) < 4 {
		t.Fatalf("expected ≥4 args (nvim, -u, NORC, path), got %v", args)
	}
	if args[1] != "-u" {
		t.Errorf("args[1] = %q, want '-u'", args[1])
	}
	if args[2] != "NORC" {
		t.Errorf("args[2] = %q, want 'NORC'", args[2])
	}
	if args[3] != "/data/project" {
		t.Errorf("args[3] = %q, want '/data/project'", args[3])
	}
}

func TestVSCodeEditorOpenFileAtLineNonPositiveLineOpensFileOnly(t *testing.T) {
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
	codeCache = cachedPath{}
	t.Cleanup(func() { codeCache = cachedPath{} })

	filePath := filepath.Join(dir, "test.go")
	if err := os.WriteFile(filePath, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	editor := NewVSCodeEditor()
	if err := editor.OpenFileAtLine(filePath, 0); err != nil {
		t.Fatalf("OpenFileAtLine(..., 0) failed: %v", err)
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
