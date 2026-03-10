package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/util"
)

// Manager handles configuration file operations.
type Manager struct {
	configPath string
}

// NewManager creates a new configuration manager.
func NewManager(customPath string) *Manager {
	var configPath string

	if customPath != "" {
		configPath = customPath
	} else {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			configPath = ".tt.json"
		} else {
			configPath = filepath.Join(homeDir, ".tt", ".tt.json")
		}
	}

	return &Manager{
		configPath: configPath,
	}
}

// Load loads the configuration from file.
func (m *Manager) Load() (*Config, error) {
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		return NewConfig(), nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Ensure defaults for missing fields
	if config.DataDir == "" {
		homeDir, _ := os.UserHomeDir()
		config.DataDir = filepath.Join(homeDir, ".tt")
	}
	if config.Editor == "" {
		config.Editor = "code"
	}
	if config.DefaultContext == "" {
		config.DefaultContext = "personal"
	}
	if config.Projects == nil {
		config.Projects = []*models.Project{}
	}
	if config.SectionColors == nil {
		config.SectionColors = make(map[string]string)
	}
	if config.ProjectViewStates == nil {
		config.ProjectViewStates = make(map[string]ProjectViewState)
	}
	if config.UndoHistoryDepth == nil {
		// Default for existing configs that don't have this field
		defaultUndoDepth := DefaultUndoHistoryDepth
		config.UndoHistoryDepth = &defaultUndoDepth
	}

	if config.Stages == nil {
		config.Stages = []string{"Brainstorm", "Design", "Implement", "Test", "Review", "Deploy"}
	}

	// Backfill new fields on builtin scoring profiles from presets
	presets := scoring.PresetProfiles()
	for i := range config.ScoringProfiles {
		p := &config.ScoringProfiles[i]
		if p.Builtin {
			if factory, ok := presets[p.ID]; ok {
				preset := factory()
				if p.UsageHint == "" {
					p.UsageHint = preset.UsageHint
				}
			}
		}
	}

	// Migrate legacy SavedViews and SmartLists to unified SavedFilters.
	if config.MigrateLegacyFilters() {
		// Auto-save so legacy fields are cleared on disk.
		if err := m.Save(&config); err != nil {
			return nil, fmt.Errorf("failed to save config after filter migration: %w", err)
		}
	}

	return &config, nil
}

// Save saves the configuration to file.
func (m *Manager) Save(config *Config) error {
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, DirPermissions); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := util.AtomicWriteFile(m.configPath, data, FilePermissions); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetConfigPath returns the current config file path.
func (m *Manager) GetConfigPath() string {
	return m.configPath
}

// Exists checks if the config file exists.
func (m *Manager) Exists() bool {
	_, err := os.Stat(m.configPath)
	return !os.IsNotExist(err)
}

// LoadOrCreate loads existing config or creates a new one.
func (m *Manager) LoadOrCreate() (*Config, error) {
	config, err := m.Load()
	if err != nil {
		return nil, err
	}

	if !m.Exists() {
		if err := m.Save(config); err != nil {
			return nil, fmt.Errorf("failed to create initial config file: %w", err)
		}
	}

	return config, nil
}

// GetBackupPath returns the path to the config backup file.
func (m *Manager) GetBackupPath() string {
	return m.configPath + ".bak"
}

// ConfigBackupExists checks if a config backup file exists.
func (m *Manager) ConfigBackupExists() bool {
	_, err := os.Stat(m.GetBackupPath())
	return !os.IsNotExist(err)
}

// BackupConfig creates a backup of the current config file.
func (m *Manager) BackupConfig() error {
	backupPath := m.GetBackupPath()

	content, err := os.ReadFile(m.configPath)
	if os.IsNotExist(err) {
		return nil // No config file to backup
	}
	if err != nil {
		return fmt.Errorf("failed to read config file for backup: %w", err)
	}

	if err := util.AtomicWriteFile(backupPath, content, FilePermissions); err != nil {
		return fmt.Errorf("failed to write config backup file: %w", err)
	}

	return nil
}

// RestoreConfigFromBackup swaps the current config file with the backup.
// This makes the operation reversible - running restore again undoes the restore.
func (m *Manager) RestoreConfigFromBackup() error {
	backupPath := m.GetBackupPath()

	// Check if backup exists
	if !m.ConfigBackupExists() {
		return fmt.Errorf("no config backup available")
	}

	// Read backup content
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read config backup file: %w", err)
	}

	// Read current content (may not exist)
	currentContent, err := os.ReadFile(m.configPath)
	if os.IsNotExist(err) {
		currentContent = []byte{}
	} else if err != nil {
		return fmt.Errorf("failed to read current config file: %w", err)
	}

	// Write backup content to main file
	if err := util.AtomicWriteFile(m.configPath, backupContent, FilePermissions); err != nil {
		return fmt.Errorf("failed to restore config file: %w", err)
	}

	// Write current content to backup (for reversibility)
	if err := util.AtomicWriteFile(backupPath, currentContent, FilePermissions); err != nil {
		return fmt.Errorf("failed to update config backup file: %w", err)
	}

	return nil
}
