package workflow

// RuleLevel indicates whether a rule operates on individual tasks or on projects.
type RuleLevel string

const (
	RuleLevelTask    RuleLevel = "task"
	RuleLevelProject RuleLevel = "project"
)

// TriggerField represents the task field that triggers a rule.
type TriggerField string

const (
	FieldStatus   TriggerField = "status"
	FieldPriority TriggerField = "priority"
	FieldSection  TriggerField = "section"
	FieldTags     TriggerField = "tags"
	FieldType     TriggerField = "type"
	// Time-based trigger fields
	FieldDueDays     TriggerField = "due_days"     // Days until due (negative = overdue)
	FieldCreatedDays TriggerField = "created_days" // Days since creation
	FieldUpdatedDays TriggerField = "updated_days" // Days since last update
	// Event-based trigger fields (detect transitions)
	FieldStatusChanged       TriggerField = "status_changed"       // Value: "old->new", "*->done", "blocked->*"
	FieldSectionChanged      TriggerField = "section_changed"      // Value: "Next->Inbox", "*->Completed"
	FieldPriorityChanged     TriggerField = "priority_changed"     // Value: "*->1", "3->1"
	FieldTagAdded            TriggerField = "tag_added"            // Value: tag name that was added
	FieldTagRemoved          TriggerField = "tag_removed"          // Value: tag name that was removed
	FieldCompleted           TriggerField = "completed"            // Value: "true" (fires on done/canceled)
	FieldReopened            TriggerField = "reopened"             // Value: "true" (fires when reopened)
	FieldDueDateChanged      TriggerField = "due_date_changed"     // Value: "added"/"removed"/"changed"/"*"
	FieldTypeChanged         TriggerField = "type_changed"         // Value: "old->new", "*->task"
	FieldRecurrenceCompleted TriggerField = "recurrence_completed" // Value: "true"
	FieldStreakBroken        TriggerField = "streak_broken"        // Value: "true"
	// Hierarchical trigger fields
	FieldParentStatus     TriggerField = "parent_status"     // Parent task status
	FieldParentType       TriggerField = "parent_type"       // Parent task type (epic, story)
	FieldParentTags       TriggerField = "parent_tags"       // Parent has tag
	FieldLevel            TriggerField = "level"             // Hierarchy depth (0=root)
	FieldHasChildren      TriggerField = "has_children"      // "true"/"false"
	FieldChildrenProgress TriggerField = "children_progress" // Percentage: 0-100
	FieldAllChildrenDone  TriggerField = "all_children_done" // "true" when 100% complete
	FieldIsBlocked        TriggerField = "is_blocked"        // Has incomplete dependencies
	FieldBlockerCount     TriggerField = "blocker_count"     // Number of incomplete blockers
	FieldDependencyCount  TriggerField = "dependency_count"  // Total number of dependencies (DependsOn)
	// Boolean state triggers (absence/presence checks)
	FieldHasDueDate       TriggerField = "has_due_date"      // Value: "true"/"false"
	FieldHasTags          TriggerField = "has_tags"          // Value: "true"/"false"
	FieldDescriptionEmpty TriggerField = "description_empty" // Value: "true"/"false"
	// Project-level trigger fields (aggregate task data)
	FieldHasInProgressTasks TriggerField = "has_in_progress_tasks" // bool: any task is in_progress
	FieldActiveTaskCount    TriggerField = "active_task_count"     // numeric: tasks where Status.IsActive()
	FieldAllTasksStatus     TriggerField = "all_tasks_status"      // string: all non-complete tasks share this status
	FieldMajorityTaskType   TriggerField = "majority_task_type"    // string: >50% of active tasks are this type
	FieldMajorityTaskStatus TriggerField = "majority_task_status"  // string: >50% of active tasks have this status
	FieldCompletionRatio    TriggerField = "completion_ratio"      // numeric 0-100
	FieldDaysSinceUpdate    TriggerField = "days_since_update"     // numeric: max of project/task UpdatedAt
	FieldProjectStatus      TriggerField = "project_status"        // string: current StatusIcon key
	FieldHasTasksWithStatus TriggerField = "has_tasks_with_status" // string: any task has this status
)

// TriggerOperation represents how to compare the field value.
type TriggerOperation string

const (
	OpEquals      TriggerOperation = "equals"
	OpNotEquals   TriggerOperation = "not_equals"
	OpContains    TriggerOperation = "contains"
	OpGreaterThan TriggerOperation = "greater_than"
	OpLessThan    TriggerOperation = "less_than"
)

