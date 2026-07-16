package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"

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

	viper.Set("data-dir", src)
	viper.Set("config", "")
	t.Cleanup(func() { viper.Set("data-dir", "") })

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

func TestWorkspaceLocationCommand_Runs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	src := filepath.Join(t.TempDir(), "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	viper.Set("data-dir", src)
	viper.Set("config", "")
	t.Cleanup(func() { viper.Set("data-dir", "") })

	if err := runWorkspaceLocation(nil, nil); err != nil {
		t.Fatalf("runWorkspaceLocation: %v", err)
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
