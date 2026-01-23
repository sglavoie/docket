package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

// RecentPaletteItem tracks a recently selected item in the command palette.
// Used to persist recent selections across sessions for boosted search results.
type RecentPaletteItem struct {
	Category string `json:"category"` // ItemCategory as string (Commands, Tasks, Projects, Tags, Sections)
	ID       string `json:"id"`       // Command name, task ID, project slug, tag, or section name
}

// WorkflowEvalState tracks when workflow rules were last evaluated for a project.
type WorkflowEvalState struct {
	LastEvalTime time.Time `json:"last_eval_time"`
	RulesHash    string    `json:"rules_hash,omitempty"` // Detects rule changes
}

// ProjectViewState holds filter/sort state for a project's tree view.
type ProjectViewState struct {
	FilterType       string   `json:"filter_type,omitempty"`
	FilterStatus     string   `json:"filter_status,omitempty"`
	FilterPriority   int      `json:"filter_priority,omitempty"`
	FilterTags       []string `json:"filter_tags,omitempty"`
	FilterSections   []string `json:"filter_sections,omitempty"`
	FilterDueDate    int      `json:"filter_due_date,omitempty"`
	FilterActionable bool     `json:"filter_actionable,omitempty"` // Actionable filter (F8)
	SortField        int      `json:"sort_field,omitempty"`
	SortAscending    bool     `json:"sort_ascending"`
	ShowCompleted    bool     `json:"show_completed"`
	SidebarEnabled   bool     `json:"sidebar_enabled"`     // Sidebar toggle state per project
	FilterBarEnabled bool     `json:"filter_bar_enabled"`  // Filter bar toggle state per project
	ViewMode         int      `json:"view_mode,omitempty"` // View mode: 0=Active, 1=Inbox, 2=Backlog, 3=Completed
}

