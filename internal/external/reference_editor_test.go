package external

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClassifyReferenceEditor(t *testing.T) {
	cases := []struct {
		editor string
		want   ReferenceEditorClass
	}{
		{"code", ReferenceEditorGUI},
		{"codium --wait", ReferenceEditorGUI},
		{"cursor --reuse-window", ReferenceEditorGUI},
		{"subl -w", ReferenceEditorGUI},
		{"zed", ReferenceEditorGUI},
		{"goland --line 1", ReferenceEditorGUI},
		{"nvim -u NORC", ReferenceEditorTerminal},
		{"vim", ReferenceEditorTerminal},
		{"hx", ReferenceEditorTerminal},
		{"tt-custom-editor", ReferenceEditorUnknown},
		{"", ReferenceEditorUnknown},
	}

	for _, tc := range cases {
		t.Run(tc.editor, func(t *testing.T) {
			got := ClassifyReferenceEditor(tc.editor)
			if got != tc.want {
				t.Fatalf("ClassifyReferenceEditor(%q) = %q, want %q", tc.editor, got, tc.want)
			}
		})
	}
}

func TestReferenceEditorCommandStripsWaitFlagsForGUIEditors(t *testing.T) {
	cmd, class, err := ReferenceEditorCommand("code --wait -w --reuse-window", "/tmp/reference.md")
	if err != nil {
		t.Fatalf("ReferenceEditorCommand returned error: %v", err)
	}
	if class != ReferenceEditorGUI {
		t.Fatalf("class = %q, want %q", class, ReferenceEditorGUI)
	}

	args := cmd.Args
	for _, arg := range args {
		if arg == "--wait" || arg == "-w" {
			t.Fatalf("ReferenceEditorCommand should strip wait flags, got args: %v", args)
		}
	}
	if len(args) != 3 || args[1] != "--reuse-window" || args[2] != "/tmp/reference.md" {
		t.Fatalf("expected [code --reuse-window /tmp/reference.md], got %v", args)
	}
}

func TestReferenceEditorCommandRejectsTerminalAndUnknownEditors(t *testing.T) {
	cases := []struct {
		editor string
		class  ReferenceEditorClass
	}{
		{"nvim -u NORC", ReferenceEditorTerminal},
		{"tt-custom-editor", ReferenceEditorUnknown},
	}

	for _, tc := range cases {
		t.Run(tc.editor, func(t *testing.T) {
			cmd, class, err := ReferenceEditorCommand(tc.editor, "/tmp/reference.md")
			if cmd != nil {
				t.Fatalf("expected nil command for rejected editor, got %v", cmd.Args)
			}
			if class != tc.class {
				t.Fatalf("class = %q, want %q", class, tc.class)
			}
			if !errors.Is(err, ErrReferenceEditorUnsupported) {
				t.Fatalf("expected ErrReferenceEditorUnsupported, got %v", err)
			}
			if !strings.Contains(err.Error(), "external_editor") {
				t.Fatalf("expected error to mention external_editor, got %v", err)
			}
			legacyConfigKey := strings.Join([]string{"notes", "editor"}, "_")
			if strings.Contains(err.Error(), legacyConfigKey) {
				t.Fatalf("expected error not to mention legacy config key %q, got %v", legacyConfigKey, err)
			}
		})
	}
}

func TestResolveReferenceEditorPreservesConfiguredFlags(t *testing.T) {
	dir := t.TempDir()
	fakeCode := filepath.Join(dir, "code")
	if err := os.WriteFile(fakeCode, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake code: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	if err := os.Setenv("PATH", dir); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}

	got, err := ResolveReferenceEditor("code --wait --reuse-window")
	if err != nil {
		t.Fatalf("ResolveReferenceEditor returned error: %v", err)
	}
	if got != "code --wait --reuse-window" {
		t.Fatalf("ResolveReferenceEditor preserved %q, want configured string", got)
	}
}

func TestOpenReferenceFileGUILaunchesFireAndForget(t *testing.T) {
	dir := t.TempDir()
	argFile := filepath.Join(dir, "args.txt")
	fakeCode := filepath.Join(dir, "code")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argFile + "\n/bin/sleep 1\nexit 0\n"
	if err := os.WriteFile(fakeCode, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake code: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	if err := os.Setenv("PATH", dir); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}

	referencePath := filepath.Join(dir, "reference.md")
	start := time.Now()
	result, err := OpenReferenceFile("code --wait --reuse-window", referencePath)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("OpenReferenceFile returned error: %v", err)
	}
	if elapsed >= 500*time.Millisecond {
		t.Fatalf("OpenReferenceFile appears to have blocked; elapsed %s", elapsed)
	}
	if result.Status != ReferenceOpenStatusOpened {
		t.Fatalf("status = %q, want %q", result.Status, ReferenceOpenStatusOpened)
	}
	if result.Class != ReferenceEditorGUI {
		t.Fatalf("class = %q, want %q", result.Class, ReferenceEditorGUI)
	}

	// The fake editor sleeps after recording its arguments, so the launch is
	// still in flight when OpenReferenceFile returns. Wait for the complete
	// two-argument record rather than any nonempty file, so a partially flushed
	// args file can never be mistaken for a successful fire-and-forget launch.
	want := []string{"--reuse-window", referencePath}
	recorded, err := readFileEventuallyMatching(argFile, 5*time.Second, func(b []byte) bool {
		return len(strings.Split(strings.TrimSpace(string(b)), "\n")) == len(want)
	})
	if err != nil {
		t.Fatalf("read arg file: %v", err)
	}
	args := strings.Split(strings.TrimSpace(string(recorded)), "\n")
	if len(args) != len(want) || args[0] != want[0] || args[1] != want[1] {
		t.Fatalf("expected ['--reuse-window', referencePath], got %v", args)
	}
}

func TestOpenReferenceFileRejectsTerminalEditorsBeforeLaunch(t *testing.T) {
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

	result, err := OpenReferenceFile("nvim -u NORC", filepath.Join(dir, "reference.md"))
	if !errors.Is(err, ErrReferenceEditorUnsupported) {
		t.Fatalf("expected ErrReferenceEditorUnsupported, got %v", err)
	}
	if result.Status != ReferenceOpenStatusUnsupportedEditor {
		t.Fatalf("status = %q, want %q", result.Status, ReferenceOpenStatusUnsupportedEditor)
	}
	if result.Class != ReferenceEditorTerminal {
		t.Fatalf("class = %q, want %q", result.Class, ReferenceEditorTerminal)
	}
	if _, statErr := os.Stat(argFile); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("terminal editor should not have launched; stat err = %v", statErr)
	}
}
