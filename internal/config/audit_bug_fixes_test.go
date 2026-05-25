package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/scoring"
)

// TestRemoveScoringProfile_CleansProjectAssignments guarantees that removing a
// scoring profile also drops every ProjectScoringProfile entry that pointed at
// it. Without the cleanup, GetProjectScoringProfile silently returns nil for
// the orphaned reference while the stale assignment quietly survives every
// Save round-trip — and resurrects if a profile with the same ID is later
// re-added under different definition.
func TestRemoveScoringProfile_CleansProjectAssignments(t *testing.T) {
	cfg := NewConfig()
	if err := cfg.AddScoringProfile(scoring.NewICEProfile()); err != nil {
		t.Fatalf("AddScoringProfile failed: %v", err)
	}
	cfg.SetProjectScoringProfile("proj-1", "ice")
	cfg.SetProjectScoringProfile("proj-2", "ice")
	cfg.SetProjectScoringProfile("proj-3", "wsjf") // unrelated profile assignment

	if !cfg.RemoveScoringProfile("ice") {
		t.Fatal("RemoveScoringProfile should return true")
	}

	if _, ok := cfg.ProjectScoringProfile["proj-1"]; ok {
		t.Error("expected proj-1 assignment dropped after profile removal")
	}
	if _, ok := cfg.ProjectScoringProfile["proj-2"]; ok {
		t.Error("expected proj-2 assignment dropped after profile removal")
	}
	if got, ok := cfg.ProjectScoringProfile["proj-3"]; !ok || got != "wsjf" {
		t.Errorf("expected unrelated proj-3 assignment preserved, got ok=%v value=%q", ok, got)
	}
}

// TestRemoveScoringProfile_NilProjectMapDoesNotPanic guarantees that removing
// a profile with no ProjectScoringProfile map present (nil) is safe.
func TestRemoveScoringProfile_NilProjectMapDoesNotPanic(t *testing.T) {
	cfg := NewConfig()
	cfg.ProjectScoringProfile = nil
	if err := cfg.AddScoringProfile(scoring.NewICEProfile()); err != nil {
		t.Fatalf("AddScoringProfile failed: %v", err)
	}
	if !cfg.RemoveScoringProfile("ice") {
		t.Fatal("RemoveScoringProfile should return true")
	}
}

// TestMigrateLegacyFilters_RewritesCollidingSmartListIDs guarantees that a
// legacy smartList ID colliding with an existing SavedFilter (or another
// smartList) receives a fresh UUID, preserving the ID uniqueness invariant.
func TestMigrateLegacyFilters_RewritesCollidingSmartListIDs(t *testing.T) {
	cfg := NewConfig()
	cfg.SavedFilters = []SavedFilter{
		{ID: "shared", Name: "Existing"},
	}
	cfg.SmartLists = []smartList{
		{ID: "shared", Name: "Colliding"},
		{ID: "unique", Name: "Unique"},
		{ID: "shared", Name: "AnotherCollision"},
	}

	if !cfg.MigrateLegacyFilters() {
		t.Fatal("expected migration to run")
	}

	ids := make(map[string]int)
	for _, f := range cfg.SavedFilters {
		ids[f.ID]++
	}
	for id, count := range ids {
		if count > 1 {
			t.Errorf("duplicate ID %q after migration (count=%d)", id, count)
		}
	}
	if len(cfg.SavedFilters) != 4 {
		t.Fatalf("expected 4 filters after migration, got %d", len(cfg.SavedFilters))
	}
	// The pre-existing "Existing" entry must keep its "shared" ID.
	if got := cfg.GetSavedFilter("shared"); got == nil || got.Name != "Existing" {
		t.Errorf("expected pre-existing 'shared' entry preserved, got %+v", got)
	}
}