// Config holds the application configuration.
type Config struct {
	DataDir                 string                       `json:"data_dir"`
	CurrentProject          string                       `json:"current_project,omitempty"`
	Editor                  string                       `json:"editor"`
	DefaultContext          string                       `json:"default_context"`
	FzfEnabled              bool                         `json:"fzf_enabled"`
	GlowEnabled             bool                         `json:"glow_enabled"`
	UndoHistoryDepth        *int                         `json:"undo_history_depth,omitempty"` // Max undo snapshots (nil=50, 0 disables, max 1000)
	Projects                []*models.Project            `json:"projects"`
	SectionColors           map[string]string            `json:"section_colors,omitempty"`            // Global section color registry
	ProjectViewStates       map[string]ProjectViewState  `json:"project_view_states,omitempty"`       // Per-project filter/sort state
	CycleProjects           []string                     `json:"cycle_projects,omitempty"`            // Project IDs for quick cycling with ` and ~
	ShowRecentTasks         *bool                        `json:"show_recent_tasks,omitempty"`         // Show Recent Tasks section on dashboard (default: true)
	RecentPaletteItems      []RecentPaletteItem          `json:"recent_palette_items,omitempty"`      // Recently selected items in command palette
	LastPaletteCategory     string                       `json:"last_palette_category,omitempty"`     // Last category filter used (e.g., ":task ")
	PaletteSearchHistory    []string                     `json:"palette_search_history,omitempty"`    // Recent search queries in command palette
	PinnedCommands          []string                     `json:"pinned_commands,omitempty"`           // Pinned command names for quick access
	SavedSearches           map[string]string            `json:"saved_searches,omitempty"`            // Named saved searches (name -> query)
	CommandHistory          []string                     `json:"command_history,omitempty"`           // Recently executed command names
	WorkflowRules           []workflow.WorkflowRule      `json:"workflow_rules,omitempty"`            // Automation rules for task changes
	DefaultRulesInitialized bool                         `json:"default_rules_initialized,omitempty"` // True after tt workflow init has run
	WorkflowEvalStates      map[string]WorkflowEvalState `json:"workflow_eval_states,omitempty"`      // Per-project workflow evaluation state
	WorkflowEvalInterval    *int                         `json:"workflow_eval_interval,omitempty"`    // Minutes between auto-evals (nil=60, 0=disabled)
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

// GetRecentPaletteItems returns the list of recently selected items in the command palette.
func (c *Config) GetRecentPaletteItems() []RecentPaletteItem {
	if c.RecentPaletteItems == nil {
		return []RecentPaletteItem{}
	}
	return c.RecentPaletteItems
}

// SetRecentPaletteItems sets the list of recently selected items in the command palette.
func (c *Config) SetRecentPaletteItems(items []RecentPaletteItem) {
	c.RecentPaletteItems = items
}

// GetLastPaletteCategory returns the last category filter used in the command palette.
func (c *Config) GetLastPaletteCategory() string {
	return c.LastPaletteCategory
}

// SetLastPaletteCategory sets the last category filter used in the command palette.
func (c *Config) SetLastPaletteCategory(category string) {
	c.LastPaletteCategory = category
}

// GetPaletteSearchHistory returns the list of recent search queries in the command palette.
func (c *Config) GetPaletteSearchHistory() []string {
	if c.PaletteSearchHistory == nil {
		return []string{}
	}
	return c.PaletteSearchHistory
}

// SetPaletteSearchHistory sets the list of recent search queries in the command palette.
func (c *Config) SetPaletteSearchHistory(history []string) {
	c.PaletteSearchHistory = history
}

// GetPinnedCommands returns the list of pinned command names.
func (c *Config) GetPinnedCommands() []string {
	if c.PinnedCommands == nil {
		return []string{}
	}
	return c.PinnedCommands
}

// SetPinnedCommands sets the list of pinned command names.
func (c *Config) SetPinnedCommands(commands []string) {
	c.PinnedCommands = commands
}

// IsCommandPinned returns true if the command is in the pinned list.
func (c *Config) IsCommandPinned(name string) bool {
	for _, cmd := range c.PinnedCommands {
		if cmd == name {
			return true
		}
	}
	return false
}

// TogglePinnedCommand adds or removes a command from the pinned list.
// Returns true if the command is now pinned, false if unpinned.
func (c *Config) TogglePinnedCommand(name string) bool {
	for i, cmd := range c.PinnedCommands {
		if cmd == name {
			// Remove from pinned
			c.PinnedCommands = append(c.PinnedCommands[:i], c.PinnedCommands[i+1:]...)
			return false
		}
	}
	// Add to pinned
	c.PinnedCommands = append(c.PinnedCommands, name)
	return true
}

// GetSavedSearches returns the map of saved searches (name -> query).
func (c *Config) GetSavedSearches() map[string]string {
	if c.SavedSearches == nil {
		return make(map[string]string)
	}
	return c.SavedSearches
}

// SaveSearch saves a search query with a given name.
// Overwrites if name already exists.
func (c *Config) SaveSearch(name, query string) {
	if c.SavedSearches == nil {
		c.SavedSearches = make(map[string]string)
	}
	c.SavedSearches[name] = query
}

// DeleteSavedSearch removes a saved search by name.
// Returns true if the search existed and was deleted.
func (c *Config) DeleteSavedSearch(name string) bool {
	if c.SavedSearches == nil {
		return false
	}
	if _, exists := c.SavedSearches[name]; exists {
		delete(c.SavedSearches, name)
		return true
	}
	return false
}

// GetSavedSearch returns the query for a saved search by name.
// Returns empty string if not found.
func (c *Config) GetSavedSearch(name string) string {
	if c.SavedSearches == nil {
		return ""
	}
	return c.SavedSearches[name]
}

// GetCommandHistory returns the list of recently executed commands.
func (c *Config) GetCommandHistory() []string {
	if c.CommandHistory == nil {
		return []string{}
	}
	return c.CommandHistory
}

// AddCommandToHistory adds a command to the execution history.
// Most recent commands are at the front. Duplicates are moved to front.
// Capped at 10 entries.
func (c *Config) AddCommandToHistory(name string) {
	const maxHistory = 10

	// Remove existing entry if present
	var newHistory []string
	for _, cmd := range c.CommandHistory {
		if cmd != name {
			newHistory = append(newHistory, cmd)
		}
	}

	// Add to front
	c.CommandHistory = append([]string{name}, newHistory...)

	// Cap at max
	if len(c.CommandHistory) > maxHistory {
		c.CommandHistory = c.CommandHistory[:maxHistory]
	}
}

// GetWorkflowRules returns all workflow rules.
func (c *Config) GetWorkflowRules() []workflow.WorkflowRule {
	if c.WorkflowRules == nil {
		return []workflow.WorkflowRule{}
	}
	return c.WorkflowRules
}

// GetEnabledWorkflowRules returns only enabled workflow rules.
func (c *Config) GetEnabledWorkflowRules() []workflow.WorkflowRule {
	var enabled []workflow.WorkflowRule
	for _, rule := range c.WorkflowRules {
		if rule.Enabled {
			enabled = append(enabled, rule)
		}
	}
	return enabled
}

// GetWorkflowRulesSorted returns rules sorted: default first, then custom.
// Within each group, rules are sorted alphabetically by name.
func (c *Config) GetWorkflowRulesSorted() (defaultRules, customRules []workflow.WorkflowRule) {
	for _, rule := range c.WorkflowRules {
		if rule.Source == "default" {
			defaultRules = append(defaultRules, rule)
		} else {
			customRules = append(customRules, rule)
		}
	}
	sort.Slice(defaultRules, func(i, j int) bool {
		return defaultRules[i].Name < defaultRules[j].Name
	})
	sort.Slice(customRules, func(i, j int) bool {
		return customRules[i].Name < customRules[j].Name
	})
	return defaultRules, customRules
}

// GetWorkflowRule returns a workflow rule by ID.
func (c *Config) GetWorkflowRule(id string) *workflow.WorkflowRule {
	for i := range c.WorkflowRules {
		if c.WorkflowRules[i].ID == id {
			return &c.WorkflowRules[i]
		}
	}
	return nil
}

// AddWorkflowRule adds a new workflow rule.
func (c *Config) AddWorkflowRule(rule workflow.WorkflowRule) {
	c.WorkflowRules = append(c.WorkflowRules, rule)
}

// UpdateWorkflowRule updates an existing workflow rule by ID.
// Returns true if the rule was found and updated.
func (c *Config) UpdateWorkflowRule(rule workflow.WorkflowRule) bool {
	for i := range c.WorkflowRules {
		if c.WorkflowRules[i].ID == rule.ID {
			c.WorkflowRules[i] = rule
			return true
		}
	}
	return false
}

// DeleteWorkflowRule removes a workflow rule by ID.
// Returns true if the rule was found and deleted.
func (c *Config) DeleteWorkflowRule(id string) bool {
	for i := range c.WorkflowRules {
		if c.WorkflowRules[i].ID == id {
			c.WorkflowRules = append(c.WorkflowRules[:i], c.WorkflowRules[i+1:]...)
			return true
		}
	}
	return false
}

// ToggleWorkflowRule enables or disables a workflow rule by ID.
// Returns the new enabled state, or false if the rule was not found.
func (c *Config) ToggleWorkflowRule(id string) (enabled bool, found bool) {
	for i := range c.WorkflowRules {
		if c.WorkflowRules[i].ID == id {
			c.WorkflowRules[i].Enabled = !c.WorkflowRules[i].Enabled
			return c.WorkflowRules[i].Enabled, true
		}
	}
	return false, false
}

// IncrementRuleTriggerCounts increments trigger counts for rules based on evaluation results.
func (c *Config) IncrementRuleTriggerCounts(counts map[string]int) {
	for i := range c.WorkflowRules {
		if count, ok := counts[c.WorkflowRules[i].ID]; ok {
			c.WorkflowRules[i].TriggerCount += int64(count)
		}
	}
}

// ResetRuleTriggerCount resets the trigger count for a rule by ID.
// Returns true if the rule was found.
func (c *Config) ResetRuleTriggerCount(id string) bool {
	for i := range c.WorkflowRules {
		if c.WorkflowRules[i].ID == id {
			c.WorkflowRules[i].TriggerCount = 0
			return true
		}
	}
	return false
}

// ResetAllRuleTriggerCounts resets trigger counts for all rules.
func (c *Config) ResetAllRuleTriggerCounts() {
	for i := range c.WorkflowRules {
		c.WorkflowRules[i].TriggerCount = 0
	}
}

// HasRuleWithID returns true if a rule with the given ID exists.
func (c *Config) HasRuleWithID(id string) bool {
	for _, rule := range c.WorkflowRules {
		if rule.ID == id {
			return true
		}
	}
	return false
}

// AddDefaultRules adds the built-in default rules that don't already exist.
// If useOriginalEnabled is true, rules keep their original Enabled state from defaults.
// Otherwise, enableRules determines the enabled state.
// Returns the number of rules added.
func (c *Config) AddDefaultRules(enableRules, useOriginalEnabled bool) int {
	defaults := workflow.GetDefaultRules()
	added := 0
	for _, rule := range defaults {
		if !c.HasRuleWithID(rule.ID) {
			if !useOriginalEnabled {
				rule.Enabled = enableRules
			}
			c.WorkflowRules = append(c.WorkflowRules, rule)
			added++
		}
	}
	c.DefaultRulesInitialized = true
	return added
}

// GetWorkflowEvalState returns the evaluation state for a project.
func (c *Config) GetWorkflowEvalState(projectID string) WorkflowEvalState {
	if c.WorkflowEvalStates == nil {
		return WorkflowEvalState{}
	}
	return c.WorkflowEvalStates[projectID]
}

// SetWorkflowEvalState sets the evaluation state for a project.
func (c *Config) SetWorkflowEvalState(projectID string, state WorkflowEvalState) {
	if c.WorkflowEvalStates == nil {
		c.WorkflowEvalStates = make(map[string]WorkflowEvalState)
	}
	c.WorkflowEvalStates[projectID] = state
}

// GetWorkflowEvalInterval returns the interval in minutes between automatic evaluations.
// Returns 60 if not set, 0 means disabled.
func (c *Config) GetWorkflowEvalInterval() int {
	if c.WorkflowEvalInterval == nil {
		return 60 // Default: 60 minutes
	}
	return *c.WorkflowEvalInterval
}

// SetWorkflowEvalInterval sets the interval in minutes between automatic evaluations.
// Set to 0 to disable automatic evaluation.
func (c *Config) SetWorkflowEvalInterval(minutes int) {
	c.WorkflowEvalInterval = &minutes
}

// ComputeWorkflowRulesHash computes a hash of enabled workflow rules.
// Used to detect when rules have changed, forcing re-evaluation.
// Hashes full rule content (not just IDs) to detect in-place edits.
func (c *Config) ComputeWorkflowRulesHash() string {
	var rules []workflow.WorkflowRule
	for _, rule := range c.WorkflowRules {
		if rule.Enabled {
			rules = append(rules, rule)
		}
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })
	data, _ := json.Marshal(rules)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:8]) // First 8 bytes is enough
}

