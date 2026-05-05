package config

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/template"
	"github.com/sglavoie/tt/internal/workflow"
)

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

// Config holds the application configuration.
type Config struct {
	Version                        int                          `json:"version"`
	DataDir                        string                       `json:"data_dir"`
	CurrentProject                 string                       `json:"current_project,omitempty"`
	Editor                         string                       `json:"editor"`
	DefaultContext                 string                       `json:"default_context"`
	FilterContext                  string                       `json:"filter_context,omitempty"`
	FilterCustomContext            string                       `json:"filter_custom_context,omitempty"`
	CustomContexts                 map[string][]string          `json:"custom_contexts,omitempty"`
	FzfEnabled                     bool                         `json:"fzf_enabled"`
	GlowEnabled                    bool                         `json:"glow_enabled"`
	FzfTimeout                     *int                         `json:"fzf_timeout,omitempty"`
	UndoHistoryDepth               *int                         `json:"undo_history_depth,omitempty"`
	PopupTimeoutSeconds            *int                         `json:"popup_timeout_seconds,omitempty"`
	Projects                       []*models.Project            `json:"projects"`
	SectionColors                  map[string]string            `json:"section_colors,omitempty"`
	ProjectViewStates              map[string]ProjectViewState  `json:"project_view_states,omitempty"`
	CycleProjects                  []string                     `json:"cycle_projects,omitempty"`
	ContextCycleProjects           map[string][]string          `json:"context_cycle_projects,omitempty"`
	ShowInfoPanel                  *bool                        `json:"show_info_panel,omitempty"`
	InfoPanelMode                  *int                         `json:"info_panel_mode,omitempty"`
	RecentPaletteItems             []RecentPaletteItem          `json:"recent_palette_items,omitempty"`
	LastPaletteCategory            string                       `json:"last_palette_category,omitempty"`
	PaletteSearchHistory           []string                     `json:"palette_search_history,omitempty"`
	PinnedCommands                 []string                     `json:"pinned_commands,omitempty"`
	SavedSearches                  map[string]string            `json:"saved_searches,omitempty"`
	CommandHistory                 []string                     `json:"command_history,omitempty"`
	WorkflowRules                  []workflow.WorkflowRule      `json:"workflow_rules,omitempty"`
	DefaultRulesInitialized        bool                         `json:"default_rules_initialized,omitempty"`
	WorkflowEvalStates             map[string]WorkflowEvalState `json:"workflow_eval_states,omitempty"`
	WorkflowEvalInterval           *int                         `json:"workflow_eval_interval,omitempty"`
	ReviewDueSoonDays              *int                         `json:"review_due_soon_days,omitempty"`
	ReviewFutureDays               *int                         `json:"review_future_days,omitempty"`
	ReviewOldDays                  *int                         `json:"review_old_days,omitempty"`
	ReviewCompletedHours           *int                         `json:"review_completed_hours,omitempty"`
	LinkedTaskJumpToNew            *bool                        `json:"linked_task_jump_to_new,omitempty"`
	MaxActivityEntries             *int                         `json:"max_activity_entries,omitempty"`
	RecentActivityLimit            *int                         `json:"recent_activity_limit,omitempty"`
	RecentActivityShowAll          *bool                        `json:"recent_activity_show_all,omitempty"`
	WeeklyStaleProjectDays         *int                         `json:"weekly_stale_project_days,omitempty"`
	WeeklyStaleTaskDays            *int                         `json:"weekly_stale_task_days,omitempty"`
	WeeklyInboxThreshold           *int                         `json:"weekly_inbox_threshold,omitempty"`
	WeeklyBlockedThreshold         *int                         `json:"weekly_blocked_threshold,omitempty"`
	WeeklyInProgressThreshold      *int                         `json:"weekly_in_progress_threshold,omitempty"`
	TutorDismissed                 *bool                        `json:"tutor_dismissed,omitempty"`
	AttachmentEditor               string                       `json:"attachment_editor,omitempty"`
	CodeEditor                     string                       `json:"code_editor,omitempty"`
	TaskTemplates                  []template.TaskTemplate      `json:"task_templates,omitempty"`
	DefaultTemplatesInit           bool                         `json:"default_templates_init,omitempty"`
	ScoringProfiles                []scoring.ScoringProfile     `json:"scoring_profiles,omitempty"`
	ProjectScoringProfile          map[string]string            `json:"project_scoring_profile,omitempty"`
	ScoreFilterCustom              *float64                     `json:"score_filter_custom,omitempty"`
	SavedViews                     []savedView                  `json:"saved_views,omitempty"`
	SavedFilters                   []SavedFilter                `json:"saved_filters,omitempty"`
	MaxPinnedBarItems              *int                         `json:"max_pinned_bar_items,omitempty"`
	MaxRecentTaskVisits            *int                         `json:"max_recent_task_visits,omitempty"`
	ActionInboxProjectID           string                       `json:"action_inbox_project_id,omitempty"`
	TUIBackgroundColor             string                       `json:"tui_background_color,omitempty"`
	TUICursorColor                 string                       `json:"tui_cursor_color,omitempty"`
	TUICursorTextColor             string                       `json:"tui_cursor_text_color,omitempty"`
	TUIZebraStriping               *bool                        `json:"tui_zebra_striping,omitempty"`
	Palette                        PaletteColors                `json:"palette,omitzero"`
	ThemeDark                      Theme                        `json:"theme_dark,omitzero"`
	ThemeDarkName                  string                       `json:"theme_dark_name,omitempty"`
	ThemeDarkRev                   int                          `json:"theme_dark_rev,omitempty"`
	ThemeLight                     Theme                        `json:"theme_light,omitzero"`
	ThemeLightName                 string                       `json:"theme_light_name,omitempty"`
	ThemeLightRev                  int                          `json:"theme_light_rev,omitempty"`
	ActiveTheme                    string                       `json:"active_theme,omitempty"`
	DefaultProjectView             string                       `json:"default_project_view,omitempty"`
	FocusQueue                     []FocusQueueItem             `json:"focus_queue,omitempty"`
	DailyPlan                      []DailyPlanItem              `json:"daily_plan,omitempty"`
	FocusDismissedIDs              []string                     `json:"focus_dismissed_ids,omitempty"`
	AlertDismissals                []AlertDismissal             `json:"alert_dismissals,omitempty"`
	RecentTaskVisits               []RecentTaskVisit            `json:"recent_task_visits,omitempty"`
	OutlinerCollapsedSections      map[string]bool              `json:"outliner_collapsed_sections,omitempty"`
	TreeDefaultColumns             map[string]bool              `json:"tree_default_columns,omitempty"`
	OutlinerDefaultColumns         map[string]bool              `json:"outliner_default_columns,omitempty"`
	CrossOutlinerSidebarEnabled    *bool                        `json:"cross_outliner_sidebar_enabled,omitempty"`
	CrossOutlinerCollapsedSections map[string]bool              `json:"cross_outliner_collapsed_sections,omitempty"`
	CrossOutlinerFoldState         map[string]bool              `json:"cross_outliner_fold_state,omitempty"`
	CrossOutlinerColumnToggles     map[string]bool              `json:"cross_outliner_column_toggles,omitempty"`
	CrossOutlinerFilterType        string                       `json:"cross_outliner_filter_type,omitempty"`
	CrossOutlinerFilterStatus      string                       `json:"cross_outliner_filter_status,omitempty"`
	CrossOutlinerFilterPriority    int                          `json:"cross_outliner_filter_priority,omitempty"`
	CrossOutlinerFilterDueDate     int                          `json:"cross_outliner_filter_due_date,omitempty"`
	CrossOutlinerFilterActionable  bool                         `json:"cross_outliner_filter_actionable,omitempty"`
	CrossOutlinerFilterHideDimmed  bool                         `json:"cross_outliner_filter_hide_dimmed,omitempty"`
	CrossOutlinerFilterStage       string                       `json:"cross_outliner_filter_stage,omitempty"`
	CrossOutlinerFilterScoreMin    float64                      `json:"cross_outliner_filter_score_min,omitempty"`
	CrossOutlinerFilterTags        []string                     `json:"cross_outliner_filter_tags,omitempty"`
	CrossOutlinerFilterSections    []string                     `json:"cross_outliner_filter_sections,omitempty"`
	CrossOutlinerFilterAttrs       map[string]string            `json:"cross_outliner_filter_attrs,omitempty"`
	CrossOutlinerSortField         int                          `json:"cross_outliner_sort_field,omitempty"`
	CrossOutlinerSortAscending     bool                         `json:"cross_outliner_sort_ascending"`
	CrossOutlinerFilterBarEnabled  bool                         `json:"cross_outliner_filter_bar_enabled"`
	TreePriorityMode               string                       `json:"tree_priority_mode,omitempty"`
	OutlinerPriorityMode           string                       `json:"outliner_priority_mode,omitempty"`
	Stages                         []string                     `json:"stages,omitempty"`
	SectionLimitActive             *int                         `json:"section_limit_active,omitempty"`
	SectionLimitNext               *int                         `json:"section_limit_next,omitempty"`
	SectionLimitInbox              *int                         `json:"section_limit_inbox,omitempty"`
	SectionLimitStandby            *int                         `json:"section_limit_standby,omitempty"`
	SectionLimitWaiting            *int                         `json:"section_limit_waiting,omitempty"`
	SectionLimitSomeday            *int                         `json:"section_limit_someday,omitempty"`
	SectionLimitReview             *int                         `json:"section_limit_review,omitempty"`
	SmartLists                     []smartList                  `json:"smart_lists,omitempty"`
	AllTasksTabOrder               []string                     `json:"all_tasks_tab_order,omitempty"`
	LastDailyReviewAt              *time.Time                   `json:"last_daily_review_at,omitempty"`
	LastWeeklyReviewAt             *time.Time                   `json:"last_weekly_review_at,omitempty"`
	AlertDailyReviewEnabled        *bool                        `json:"alert_daily_review_enabled,omitempty"`
	AlertWeeklyReviewEnabled       *bool                        `json:"alert_weekly_review_enabled,omitempty"`
	AlertOverdueEnabled            *bool                        `json:"alert_overdue_enabled,omitempty"`
	AlertDueSoonEnabled            *bool                        `json:"alert_due_soon_enabled,omitempty"`
	AlertStaleEnabled              *bool                        `json:"alert_stale_enabled,omitempty"`
	AlertInboxEnabled              *bool                        `json:"alert_inbox_enabled,omitempty"`
	AlertBlockedEnabled            *bool                        `json:"alert_blocked_enabled,omitempty"`
	AlertProjectHealthEnabled      *bool                        `json:"alert_project_health_enabled,omitempty"`
	AlertDueSoonDays               *int                         `json:"alert_due_soon_days,omitempty"`
	AlertStaleDays                 *int                         `json:"alert_stale_days,omitempty"`
	AlertInboxThreshold            *int                         `json:"alert_inbox_threshold,omitempty"`
	AlertBlockedDays               *int                         `json:"alert_blocked_days,omitempty"`
	AlertProjectStaleDays          *int                         `json:"alert_project_stale_days,omitempty"`
	AlertDailyReviewHour           *int                         `json:"alert_daily_review_hour,omitempty"`
	AlertWeeklyReviewDay           *int                         `json:"alert_weekly_review_day,omitempty"`
	WeekEndDay                     string                       `json:"week_end_day,omitempty"`
	TimeFormat                     string                       `json:"time_format,omitempty"`
	KanbanSettings                 KanbanSettings               `json:"kanban_settings,omitzero"`
}
