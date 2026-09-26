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

// TestWorkspaceMoveCommandFailsWhenNotConfirmed keeps a declined or unanswered
// prompt from reading as a successful move to a script checking the exit code.
func TestWorkspaceMoveCommandFailsWhenNotConfirmed(t *testing.T) {
	for _, tc := range []struct{ name, input string }{
		{"declined", "n\n"},
		{"eof", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			src := filepath.Join(t.TempDir(), "src")
			dst := filepath.Join(t.TempDir(), "dst")
			if err := os.MkdirAll(src, 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			setCommandDataDir(t, src)
			workspaceMoveForce = false
			previous := confirmInput
			confirmInput = strings.NewReader(tc.input)
			t.Cleanup(func() { confirmInput = previous })

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
		})
	}
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
	// The retired aggregate config file never exists in a live workspace, so
	// naming it would send the reader looking for nothing.
	if strings.Contains(stdout, ".tt.json") || strings.Contains(stdout, "Config file") {
		t.Errorf("workspace location still names the retired config file:\n%s", stdout)
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