// ShouldEvaluateWorkflow returns true if workflow rules should be evaluated for a project.
// Returns false if no enabled rules, evaluation is disabled, or recently evaluated.
func (c *Config) ShouldEvaluateWorkflow(projectID string) bool {
	if len(c.GetEnabledWorkflowRules()) == 0 {
		return false
	}

	interval := c.GetWorkflowEvalInterval()
	if interval == 0 {
		return false // Disabled
	}

	state := c.GetWorkflowEvalState(projectID)
	currentHash := c.ComputeWorkflowRulesHash()

	// Force evaluation if rules changed
	if state.RulesHash != currentHash {
		return true
	}

	// Never evaluated
	if state.LastEvalTime.IsZero() {
		return true
	}

	// Check time interval
	elapsed := time.Since(state.LastEvalTime)
	return elapsed >= time.Duration(interval)*time.Minute
}

// InvalidateWorkflowEvalStates clears the rules hash from all eval states,
// forcing re-evaluation on next check. Called when rules are modified.
func (c *Config) InvalidateWorkflowEvalStates() {
	if c.WorkflowEvalStates == nil {
		return
	}
	for projectID, state := range c.WorkflowEvalStates {
		state.RulesHash = ""
		c.WorkflowEvalStates[projectID] = state
	}
}
