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
	FilterType                string            `json:"filter_type,omitempty"`
	FilterStatus              string            `json:"filter_status,omitempty"`
	FilterPriority            int               `json:"filter_priority,omitempty"`
	FilterTags                []string          `json:"filter_tags,omitempty"`
	FilterSections            []string          `json:"filter_sections,omitempty"`
	FilterDueDate             int               `json:"filter_due_date,omitempty"`
	FilterActionable          bool              `json:"filter_actionable,omitempty"`  // Actionable filter (F8)
	FilterHideDimmed          bool              `json:"filter_hide_dimmed,omitempty"` // Hide dimmed tasks filter
	FilterStage               string            `json:"filter_stage,omitempty"`       // Tree workflow stage filter
	SortField                 int               `json:"sort_field,omitempty"`
	SortAscending             bool              `json:"sort_ascending"`
	ShowCompleted             bool              `json:"show_completed"`
	SidebarEnabled            bool              `json:"sidebar_enabled"`                       // Sidebar toggle state per project
	FilterBarEnabled          bool              `json:"filter_bar_enabled"`                    // Filter bar toggle state per project
	ViewMode                  int               `json:"view_mode,omitempty"`                   // View mode: 0=Active, 1=Inbox, 2=Backlog, 3=Completed
	CustomSectionTab          string            `json:"custom_section_tab,omitempty"`          // Selected custom section tab name (empty = "All")
	OutlinerFilterType        string            `json:"outliner_filter_type,omitempty"`        // Outliner task type filter
	OutlinerFilterStatus      string            `json:"outliner_filter_status,omitempty"`      // Outliner status filter
	OutlinerFilterPriority    int               `json:"outliner_filter_priority,omitempty"`    // Outliner 0=all, 1=high only
	OutlinerFilterTags        []string          `json:"outliner_filter_tags,omitempty"`        // Outliner tag filter names
	OutlinerFilterDueDate     int               `json:"outliner_filter_due_date,omitempty"`    // Outliner due date filter enum
	OutlinerFilterSections    []string          `json:"outliner_filter_sections,omitempty"`    // Outliner section filter names
	OutlinerFilterActionable  bool              `json:"outliner_filter_actionable,omitempty"`  // Outliner actionable filter
	OutlinerFilterHideDimmed  bool              `json:"outliner_filter_hide_dimmed,omitempty"` // Outliner hide dimmed filter
	OutlinerFilterStage       string            `json:"outliner_filter_stage,omitempty"`       // Outliner workflow stage filter
	OutlinerFilterAttrs       map[string]string `json:"outliner_filter_attrs,omitempty"`       // Outliner attribute filters
	OutlinerFilterScoreMin    float64           `json:"outliner_filter_score_min,omitempty"`   // Outliner score threshold
	TreeColumnToggles         map[string]bool   `json:"tree_column_toggles,omitempty"`         // Per-column visibility overrides for tree view
	OutlinerColumnToggles     map[string]bool   `json:"outliner_column_toggles,omitempty"`     // Per-column visibility overrides for outliner view
	TreePriorityMode          string            `json:"tree_priority_mode,omitempty"`          // Per-project priority display mode for tree view
	OutlinerPriorityMode      string            `json:"outliner_priority_mode,omitempty"`      // Per-project priority display mode for outliner view
	TreeFoldState             map[string]bool   `json:"tree_fold_state,omitempty"`             // Collapsed node IDs in project tree (persists across restarts)
	OutlinerFoldState         map[string]bool   `json:"outliner_fold_state,omitempty"`         // Collapsed node IDs in outliner (persists across restarts)
	OutlinerCollapsedSections map[string]bool   `json:"outliner_collapsed_sections,omitempty"` // Collapsed section headers in outliner (persists across restarts)
	OutlinerSortField         int               `json:"outliner_sort_field,omitempty"`         // Outliner sort field (int encoding of SortField enum)
	OutlinerSortAscending     bool              `json:"outliner_sort_ascending"`               // Outliner sort direction
	OutlinerActivePerspective int               `json:"outliner_active_perspective,omitempty"` // Outliner active perspective index (0-9)
	OutlinerFilterBarEnabled  bool              `json:"outliner_filter_bar_enabled"`           // Outliner filter bar toggle state
}

