package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/template"
	"github.com/sglavoie/tt/internal/util"
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
	SidebarEnabled   bool     `json:"sidebar_enabled"`              // Sidebar toggle state per project
	FilterBarEnabled bool     `json:"filter_bar_enabled"`           // Filter bar toggle state per project
	ViewMode         int      `json:"view_mode,omitempty"`          // View mode: 0=Active, 1=Inbox, 2=Backlog, 3=Completed
	CustomSectionTab string   `json:"custom_section_tab,omitempty"` // Selected custom section tab name (empty = "All")
}

// SavedView represents a named filter+sort+view-mode preset that can be recalled instantly.
// It captures everything defining "what tasks are visible and how they're ordered".
type SavedView struct {
	Name             string            `json:"name"`
	ViewMode         int               `json:"view_mode,omitempty"`
	FilterType       string            `json:"filter_type,omitempty"`
	FilterStatus     string            `json:"filter_status,omitempty"`
	FilterPriority   int               `json:"filter_priority,omitempty"`
	FilterTags       []string          `json:"filter_tags,omitempty"`
	FilterSections   []string          `json:"filter_sections,omitempty"`
	FilterDueDate    int               `json:"filter_due_date,omitempty"`
	FilterActionable bool              `json:"filter_actionable,omitempty"`
	FilterAttrs      map[string]string `json:"filter_attrs,omitempty"`
	FilterScoreMin   float64           `json:"filter_score_min,omitempty"`
	SortField        int               `json:"sort_field,omitempty"`
	SortAscending    bool              `json:"sort_ascending"`
	CustomSectionTab string            `json:"custom_section_tab,omitempty"`
}

