package config

import (
	"encoding/json"
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
	if saveErr := manager.Save(cfg); saveErr != nil {
		t.Fatalf("Save failed: %v", saveErr)
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
	cfg.DataDir = "/home/user/.tt"

	path := cfg.GetProjectPath("my-project")
	expected := "/home/user/.tt/projects/my-project"

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

	// Save() now auto-creates backup, so backup should exist after second save
	// (LoadOrCreate does one save, then we do another — the second one backs up the first)
	if !manager.ConfigBackupExists() {
		t.Error("backup should exist after Save (auto-backup)")
	}

	// Save a modified config, then verify backup matches pre-modification state
	preModContent, _ := os.ReadFile(configPath)
	cfg.Editor = "nano"
	if err := manager.Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Backup should contain the pre-modification content
	backupContent, _ := os.ReadFile(backupPath)
	if string(preModContent) != string(backupContent) {
		t.Error("backup content should match the config state before the last save")
	}
}

func TestConfigManager_RestoreConfigFromBackup(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	manager := NewManager(configPath)

	// Create initial config with one project
	cfg, _ := manager.LoadOrCreate()
	cfg.AddProject(&models.Project{ID: "p1", Slug: "project-1", Name: "Project 1"})
	_ = manager.Save(cfg)

	// Create backup
	_ = manager.BackupConfig()

	// Modify config (add another project)
	cfg.AddProject(&models.Project{ID: "p2", Slug: "project-2", Name: "Project 2"})
	_ = manager.Save(cfg)

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
	_ = manager.Save(cfg)
	_ = manager.BackupConfig()

	// State B: 2 projects
	cfg.AddProject(&models.Project{ID: "p2", Slug: "project-2", Name: "Project 2"})
	_ = manager.Save(cfg)

	// Restore to State A
	_ = manager.RestoreConfigFromBackup()
	cfg, _ = manager.Load()
	if len(cfg.Projects) != 1 {
		t.Errorf("expected 1 project after first restore, got %d", len(cfg.Projects))
	}

	// Restore again should go back to State B (reversibility)
	_ = manager.RestoreConfigFromBackup()
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
	_, _ = manager.LoadOrCreate()

	// Still no backup
	if manager.ConfigBackupExists() {
		t.Error("backup should not exist after LoadOrCreate")
	}

	// Create backup manually
	_ = os.WriteFile(backupPath, []byte("{}"), 0644)

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
	_, _ = manager.LoadOrCreate()

	// Restore without backup should fail
	err := manager.RestoreConfigFromBackup()
	if err == nil {
		t.Error("RestoreConfigFromBackup should fail when no backup exists")
	}
}

func TestConfig_GetGlobalSectionColor(t *testing.T) {
	cfg := NewConfig()

	// Default should be gray for unknown sections
	if color := cfg.GetGlobalSectionColor("Unknown"); color != "gray" {
		t.Errorf("expected gray for unknown section, got %s", color)
	}

	// Set a color and retrieve it
	cfg.SetGlobalSectionColor("Active", "green")
	if color := cfg.GetGlobalSectionColor("Active"); color != "green" {
		t.Errorf("expected green, got %s", color)
	}

	// Empty string in map should return gray
	cfg.SectionColors["Empty"] = ""
	if color := cfg.GetGlobalSectionColor("Empty"); color != "gray" {
		t.Errorf("expected gray for empty string color, got %s", color)
	}
}

func TestConfig_SetGlobalSectionColor(t *testing.T) {
	cfg := NewConfig()

	cfg.SetGlobalSectionColor("Active", "green")
	cfg.SetGlobalSectionColor("Done", "blue")

	if cfg.SectionColors["Active"] != "green" {
		t.Errorf("expected Active to be green")
	}
	if cfg.SectionColors["Done"] != "blue" {
		t.Errorf("expected Done to be blue")
	}

	// Overwrite existing color
	cfg.SetGlobalSectionColor("Active", "red")
	if cfg.SectionColors["Active"] != "red" {
		t.Errorf("expected Active to be red after overwrite")
	}
}

func TestConfig_SetGlobalSectionColor_NilMap(t *testing.T) {
	cfg := &Config{}
	cfg.SectionColors = nil

	// Should initialize map and set color without panicking
	cfg.SetGlobalSectionColor("Active", "green")

	if cfg.SectionColors == nil {
		t.Error("SectionColors should be initialized")
	}
	if cfg.SectionColors["Active"] != "green" {
		t.Errorf("expected Active to be green")
	}
}

func TestConfig_MigrateProjectSectionColors(t *testing.T) {
	cfg := NewConfig()

	// Set an existing global color
	cfg.SetGlobalSectionColor("Active", "blue")

	// Project colors to migrate
	projectColors := map[string]string{
		"Active": "green",  // Should NOT overwrite global (first-seen wins for global)
		"Review": "yellow", // Should be added
		"Done":   "gray",   // Should NOT be added (gray is default)
		"Empty":  "",       // Should NOT be added (empty string)
	}

	cfg.MigrateProjectSectionColors(projectColors)

	// Active should still be blue (global wins)
	if color := cfg.GetGlobalSectionColor("Active"); color != "blue" {
		t.Errorf("expected Active to remain blue, got %s", color)
	}

	// Review should be added
	if color := cfg.GetGlobalSectionColor("Review"); color != "yellow" {
		t.Errorf("expected Review to be yellow, got %s", color)
	}

	// Done should be gray (default, not added)
	if _, exists := cfg.SectionColors["Done"]; exists {
		t.Error("Done should not be added to global registry (gray is default)")
	}

	// Empty should not be added
	if _, exists := cfg.SectionColors["Empty"]; exists {
		t.Error("Empty should not be added to global registry")
	}
}

func TestConfig_MigrateProjectSectionColors_NilInput(t *testing.T) {
	cfg := NewConfig()
	cfg.SetGlobalSectionColor("Active", "green")

	// Should not panic with nil input
	cfg.MigrateProjectSectionColors(nil)

	// Existing color should remain
	if color := cfg.GetGlobalSectionColor("Active"); color != "green" {
		t.Errorf("expected Active to remain green, got %s", color)
	}
}

func TestConfig_ApplyGlobalColorsToProject(t *testing.T) {
	cfg := NewConfig()
	cfg.SetGlobalSectionColor("Active", "green")
	cfg.SetGlobalSectionColor("Done", "blue")

	project := models.NewProject("test", "Test", "/tmp/test")
	project.Sections = []string{"Backlog", "Active", "Done", "Review"}

	cfg.ApplyGlobalColorsToProject(project)

	// Active and Done should have global colors
	if project.SectionColors["Active"] != "green" {
		t.Errorf("expected Active to be green, got %s", project.SectionColors["Active"])
	}
	if project.SectionColors["Done"] != "blue" {
		t.Errorf("expected Done to be blue, got %s", project.SectionColors["Done"])
	}

	// Backlog and Review should not have colors (no global color set)
	if _, exists := project.SectionColors["Backlog"]; exists {
		t.Error("Backlog should not have a color (no global color)")
	}
	if _, exists := project.SectionColors["Review"]; exists {
		t.Error("Review should not have a color (no global color)")
	}
}

func TestConfig_ApplyGlobalColorsToProject_NilProjectColors(t *testing.T) {
	cfg := NewConfig()
	cfg.SetGlobalSectionColor("Active", "green")

	project := models.NewProject("test", "Test", "/tmp/test")
	project.SectionColors = nil
	project.Sections = []string{"Active"}

	// Should initialize map and apply colors without panicking
	cfg.ApplyGlobalColorsToProject(project)

	if project.SectionColors == nil {
		t.Error("SectionColors should be initialized")
	}
	if project.SectionColors["Active"] != "green" {
		t.Errorf("expected Active to be green")
	}
}

func TestConfig_SectionColorsPersistedInJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	manager := NewManager(configPath)
	cfg, _ := manager.LoadOrCreate()

	// Set some section colors
	cfg.SetGlobalSectionColor("Active", "green")
	cfg.SetGlobalSectionColor("Done", "blue")

	// Save and reload
	if err := manager.Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Colors should persist
	if loaded.GetGlobalSectionColor("Active") != "green" {
		t.Errorf("expected Active to be green after reload")
	}
	if loaded.GetGlobalSectionColor("Done") != "blue" {
		t.Errorf("expected Done to be blue after reload")
	}
}

