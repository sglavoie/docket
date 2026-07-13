package cmd

import (
	"os"
	"path/filepath"
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
