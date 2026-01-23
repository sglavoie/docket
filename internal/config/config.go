package config

import (
	"os"
	"path/filepath"
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
	ReviewDueSoonDays       *int                         `json:"review_due_soon_days,omitempty"`      // nil=7, 0=hide section
	ReviewFutureDays        *int                         `json:"review_future_days,omitempty"`        // nil=30, 0=hide section (tasks due in >= this many days)
	ReviewOldDays           *int                         `json:"review_old_days,omitempty"`           // nil=14, 0=hide section (based on UpdatedAt)
	ReviewCompletedHours    *int                         `json:"review_completed_hours,omitempty"`    // nil=24, 0=hide section
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

// EnsureDataDir creates the data directory if it doesn't exist.
func (c *Config) EnsureDataDir() error {
	return os.MkdirAll(c.DataDir, 0755)
}

// GetProjectPath returns the full path for a project's data directory.
func (c *Config) GetProjectPath(slug string) string {
	return filepath.Join(c.DataDir, slug)
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