// Config holds the application configuration.
type Config struct {
	DataDir                   string                       `json:"data_dir"`
	CurrentProject            string                       `json:"current_project,omitempty"`
	Editor                    string                       `json:"editor"`
	DefaultContext            string                       `json:"default_context"`
	FilterContext             string                       `json:"filter_context,omitempty"`        // Persisted context filter for dashboard/cycling
	FilterCustomContext       string                       `json:"filter_custom_context,omitempty"` // Custom context filter (mutually exclusive with FilterContext)
	CustomContexts            map[string][]string          `json:"custom_contexts,omitempty"`       // Custom context name → project IDs
	FzfEnabled                bool                         `json:"fzf_enabled"`
	GlowEnabled               bool                         `json:"glow_enabled"`
	UndoHistoryDepth          *int                         `json:"undo_history_depth,omitempty"` // Max undo snapshots (nil=50, 0 disables, max 1000)
	Projects                  []*models.Project            `json:"projects"`
	SectionColors             map[string]string            `json:"section_colors,omitempty"`               // Global section color registry
	ProjectViewStates         map[string]ProjectViewState  `json:"project_view_states,omitempty"`          // Per-project filter/sort state
	CycleProjects             []string                     `json:"cycle_projects,omitempty"`               // Project IDs for quick cycling with ` and ~ (global, when no context filter)
	ContextCycleProjects      map[string][]string          `json:"context_cycle_projects,omitempty"`       // Per-context cycle project IDs (context name -> project IDs)
	ShowInfoPanel             *bool                        `json:"show_info_panel,omitempty"`              // Show info panel section on dashboard (default: true)
	RecentPaletteItems        []RecentPaletteItem          `json:"recent_palette_items,omitempty"`         // Recently selected items in command palette
	LastPaletteCategory       string                       `json:"last_palette_category,omitempty"`        // Last category filter used (e.g., ":task ")
	PaletteSearchHistory      []string                     `json:"palette_search_history,omitempty"`       // Recent search queries in command palette
	PinnedCommands            []string                     `json:"pinned_commands,omitempty"`              // Pinned command names for quick access
	SavedSearches             map[string]string            `json:"saved_searches,omitempty"`               // Named saved searches (name -> query)
	CommandHistory            []string                     `json:"command_history,omitempty"`              // Recently executed command names
	WorkflowRules             []workflow.WorkflowRule      `json:"workflow_rules,omitempty"`               // Automation rules for task changes
	DefaultRulesInitialized   bool                         `json:"default_rules_initialized,omitempty"`    // True after tt workflow init has run
	WorkflowEvalStates        map[string]WorkflowEvalState `json:"workflow_eval_states,omitempty"`         // Per-project workflow evaluation state
	WorkflowEvalInterval      *int                         `json:"workflow_eval_interval,omitempty"`       // Minutes between auto-evals (nil=60, 0=disabled)
	ReviewDueSoonDays         *int                         `json:"review_due_soon_days,omitempty"`         // nil=7, 0=hide section
	ReviewFutureDays          *int                         `json:"review_future_days,omitempty"`           // nil=30, 0=hide section (tasks due in >= this many days)
	ReviewOldDays             *int                         `json:"review_old_days,omitempty"`              // nil=14, 0=hide section (based on UpdatedAt)
	ReviewCompletedHours      *int                         `json:"review_completed_hours,omitempty"`       // nil=24, 0=hide section
	LinkedTaskJumpToNew       *bool                        `json:"linked_task_jump_to_new,omitempty"`      // nil=false: stay on source, true=jump to new
	MaxActivityEntries        *int                         `json:"max_activity_entries,omitempty"`         // nil=50, 0=disabled, max activity log entries per task
	RecentActivityLimit       *int                         `json:"recent_activity_limit,omitempty"`        // nil=20, max tasks in recent activity view
	RecentActivityShowAll     *bool                        `json:"recent_activity_show_all,omitempty"`     // nil=true, show completed tasks too
	WeeklyStaleProjectDays    *int                         `json:"weekly_stale_project_days,omitempty"`    // nil=14, days without updates for stale projects
	WeeklyStaleTaskDays       *int                         `json:"weekly_stale_task_days,omitempty"`       // nil=21, days without updates for stale tasks
	WeeklyInboxThreshold      *int                         `json:"weekly_inbox_threshold,omitempty"`       // nil=5, inbox count threshold
	WeeklyBlockedThreshold    *int                         `json:"weekly_blocked_threshold,omitempty"`     // nil=3, blocked count threshold
	WeeklyInProgressThreshold *int                         `json:"weekly_in_progress_threshold,omitempty"` // nil=5, in-progress count threshold
	TutorDismissed            *bool                        `json:"tutor_dismissed,omitempty"`              // nil=false, tutor prompt dismissed on dashboard
	AttachmentEditor          string                       `json:"attachment_editor,omitempty"`            // Editor for general attachments (empty = OS default)
	CodeEditor                string                       `json:"code_editor,omitempty"`                  // Editor for code references (default: code)
	TaskTemplates             []template.TaskTemplate      `json:"task_templates,omitempty"`               // Task templates for quick task creation
	DefaultTemplatesInit      bool                         `json:"default_templates_init,omitempty"`       // True after tt template init has run
	ScoringProfiles           []scoring.ScoringProfile     `json:"scoring_profiles,omitempty"`             // Available scoring profiles
	ProjectScoringProfile     map[string]string            `json:"project_scoring_profile,omitempty"`      // Project ID → scoring profile ID
	ScoreFilterCustom         *float64                     `json:"score_filter_custom,omitempty"`          // Custom score filter threshold for FS→c
	SavedViews                []SavedView                  `json:"saved_views,omitempty"`                  // Named filter+sort+view-mode presets
}

