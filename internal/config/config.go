package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/sglavoie/tt/internal/models"
)

// Config holds the application configuration.
type Config struct {
	DataDir        string            `json:"data_dir"`
	CurrentProject string            `json:"current_project,omitempty"`
	Editor         string            `json:"editor"`
	DefaultContext string            `json:"default_context"`
	FzfEnabled     bool              `json:"fzf_enabled"`
	GlowEnabled    bool              `json:"glow_enabled"`
	Projects       []*models.Project `json:"projects"`
}

// NewConfig creates a new configuration with default values.
func NewConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	return &Config{
		DataDir:        filepath.Join(homeDir, ".tt-data"),
		CurrentProject: "",
		Editor:         "code",
		DefaultContext: "personal",
		FzfEnabled:     true,
		GlowEnabled:    true,
		Projects:       []*models.Project{},
	}
}

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
			configPath = filepath.Join(homeDir, ".tt.json")
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
		config.DataDir = filepath.Join(homeDir, ".tt-data")
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

	return &config, nil
}

// Save saves the configuration to file.
func (m *Manager) Save(config *Config) error {
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
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

// GetProject returns a project by ID or slug.
func (c *Config) GetProject(idOrSlug string) *models.Project {
	for _, p := range c.Projects {
		if p.ID == idOrSlug || p.Slug == idOrSlug || p.Name == idOrSlug {
			return p
		}
	}
	return nil
}

// GetCurrentProject returns the currently selected project.
func (c *Config) GetCurrentProject() *models.Project {
	if c.CurrentProject == "" {
		return nil
	}
	return c.GetProject(c.CurrentProject)
}

// AddProject adds a project to the config.
func (c *Config) AddProject(project *models.Project) {
	c.Projects = append(c.Projects, project)
}

// RemoveProject removes a project by ID.
func (c *Config) RemoveProject(id string) bool {
	for i, p := range c.Projects {
		if p.ID == id {
			c.Projects = append(c.Projects[:i], c.Projects[i+1:]...)
			if c.CurrentProject == id {
				c.CurrentProject = ""
			}
			return true
		}
	}
	return false
}

// GetActiveProjects returns non-archived projects.
func (c *Config) GetActiveProjects() []*models.Project {
	var active []*models.Project
	for _, p := range c.Projects {
		if !p.Archived {
			active = append(active, p)
		}
	}
	return active
}

// GetArchivedProjects returns archived projects.
func (c *Config) GetArchivedProjects() []*models.Project {
	var archived []*models.Project
	for _, p := range c.Projects {
		if p.Archived {
			archived = append(archived, p)
		}
	}
	return archived
}

// SetCurrentProject sets the current project by ID.
func (c *Config) SetCurrentProject(id string) bool {
	project := c.GetProject(id)
	if project == nil {
		return false
	}
	c.CurrentProject = project.ID
	return true
}

// EnsureDataDir creates the data directory if it doesn't exist.
func (c *Config) EnsureDataDir() error {
	return os.MkdirAll(c.DataDir, 0755)
}

// GetProjectPath returns the full path for a project's data directory.
func (c *Config) GetProjectPath(slug string) string {
	return filepath.Join(c.DataDir, slug)
}

// GetAllContexts returns a sorted list of unique context values from all projects.
func (c *Config) GetAllContexts() []string {
	contextSet := make(map[string]bool)
	for _, p := range c.Projects {
		if p.Context != "" {
			contextSet[p.Context] = true
		}
	}
	contexts := make([]string, 0, len(contextSet))
	for ctx := range contextSet {
		contexts = append(contexts, ctx)
	}
	sort.Strings(contexts)
	return contexts
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

	if err := os.WriteFile(backupPath, content, 0644); err != nil {
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
	if err := os.WriteFile(m.configPath, backupContent, 0644); err != nil {
		return fmt.Errorf("failed to restore config file: %w", err)
	}

	// Write current content to backup (for reversibility)
	if err := os.WriteFile(backupPath, currentContent, 0644); err != nil {
		return fmt.Errorf("failed to update config backup file: %w", err)
	}

	return nil
}
