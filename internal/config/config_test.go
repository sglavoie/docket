package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestNewConfig(t *testing.T) {
	cfg := NewConfig()

	if cfg.DataDir == "" {
		t.Error("DataDir should not be empty")
	}
	if cfg.Editor != "code" {
		t.Errorf("expected default Editor 'code', got '%s'", cfg.Editor)
	}
	if cfg.DefaultContext != "personal" {
		t.Errorf("expected default Context 'personal', got '%s'", cfg.DefaultContext)
	}
	if !cfg.FzfEnabled {
		t.Error("FzfEnabled should default to true")
	}
	if !cfg.GlowEnabled {
		t.Error("GlowEnabled should default to true")
	}
	if cfg.Projects == nil {
		t.Error("Projects should not be nil")
	}
}

func TestConfigManager_LoadSave(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	manager := NewManager(configPath)

	// Initial load should create default config
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Modify config
	cfg.Editor = "vim"
	cfg.DefaultContext = "work"
	cfg.FzfEnabled = false

	// Save
	if err := manager.Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load again
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

	// Should not exist initially
	if manager.Exists() {
		t.Error("config should not exist initially")
	}

	// LoadOrCreate should create the file
	cfg, err := manager.LoadOrCreate()
	if err != nil {
		t.Fatalf("LoadOrCreate failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("expected config, got nil")
	}

	// File should now exist
	if !manager.Exists() {
		t.Error("config file should exist after LoadOrCreate")
	}

	// Verify file was written
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

func TestConfig_Projects(t *testing.T) {
	cfg := NewConfig()

	project1 := &models.Project{
		ID:   "p1",
		Slug: "project-one",
		Name: "Project One",
	}
	project2 := &models.Project{
		ID:   "p2",
		Slug: "project-two",
		Name: "Project Two",
	}

	// Add projects
	cfg.AddProject(project1)
	cfg.AddProject(project2)

	if len(cfg.Projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(cfg.Projects))
	}

	// Get by ID
	found := cfg.GetProject("p1")
	if found == nil || found.ID != "p1" {
		t.Error("GetProject by ID failed")
	}

	// Get by slug
	found = cfg.GetProject("project-two")
	if found == nil || found.ID != "p2" {
		t.Error("GetProject by slug failed")
	}

	// Get by name
	found = cfg.GetProject("Project One")
	if found == nil || found.ID != "p1" {
		t.Error("GetProject by name failed")
	}

	// Get non-existent
	found = cfg.GetProject("non-existent")
	if found != nil {
		t.Error("expected nil for non-existent project")
	}

	// Remove project
	removed := cfg.RemoveProject("p1")
	if !removed {
		t.Error("RemoveProject should return true")
	}
	if len(cfg.Projects) != 1 {
		t.Errorf("expected 1 project after removal, got %d", len(cfg.Projects))
	}

	// Remove non-existent
	removed = cfg.RemoveProject("non-existent")
	if removed {
		t.Error("RemoveProject should return false for non-existent")
	}
}

func TestConfig_CurrentProject(t *testing.T) {
	cfg := NewConfig()

	project := &models.Project{
		ID:   "p1",
		Slug: "test-project",
		Name: "Test Project",
	}
	cfg.AddProject(project)

	// No current project initially
	if cfg.GetCurrentProject() != nil {
		t.Error("expected nil current project initially")
	}

	// Set current project
	if !cfg.SetCurrentProject("p1") {
		t.Error("SetCurrentProject should return true")
	}
	if cfg.CurrentProject != "p1" {
		t.Errorf("expected CurrentProject 'p1', got '%s'", cfg.CurrentProject)
	}

	// Get current project
	current := cfg.GetCurrentProject()
	if current == nil || current.ID != "p1" {
		t.Error("GetCurrentProject failed")
	}

	// Set non-existent project
	if cfg.SetCurrentProject("non-existent") {
		t.Error("SetCurrentProject should return false for non-existent")
	}
	// Should still be set to p1
	if cfg.CurrentProject != "p1" {
		t.Error("CurrentProject should not change on failed set")
	}

	// Remove current project clears CurrentProject
	cfg.RemoveProject("p1")
	if cfg.CurrentProject != "" {
		t.Error("CurrentProject should be cleared when project is removed")
	}
}

func TestConfig_GetActiveArchivedProjects(t *testing.T) {
	cfg := NewConfig()

	active1 := &models.Project{ID: "a1", Name: "Active 1", Archived: false}
	active2 := &models.Project{ID: "a2", Name: "Active 2", Archived: false}
	archived := &models.Project{ID: "ar1", Name: "Archived 1", Archived: true}

	cfg.AddProject(active1)
	cfg.AddProject(active2)
	cfg.AddProject(archived)

	// Active projects
	activeProjects := cfg.GetActiveProjects()
	if len(activeProjects) != 2 {
		t.Errorf("expected 2 active projects, got %d", len(activeProjects))
	}

	// Archived projects
	archivedProjects := cfg.GetArchivedProjects()
	if len(archivedProjects) != 1 {
		t.Errorf("expected 1 archived project, got %d", len(archivedProjects))
	}
}

func TestConfig_EnsureDataDir(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "tt-data", "nested")

	cfg := NewConfig()
	cfg.DataDir = dataDir

	if err := cfg.EnsureDataDir(); err != nil {
		t.Fatalf("EnsureDataDir failed: %v", err)
	}

	// Check directory exists
	info, err := os.Stat(dataDir)
	if os.IsNotExist(err) {
		t.Error("data directory was not created")
	}
	if !info.IsDir() {
		t.Error("expected directory, got file")
	}
}