func TestConfig_NewConfigHasSectionColors(t *testing.T) {
	cfg := NewConfig()

	if cfg.SectionColors == nil {
		t.Error("NewConfig should initialize SectionColors map")
	}
}

func TestNewConfig_HasVersion(t *testing.T) {
	cfg := NewConfig()
	if cfg.Version != 1 {
		t.Errorf("expected Version 1, got %d", cfg.Version)
	}
}

func TestConfigManager_MigratesVersionZero(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	// Write a config without version field (simulates old config)
	data := []byte(`{"data_dir": "/tmp/test", "editor": "vim"}`)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Version != 1 {
		t.Errorf("expected Version 1 after migration, got %d", cfg.Version)
	}

	// Verify it was persisted
	cfg2, err := manager.Load()
	if err != nil {
		t.Fatalf("second Load failed: %v", err)
	}
	if cfg2.Version != 1 {
		t.Errorf("expected Version 1 on reload, got %d", cfg2.Version)
	}
}

func TestConfigManager_PreservesExistingVersion(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	// Write config with version already set
	data := []byte(`{"version": 1, "data_dir": "/tmp/test", "editor": "vim"}`)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Version != 1 {
		t.Errorf("expected Version 1, got %d", cfg.Version)
	}
}

func TestConfigManager_LoadRecoversFromBackup(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")
	backupPath := configPath + ".bak"

	// Write valid backup
	backupData := `{"editor": "vim", "version": 1}`
	if err := os.WriteFile(backupPath, []byte(backupData), 0644); err != nil {
		t.Fatalf("failed to write backup: %v", err)
	}

	// Write corrupted main config
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

	// Main config should now be valid (recovered)
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

	// Both corrupted
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

	// Corrupted config, no backup
	_ = os.WriteFile(configPath, []byte("{bad"), 0644)

	manager := NewManager(configPath)
	_, err := manager.Load()
	if err == nil {
		t.Error("Load should fail when config is corrupted and no backup exists")
	}
}

func TestConfigManager_LoadFiltersInvalidProjects(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	// Config with one valid and two invalid projects
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

func TestConfig_LoadInitializesSectionColors(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	// Write config without section_colors field
	content := `{"editor": "vim"}`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	manager := NewManager(configPath)
	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// SectionColors should be initialized
	if cfg.SectionColors == nil {
		t.Error("SectionColors should be initialized on load")
	}
}
