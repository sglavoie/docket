package workflow

import "github.com/sglavoie/tt/internal/models"

// RuleLevel indicates whether a rule operates on individual tasks or on projects.
type RuleLevel string

const (
	RuleLevelTask    RuleLevel = "task"
	RuleLevelProject RuleLevel = "project"
)

// TriggerField represents the task field that triggers a rule.
type TriggerField string

const (
	// Canonical task fields.
	FieldState    TriggerField = "state"
	FieldPriority TriggerField = "priority"
	FieldTags     TriggerField = "tags"
	FieldType     TriggerField = "type"
	// Time-based trigger fields
	FieldDueDays     TriggerField = "due_days"     // Days until due (negative = overdue)
	FieldCreatedDays TriggerField = "created_days" // Days since creation
	FieldUpdatedDays TriggerField = "updated_days" // Days since last update
	// Event-based trigger fields (detect transitions)
	FieldStateChanged        TriggerField = "state_changed"        // Value: "old->new", "*->done", "waiting,paused->ready"
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
	FieldParentState      TriggerField = "parent_state"      // Parent task workflow state
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
	FieldActiveTaskCount    TriggerField = "active_task_count"     // numeric: nonterminal tasks
	FieldAllTasksState      TriggerField = "all_tasks_state"       // string: all nonterminal tasks share this state
	FieldMajorityTaskType   TriggerField = "majority_task_type"    // string: >50% of active tasks are this type
	FieldMajorityTaskState  TriggerField = "majority_task_state"   // string: >50% of active tasks have this state
	FieldCompletionRatio    TriggerField = "completion_ratio"      // numeric 0-100
	FieldDaysSinceUpdate    TriggerField = "days_since_update"     // numeric: max of project/task UpdatedAt
	FieldProjectStatus      TriggerField = "project_status"        // string: current StatusIcon key
	FieldHasTasksWithState  TriggerField = "has_tasks_with_state"  // string/set: any task has a matching state

	// Legacy lifecycle fields remain decodable until the versioned workflow
	// migration rewrites persisted rules. They are intentionally omitted from
	// authoring metadata returned by ValidTriggerFields.
	FieldStatus             TriggerField = "status"
	FieldSection            TriggerField = "section"
	FieldStatusChanged      TriggerField = "status_changed"
	FieldSectionChanged     TriggerField = "section_changed"
	FieldParentStatus       TriggerField = "parent_status"
	FieldAllTasksStatus     TriggerField = "all_tasks_status"
	FieldMajorityTaskStatus TriggerField = "majority_task_status"
	FieldHasTasksWithStatus TriggerField = "has_tasks_with_status"
)

// TriggerOperation represents how to compare the field value.
type TriggerOperation string

const (
	OpEquals      TriggerOperation = "equals"
	OpNotEquals   TriggerOperation = "not_equals"
	OpContains    TriggerOperation = "contains"
	OpGreaterThan TriggerOperation = "greater_than"
	OpLessThan    TriggerOperation = "less_than"
	OpIn          TriggerOperation = "in"
	OpNotIn       TriggerOperation = "not_in"
)

// ActionType represents what action to take when a rule matches.
type ActionType string

const (
	ActionSetState       ActionType = "set_state"
	ActionSetPriority    ActionType = "set_priority"
	ActionAddTag         ActionType = "add_tag"
	ActionRemoveTag      ActionType = "remove_tag"
	ActionSetDueRelative ActionType = "set_due_relative"
	ActionClearDue       ActionType = "clear_due"
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

	// Legacy lifecycle actions remain decodable until persisted rules migrate.
	// They are intentionally omitted from ValidActionTypes.
	ActionSetStatus  ActionType = "set_status"
	ActionSetSection ActionType = "set_section"
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

// RuleScope restricts a rule to specific projects, task types, or states.
// Nil scope means no restriction. Non-empty fields use AND logic.
type RuleScope struct {
	Projects []string               `json:"projects,omitempty"` // Match by project name or ID
	Types    []string               `json:"types,omitempty"`    // Match by task type
	States   []models.WorkflowState `json:"states,omitempty"`   // Match by canonical workflow state

	// Sections is decode-only compatibility state for persisted legacy rules.
	// The v3-to-v4 domain migration owns converting it to States.
	Sections []string `json:"sections,omitempty"`
}

// WorkflowRule defines an automation rule.
type WorkflowRule struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Level        RuleLevel     `json:"level,omitempty"` // "task" (default) or "project"
	Enabled      bool          `json:"enabled"`
	Priority     int           `json:"priority,omitempty"`      // Evaluation order: 1=highest (runs first), 0=unset (sorts last)
	Trigger      RuleTrigger   `json:"trigger,omitzero"`        // Legacy single trigger (decode compatibility)
	Triggers     []RuleTrigger `json:"triggers,omitempty"`      // Multiple triggers
	TriggerLogic TriggerLogic  `json:"trigger_logic,omitempty"` // "and" or "or" (default: "and")
	TriggerGroup *TriggerGroup `json:"trigger_group,omitempty"` // Nested trigger groups (takes precedence over flat Triggers/TriggerLogic)
	Actions      []RuleAction  `json:"actions"`
	TriggerCount int64         `json:"trigger_count,omitempty"`
	Source       string        `json:"source,omitempty"` // "default" for built-in rules, empty for user-created
	Scope        *RuleScope    `json:"scope,omitempty"`  // Optional scope restriction
}