func TestConfig_GetProjectPath(t *testing.T) {
	cfg := NewConfig()
	cfg.DataDir = "/home/user/.tt-data"

	path := cfg.GetProjectPath("my-project")
	expected := "/home/user/.tt-data/my-project"

	if path != expected {
		t.Errorf("expected path '%s', got '%s'", expected, path)
	}
}

func TestConfigManager_DefaultPath(t *testing.T) {
	manager := NewManager("")

	path := manager.GetConfigPath()
	if path == "" {
		t.Error("default path should not be empty")
	}

	// Should contain .tt.json
	if filepath.Base(path) != ".tt.json" {
		t.Errorf("expected config file '.tt.json', got '%s'", filepath.Base(path))
	}
}

func TestConfig_LoadWithMissingFields(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "partial-config.json")

	// Write minimal JSON
	content := `{"editor": "nano"}`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Should have loaded value
	if cfg.Editor != "nano" {
		t.Errorf("expected Editor 'nano', got '%s'", cfg.Editor)
	}

	// Should have default values for missing fields
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

func TestConfigManager_BackupConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")
	backupPath := configPath + ".bak"

	manager := NewManager(configPath)

	// Create and save a config
	cfg, err := manager.LoadOrCreate()
	if err != nil {
		t.Fatalf("LoadOrCreate failed: %v", err)
	}
	cfg.Editor = "vim"
	cfg.AddProject(&models.Project{ID: "p1", Slug: "test", Name: "Test"})
	if err := manager.Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Backup should not exist yet
	if manager.ConfigBackupExists() {
		t.Error("backup should not exist initially")
	}

	// Create backup
	if err := manager.BackupConfig(); err != nil {
		t.Fatalf("BackupConfig failed: %v", err)
	}

	// Backup should now exist
	if !manager.ConfigBackupExists() {
		t.Error("backup should exist after BackupConfig")
	}

	// Backup content should match original
	originalContent, _ := os.ReadFile(configPath)
	backupContent, _ := os.ReadFile(backupPath)
	if string(originalContent) != string(backupContent) {
		t.Error("backup content should match original")
	}
}

func TestConfigManager_RestoreConfigFromBackup(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	manager := NewManager(configPath)

	// Create initial config with one project
	cfg, _ := manager.LoadOrCreate()
	cfg.AddProject(&models.Project{ID: "p1", Slug: "project-1", Name: "Project 1"})
	manager.Save(cfg)

	// Create backup
	manager.BackupConfig()

	// Modify config (add another project)
	cfg.AddProject(&models.Project{ID: "p2", Slug: "project-2", Name: "Project 2"})
	manager.Save(cfg)

	// Verify we have 2 projects
	cfg, _ = manager.Load()
	if len(cfg.Projects) != 2 {
		t.Fatalf("expected 2 projects before restore, got %d", len(cfg.Projects))
	}

	// Restore from backup
	if err := manager.RestoreConfigFromBackup(); err != nil {
		t.Fatalf("RestoreConfigFromBackup failed: %v", err)
	}

	// Load restored config - should have 1 project
	cfg, _ = manager.Load()
	if len(cfg.Projects) != 1 {
		t.Errorf("expected 1 project after restore, got %d", len(cfg.Projects))
	}
}

func TestConfigManager_RestoreConfigFromBackup_Reversibility(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	manager := NewManager(configPath)

	// State A: 1 project
	cfg, _ := manager.LoadOrCreate()
	cfg.AddProject(&models.Project{ID: "p1", Slug: "project-1", Name: "Project 1"})
	manager.Save(cfg)
	manager.BackupConfig()

	// State B: 2 projects
	cfg.AddProject(&models.Project{ID: "p2", Slug: "project-2", Name: "Project 2"})
	manager.Save(cfg)

	// Restore to State A
	manager.RestoreConfigFromBackup()
	cfg, _ = manager.Load()
	if len(cfg.Projects) != 1 {
		t.Errorf("expected 1 project after first restore, got %d", len(cfg.Projects))
	}

	// Restore again should go back to State B (reversibility)
	manager.RestoreConfigFromBackup()
	cfg, _ = manager.Load()
	if len(cfg.Projects) != 2 {
		t.Errorf("expected 2 projects after second restore, got %d", len(cfg.Projects))
	}
}

func TestConfigManager_ConfigBackupExists(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")
	backupPath := configPath + ".bak"

	manager := NewManager(configPath)

	// No backup initially
	if manager.ConfigBackupExists() {
		t.Error("backup should not exist initially")
	}

	// Create config file
	manager.LoadOrCreate()

	// Still no backup
	if manager.ConfigBackupExists() {
		t.Error("backup should not exist after LoadOrCreate")
	}

	// Create backup manually
	os.WriteFile(backupPath, []byte("{}"), 0644)

	// Now backup exists
	if !manager.ConfigBackupExists() {
		t.Error("backup should exist after manual creation")
	}
}

func TestConfigManager_GetBackupPath(t *testing.T) {
	configPath := "/home/user/.tt.json"
	manager := NewManager(configPath)

	expected := "/home/user/.tt.json.bak"
	if manager.GetBackupPath() != expected {
		t.Errorf("expected backup path '%s', got '%s'", expected, manager.GetBackupPath())
	}
}

func TestConfigManager_RestoreConfigFromBackup_NoBackup(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	manager := NewManager(configPath)
	manager.LoadOrCreate()

	// Restore without backup should fail
	err := manager.RestoreConfigFromBackup()
	if err == nil {
		t.Error("RestoreConfigFromBackup should fail when no backup exists")
	}
}
