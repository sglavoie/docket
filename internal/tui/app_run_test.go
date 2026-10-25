package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/config"
)

// TestMachineStateAndPIDLockPathsRemainDistinct verifies that PID locks remain
// data-path keyed even though machine.json is now workspace-ID keyed.
func TestMachineStateAndPIDLockPathsRemainDistinct(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	local := config.LocalDataRoot()

	// When the data dir is the local root, the lock lives directly under it.
	if got, want := config.TUIProcessLockPath(local), filepath.Join(local, "pid.lock"); got != want {
		t.Errorf("TUIProcessLockPath(local) = %q, want %q", got, want)
	}

	// An iCloud-style (non-local) data dir must not host the lock; it lives under
	// the local runtime dir instead.
	icloud := filepath.Join(home, "Library", "Mobile Documents", "com~apple~CloudDocs", "tt")
	lock := config.TUIProcessLockPath(icloud)
	if strings.HasPrefix(lock, filepath.Clean(icloud)) {
		t.Errorf("TUIProcessLockPath(icloud) = %q must not live under the synced data dir", lock)
	}
	if !strings.HasPrefix(lock, filepath.Clean(local)) {
		t.Errorf("TUIProcessLockPath(icloud) = %q must live under the local root %q", lock, local)
	}

	// Data-dir-specific: a different data dir yields a different lock path.
	if other := config.TUIProcessLockPath(icloud + "-other"); other == lock {
		t.Errorf("distinct data dirs must yield distinct lock paths, both = %q", lock)
	}

	machinePath, err := config.MachineStatePath("11111111-1111-4111-8111-111111111111")
	if err != nil {
		t.Fatalf("MachineStatePath: %v", err)
	}
	if filepath.Dir(machinePath) == filepath.Dir(lock) {
		t.Fatalf("machine state and PID lock unexpectedly share directory %q", filepath.Dir(lock))
	}
}

func TestTUIEnvironmentIgnoresOnlyInheritedNoColor(t *testing.T) {
	environ := []string{
		"TERM=xterm-kitty",
		"NO_COLOR=1",
		"COLORTERM=truecolor",
	}

	got := tuiEnvironment(environ, false)
	want := []string{"TERM=xterm-kitty", "COLORTERM=truecolor"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("tuiEnvironment() = %q, want %q", got, want)
	}
	if environ[1] != "NO_COLOR=1" {
		t.Fatalf("tuiEnvironment mutated its input: %q", environ)
	}
}

func TestTUIEnvironmentPreservesNoColorWhenExplicitlyDisabled(t *testing.T) {
	environ := []string{"TERM=xterm-kitty", "NO_COLOR=1"}

	got := tuiEnvironment(environ, true)
	if strings.Join(got, "\n") != strings.Join(environ, "\n") {
		t.Fatalf("tuiEnvironment() = %q, want %q", got, environ)
	}
}
