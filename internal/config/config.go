package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// ProjectViewState holds filter/sort state for a project's tree view.
type ProjectViewState struct {
	FilterType       string   `json:"filter_type,omitempty"`
	FilterStatus     string   `json:"filter_status,omitempty"`
	FilterPriority   int      `json:"filter_priority,omitempty"`
	FilterTags       []string `json:"filter_tags,omitempty"`
	FilterSections   []string `json:"filter_sections,omitempty"`
	FilterDueDate    int      `json:"filter_due_date,omitempty"`
	SortField        int      `json:"sort_field,omitempty"`
	SortAscending    bool     `json:"sort_ascending"`
	ShowCompleted    bool     `json:"show_completed"`
	SidebarEnabled   bool     `json:"sidebar_enabled"`    // Sidebar toggle state per project
	FilterBarEnabled bool     `json:"filter_bar_enabled"` // Filter bar toggle state per project
}

// Config holds the application configuration.
type Config struct {
	DataDir           string                      `json:"data_dir"`
	CurrentProject    string                      `json:"current_project,omitempty"`
	Editor            string                      `json:"editor"`
	DefaultContext    string                      `json:"default_context"`
	FzfEnabled        bool                        `json:"fzf_enabled"`
	GlowEnabled       bool                        `json:"glow_enabled"`
	UndoHistoryDepth  *int                        `json:"undo_history_depth,omitempty"` // Max undo snapshots (nil=50, 0 disables, max 1000)
	Projects          []*models.Project           `json:"projects"`
	SectionColors     map[string]string           `json:"section_colors,omitempty"`      // Global section color registry
	ProjectViewStates map[string]ProjectViewState `json:"project_view_states,omitempty"` // Per-project filter/sort state
	CycleProjects     []string                    `json:"cycle_projects,omitempty"`      // Project IDs for quick cycling with ` and ~
	ShowRecentTasks   *bool                       `json:"show_recent_tasks,omitempty"`   // Show Recent Tasks section on dashboard (default: true)
}

