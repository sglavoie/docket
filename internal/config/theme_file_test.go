package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
