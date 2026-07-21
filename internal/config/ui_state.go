package config

type RecentPaletteItem struct {
	Category string `json:"category"`
	ID       string `json:"id"`
	Count    int    `json:"count,omitempty"`
}

const (
	// CurrentTUIStateVersion versions route and presentation state inside the
	// machine, planning, and settings domains independently from their envelope
	// schema. Each owning domain can therefore migrate and commit atomically.
	CurrentTUIStateVersion = 1

	StartViewHome    = "home"
	StartViewPlanner = "planner"
	StartViewTasks   = "tasks"
	StartViewProject = "project"
	StartViewReview  = "review"
	StartViewManage  = "manage"

	DefaultStartView = StartViewPlanner

	// MaxRecentPaletteItems is the durable cap for the one palette history that
	// survives the TUI-state migration.
	MaxRecentPaletteItems = 10
)

// StartRouteState carries only the route semantics needed to preserve an old
// startup choice while its top-level value is rewritten to one of the six
// canonical destinations. Empty fields mean the destination's normal default.
// It deliberately does not capture arbitrary live view state.
type StartRouteState struct {
	PlannerTab   string `json:"planner_tab,omitempty"`
	TaskLayout   string `json:"task_layout,omitempty"`
	TaskFocus    string `json:"task_focus,omitempty"`
	TaskSort     string `json:"task_sort,omitempty"`
	TaskGroup    string `json:"task_group,omitempty"`
	ReviewModule string `json:"review_module,omitempty"`
}

const (
	StartPlannerToday   = "today"
	StartPlannerBacklog = "backlog"

	StartTaskOutline = "outline"
	StartTaskList    = "list"
	StartTaskActive  = "active"

	StartTaskSortPriority = "priority"
	StartTaskGroupDueDate = "due-date"

	StartReviewDaily = "daily"
)
