package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestConfigManager_BackupConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")
	// Persistence and backups are per-domain now; assert against the workspace
	// catalog domain file and its .bak sidecar.
	catalogPath := WorkspaceCatalogPath(tmpDir)
	backupPath := catalogPath + ".bak"

	manager := NewManager(configPath)

	cfg, err := manager.LoadOrCreate()
	if err != nil {
		t.Fatalf("LoadOrCreate failed: %v", err)
	}
	cfg.Editor = "vim"
	cfg.AddProject(&models.Project{ID: "p1", Slug: "test", Name: "Test"})
	if err := manager.Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if !manager.ConfigBackupExists() {
		t.Error("backup should exist after Save (auto-backup)")
	}

	preModContent, _ := os.ReadFile(catalogPath)
	cfg.Editor = "nano"
	if err := manager.Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	backupContent, _ := os.ReadFile(backupPath)
	if string(preModContent) != string(backupContent) {
		t.Error("backup content should match the config state before the last save")
	}
}

func TestConfigManager_RestoreConfigFromBackup(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	manager := NewManager(configPath)

	cfg, _ := manager.LoadOrCreate()
	cfg.AddProject(&models.Project{ID: "p1", Slug: "project-1", Name: "Project 1"})
	_ = manager.Save(cfg)

	_ = manager.BackupConfig()

	cfg.AddProject(&models.Project{ID: "p2", Slug: "project-2", Name: "Project 2"})
	_ = manager.Save(cfg)

	cfg, _ = manager.Load()
	if len(cfg.Projects) != 2 {
		t.Fatalf("expected 2 projects before restore, got %d", len(cfg.Projects))
	}

	if err := manager.RestoreConfigFromBackup(); err != nil {
		t.Fatalf("RestoreConfigFromBackup failed: %v", err)
	}

	cfg, _ = manager.Load()
	if len(cfg.Projects) != 1 {
		t.Errorf("expected 1 project after restore, got %d", len(cfg.Projects))
	}
}

func TestConfigManager_RestoreConfigFromBackup_Reversibility(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	manager := NewManager(configPath)

	cfg, _ := manager.LoadOrCreate()
	cfg.AddProject(&models.Project{ID: "p1", Slug: "project-1", Name: "Project 1"})
	_ = manager.Save(cfg)
	_ = manager.BackupConfig()

	cfg.AddProject(&models.Project{ID: "p2", Slug: "project-2", Name: "Project 2"})
	_ = manager.Save(cfg)

	_ = manager.RestoreConfigFromBackup()
	cfg, _ = manager.Load()
	if len(cfg.Projects) != 1 {
		t.Errorf("expected 1 project after first restore, got %d", len(cfg.Projects))
	}

	_ = manager.RestoreConfigFromBackup()
	cfg, _ = manager.Load()
	if len(cfg.Projects) != 2 {
		t.Errorf("expected 2 projects after second restore, got %d", len(cfg.Projects))
	}
}

