package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sglavoie/tt/internal/logging"
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
			fmt.Fprintf(os.Stderr, "warning: could not determine home directory, using .tt.json in current directory\n")
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
func (m *Manager) Load() (*Config, error) { //nolint:gocyclo
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		return NewConfig(), nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		// Try to recover from backup
		backupPath := m.configPath + ".bak"
		backupData, backupErr := os.ReadFile(backupPath)
		if backupErr == nil {
			var backupConfig Config
			if json.Unmarshal(backupData, &backupConfig) == nil {
				if backupConfig.DataDir != "" && backupConfig.Projects != nil {
					config = backupConfig
					logging.Warn("config file was corrupted, restored from backup %s", backupPath)
					// Persist the recovered config so next load doesn't hit corruption again
					if writeErr := util.AtomicWriteFile(m.configPath, backupData, FilePermissions); writeErr != nil {
						return nil, fmt.Errorf("recovered config from backup but failed to persist it: %w", writeErr)
					}
				} else {
					// Both config and backup are corrupted — require manual recovery.
					return nil, fmt.Errorf("config and backup both corrupted; restore from %s or delete to reset", backupPath)
				}
			} else {
				return nil, fmt.Errorf("failed to parse config file and backup is also corrupted: config error: %w; backup path: %s", err, backupPath)
			}
		} else {
			return nil, fmt.Errorf("failed to parse config file (no backup available): %w", err)
		}
	}

	// Ensure defaults for missing fields
	if config.DataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to determine home directory for data dir: %w", err)
		}
		config.DataDir = filepath.Join(homeDir, ".tt")
	}
	if config.Editor == "" {
		if ed := os.Getenv("EDITOR"); ed != "" {
			config.Editor = ed
		} else if vis := os.Getenv("VISUAL"); vis != "" {
			config.Editor = vis
		} else {
			for _, candidate := range []string{"vi", "nano"} {
				if _, err := exec.LookPath(candidate); err == nil {
					config.Editor = candidate
					break
				}
			}
			if config.Editor == "" {
				config.Editor = "vi" // last resort even if not found
			}
		}
	}
	if _, err := exec.LookPath(config.Editor); err != nil {
		fmt.Fprintf(os.Stderr, "warning: editor '%s' not found in PATH; editing commands may fail\n", config.Editor)
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
	// Clamp UndoHistoryDepth to valid range
	if *config.UndoHistoryDepth < 0 {
		logging.Warn("UndoHistoryDepth %d is negative, clamping to 0", *config.UndoHistoryDepth)
		zero := 0
		config.UndoHistoryDepth = &zero
	} else if *config.UndoHistoryDepth > 1000 {
		logging.Warn("UndoHistoryDepth %d exceeds maximum, clamping to 1000", *config.UndoHistoryDepth)
		max := 1000
		config.UndoHistoryDepth = &max
	}

	// Clamp WorkflowEvalInterval to valid range (0=disabled, max 10080=1 week)
	if config.WorkflowEvalInterval != nil && *config.WorkflowEvalInterval < 0 {
		logging.Warn("WorkflowEvalInterval %d is negative, clamping to 0 (disabled)", *config.WorkflowEvalInterval)
		zero := 0
		config.WorkflowEvalInterval = &zero
	}
	// Clamp ReviewDueSoonDays to valid range (0=hide section)
	if config.ReviewDueSoonDays != nil && *config.ReviewDueSoonDays < 0 {
		logging.Warn("ReviewDueSoonDays %d is negative, clamping to 0", *config.ReviewDueSoonDays)
		zero := 0
		config.ReviewDueSoonDays = &zero
	}
	// Clamp ReviewOldDays to valid range (0=hide section)
	if config.ReviewOldDays != nil && *config.ReviewOldDays < 0 {
		logging.Warn("ReviewOldDays %d is negative, clamping to 0", *config.ReviewOldDays)
		zero := 0
		config.ReviewOldDays = &zero
	}
	// Clamp ReviewFutureDays to valid range (0=hide section)
	if config.ReviewFutureDays != nil && *config.ReviewFutureDays < 0 {
		logging.Warn("ReviewFutureDays %d is negative, clamping to 0", *config.ReviewFutureDays)
		zero := 0
		config.ReviewFutureDays = &zero
	}
	// Clamp ReviewCompletedHours to valid range (0=hide section)
	if config.ReviewCompletedHours != nil && *config.ReviewCompletedHours < 0 {
		logging.Warn("ReviewCompletedHours %d is negative, clamping to 0", *config.ReviewCompletedHours)
		zero := 0
		config.ReviewCompletedHours = &zero
	}

	if config.Stages == nil {
		config.Stages = []string{"Brainstorm", "Design", "Implement", "Test", "Review", "Deploy"}
	}
	if config.CustomContexts == nil {
		config.CustomContexts = make(map[string][]string)
	}
	if config.ProjectScoringProfile == nil {
		config.ProjectScoringProfile = make(map[string]string)
	}
	if config.SavedFilters == nil {
		config.SavedFilters = []SavedFilter{}
	}

	// Filter out projects with missing required fields
	validProjects := make([]*models.Project, 0, len(config.Projects))
	for _, p := range config.Projects {
		if p.ID == "" || p.Name == "" {
			fmt.Fprintf(os.Stderr, "warning: dropping project with missing ID/name from config (ID=%q Name=%q)\n", p.ID, p.Name)
			continue
		}
		validProjects = append(validProjects, p)
	}
	if len(validProjects) != len(config.Projects) {
		config.Projects = validProjects
	}

	// Warn on duplicate project IDs or slugs.
	seenIDs := make(map[string]bool)
	seenSlugs := make(map[string]bool)
	for _, p := range validProjects {
		if seenIDs[p.ID] {
			fmt.Fprintf(os.Stderr, "warning: duplicate project ID %q in config\n", p.ID)
		}
		if seenSlugs[p.Slug] {
			fmt.Fprintf(os.Stderr, "warning: duplicate project slug %q in config\n", p.Slug)
		}
		seenIDs[p.ID] = true
		seenSlugs[p.Slug] = true
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

	// Prune stale data that accumulates over time.
	pruned := config.PruneExpiredAlertDismissals()
	pruned = config.PruneFocusDismissedIDs() || pruned
	if pruned {
		if err := m.Save(&config); err != nil {
			logging.Warn("failed to save config after pruning stale data: %v", err)
		}
	}

	// Config version migration and validation.
	// Strategy: "defaults-on-load" — missing fields are backfilled with defaults
	// above, so older configs gain new fields transparently without a migration.
	// Explicit migration blocks below are only needed for breaking changes that
	// require data transformation (e.g., renaming a field, restructuring data).
	// Each version bump gets a dedicated migration block below.
	// When adding version 2, add a `if config.Version == 1 { ... }` block
	// that transforms the data, then set config.Version = 2 and save.
	// Bump CurrentConfigVersion in config.go to match.
	if config.Version == 0 {
		config.Version = 1
		if err := m.Save(&config); err != nil {
			return nil, fmt.Errorf("failed to save config after version migration: %w", err)
		}
	} else if config.Version > CurrentConfigVersion {
		return nil, fmt.Errorf("config version %d not supported, please upgrade tt", config.Version)
	}

	return &config, nil
}

// Save saves the configuration to file.
// A backup of the existing config is created before writing, enabling
// recovery if the new file is corrupted.
func (m *Manager) Save(config *Config) error {
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, DirPermissions); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	lockPath := m.configPath + ".lock"
	return util.WithFileLock(lockPath, true, func() error {
		// Backup current config before overwriting (best-effort).
		// On failure, try an alternate path before giving up.
		if err := m.BackupConfig(); err != nil {
			altPath := m.configPath + ".bak2"
			if altErr := m.backupConfigToPath(altPath); altErr != nil {
				fmt.Fprintf(os.Stderr, "warning: config backup failed: %v\n", err)
			}
		}

		data, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal config: %w", err)
		}

		if err := util.AtomicWriteFile(m.configPath, data, FilePermissions); err != nil {
			return fmt.Errorf("failed to write config file: %w", err)
		}

		return nil
	})
}

// GetConfigPath returns the current config file path.
func (m *Manager) GetConfigPath() string {
	return m.configPath
}

// Exists checks if the config file exists.
func (m *Manager) Exists() bool {
	_, err := os.Stat(m.configPath)
	return err == nil
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
	return err == nil
}

// backupConfigToPath writes the current config file to an arbitrary backup path.
func (m *Manager) backupConfigToPath(path string) error {
	content, err := os.ReadFile(m.configPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read config file for backup: %w", err)
	}
	if err := util.AtomicWriteFile(path, content, FilePermissions); err != nil {
		return fmt.Errorf("failed to write config backup to %s: %w", path, err)
	}
	return nil
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