// savedView represents a named filter+sort+view-mode preset that can be recalled instantly.
// It captures everything defining "what tasks are visible and how they're ordered".
type savedView struct {
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

// Theme groups palette colors together with background and cursor overrides for
// a single named theme (dark or light). Empty strings mean "use ANSI default".
type Theme struct {
	Palette         PaletteColors `json:"palette,omitzero"`
	BackgroundColor string        `json:"background_color,omitempty"`
	CursorColor     string        `json:"cursor_color,omitempty"`
	CursorTextColor string        `json:"cursor_text_color,omitempty"`
}

// PaletteColors holds user-configurable TUI palette overrides.
// Empty strings mean "use ANSI default".
type PaletteColors struct {
	NormalText     string `json:"normal_text,omitempty"`
	Muted          string `json:"muted,omitempty"`
	Dimmed         string `json:"dimmed,omitempty"`
	DimmedBg       string `json:"dimmed_bg,omitempty"`
	DimmedFg       string `json:"dimmed_fg,omitempty"`
	Primary        string `json:"primary,omitempty"`
	Secondary      string `json:"secondary,omitempty"`
	Accent         string `json:"accent,omitempty"`
	Success        string `json:"success,omitempty"`
	Warning        string `json:"warning,omitempty"`
	Error          string `json:"error,omitempty"`
	Border         string `json:"border,omitempty"`
	Priority1Title string `json:"priority1_title,omitempty"`
	Priority2Title string `json:"priority2_title,omitempty"`
	Priority3Title string `json:"priority3_title,omitempty"`
	Priority4Title string `json:"priority4_title,omitempty"`
	Priority5Title string `json:"priority5_title,omitempty"`
}

// Config holds the application configuration.
type Config struct {
	Version                        int                          `json:"version"`
	DataDir                        string                       `json:"data_dir"`
	CurrentProject                 string                       `json:"current_project,omitempty"`
	Editor                         string                       `json:"editor"`
	DefaultContext                 string                       `json:"default_context"`
	FilterContext                  string                       `json:"filter_context,omitempty"`        // Persisted context filter for dashboard/cycling
	FilterCustomContext            string                       `json:"filter_custom_context,omitempty"` // Custom context filter (mutually exclusive with FilterContext)
	CustomContexts                 map[string][]string          `json:"custom_contexts,omitempty"`       // Custom context name → project IDs
	FzfEnabled                     bool                         `json:"fzf_enabled"`
	GlowEnabled                    bool                         `json:"glow_enabled"`
	FzfTimeout                     *int                         `json:"fzf_timeout,omitempty"`           // Fzf invocation timeout in seconds (nil=30)
	UndoHistoryDepth               *int                         `json:"undo_history_depth,omitempty"`    // Max undo snapshots (nil=50, 0 disables, max 1000)
	PopupTimeoutSeconds            *int                         `json:"popup_timeout_seconds,omitempty"` // nil=0 (disabled), seconds before prefix popup auto-dismisses
	Projects                       []*models.Project            `json:"projects"`
	SectionColors                  map[string]string            `json:"section_colors,omitempty"`                    // Global section color registry
	ProjectViewStates              map[string]ProjectViewState  `json:"project_view_states,omitempty"`               // Per-project filter/sort state
	CycleProjects                  []string                     `json:"cycle_projects,omitempty"`                    // Project IDs for quick cycling with ` and ~ (global, when no context filter)
	ContextCycleProjects           map[string][]string          `json:"context_cycle_projects,omitempty"`            // Per-context cycle project IDs (context name -> project IDs)
	ShowInfoPanel                  *bool                        `json:"show_info_panel,omitempty"`                   // Legacy: show info panel section on dashboard (default: true)
	InfoPanelMode                  *int                         `json:"info_panel_mode,omitempty"`                   // Info panel mode: 0=off, 1=info cards, 2=project summary
	RecentPaletteItems             []RecentPaletteItem          `json:"recent_palette_items,omitempty"`              // Recently selected items in command palette
	LastPaletteCategory            string                       `json:"last_palette_category,omitempty"`             // Last category filter used (e.g., ":task ")
	PaletteSearchHistory           []string                     `json:"palette_search_history,omitempty"`            // Recent search queries in command palette
	PinnedCommands                 []string                     `json:"pinned_commands,omitempty"`                   // Pinned command names for quick access
	SavedSearches                  map[string]string            `json:"saved_searches,omitempty"`                    // Named saved searches (name -> query)
	CommandHistory                 []string                     `json:"command_history,omitempty"`                   // Recently executed command names
	WorkflowRules                  []workflow.WorkflowRule      `json:"workflow_rules,omitempty"`                    // Automation rules for task changes
	DefaultRulesInitialized        bool                         `json:"default_rules_initialized,omitempty"`         // True after tt workflow init has run
	WorkflowEvalStates             map[string]WorkflowEvalState `json:"workflow_eval_states,omitempty"`              // Per-project workflow evaluation state
	WorkflowEvalInterval           *int                         `json:"workflow_eval_interval,omitempty"`            // Minutes between auto-evals (nil=60, 0=disabled)
	ReviewDueSoonDays              *int                         `json:"review_due_soon_days,omitempty"`              // nil=7, 0=hide section
	ReviewFutureDays               *int                         `json:"review_future_days,omitempty"`                // nil=30, 0=hide section (tasks due in >= this many days)
	ReviewOldDays                  *int                         `json:"review_old_days,omitempty"`                   // nil=14, 0=hide section (based on UpdatedAt)
	ReviewCompletedHours           *int                         `json:"review_completed_hours,omitempty"`            // nil=24, 0=hide section
	LinkedTaskJumpToNew            *bool                        `json:"linked_task_jump_to_new,omitempty"`           // nil=false: stay on source, true=jump to new
	MaxActivityEntries             *int                         `json:"max_activity_entries,omitempty"`              // nil=50, 0=disabled, max activity log entries per task
	RecentActivityLimit            *int                         `json:"recent_activity_limit,omitempty"`             // nil=20, max tasks in recent activity view
	RecentActivityShowAll          *bool                        `json:"recent_activity_show_all,omitempty"`          // nil=true, show completed tasks too
	WeeklyStaleProjectDays         *int                         `json:"weekly_stale_project_days,omitempty"`         // nil=14, days without updates for stale projects
	WeeklyStaleTaskDays            *int                         `json:"weekly_stale_task_days,omitempty"`            // nil=21, days without updates for stale tasks
	WeeklyInboxThreshold           *int                         `json:"weekly_inbox_threshold,omitempty"`            // nil=5, inbox count threshold
	WeeklyBlockedThreshold         *int                         `json:"weekly_blocked_threshold,omitempty"`          // nil=3, blocked count threshold
	WeeklyInProgressThreshold      *int                         `json:"weekly_in_progress_threshold,omitempty"`      // nil=5, in-progress count threshold
	TutorDismissed                 *bool                        `json:"tutor_dismissed,omitempty"`                   // nil=false, tutor prompt dismissed on dashboard
	AttachmentEditor               string                       `json:"attachment_editor,omitempty"`                 // Editor for general attachments (empty = OS default)
	CodeEditor                     string                       `json:"code_editor,omitempty"`                       // Editor for code references (default: code)
	TaskTemplates                  []template.TaskTemplate      `json:"task_templates,omitempty"`                    // Task templates for quick task creation
	DefaultTemplatesInit           bool                         `json:"default_templates_init,omitempty"`            // True after tt template init has run
	ScoringProfiles                []scoring.ScoringProfile     `json:"scoring_profiles,omitempty"`                  // Available scoring profiles
	ProjectScoringProfile          map[string]string            `json:"project_scoring_profile,omitempty"`           // Project ID → scoring profile ID
	ScoreFilterCustom              *float64                     `json:"score_filter_custom,omitempty"`               // Custom score filter threshold for FS→c
	SavedViews                     []savedView                  `json:"saved_views,omitempty"`                       // Legacy: migrated to SavedFilters
	SavedFilters                   []SavedFilter                `json:"saved_filters,omitempty"`                     // Unified filter presets (snapshot + criteria)
	MaxPinnedBarItems              *int                         `json:"max_pinned_bar_items,omitempty"`              // nil=5, max pinned tasks in status bar
	MaxRecentTaskVisits            *int                         `json:"max_recent_task_visits,omitempty"`            // nil=15, max tasks in quick-switch overlay
	ActionInboxProjectID           string                       `json:"action_inbox_project_id,omitempty"`           // Project ID for focus board action inbox
	TUIBackgroundColor             string                       `json:"tui_background_color,omitempty"`              // HEX background color for TUI (#rgb or #rrggbb, empty = terminal default) — legacy, migrated to ThemeDark
	TUICursorColor                 string                       `json:"tui_cursor_color,omitempty"`                  // HEX background color for cursor/selection rows (empty = #4c0163) — legacy, migrated to ThemeDark
	TUICursorTextColor             string                       `json:"tui_cursor_text_color,omitempty"`             // HEX foreground color for cursor/selection text (empty = #fbeeff) — legacy, migrated to ThemeDark
	TUIZebraStriping               *bool                        `json:"tui_zebra_striping,omitempty"`                // Alternating row backgrounds in task list views (default: false)
	Palette                        PaletteColors                `json:"palette,omitzero"`                            // User-configurable TUI palette overrides — legacy, migrated to ThemeDark
	ThemeDark                      Theme                        `json:"theme_dark,omitzero"`                         // Dark theme palette + bg/cursor colors
	ThemeDarkName                  string                       `json:"theme_dark_name,omitempty"`                   // Name of last loaded/saved dark preset; empty = built-in defaults
	ThemeLight                     Theme                        `json:"theme_light,omitzero"`                        // Light theme palette + bg/cursor colors
	ThemeLightName                 string                       `json:"theme_light_name,omitempty"`                  // Name of last loaded/saved light preset; empty = built-in defaults
	ThemeLightRev                  int                          `json:"theme_light_rev,omitempty"`                   // Revision of ThemeLight defaults; triggers force-refresh when behind LightThemeDefaultsRev
	ActiveTheme                    string                       `json:"active_theme,omitempty"`                      // "dark" or "light"; defaults to "dark"
	DefaultProjectView             string                       `json:"default_project_view,omitempty"`              // Default view when opening a project: outliner, project_tree
	FocusQueue                     []FocusQueueItem             `json:"focus_queue,omitempty"`                       // Persisted focus board work queue
	DailyPlan                      []DailyPlanItem              `json:"daily_plan,omitempty"`                        // Persisted daily plan (tasks planned for today)
	FocusDismissedIDs              []string                     `json:"focus_dismissed_ids,omitempty"`               // Dismissed suggestion task IDs
	AlertDismissals                []AlertDismissal             `json:"alert_dismissals,omitempty"`                  // Dismissed proactive alerts
	RecentTaskVisits               []RecentTaskVisit            `json:"recent_task_visits,omitempty"`                // Recently visited tasks for quick-switch
	OutlinerCollapsedSections      map[string]bool              `json:"outliner_collapsed_sections,omitempty"`       // Default collapsed sections in outliner All perspective
	TreeDefaultColumns             map[string]bool              `json:"tree_default_columns,omitempty"`              // Global default column visibility for tree view
	OutlinerDefaultColumns         map[string]bool              `json:"outliner_default_columns,omitempty"`          // Global default column visibility for outliner view
	CrossOutlinerSidebarEnabled    *bool                        `json:"cross_outliner_sidebar_enabled,omitempty"`    // nil=true; default sidebar state for cross-project outliner
	CrossOutlinerCollapsedSections map[string]bool              `json:"cross_outliner_collapsed_sections,omitempty"` // Persisted collapsed sections for cross-project outliner
	CrossOutlinerFoldState         map[string]bool              `json:"cross_outliner_fold_state,omitempty"`         // Collapsed task IDs in cross-project outliner (persists across restarts)
	CrossOutlinerColumnToggles     map[string]bool              `json:"cross_outliner_column_toggles,omitempty"`     // Per-column visibility overrides for cross-project outliner
	CrossOutlinerFilterType        string                       `json:"cross_outliner_filter_type,omitempty"`        // Cross-project outliner task type filter
	CrossOutlinerFilterStatus      string                       `json:"cross_outliner_filter_status,omitempty"`      // Cross-project outliner status filter
	CrossOutlinerFilterPriority    int                          `json:"cross_outliner_filter_priority,omitempty"`    // Cross-project outliner priority filter
	CrossOutlinerFilterDueDate     int                          `json:"cross_outliner_filter_due_date,omitempty"`    // Cross-project outliner due date filter
	CrossOutlinerFilterActionable  bool                         `json:"cross_outliner_filter_actionable,omitempty"`  // Cross-project outliner actionable filter
	CrossOutlinerFilterHideDimmed  bool                         `json:"cross_outliner_filter_hide_dimmed,omitempty"` // Cross-project outliner hide dimmed filter
	CrossOutlinerFilterStage       string                       `json:"cross_outliner_filter_stage,omitempty"`       // Cross-project outliner stage filter
	CrossOutlinerFilterScoreMin    float64                      `json:"cross_outliner_filter_score_min,omitempty"`   // Cross-project outliner score threshold
	CrossOutlinerFilterTags        []string                     `json:"cross_outliner_filter_tags,omitempty"`        // Cross-project outliner tag filter
	CrossOutlinerFilterSections    []string                     `json:"cross_outliner_filter_sections,omitempty"`    // Cross-project outliner section filter
	CrossOutlinerFilterAttrs       map[string]string            `json:"cross_outliner_filter_attrs,omitempty"`       // Cross-project outliner attribute filters
	CrossOutlinerSortField         int                          `json:"cross_outliner_sort_field,omitempty"`         // Cross-project outliner sort field
	CrossOutlinerSortAscending     bool                         `json:"cross_outliner_sort_ascending"`               // Cross-project outliner sort direction
	CrossOutlinerFilterBarEnabled  bool                         `json:"cross_outliner_filter_bar_enabled"`           // Cross-project outliner filter bar toggle
	TreePriorityMode               string                       `json:"tree_priority_mode,omitempty"`                // Global priority display mode for tree view
	OutlinerPriorityMode           string                       `json:"outliner_priority_mode,omitempty"`            // Global priority display mode for outliner view
	Stages                         []string                     `json:"stages,omitempty"`                            // Workflow stages (e.g., Brainstorm, Design, Implement)
	SectionLimitActive             *int                         `json:"section_limit_active,omitempty"`
	SectionLimitNext               *int                         `json:"section_limit_next,omitempty"`
	SectionLimitInbox              *int                         `json:"section_limit_inbox,omitempty"`
	SectionLimitStandby            *int                         `json:"section_limit_standby,omitempty"`
	SectionLimitWaiting            *int                         `json:"section_limit_waiting,omitempty"`
	SectionLimitSomeday            *int                         `json:"section_limit_someday,omitempty"`
	SectionLimitReview             *int                         `json:"section_limit_review,omitempty"`
	SmartLists                     []smartList                  `json:"smart_lists,omitempty"`         // User-defined smart list tabs for AllTasks view
	AllTasksTabOrder               []string                     `json:"all_tasks_tab_order,omitempty"` // Built-in tab names + smart list UUIDs
	// Proactive alert system: review tracking
	LastDailyReviewAt  *time.Time `json:"last_daily_review_at,omitempty"`
	LastWeeklyReviewAt *time.Time `json:"last_weekly_review_at,omitempty"`

	// Proactive alert system: per-type toggles (nil = enabled)
	AlertDailyReviewEnabled   *bool `json:"alert_daily_review_enabled,omitempty"`
	AlertWeeklyReviewEnabled  *bool `json:"alert_weekly_review_enabled,omitempty"`
	AlertOverdueEnabled       *bool `json:"alert_overdue_enabled,omitempty"`
	AlertDueSoonEnabled       *bool `json:"alert_due_soon_enabled,omitempty"`
	AlertStaleEnabled         *bool `json:"alert_stale_enabled,omitempty"`
	AlertInboxEnabled         *bool `json:"alert_inbox_enabled,omitempty"`
	AlertBlockedEnabled       *bool `json:"alert_blocked_enabled,omitempty"`
	AlertProjectHealthEnabled *bool `json:"alert_project_health_enabled,omitempty"`

	// Proactive alert system: thresholds
	AlertDueSoonDays      *int `json:"alert_due_soon_days,omitempty"`      // nil=3
	AlertStaleDays        *int `json:"alert_stale_days,omitempty"`         // nil=14
	AlertInboxThreshold   *int `json:"alert_inbox_threshold,omitempty"`    // nil=5
	AlertBlockedDays      *int `json:"alert_blocked_days,omitempty"`       // nil=7
	AlertProjectStaleDays *int `json:"alert_project_stale_days,omitempty"` // nil=14

	// Proactive alert system: schedule
	AlertDailyReviewHour *int `json:"alert_daily_review_hour,omitempty"` // nil=9 (hour 0-23)
	AlertWeeklyReviewDay *int `json:"alert_weekly_review_day,omitempty"` // nil=1 (Monday, 0=Sunday)
}

// AlertDismissal records a user-dismissed alert so it can be suppressed until
// it expires or the underlying condition resolves.
type AlertDismissal struct {
	Key         string    `json:"key"`
	Category    string    `json:"category"`
	DismissedAt time.Time `json:"dismissed_at"`
}

// FocusQueueItem stores a task reference for the focus queue.
type FocusQueueItem struct {
	TaskID    string `json:"task_id"`
	ProjectID string `json:"project_id"`
}

// DailyPlanItem stores a task reference for the daily plan.
type DailyPlanItem struct {
	TaskID      string `json:"task_id"`
	ProjectID   string `json:"project_id"`
	PlannedDate string `json:"planned_date"` // YYYY-MM-DD
}

// RecentTaskVisit stores a task reference for the recent task quick-switch list.
type RecentTaskVisit struct {
	TaskID    string `json:"task_id"`
	ProjectID string `json:"project_id"`
	Pinned    bool   `json:"pinned,omitempty"`
}

// NewConfig creates a new configuration with default values.
func NewConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	defaultUndoDepth := 50
	return &Config{
		Version:           1,
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

// EnsureActionInboxProject checks if the action inbox project exists and creates one if not.
// Returns the project and true if a new project was created.
func (c *Config) EnsureActionInboxProject() (*models.Project, bool) {
	// Check if we already have a reference
	if c.ActionInboxProjectID != "" {
		for _, p := range c.Projects {
			if p.ID == c.ActionInboxProjectID {
				return p, false
			}
		}
	}

	// Check if an internal action-inbox project already exists
	for _, p := range c.Projects {
		if p.Internal && p.Slug == "action-inbox" {
			c.ActionInboxProjectID = p.ID
			return p, false
		}
	}

	// Create new action inbox project
	now := time.Now()
	project := &models.Project{
		ID:            "action-inbox-" + now.Format("20060102150405"),
		Slug:          "action-inbox",
		Name:          "Action Inbox",
		Context:       "personal",
		Path:          filepath.Join(c.GetInternalDir(), "action-inbox"),
		CreatedAt:     now,
		UpdatedAt:     now,
		Internal:      true,
		Tasks:         []*models.Task{},
		NextShortID:   1,
		Sections:      []string{models.SectionNext},
		SectionColors: make(map[string]string),
	}
	c.Projects = append(c.Projects, project)
	c.ActionInboxProjectID = project.ID
	return project, true
}

// GetActionInboxProject returns the action inbox project, or nil if none exists.
func (c *Config) GetActionInboxProject() *models.Project {
	if c.ActionInboxProjectID == "" {
		return nil
	}
	for _, p := range c.Projects {
		if p.ID == c.ActionInboxProjectID {
			return p
		}
	}
	return nil
}

// File permission constants (re-exported from util for backward compatibility).
const (
	DirPermissions  = util.DirPermissions
	FilePermissions = util.FilePermissions
)

// DefaultUndoHistoryDepth is the default number of undo snapshots to keep.
const DefaultUndoHistoryDepth = 50

// CurrentConfigVersion is the latest config schema version supported by this build.
const CurrentConfigVersion = 2

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

// GetFzfTimeout returns the fzf invocation timeout in seconds.
// Defaults to 30 if not set; values <= 0 are clamped to 1.
func (c *Config) GetFzfTimeout() int {
	if c.FzfTimeout == nil {
		return 30
	}
	if *c.FzfTimeout <= 0 {
		return 1
	}
	return *c.FzfTimeout
}

// ZebraStriping returns true when alternating row backgrounds are enabled.
// Defaults to false.
func (c *Config) ZebraStriping() bool {
	return c.TUIZebraStriping != nil && *c.TUIZebraStriping
}

// GetPopupTimeoutSeconds returns the auto-dismiss timeout for prefix popups in seconds.
// Returns 0 when nil (disabled — popup stays until manual dismiss).
func (c *Config) GetPopupTimeoutSeconds() int {
	if c.PopupTimeoutSeconds == nil {
		return 0
	}
	return *c.PopupTimeoutSeconds
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

// ActiveThemeConfig returns the Theme for the currently active theme.
// Returns ThemeLight when ActiveTheme == "light", otherwise ThemeDark.
func (c *Config) ActiveThemeConfig() Theme {
	if c.ActiveTheme == "light" {
		return c.ThemeLight
	}
	return c.ThemeDark
}

// SetActiveTheme writes a Theme to the named slot ("dark" or "light").
func (c *Config) SetActiveTheme(name string, t Theme) {
	if name == "light" {
		c.ThemeLight = t
	} else {
		c.ThemeDark = t
	}
}

// isThemeZero reports whether a Theme has no non-empty values.
func isThemeZero(t Theme) bool {
	return t.BackgroundColor == "" && t.CursorColor == "" && t.CursorTextColor == "" &&
		t.Palette == (PaletteColors{})
}

// MigratePaletteToThemes migrates legacy flat palette/bg/cursor fields into the
// new ThemeDark/ThemeLight slots. It follows the same pattern as MigrateLegacyFilters.
// Returns true if any change was made (caller should save the config).
func (c *Config) MigratePaletteToThemes() bool {
	changed := false

	// If ThemeDark is empty and any legacy field is set, copy legacy → ThemeDark.
	legacyHasData := c.TUIBackgroundColor != "" || c.TUICursorColor != "" ||
		c.TUICursorTextColor != "" || c.Palette != (PaletteColors{})
	if isThemeZero(c.ThemeDark) && legacyHasData {
		c.ThemeDark = Theme{
			Palette:         c.Palette,
			BackgroundColor: c.TUIBackgroundColor,
			CursorColor:     c.TUICursorColor,
			CursorTextColor: c.TUICursorTextColor,
		}
		// Zero out legacy fields.
		c.Palette = PaletteColors{}
		c.TUIBackgroundColor = ""
		c.TUICursorColor = ""
		c.TUICursorTextColor = ""
		changed = true
	}

	// Ensure ThemeLight is populated with curated defaults, and refresh when
	// the stored revision is behind the current LightThemeDefaultsRev.
	if isThemeZero(c.ThemeLight) {
		c.ThemeLight = DefaultLightTheme()
		c.ThemeLightRev = LightThemeDefaultsRev
		changed = true
	} else if c.ThemeLightRev < LightThemeDefaultsRev {
		c.ThemeLight = DefaultLightTheme()
		c.ThemeLightName = ""
		c.ThemeLightRev = LightThemeDefaultsRev
		changed = true
	}

	// Ensure ActiveTheme has a value.
	if c.ActiveTheme == "" {
		c.ActiveTheme = "dark"
		changed = true
	}

	return changed
}
