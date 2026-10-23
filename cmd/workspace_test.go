package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
)

func TestWorkspaceMoveCommand_MovesAndSwitchesPointer(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	src := filepath.Join(t.TempDir(), "src")
	dst := filepath.Join(t.TempDir(), "dst")
	if err := os.MkdirAll(filepath.Join(src, "projects", "alpha"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "projects", "alpha", "tasks.md"), []byte("# Alpha"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	setCommandDataDir(t, src)

	workspaceMoveForce = true
	t.Cleanup(func() { workspaceMoveForce = false })

	if err := runWorkspaceMove(nil, []string{dst}); err != nil {
		t.Fatalf("runWorkspaceMove: %v", err)
	}

	if got, _ := os.ReadFile(filepath.Join(dst, "projects", "alpha", "tasks.md")); string(got) != "# Alpha" {
		t.Errorf("moved content = %q, want %q", got, "# Alpha")
	}
	ptr, ok := config.ReadActiveWorkspacePointer()
	if !ok || ptr != filepath.Clean(dst) {
		t.Errorf("active pointer = %q (ok=%v), want %q", ptr, ok, filepath.Clean(dst))
	}
	// Source retained as a backup.
	if _, err := os.Stat(filepath.Join(src, "projects", "alpha", "tasks.md")); err != nil {
		t.Errorf("source must be retained: %v", err)
	}
}

// TestWorkspaceMoveCommandFailsWhenUnanswered keeps an unanswered prompt — a
// script that piped no answer — from reading as a successful move to a script
// checking the exit code.
func TestWorkspaceMoveCommandFailsWhenUnanswered(t *testing.T) {
	dst := prepareUnconfirmedWorkspaceMove(t, "")

	_, _, err := captureDoctorOutput(t, func() error { return runWorkspaceMove(nil, []string{dst}) })
	if !errors.Is(err, errWorkspaceMoveNotConfirmed) {
		t.Fatalf("runWorkspaceMove error = %v, want %v", err, errWorkspaceMoveNotConfirmed)
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error %q does not name --force", err)
	}
	if _, statErr := os.Stat(dst); !os.IsNotExist(statErr) {
		t.Errorf("unconfirmed move touched the destination: %v", statErr)
	}
}

// TestWorkspaceMoveCommandDeclinedChangesNothing treats an explicit "no" as
// the operator's choice: it says nothing changed, succeeds, and does not
// suggest --force, which would only skip the prompt they just answered.
func TestWorkspaceMoveCommandDeclinedChangesNothing(t *testing.T) {
	for _, input := range []string{"n\n", "no\n", "\n"} {
		t.Run(strings.TrimSpace(input), func(t *testing.T) {
			dst := prepareUnconfirmedWorkspaceMove(t, input)

			stdout, _, err := captureDoctorOutput(t, func() error { return runWorkspaceMove(nil, []string{dst}) })
			if err != nil {
				t.Fatalf("runWorkspaceMove error = %v, want nil for a declined prompt", err)
			}
			if !strings.Contains(stdout, workspaceMoveCanceled) {
				t.Errorf("declined move output lacks %q:\n%s", workspaceMoveCanceled, stdout)
			}
			if strings.Contains(stdout, "--force") {
				t.Errorf("declined move still suggests --force:\n%s", stdout)
			}
			if _, statErr := os.Stat(dst); !os.IsNotExist(statErr) {
				t.Errorf("declined move touched the destination: %v", statErr)
			}
		})
	}
}

// prepareUnconfirmedWorkspaceMove seeds a source workspace directory, answers
// the prompt with input, and returns a destination that does not exist yet.
func prepareUnconfirmedWorkspaceMove(t *testing.T, input string) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	src := filepath.Join(t.TempDir(), "src")
	dst := filepath.Join(t.TempDir(), "dst")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	setCommandDataDir(t, src)
	workspaceMoveForce = false
	previous := confirmInput
	confirmInput = strings.NewReader(input)
	t.Cleanup(func() { confirmInput = previous })
	return dst
}

// TestWorkspaceMoveCommandRefusesTheSameDirectoryBeforePrompting reads no
// answer: a move that can only fail never asks for one.
func TestWorkspaceMoveCommandRefusesTheSameDirectoryBeforePrompting(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	src := filepath.Join(t.TempDir(), "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	setCommandDataDir(t, src)
	workspaceMoveForce = false
	previous := confirmInput
	confirmInput = strings.NewReader("y\n")
	t.Cleanup(func() { confirmInput = previous })

	stdout, _, err := captureDoctorOutput(t, func() error { return runWorkspaceMove(nil, []string{src}) })
	if err == nil || !strings.Contains(err.Error(), "same directory") {
		t.Fatalf("runWorkspaceMove error = %v, want a same-directory refusal", err)
	}
	if strings.Contains(stdout, "Proceed?") {
		t.Errorf("same-directory move prompted before refusing:\n%s", stdout)
	}
}

func TestWorkspaceLocationCommand_Runs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	src := filepath.Join(t.TempDir(), "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	setCommandDataDir(t, src)

	stdout, _, err := captureDoctorOutput(t, func() error { return runWorkspaceLocation(nil, nil) })
	if err != nil {
		t.Fatalf("runWorkspaceLocation: %v", err)
	}
	for _, want := range []string{
		"Workspace data directory: ",
		"Catalog:                  " + config.DisplayPath(filepath.Join(src, ".tt", "workspace.json")),
		"Logs:                     " + config.DisplayPath(filepath.Join(home, ".tt", "logs")) + " (machine-local)",
		"Backups:                  " + config.DisplayPath(filepath.Join(src, ".tt", "backups")),
		"Exports:                  " + config.DisplayPath(filepath.Join(src, "exports")),
		"Conflicts:                " + config.DisplayPath(filepath.Join(src, ".tt", "conflicts")),
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("workspace location output lacks %q:\n%s", want, stdout)
		}
	}
	if _, statErr := os.Stat(filepath.Join(home, ".tt", "logs")); !os.IsNotExist(statErr) {
		t.Errorf("workspace location created the logs dir: %v", statErr)
	}
	// --data-dir is in effect here, so the pointer line must not claim the
	// default resolution chose this workspace.
	if !strings.Contains(stdout, "Active workspace pointer: (none) (overridden by --data-dir)") {
		t.Errorf("workspace location does not say --data-dir overrides the pointer:\n%s", stdout)
	}
	// The retired aggregate config file never exists in a live workspace, so
	// naming it would send the reader looking for nothing.
	if strings.Contains(stdout, ".tt.json") || strings.Contains(stdout, "Config file") {
		t.Errorf("workspace location still names the retired config file:\n%s", stdout)
	}
}

// TestWorkspaceLocationSaysWhenNoWorkspaceIsThere keeps the listed paths from
// reading as files that exist, and drops the note once a workspace does.
func TestWorkspaceLocationSaysWhenNoWorkspaceIsThere(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	src := filepath.Join(t.TempDir(), "src")
	setCommandDataDir(t, src)
	const note = "(no workspace here yet; run 'tt' to create one)"

	stdout, _, err := captureDoctorOutput(t, func() error { return runWorkspaceLocation(nil, nil) })
	if err != nil || !strings.Contains(stdout, note) {
		t.Fatalf("location of a missing workspace = %v, want the note:\n%s", err, stdout)
	}
	if _, statErr := os.Stat(src); !os.IsNotExist(statErr) {
		t.Fatalf("workspace location created the directory: %v", statErr)
	}

	if _, bootErr := LoadCLIWorkspace(); bootErr != nil {
		t.Fatalf("bootstrap workspace: %v", bootErr)
	}
	stdout, _, err = captureDoctorOutput(t, func() error { return runWorkspaceLocation(nil, nil) })
	if err != nil || strings.Contains(stdout, note) {
		t.Fatalf("location of an existing workspace = %v, still notes it missing:\n%s", err, stdout)
	}
}

func TestWorkspaceMoveHelpExplainsTUILocksAndForceSafety(t *testing.T) {
	if !strings.Contains(workspaceMoveCmd.Long, "active TUI blocks the move") {
		t.Fatalf("workspace move help must explain active TUI exclusion:\n%s", workspaceMoveCmd.Long)
	}
	if !strings.Contains(workspaceMoveCmd.Long, "--force skips only the confirmation prompt") {
		t.Fatalf("workspace move help must explain --force safety scope:\n%s", workspaceMoveCmd.Long)
	}
	flag := workspaceMoveCmd.Flags().Lookup("force")
	if flag == nil || !strings.Contains(flag.Usage, "never bypass TUI or workspace safety locks") {
		t.Fatalf("--force usage must preserve lock guarantees: %+v", flag)
	}
}