// NewConfig creates a new configuration with default values.
func NewConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	defaultUndoDepth := 50
	return &Config{
		DataDir:           filepath.Join(homeDir, ".tt"),
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

// EnsureSandboxProject checks if a sandbox project exists and creates one if not.
// Returns true if a new sandbox project was created.
func (c *Config) EnsureSandboxProject() bool {
	for _, p := range c.Projects {
		if p.Sandbox {
			return false
		}
	}
	now := time.Now()
	project := &models.Project{
		ID:            "sandbox-" + now.Format("20060102150405"),
		Slug:          "_sandbox",
		Name:          "Sandbox",
		Context:       "personal",
		Path:          filepath.Join(c.GetInternalDir(), "sandbox"),
		CreatedAt:     now,
		UpdatedAt:     now,
		Sandbox:       true,
		Tasks:         []*models.Task{},
		NextShortID:   1,
		Sections:      []string{models.SectionNext},
		SectionColors: make(map[string]string),
	}
	c.Projects = append(c.Projects, project)
	return true
}

// File permission constants (re-exported from util for backward compatibility).
const (
	DirPermissions  = util.DirPermissions
	FilePermissions = util.FilePermissions
)

// DefaultUndoHistoryDepth is the default number of undo snapshots to keep.
const DefaultUndoHistoryDepth = 50

// GetEditor returns the configured editor.
func (c *Config) GetEditor() string {
	return c.Editor
}

// EnsureDataDir creates the data directory and its standard subdirectories if they don't exist.
func (c *Config) EnsureDataDir() error {
	if err := os.MkdirAll(c.DataDir, DirPermissions); err != nil {
		return err
	}
	if err := os.MkdirAll(c.GetInternalDir(), DirPermissions); err != nil {
		return err
	}
	return os.MkdirAll(filepath.Join(c.DataDir, "projects"), DirPermissions)
}

// GetProjectPath returns the full path for a project's data directory.
func (c *Config) GetProjectPath(slug string) string {
	return filepath.Join(c.DataDir, "projects", slug)
}

// GetInternalDir returns the path to the internal .tt directory within the data directory.
func (c *Config) GetInternalDir() string {
	return filepath.Join(c.DataDir, ".tt")
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

// GetAttachmentEditor returns the configured editor for general attachments.
// Returns empty string if not set (use OS default).
func (c *Config) GetAttachmentEditor() string {
	return c.AttachmentEditor
}

// GetCodeEditor returns the configured editor for code references.
// Defaults to "code" (VS Code) if not set.
func (c *Config) GetCodeEditor() string {
	if c.CodeEditor == "" {
		return "code"
	}
	return c.CodeEditor
}

// GetScoringProfile returns a scoring profile by ID, or nil if not found.
func (c *Config) GetScoringProfile(id string) *scoring.ScoringProfile {
	for i := range c.ScoringProfiles {
		if c.ScoringProfiles[i].ID == id {
			return &c.ScoringProfiles[i]
		}
	}
	return nil
}

// GetProjectScoringProfile returns the active scoring profile for a project, or nil.
func (c *Config) GetProjectScoringProfile(projectID string) *scoring.ScoringProfile {
	if c.ProjectScoringProfile == nil {
		return nil
	}
	profileID, ok := c.ProjectScoringProfile[projectID]
	if !ok {
		return nil
	}
	return c.GetScoringProfile(profileID)
}

// SetProjectScoringProfile assigns a scoring profile to a project.
func (c *Config) SetProjectScoringProfile(projectID, profileID string) {
	if c.ProjectScoringProfile == nil {
		c.ProjectScoringProfile = make(map[string]string)
	}
	c.ProjectScoringProfile[projectID] = profileID
}

// UnsetProjectScoringProfile removes the scoring profile assignment for a project.
func (c *Config) UnsetProjectScoringProfile(projectID string) {
	if c.ProjectScoringProfile != nil {
		delete(c.ProjectScoringProfile, projectID)
	}
}

// AddScoringProfile adds a scoring profile to the config.
// Returns an error if a profile with the same ID already exists.
func (c *Config) AddScoringProfile(p scoring.ScoringProfile) error {
	if c.GetScoringProfile(p.ID) != nil {
		return fmt.Errorf("scoring profile %q already exists", p.ID)
	}
	c.ScoringProfiles = append(c.ScoringProfiles, p)
	return nil
}

// RemoveScoringProfile removes a scoring profile by ID.
// Returns false if the profile was not found.
func (c *Config) RemoveScoringProfile(id string) bool {
	for i := range c.ScoringProfiles {
		if c.ScoringProfiles[i].ID == id {
			c.ScoringProfiles = append(c.ScoringProfiles[:i], c.ScoringProfiles[i+1:]...)
			return true
		}
	}
	return false
}
