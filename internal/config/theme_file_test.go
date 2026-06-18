package config

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	tmpDir := t.TempDir()
	return NewManager(filepath.Join(tmpDir, "tt.json"))
}

func testThemeFile(name string) ThemeFile {
	return NewThemeFile(name, "dark", "0.0.0", Theme{
		BackgroundColor: "#000000",
		Palette:         PaletteColors{Primary: "#ffffff"},
	})
}

func TestDeleteTheme_HappyPath(t *testing.T) {
	mgr := newTestManager(t)
	tf := testThemeFile("my-theme")

	if err := mgr.WriteTheme(tf); err != nil {
		t.Fatalf("WriteTheme: %v", err)
	}

	path, _ := mgr.ThemePath("my-theme")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file should exist before delete: %v", err)
	}

	if err := mgr.DeleteTheme("my-theme"); err != nil {
		t.Fatalf("DeleteTheme: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should be gone after delete")
	}

	themes, err := mgr.ListThemes()
	if err != nil {
		t.Fatalf("ListThemes: %v", err)
	}
	if len(themes) != 0 {
		t.Errorf("ListThemes after delete: got %d themes, want 0", len(themes))
	}
}

func TestDeleteTheme_MissingFile(t *testing.T) {
	mgr := newTestManager(t)

	err := mgr.DeleteTheme("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent theme, got nil")
	}
	if got := err.Error(); len(got) == 0 {
		t.Error("error message should be non-empty")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error %q should mention the theme name", err.Error())
	}
}

// TestWriteTheme_AtomicReplacement verifies that WriteTheme replaces an
// existing theme file atomically: a concurrent reader must never observe an
// empty/truncated/partial file. A non-atomic implementation that uses
// os.WriteFile (which opens O_TRUNC|O_WRONLY) exposes a window where the
// file is zero bytes or partially written.
func TestWriteTheme_AtomicReplacement(t *testing.T) {
	mgr := newTestManager(t)
	tf := testThemeFile("atomic-theme")
	if err := mgr.WriteTheme(tf); err != nil {
		t.Fatalf("seed WriteTheme: %v", err)
	}
	path, _ := mgr.ThemePath("atomic-theme")

	var stop atomic.Bool
	var wg sync.WaitGroup
	var observedEmpty atomic.Bool

	// Reader goroutine: continuously stats and reads the theme file.
	wg.Go(func() {
		for !stop.Load() {
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			if info.Size() == 0 {
				observedEmpty.Store(true)
				return
			}
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			if len(data) == 0 {
				observedEmpty.Store(true)
				return
			}
			if _, err := UnmarshalThemeFile(data); err != nil {
				// Partial JSON parses as invalid — treat as corruption.
				observedEmpty.Store(true)
				return
			}
		}
	})

	// Writer: repeatedly overwrite with a fresh theme.
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if err := mgr.WriteTheme(tf); err != nil {
			stop.Store(true)
			wg.Wait()
			t.Fatalf("WriteTheme: %v", err)
		}
	}
	stop.Store(true)
	wg.Wait()

	if observedEmpty.Load() {
		t.Fatal("reader observed empty/partial theme file during concurrent WriteTheme — file replacement is not atomic")
	}
}

func TestDeleteTheme_RejectsTraversal(t *testing.T) {
	mgr := newTestManager(t)

	unsafe := []string{
		"../escape",
		"foo/bar",
		"",
		"../../etc/passwd",
		"foo\\bar",
	}
	for _, name := range unsafe {
		if err := mgr.DeleteTheme(name); err == nil {
			t.Errorf("DeleteTheme(%q): expected error, got nil", name)
		}
	}
}