// ActionType represents what action to take when a rule matches.
type ActionType string

const (
	ActionSetStatus      ActionType = "set_status"
	ActionSetPriority    ActionType = "set_priority"
	ActionSetSection     ActionType = "set_section"
	ActionAddTag         ActionType = "add_tag"
	ActionRemoveTag      ActionType = "remove_tag"
	ActionSetDueRelative ActionType = "set_due_relative"
	ActionClearDue       ActionType = "clear_due"
	ActionSetActive      ActionType = "set_active"
	ActionSetPinned      ActionType = "set_pinned"
	ActionAddComment     ActionType = "add_comment"
	ActionSetType        ActionType = "set_type"
	// Hierarchical actions
	ActionCompleteParent ActionType = "complete_parent" // Mark parent task as done
	ActionBlockParent    ActionType = "block_parent"    // Mark parent task as blocked
	ActionCascadeTag     ActionType = "cascade_tag"     // Add tag to all children
	ActionInheritTag     ActionType = "inherit_tag"     // Copy tag from parent
	ActionNotifyParent   ActionType = "notify_parent"   // Add comment to parent task
	// New actions
	ActionSetSnooze        ActionType = "set_snooze"         // Value: relative duration "+3d"
	ActionCreateFollowup   ActionType = "create_followup"    // Value: title for follow-up task
	ActionMoveToProject    ActionType = "move_to_project"    // Value: project name
	ActionSetRecurrence    ActionType = "set_recurrence"     // Value: recurrence pattern string
	ActionSetProjectStatus ActionType = "set_project_status" // Value: project status icon key
)

// RuleTrigger defines when a rule should fire.
type RuleTrigger struct {
	Field     TriggerField     `json:"field"`
	Operation TriggerOperation `json:"operation"`
	Value     string           `json:"value"`
}

// RuleAction defines what happens when a rule matches.
type RuleAction struct {
	ActionType ActionType   `json:"action_type"`
	Value      string       `json:"value"`
	Condition  *RuleTrigger `json:"condition,omitempty"` // Optional: action only executes if condition matches
}

// TriggerLogic defines how multiple triggers are combined.
type TriggerLogic string

const (
	LogicAnd TriggerLogic = "and" // All triggers must match (default)
	LogicOr  TriggerLogic = "or"  // Any trigger must match
)

// maxTriggerGroupDepth is the maximum nesting depth allowed for trigger groups.
const maxTriggerGroupDepth = 5

// TriggerGroup represents a nested group of triggers with its own logic.
// Supports recursive nesting: groups can contain leaf triggers and sub-groups.
type TriggerGroup struct {
	Logic    TriggerLogic   `json:"logic"`
	Triggers []RuleTrigger  `json:"triggers,omitempty"`
	Groups   []TriggerGroup `json:"groups,omitempty"`
}

// RuleScope restricts a rule to specific projects, task types, or sections.
// Nil scope means no restriction. Non-empty fields use AND logic.
type RuleScope struct {
	Projects []string `json:"projects,omitempty"` // Match by project name or ID
	Types    []string `json:"types,omitempty"`    // Match by task type
	Sections []string `json:"sections,omitempty"` // Match by section
}

// WorkflowRule defines an automation rule.
type WorkflowRule struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Level        RuleLevel     `json:"level,omitempty"` // "task" (default) or "project"
	Enabled      bool          `json:"enabled"`
	Priority     int           `json:"priority,omitempty"`      // Evaluation order: 1=highest (runs first), 0=unset (sorts last)
	Trigger      RuleTrigger   `json:"trigger"`                 // Legacy single trigger (for backward compat)
	Triggers     []RuleTrigger `json:"triggers,omitempty"`      // Multiple triggers
	TriggerLogic TriggerLogic  `json:"trigger_logic,omitempty"` // "and" or "or" (default: "and")
	TriggerGroup *TriggerGroup `json:"trigger_group,omitempty"` // Nested trigger groups (takes precedence over flat Triggers/TriggerLogic)
	Actions      []RuleAction  `json:"actions"`
	TriggerCount int64         `json:"trigger_count,omitempty"`
	Source       string        `json:"source,omitempty"` // "default" for built-in rules, empty for user-created
	Scope        *RuleScope    `json:"scope,omitempty"`  // Optional scope restriction
}
