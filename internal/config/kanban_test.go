package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestConfigManager_LoadBackfillsKanbanDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	data, _ := json.Marshal(map[string]any{
		"version":  CurrentConfigVersion,
		"data_dir": "/tmp/tt-test",
		"projects": []any{},
	})
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := NewManager(configPath).Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.KanbanSettings.DefaultMode != defaultKanbanMode {
		t.Errorf("expected DefaultMode %q, got %q", defaultKanbanMode, cfg.KanbanSettings.DefaultMode)
	}
	if cfg.KanbanSettings.DefaultStartColumn != defaultKanbanStartColumn {
		t.Errorf("expected DefaultStartColumn %q, got %q", defaultKanbanStartColumn, cfg.KanbanSettings.DefaultStartColumn)
	}
}

func TestConfigManager_LoadNormalizesInvalidKanbanSettings(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	data, _ := json.Marshal(map[string]any{
		"version":  CurrentConfigVersion,
		"data_dir": "/tmp/tt-test",
		"projects": []any{},
		"kanban_settings": map[string]any{
			"default_mode":       "unknown",
			"detail_pane_height": 99,
			"enabled_modes":      []string{"status", "bogus", "status", "priority"},
		},
	})
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := NewManager(configPath).Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.KanbanSettings.DefaultMode != defaultKanbanMode {
		t.Errorf("expected invalid DefaultMode to become %q, got %q", defaultKanbanMode, cfg.KanbanSettings.DefaultMode)
	}
	if cfg.KanbanSettings.DefaultStartColumn != defaultKanbanStartColumn {
		t.Errorf("expected empty DefaultStartColumn to become %q, got %q", defaultKanbanStartColumn, cfg.KanbanSettings.DefaultStartColumn)
	}
	if cfg.KanbanSettings.DetailPaneHeight == nil || *cfg.KanbanSettings.DetailPaneHeight != maxKanbanDetailHeight {
		t.Errorf("expected DetailPaneHeight clamped to %d, got %v", maxKanbanDetailHeight, cfg.KanbanSettings.DetailPaneHeight)
	}
	wantModes := []string{"status", "priority"}
	if !reflect.DeepEqual(cfg.KanbanSettings.EnabledModes, wantModes) {
		t.Errorf("expected EnabledModes %v, got %v", wantModes, cfg.KanbanSettings.EnabledModes)
	}
}

func TestConfigManager_LoadPreservesValidKanbanSettings(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	height := 4
	data, _ := json.Marshal(map[string]any{
		"version":  CurrentConfigVersion,
		"data_dir": "/tmp/tt-test",
		"projects": []any{},
		"kanban_settings": KanbanSettings{
			DefaultMode:        "score_buckets",
			DefaultStartColumn: "done",
			EnabledModes:       []string{"score_buckets", "section"},
			DetailPaneHeight:   &height,
		},
	})
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := NewManager(configPath).Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.KanbanSettings.DefaultMode != "score_buckets" {
		t.Errorf("expected DefaultMode preserved, got %q", cfg.KanbanSettings.DefaultMode)
	}
	if cfg.KanbanSettings.DefaultStartColumn != "done" {
		t.Errorf("expected DefaultStartColumn preserved, got %q", cfg.KanbanSettings.DefaultStartColumn)
	}
	if cfg.KanbanSettings.DetailPaneHeight == nil || *cfg.KanbanSettings.DetailPaneHeight != height {
		t.Errorf("expected DetailPaneHeight %d, got %v", height, cfg.KanbanSettings.DetailPaneHeight)
	}
	wantModes := []string{"score_buckets", "section"}
	if !reflect.DeepEqual(cfg.KanbanSettings.EnabledModes, wantModes) {
		t.Errorf("expected EnabledModes %v, got %v", wantModes, cfg.KanbanSettings.EnabledModes)
	}
}
