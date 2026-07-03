package config

import (
	"path/filepath"
	"testing"
)

// TestExportPathDerivesFromConfigDir verifies the export directory and per-project
// export path are rooted at the config file's directory rather than a hardcoded
// ~/.tt/exports, so exports follow the active (possibly iCloud) data dir.
func TestExportPathDerivesFromConfigDir(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "tt")
	configPath := filepath.Join(dataDir, ".tt.json")
	m := NewManager(configPath)

	wantDir := filepath.Join(dataDir, "exports")
	if got := m.ExportsDir(); got != wantDir {
		t.Errorf("ExportsDir() = %q, want %q", got, wantDir)
	}

	wantPath := filepath.Join(wantDir, "my-project.md")
	if got := m.ExportPath("my-project"); got != wantPath {
		t.Errorf("ExportPath() = %q, want %q", got, wantPath)
	}
}