// TestConfigManager_LoadClampsDashboardBottomLayoutOutOfRange guarantees that
// an out-of-range stored DashboardBottomLayout (e.g., from a downgrade or a
// hand-edited config) is clamped to a valid value at Load time so the bogus
// value doesn't keep round-tripping through every Save.
func TestConfigManager_LoadClampsDashboardBottomLayoutOutOfRange(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	data, _ := json.Marshal(map[string]any{
		"version":                 1,
		"data_dir":                "/tmp/tt-test",
		"projects":                []any{},
		"dashboard_bottom_layout": 99,
	})
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.DashboardBottomLayout == nil {
		t.Fatal("DashboardBottomLayout should not be nil after clamp")
	}
	if *cfg.DashboardBottomLayout != int(DashboardLayoutHorizontal) {
		t.Errorf("expected DashboardBottomLayout clamped to %d, got %d",
			int(DashboardLayoutHorizontal), *cfg.DashboardBottomLayout)
	}
}

// TestConfigManager_LoadClampsNegativeDashboardBottomLayout mirrors the
// out-of-range test for negative values.
func TestConfigManager_LoadClampsNegativeDashboardBottomLayout(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	data, _ := json.Marshal(map[string]any{
		"version":                 1,
		"data_dir":                "/tmp/tt-test",
		"projects":                []any{},
		"dashboard_bottom_layout": -3,
	})
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.DashboardBottomLayout == nil || *cfg.DashboardBottomLayout != 0 {
		t.Errorf("expected DashboardBottomLayout clamped to 0, got %v", cfg.DashboardBottomLayout)
	}
}

// TestConfigManager_LoadKeepsValidDashboardBottomLayout ensures the clamp
// does not perturb a legitimate stored value.
func TestConfigManager_LoadKeepsValidDashboardBottomLayout(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	data, _ := json.Marshal(map[string]any{
		"version":                 1,
		"data_dir":                "/tmp/tt-test",
		"projects":                []any{},
		"dashboard_bottom_layout": int(DashboardLayoutVertical),
	})
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.DashboardBottomLayout == nil || *cfg.DashboardBottomLayout != int(DashboardLayoutVertical) {
		t.Errorf("expected DashboardBottomLayout preserved as %d, got %v",
			int(DashboardLayoutVertical), cfg.DashboardBottomLayout)
	}
}

// TestConfigManager_LoadClampsInfoPanelModeOutOfRange guarantees that a bogus
// stored InfoPanelMode is clamped at Load time.
func TestConfigManager_LoadClampsInfoPanelModeOutOfRange(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	data, _ := json.Marshal(map[string]any{
		"version":         1,
		"data_dir":        "/tmp/tt-test",
		"projects":        []any{},
		"info_panel_mode": 77,
	})
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.InfoPanelMode == nil || *cfg.InfoPanelMode != int(InfoPanelInfoCards) {
		t.Errorf("expected InfoPanelMode clamped to %d, got %v",
			int(InfoPanelInfoCards), cfg.InfoPanelMode)
	}
}

// TestConfigManager_LoadClampedValueRoundTrips guarantees the clamped value
// actually persists through a Save round-trip so the bogus value no longer
// resurrects on subsequent reads.
func TestConfigManager_LoadClampedValueRoundTrips(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	data, _ := json.Marshal(map[string]any{
		"version":                 1,
		"data_dir":                "/tmp/tt-test",
		"projects":                []any{},
		"dashboard_bottom_layout": 42,
		"info_panel_mode":         42,
	})
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if err := manager.Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	reloaded, err := manager.Load()
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if reloaded.DashboardBottomLayout == nil || *reloaded.DashboardBottomLayout != 0 {
		t.Errorf("expected DashboardBottomLayout = 0 after round-trip, got %v",
			reloaded.DashboardBottomLayout)
	}
	if reloaded.InfoPanelMode == nil || *reloaded.InfoPanelMode != int(InfoPanelInfoCards) {
		t.Errorf("expected InfoPanelMode = %d after round-trip, got %v",
			int(InfoPanelInfoCards), reloaded.InfoPanelMode)
	}
}
