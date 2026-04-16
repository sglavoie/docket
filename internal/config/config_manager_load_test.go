package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigManager_LoadSave(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	manager := NewManager(configPath)

	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	cfg.Editor = "vim"
	cfg.DefaultContext = "work"
	cfg.FzfEnabled = false

	if saveErr := manager.Save(cfg); saveErr != nil {
		t.Fatalf("Save failed: %v", saveErr)
	}

	loaded, err := manager.Load()
	if err != nil {
		t.Fatalf("Load after save failed: %v", err)
	}

	if loaded.Editor != "vim" {
		t.Errorf("expected Editor 'vim', got '%s'", loaded.Editor)
	}
	if loaded.DefaultContext != "work" {
		t.Errorf("expected DefaultContext 'work', got '%s'", loaded.DefaultContext)
	}
	if loaded.FzfEnabled {
		t.Error("expected FzfEnabled false")
	}
}

func TestConfigManager_LoadOrCreate(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "new-config.json")

	manager := NewManager(configPath)

	if manager.Exists() {
		t.Error("config should not exist initially")
	}

	cfg, err := manager.LoadOrCreate()
	if err != nil {
		t.Fatalf("LoadOrCreate failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("expected config, got nil")
	}

	if !manager.Exists() {
		t.Error("config file should exist after LoadOrCreate")
	}

	_, err = os.Stat(configPath)
	if os.IsNotExist(err) {
		t.Error("config file was not created")
	}
}

func TestConfigManager_GetConfigPath(t *testing.T) {
	customPath := "/custom/path/config.json"
	manager := NewManager(customPath)

	if manager.GetConfigPath() != customPath {
		t.Errorf("expected path '%s', got '%s'", customPath, manager.GetConfigPath())
	}
}

func TestConfigManager_DefaultPath(t *testing.T) {
	manager := NewManager("")

	path := manager.GetConfigPath()
	if path == "" {
		t.Error("default path should not be empty")
	}

	if filepath.Base(path) != ".tt.json" {
		t.Errorf("expected config file '.tt.json', got '%s'", filepath.Base(path))
	}
}

func TestConfig_LoadWithMissingFields(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "partial-config.json")

	content := `{"editor": "nano"}`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Editor != "nano" {
		t.Errorf("expected Editor 'nano', got '%s'", cfg.Editor)
	}
	if cfg.DefaultContext != "personal" {
		t.Errorf("expected default DefaultContext 'personal', got '%s'", cfg.DefaultContext)
	}
	if cfg.DataDir == "" {
		t.Error("DataDir should have default value")
	}
	if cfg.Projects == nil {
		t.Error("Projects should be initialized to empty slice")
	}
}

func TestConfigManager_MigratesVersionZero(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	data := []byte(`{"data_dir": "/tmp/test", "editor": "vim"}`)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Version != 2 {
		t.Errorf("expected Version 2 after migration, got %d", cfg.Version)
	}

	cfg2, err := manager.Load()
	if err != nil {
		t.Fatalf("second Load failed: %v", err)
	}
	if cfg2.Version != 2 {
		t.Errorf("expected Version 2 on reload, got %d", cfg2.Version)
	}
}

func TestConfigManager_PreservesExistingVersion(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	// v1 config gets migrated to v2 on load (v1→v2 migration).
	data := []byte(`{"version": 1, "data_dir": "/tmp/test", "editor": "vim"}`)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Version != 2 {
		t.Errorf("expected Version 2 (migrated from v1), got %d", cfg.Version)
	}
}

func TestConfigManager_LoadFiltersInvalidProjects(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	data := `{
		"version": 1,
		"projects": [
			{"id": "p1", "slug": "valid", "name": "Valid Project", "path": "/tmp/valid"},
			{"id": "", "slug": "no-id", "name": "No ID", "path": "/tmp/noid"},
			{"id": "p3", "slug": "no-name", "name": "", "path": "/tmp/noname"}
		]
	}`
	if err := os.WriteFile(configPath, []byte(data), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Projects) != 1 {
		t.Errorf("expected 1 valid project, got %d", len(cfg.Projects))
	}
	if cfg.Projects[0].ID != "p1" {
		t.Errorf("expected project ID 'p1', got '%s'", cfg.Projects[0].ID)
	}
}