// NewConfig creates a new configuration with default values.
func NewConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	defaultUndoDepth := 50
	return &Config{
		DataDir:           filepath.Join(homeDir, ".tt-data"),
		CurrentProject:    "",
		Editor:            "code",
		DefaultContext:    "personal",
		FzfEnabled:        true,
		GlowEnabled:       true,
		UndoHistoryDepth:  &defaultUndoDepth,
		Projects:          []*models.Project{},
		SectionColors:     make(map[string]string),
		ProjectViewStates: make(map[string]ProjectViewState),
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
	if config.SectionColors == nil {
		config.SectionColors = make(map[string]string)
	}
	if config.ProjectViewStates == nil {
		config.ProjectViewStates = make(map[string]ProjectViewState)
	}
	if config.UndoHistoryDepth == nil {
		// Default for existing configs that don't have this field
		defaultUndoDepth := 50
		config.UndoHistoryDepth = &defaultUndoDepth
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

// sortProjectsByName sorts projects alphabetically by name (case-insensitive).
func sortProjectsByName(projects []*models.Project) {
	sort.Slice(projects, func(i, j int) bool {
		return strings.ToLower(projects[i].Name) < strings.ToLower(projects[j].Name)
	})
}

// sortProjectsWithPinnedFirst sorts projects with pinned ones first, then non-pinned.
// Each group is sorted alphabetically by name (case-insensitive).
func sortProjectsWithPinnedFirst(projects []*models.Project) {
	sort.Slice(projects, func(i, j int) bool {
		if projects[i].Pinned != projects[j].Pinned {
			return projects[i].Pinned
		}
		return strings.ToLower(projects[i].Name) < strings.ToLower(projects[j].Name)
	})
}

// GetActiveProjects returns non-archived projects sorted with pinned first, then alphabetically.
func (c *Config) GetActiveProjects() []*models.Project {
	var active []*models.Project
	for _, p := range c.Projects {
		if !p.Archived {
			active = append(active, p)
		}
	}
	sortProjectsWithPinnedFirst(active)
	return active
}

// GetArchivedProjects returns archived projects sorted alphabetically.
func (c *Config) GetArchivedProjects() []*models.Project {
	var archived []*models.Project
	for _, p := range c.Projects {
		if p.Archived {
			archived = append(archived, p)
		}
	}
	sortProjectsByName(archived)
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

// GetGlobalSectionColor returns the color for a section from the global registry.
// Falls back to "gray" if not found.
func (c *Config) GetGlobalSectionColor(section string) string {
	if c.SectionColors == nil {
		return "gray"
	}
	if color, ok := c.SectionColors[section]; ok && color != "" {
		return color
	}
	return "gray"
}

// SetGlobalSectionColor sets a section color in the global registry.
func (c *Config) SetGlobalSectionColor(section, color string) {
	if c.SectionColors == nil {
		c.SectionColors = make(map[string]string)
	}
	c.SectionColors[section] = color
}

// MigrateProjectSectionColors adds project section colors to global registry
// if they don't already exist (first-seen wins).
func (c *Config) MigrateProjectSectionColors(projectColors map[string]string) {
	if projectColors == nil {
		return
	}
	if c.SectionColors == nil {
		c.SectionColors = make(map[string]string)
	}
	for section, color := range projectColors {
		if _, exists := c.SectionColors[section]; !exists && color != "" && color != "gray" {
			c.SectionColors[section] = color
		}
	}
}

// ApplyGlobalColorsToProject updates a project's section colors from global registry.
func (c *Config) ApplyGlobalColorsToProject(project *models.Project) {
	if project.SectionColors == nil {
		project.SectionColors = make(map[string]string)
	}
	for _, section := range project.Sections {
		if globalColor := c.GetGlobalSectionColor(section); globalColor != "gray" {
			project.SectionColors[section] = globalColor
		}
	}
}

// GetProjectViewState returns the saved view state for a project.
func (c *Config) GetProjectViewState(projectID string) (ProjectViewState, bool) {
	if c.ProjectViewStates == nil {
		return ProjectViewState{}, false
	}
	state, exists := c.ProjectViewStates[projectID]
	return state, exists
}

// SetProjectViewState saves the view state for a project.
func (c *Config) SetProjectViewState(projectID string, state ProjectViewState) {
	if c.ProjectViewStates == nil {
		c.ProjectViewStates = make(map[string]ProjectViewState)
	}
	c.ProjectViewStates[projectID] = state
}

// GetCycleProjects returns active projects that are in the cycle list, in order.
// Filters out archived projects and projects that no longer exist.
func (c *Config) GetCycleProjects() []*models.Project {
	var projects []*models.Project
	for _, id := range c.CycleProjects {
		if p := c.GetProject(id); p != nil && !p.Archived {
			projects = append(projects, p)
		}
	}
	return projects
}

// SetCycleProjects sets the list of project IDs for quick cycling.
func (c *Config) SetCycleProjects(ids []string) {
	c.CycleProjects = ids
}

// GetCycleProjectIndex returns the index of a project in the cycle list, or -1 if not found.
func (c *Config) GetCycleProjectIndex(projectID string) int {
	cycleProjects := c.GetCycleProjects()
	for i, p := range cycleProjects {
		if p.ID == projectID {
			return i
		}
	}
	return -1
}

// GetShowRecentTasks returns whether to show the Recent Tasks section on dashboard.
// Defaults to true if not set.
func (c *Config) GetShowRecentTasks() bool {
	if c.ShowRecentTasks == nil {
		return true
	}
	return *c.ShowRecentTasks
}

// SetShowRecentTasks sets whether to show the Recent Tasks section on dashboard.
func (c *Config) SetShowRecentTasks(show bool) {
	c.ShowRecentTasks = &show
}