func TestConfigManager_ConfigBackupExists(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	manager := NewManager(configPath)

	if manager.ConfigBackupExists() {
		t.Error("backup should not exist initially")
	}

	_, _ = manager.LoadOrCreate()

	if manager.ConfigBackupExists() {
		t.Error("backup should not exist after LoadOrCreate")
	}

	// A .bak sidecar next to any domain artifact counts as a config backup.
	backupPath := manager.GetBackupPath()
	if err := os.MkdirAll(filepath.Dir(backupPath), 0755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(backupPath, []byte("{}"), 0644)

	if !manager.ConfigBackupExists() {
		t.Error("backup should exist after manual creation")
	}
}

func TestConfigManager_GetBackupPath(t *testing.T) {
	configPath := "/home/user/.tt.json"
	manager := NewManager(configPath)

	// Backups span the decomposed domain files; GetBackupPath returns the
	// workspace catalog domain's .bak sidecar as the representative path.
	expected := WorkspaceCatalogPath("/home/user") + ".bak"
	if manager.GetBackupPath() != expected {
		t.Errorf("expected backup path '%s', got '%s'", expected, manager.GetBackupPath())
	}
}

func TestConfigManager_RestoreConfigFromBackup_NoBackup(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	manager := NewManager(configPath)
	_, _ = manager.LoadOrCreate()

	err := manager.RestoreConfigFromBackup()
	if err == nil {
		t.Error("RestoreConfigFromBackup should fail when no backup exists")
	}
}

func TestConfigManager_LoadRecoversFromBackup(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")
	backupPath := configPath + ".bak"

	backupData := `{"editor": "vim", "version": 1, "data_dir": "/tmp/tt-test", "projects": []}`
	if err := os.WriteFile(backupPath, []byte(backupData), 0644); err != nil {
		t.Fatalf("failed to write backup: %v", err)
	}

	if err := os.WriteFile(configPath, []byte("{corrupted"), 0644); err != nil {
		t.Fatalf("failed to write corrupted config: %v", err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load should recover from backup, got error: %v", err)
	}

	if cfg.Editor != "vim" {
		t.Errorf("expected Editor 'vim' from backup, got '%s'", cfg.Editor)
	}

	raw, _ := os.ReadFile(configPath)
	var check Config
	if err := json.Unmarshal(raw, &check); err != nil {
		t.Errorf("recovered config file should be valid JSON: %v", err)
	}
}

func TestConfigManager_LoadFailsWhenBothCorrupted(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")
	backupPath := configPath + ".bak"

	_ = os.WriteFile(configPath, []byte("{bad"), 0644)
	_ = os.WriteFile(backupPath, []byte("{also bad"), 0644)

	manager := NewManager(configPath)
	_, err := manager.Load()
	if err == nil {
		t.Error("Load should fail when both config and backup are corrupted")
	}
}

func TestConfigManager_LoadFailsWhenNoBackup(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	_ = os.WriteFile(configPath, []byte("{bad"), 0644)

	manager := NewManager(configPath)
	_, err := manager.Load()
	if err == nil {
		t.Error("Load should fail when config is corrupted and no backup exists")
	}
}

func TestConfigManager_BackupWithNilProjects(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	configPath := filepath.Join(tmpDir, "tt.json")
	backupPath := configPath + ".bak"

	if err := os.WriteFile(configPath, []byte("not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	backupJSON := `{"projects": null}`
	if err := os.WriteFile(backupPath, []byte(backupJSON), 0600); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load should recover and normalize nil projects backup, got: %v", err)
	}
	if cfg.Projects == nil {
		t.Fatal("Load should normalize nil projects to an empty slice")
	}
	if cfg.DataDir != filepath.Join(tmpDir, ".tt") {
		t.Fatalf("Load should normalize empty data_dir, got %q", cfg.DataDir)
	}
}

func TestConfigManager_BackupWithEmptyJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	configPath := filepath.Join(tmpDir, "tt.json")
	backupPath := configPath + ".bak"

	if err := os.WriteFile(configPath, []byte("not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backupPath, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load should recover from minimal backup, got: %v", err)
	}
	if cfg.Projects == nil {
		t.Fatal("Load should normalize missing projects to an empty slice")
	}
	if cfg.DataDir != filepath.Join(tmpDir, ".tt") {
		t.Fatalf("Load should normalize missing data_dir, got %q", cfg.DataDir)
	}
}

func TestConfigManager_Load_BothCorrupted(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")
	backupPath := configPath + ".bak"

	manager := NewManager(configPath)

	if err := os.WriteFile(configPath, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backupPath, []byte(`{bad`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := manager.Load()
	if err == nil {
		t.Fatal("expected error when both config and backup are corrupted")
	}
	if !strings.Contains(err.Error(), "backup is also corrupted") {
		t.Errorf("expected backup corruption error, got: %v", err)
	}
}

func TestConfigManager_Load_BackupValidRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")
	backupPath := configPath + ".bak"

	manager := NewManager(configPath)

	validCfg := NewConfig()
	validCfg.DataDir = tmpDir
	validData, _ := json.Marshal(validCfg)
	if err := os.WriteFile(backupPath, validData, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(configPath, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("expected recovery from valid backup, got error: %v", err)
	}
	if cfg.DataDir != tmpDir {
		t.Errorf("expected DataDir %q from backup, got %q", tmpDir, cfg.DataDir)
	}
}