func TestConfigManager_LoadKeepsAllValidProjects(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	data := `{
		"version": 1,
		"projects": [
			{"id": "p1", "slug": "one", "name": "One", "path": "/tmp/one"},
			{"id": "p2", "slug": "two", "name": "Two", "path": "/tmp/two"}
		]
	}`
	if err := os.WriteFile(configPath, []byte(data), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(cfg.Projects))
	}
}

func TestConfigManager_ClampsNegativeWorkflowEvalInterval(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	neg := -5
	data, _ := json.Marshal(map[string]any{
		"version":                1,
		"data_dir":               "/tmp/tt-test",
		"projects":               []any{},
		"workflow_eval_interval": neg,
	})
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.WorkflowEvalInterval == nil {
		t.Fatal("WorkflowEvalInterval should not be nil")
	}
	if *cfg.WorkflowEvalInterval != 0 {
		t.Errorf("expected WorkflowEvalInterval clamped to 0, got %d", *cfg.WorkflowEvalInterval)
	}
}

func TestConfigManager_ClampsNegativeReviewDueSoonDays(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	data, _ := json.Marshal(map[string]any{
		"version":              1,
		"data_dir":             "/tmp/tt-test",
		"projects":             []any{},
		"review_due_soon_days": -3,
	})
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.ReviewDueSoonDays == nil {
		t.Fatal("ReviewDueSoonDays should not be nil")
	}
	if *cfg.ReviewDueSoonDays != 0 {
		t.Errorf("expected ReviewDueSoonDays clamped to 0, got %d", *cfg.ReviewDueSoonDays)
	}
}

func TestConfigManager_ClampsNegativeReviewOldDays(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	data, _ := json.Marshal(map[string]any{
		"version":         1,
		"data_dir":        "/tmp/tt-test",
		"projects":        []any{},
		"review_old_days": -10,
	})
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.ReviewOldDays == nil {
		t.Fatal("ReviewOldDays should not be nil")
	}
	if *cfg.ReviewOldDays != 0 {
		t.Errorf("expected ReviewOldDays clamped to 0, got %d", *cfg.ReviewOldDays)
	}
}

func TestConfigManager_PositiveReviewFieldsUnchanged(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	data, _ := json.Marshal(map[string]any{
		"version":                1,
		"data_dir":               "/tmp/tt-test",
		"projects":               []any{},
		"workflow_eval_interval": 30,
		"review_due_soon_days":   5,
		"review_old_days":        14,
	})
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.WorkflowEvalInterval == nil || *cfg.WorkflowEvalInterval != 30 {
		t.Errorf("expected WorkflowEvalInterval 30, got %v", cfg.WorkflowEvalInterval)
	}
	if cfg.ReviewDueSoonDays == nil || *cfg.ReviewDueSoonDays != 5 {
		t.Errorf("expected ReviewDueSoonDays 5, got %v", cfg.ReviewDueSoonDays)
	}
	if cfg.ReviewOldDays == nil || *cfg.ReviewOldDays != 14 {
		t.Errorf("expected ReviewOldDays 14, got %v", cfg.ReviewOldDays)
	}
}

func TestConfigManager_NegativeMaxPinnedAndRecentClamped(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	data, _ := json.Marshal(map[string]any{
		"version":                1,
		"data_dir":               "/tmp/tt-test",
		"projects":               []any{},
		"max_pinned_bar_items":   -5,
		"max_recent_task_visits": -10,
	})
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.MaxPinnedBarItems == nil {
		t.Fatal("MaxPinnedBarItems should not be nil")
	}
	if *cfg.MaxPinnedBarItems != 0 {
		t.Errorf("expected MaxPinnedBarItems clamped to 0, got %d", *cfg.MaxPinnedBarItems)
	}
	if cfg.MaxRecentTaskVisits == nil {
		t.Fatal("MaxRecentTaskVisits should not be nil")
	}
	if *cfg.MaxRecentTaskVisits != 0 {
		t.Errorf("expected MaxRecentTaskVisits clamped to 0, got %d", *cfg.MaxRecentTaskVisits)
	}
}
