package workflow

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

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
	FieldSectionChanged      TriggerField = "section_changed"      // Value: "Backlog->Active", "*->Archive"
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

// String returns a human-readable description of the trigger.
func (t RuleTrigger) String() string {
	opStr := string(t.Operation)
	switch t.Operation {
	case OpEquals:
		opStr = "="
	case OpNotEquals:
		opStr = "!="
	case OpContains:
		opStr = "contains"
	case OpGreaterThan:
		opStr = ">"
	case OpLessThan:
		opStr = "<"
	}
	return fmt.Sprintf("%s %s %s", t.Field, opStr, t.Value)
}

// RuleAction defines what happens when a rule matches.
type RuleAction struct {
	ActionType ActionType   `json:"action_type"`
	Value      string       `json:"value"`
	Condition  *RuleTrigger `json:"condition,omitempty"` // Optional: action only executes if condition matches
}

// String returns a human-readable description of the action.
func (a RuleAction) String() string {
	switch a.ActionType {
	case ActionSetStatus:
		return fmt.Sprintf("set status to %s", a.Value)
	case ActionSetPriority:
		return fmt.Sprintf("set priority to %s", a.Value)
	case ActionSetSection:
		if a.Value == "" {
			return "move to Active"
		}
		return fmt.Sprintf("move to %s", a.Value)
	case ActionAddTag:
		return fmt.Sprintf("add tag #%s", a.Value)
	case ActionRemoveTag:
		return fmt.Sprintf("remove tag #%s", a.Value)
	case ActionSetDueRelative:
		return fmt.Sprintf("set due date %s", a.Value)
	case ActionClearDue:
		return "clear due date"
	case ActionSetActive:
		if a.Value == "true" {
			return "set active"
		}
		return "clear active"
	case ActionSetPinned:
		if a.Value == "true" {
			return "pin task"
		}
		return "unpin task"
	case ActionAddComment:
		return fmt.Sprintf("add comment: %s", a.Value)
	case ActionSetType:
		return fmt.Sprintf("set type to %s", a.Value)
	case ActionCompleteParent:
		return "mark parent as done"
	case ActionBlockParent:
		return "mark parent as blocked"
	case ActionCascadeTag:
		return fmt.Sprintf("add tag #%s to children", a.Value)
	case ActionInheritTag:
		return fmt.Sprintf("inherit tag #%s from parent", a.Value)
	case ActionNotifyParent:
		return fmt.Sprintf("notify parent: %s", a.Value)
	case ActionSetSnooze:
		return fmt.Sprintf("snooze for %s", a.Value)
	case ActionCreateFollowup:
		return fmt.Sprintf("create follow-up: %s", a.Value)
	case ActionMoveToProject:
		return fmt.Sprintf("move to project %s", a.Value)
	case ActionSetRecurrence:
		return fmt.Sprintf("set recurrence to %s", a.Value)
	case ActionSetProjectStatus:
		if icon := models.ProjectStatusIconByKey(a.Value); icon != nil {
			return fmt.Sprintf("set project status to %s %s", icon.Emoji, icon.Label)
		}
		return fmt.Sprintf("set project status to %s", a.Value)
	default:
		return fmt.Sprintf("%s: %s", a.ActionType, a.Value)
	}
}

// stringWithCondition returns the action string with condition suffix if present.
func (a RuleAction) stringWithCondition() string {
	s := a.String()
	if a.Condition != nil {
		s += fmt.Sprintf(" (if %s)", a.Condition.String())
	}
	return s
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

// NewRule creates a new workflow rule with a generated ID.
func NewRule(name string, trigger RuleTrigger, actions []RuleAction) *WorkflowRule {
	return &WorkflowRule{
		ID:      uuid.New().String()[:8],
		Name:    name,
		Enabled: true,
		Trigger: trigger,
		Actions: actions,
	}
}

// NewCompoundRule creates a new workflow rule with multiple triggers.
func NewCompoundRule(name string, triggers []RuleTrigger, logic TriggerLogic, actions []RuleAction) *WorkflowRule {
	return &WorkflowRule{
		ID:           uuid.New().String()[:8],
		Name:         name,
		Enabled:      true,
		Triggers:     triggers,
		TriggerLogic: logic,
		Actions:      actions,
	}
}

// GetTriggers returns all triggers for the rule.
// Handles both legacy single Trigger and new Triggers slice.
func (r *WorkflowRule) GetTriggers() []RuleTrigger {
	if len(r.Triggers) > 0 {
		return r.Triggers
	}
	// Legacy: single trigger
	if r.Trigger.Field != "" {
		return []RuleTrigger{r.Trigger}
	}
	return nil
}

// GetTriggerLogic returns the trigger logic, defaulting to AND.
func (r *WorkflowRule) GetTriggerLogic() TriggerLogic {
	if r.TriggerLogic == LogicOr {
		return LogicOr
	}
	return LogicAnd
}

// IsCompound returns true if the rule has multiple triggers.
func (r *WorkflowRule) IsCompound() bool {
	return len(r.Triggers) > 1
}

// HasTriggerGroup returns true if the rule uses nested trigger groups.
func (r *WorkflowRule) HasTriggerGroup() bool {
	return r.TriggerGroup != nil
}

// GetLevel returns the rule's level, defaulting to task when empty.
func (r *WorkflowRule) GetLevel() RuleLevel {
	if r.Level == RuleLevelProject {
		return RuleLevelProject
	}
	return RuleLevelTask
}

// IsProjectLevel returns true if this rule operates at the project level.
func (r *WorkflowRule) IsProjectLevel() bool {
	return r.GetLevel() == RuleLevelProject
}

// GetTriggerGroup returns the canonical trigger group for the rule.
// If TriggerGroup is set, it is returned directly.
// Otherwise, a group is synthesized from the flat Triggers/TriggerLogic fields.
func (r *WorkflowRule) GetTriggerGroup() *TriggerGroup {
	if r.TriggerGroup != nil {
		return r.TriggerGroup
	}
	triggers := r.GetTriggers()
	if len(triggers) == 0 {
		return nil
	}
	return &TriggerGroup{
		Logic:    r.GetTriggerLogic(),
		Triggers: triggers,
	}
}

// Validate checks that a TriggerGroup is properly configured.
func (g *TriggerGroup) Validate() error {
	return g.validateAtDepth(0)
}

func (g *TriggerGroup) validateAtDepth(depth int) error {
	if depth >= maxTriggerGroupDepth {
		return fmt.Errorf("trigger group nesting exceeds maximum depth of %d", maxTriggerGroupDepth)
	}
	if g.Logic != LogicAnd && g.Logic != LogicOr && g.Logic != "" {
		return fmt.Errorf("invalid trigger group logic: %s (valid: and, or)", g.Logic)
	}
	if len(g.Triggers) == 0 && len(g.Groups) == 0 {
		return fmt.Errorf("trigger group must contain at least one trigger or sub-group")
	}
	for i, t := range g.Triggers {
		if err := t.Validate(); err != nil {
			return fmt.Errorf("trigger %d in group: %w", i+1, err)
		}
	}
	for i, sub := range g.Groups {
		if err := sub.validateAtDepth(depth + 1); err != nil {
			return fmt.Errorf("sub-group %d: %w", i+1, err)
		}
	}
	return nil
}

// Summary returns a human-readable expression for the trigger group.
func (g *TriggerGroup) Summary() string {
	logic := g.getLogic()
	var parts []string
	for _, t := range g.Triggers {
		parts = append(parts, t.String())
	}
	for _, sub := range g.Groups {
		parts = append(parts, "("+sub.Summary()+")")
	}
	joiner := " AND "
	if logic == LogicOr {
		joiner = " OR "
	}
	return strings.Join(parts, joiner)
}

// getLogic returns the trigger group's logic, defaulting to AND.
func (g *TriggerGroup) getLogic() TriggerLogic {
	if g.Logic == LogicOr {
		return LogicOr
	}
	return LogicAnd
}

// Validate checks if the rule is properly configured.
func (r *WorkflowRule) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("rule name is required")
	}

	// Validate trigger group if present, otherwise validate flat triggers
	if r.TriggerGroup != nil {
		if err := r.TriggerGroup.Validate(); err != nil {
			return fmt.Errorf("invalid trigger group: %w", err)
		}
	} else {
		triggers := r.GetTriggers()
		if len(triggers) == 0 {
			return fmt.Errorf("at least one trigger is required")
		}
		for i, trigger := range triggers {
			if err := trigger.Validate(); err != nil {
				if len(triggers) > 1 {
					return fmt.Errorf("invalid trigger %d: %w", i+1, err)
				}
				return fmt.Errorf("invalid trigger: %w", err)
			}
		}

		if r.TriggerLogic != "" && r.TriggerLogic != LogicAnd && r.TriggerLogic != LogicOr {
			return fmt.Errorf("invalid trigger logic: %s (valid: and, or)", r.TriggerLogic)
		}
	}

	if len(r.Actions) == 0 {
		return fmt.Errorf("at least one action is required")
	}
	for i, action := range r.Actions {
		if err := action.Validate(); err != nil {
			return fmt.Errorf("invalid action %d: %w", i+1, err)
		}
	}

	if err := r.validateScope(); err != nil {
		return fmt.Errorf("invalid scope: %w", err)
	}

	if err := r.validateLevel(); err != nil {
		return err
	}

	return nil
}

// Summary returns a short description of the rule.
func (r *WorkflowRule) Summary() string {
	var actionStrs []string
	for _, a := range r.Actions {
		actionStrs = append(actionStrs, a.stringWithCondition())
	}

	var prefix string
	if r.Priority > 0 {
		prefix = fmt.Sprintf("[P%d] ", r.Priority)
	}

	// Use TriggerGroup summary if present
	if r.TriggerGroup != nil {
		return fmt.Sprintf("%sWhen (%s): %s", prefix, r.TriggerGroup.Summary(), strings.Join(actionStrs, ", "))
	}

	triggers := r.GetTriggers()
	if len(triggers) == 0 {
		return fmt.Sprintf("%s(no triggers): %s", prefix, strings.Join(actionStrs, ", "))
	}
	if len(triggers) == 1 {
		return fmt.Sprintf("%sWhen %s: %s", prefix, triggers[0].String(), strings.Join(actionStrs, ", "))
	}

	// Compound triggers
	var triggerStrs []string
	for _, t := range triggers {
		triggerStrs = append(triggerStrs, t.String())
	}
	logic := " AND "
	if r.GetTriggerLogic() == LogicOr {
		logic = " OR "
	}
	return fmt.Sprintf("%sWhen (%s): %s", prefix, strings.Join(triggerStrs, logic), strings.Join(actionStrs, ", "))
}

// Validate checks if the trigger is properly configured.
func (t RuleTrigger) Validate() error {
	validFields := map[TriggerField]bool{
		FieldStatus:      true,
		FieldPriority:    true,
		FieldSection:     true,
		FieldTags:        true,
		FieldType:        true,
		FieldDueDays:     true,
		FieldCreatedDays: true,
		FieldUpdatedDays: true,
		// Event-based triggers
		FieldStatusChanged:       true,
		FieldSectionChanged:      true,
		FieldPriorityChanged:     true,
		FieldTagAdded:            true,
		FieldTagRemoved:          true,
		FieldCompleted:           true,
		FieldReopened:            true,
		FieldDueDateChanged:      true,
		FieldTypeChanged:         true,
		FieldRecurrenceCompleted: true,
		FieldStreakBroken:        true,
		// Boolean state triggers
		FieldHasDueDate:       true,
		FieldHasTags:          true,
		FieldDescriptionEmpty: true,
		// Hierarchical triggers
		FieldParentStatus:     true,
		FieldParentType:       true,
		FieldParentTags:       true,
		FieldLevel:            true,
		FieldHasChildren:      true,
		FieldChildrenProgress: true,
		FieldAllChildrenDone:  true,
		FieldIsBlocked:        true,
		FieldBlockerCount:     true,
		FieldDependencyCount:  true,
		// Project-level triggers
		FieldHasInProgressTasks: true,
		FieldActiveTaskCount:    true,
		FieldAllTasksStatus:     true,
		FieldMajorityTaskType:   true,
		FieldMajorityTaskStatus: true,
		FieldCompletionRatio:    true,
		FieldDaysSinceUpdate:    true,
		FieldProjectStatus:      true,
		FieldHasTasksWithStatus: true,
	}
	if !validFields[t.Field] {
		return fmt.Errorf("invalid field: %s", t.Field)
	}

	validOps := map[TriggerOperation]bool{
		OpEquals:      true,
		OpNotEquals:   true,
		OpContains:    true,
		OpGreaterThan: true,
		OpLessThan:    true,
	}
	if !validOps[t.Operation] {
		return fmt.Errorf("invalid operation: %s", t.Operation)
	}

	if t.Value == "" {
		return fmt.Errorf("trigger value is required")
	}

	return nil
}

// Validate checks if the action is properly configured.
func (a RuleAction) Validate() error {
	validActions := map[ActionType]bool{
		ActionSetStatus:      true,
		ActionSetPriority:    true,
		ActionSetSection:     true,
		ActionAddTag:         true,
		ActionRemoveTag:      true,
		ActionSetDueRelative: true,
		ActionClearDue:       true,
		ActionSetActive:      true,
		ActionSetPinned:      true,
		ActionAddComment:     true,
		ActionSetType:        true,
		// Hierarchical actions
		ActionCompleteParent: true,
		ActionBlockParent:    true,
		ActionCascadeTag:     true,
		ActionInheritTag:     true,
		ActionNotifyParent:   true,
		// New actions
		ActionSetSnooze:        true,
		ActionCreateFollowup:   true,
		ActionMoveToProject:    true,
		ActionSetRecurrence:    true,
		ActionSetProjectStatus: true,
	}
	if !validActions[a.ActionType] {
		return fmt.Errorf("invalid action type: %s", a.ActionType)
	}

	// Actions that don't require a value
	noValueRequired := map[ActionType]bool{
		ActionSetSection:     true, // empty value means "Active" (no section)
		ActionClearDue:       true, // no value needed
		ActionCompleteParent: true, // no value needed
		ActionBlockParent:    true, // no value needed
	}

	if !noValueRequired[a.ActionType] && a.Value == "" {
		return fmt.Errorf("action value is required")
	}

	// Validate specific action value formats
	switch a.ActionType {
	case ActionSetActive, ActionSetPinned:
		if a.Value != "true" && a.Value != "false" {
			return fmt.Errorf("%s value must be 'true' or 'false'", a.ActionType)
		}
	case ActionSetDueRelative:
		if _, err := ParseRelativeDuration(a.Value); err != nil {
			return fmt.Errorf("invalid relative duration: %s (use format like +3d, +1w, +2m)", a.Value)
		}
	case ActionSetSnooze:
		if _, err := ParseRelativeDuration(a.Value); err != nil {
			return fmt.Errorf("invalid snooze duration: %s (use format like +3d, +1w, +2m)", a.Value)
		}
	case ActionSetRecurrence:
		if models.ParseRecurrence(a.Value) == nil {
			return fmt.Errorf("invalid recurrence pattern: %s (use format like daily, weekly, 2d, 3w)", a.Value)
		}
	case ActionSetProjectStatus:
		if models.ProjectStatusIconByKey(a.Value) == nil {
			return fmt.Errorf("invalid project status icon key: %s", a.Value)
		}
	}

	// Validate embedded condition if present
	if a.Condition != nil {
		if err := a.Condition.Validate(); err != nil {
			return fmt.Errorf("invalid action condition: %w", err)
		}
	}

	return nil
}

// ParseRelativeDuration parses a relative duration string like "+3d", "+1w", "+2m".
// Returns the duration in hours. Supported suffixes: d (days), w (weeks), m (months), y (years).
func ParseRelativeDuration(s string) (int, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("duration too short")
	}

	// Handle optional + prefix
	if s[0] == '+' {
		s = s[1:]
	}

	if len(s) < 2 {
		return 0, fmt.Errorf("duration too short")
	}

	// Parse the number
	numStr := s[:len(s)-1]
	unit := s[len(s)-1]

	num := 0
	for _, c := range numStr {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid number: %s", numStr)
		}
		num = num*10 + int(c-'0')
	}

	if num <= 0 {
		return 0, fmt.Errorf("duration must be positive")
	}

	// Convert to days for storage
	switch unit {
	case 'd':
		return num, nil
	case 'w':
		return num * 7, nil
	case 'm':
		return num * 30, nil // approximate
	case 'y':
		return num * 365, nil // approximate
	default:
		return 0, fmt.Errorf("invalid unit: %c (use d, w, m, or y)", unit)
	}
}

// ValidTriggerFields returns all valid trigger field options.
func ValidTriggerFields() []TriggerField {
	return []TriggerField{
		FieldStatus, FieldPriority, FieldSection, FieldTags, FieldType,
		FieldDueDays, FieldCreatedDays, FieldUpdatedDays,
		// Event-based triggers
		FieldStatusChanged, FieldSectionChanged, FieldPriorityChanged,
		FieldTagAdded, FieldTagRemoved, FieldCompleted, FieldReopened,
		FieldDueDateChanged, FieldTypeChanged, FieldRecurrenceCompleted, FieldStreakBroken,
		// Boolean state triggers
		FieldHasDueDate, FieldHasTags, FieldDescriptionEmpty,
		// Hierarchical triggers
		FieldParentStatus, FieldParentType, FieldParentTags, FieldLevel,
		FieldHasChildren, FieldChildrenProgress, FieldAllChildrenDone,
		FieldIsBlocked, FieldBlockerCount, FieldDependencyCount,
		// Project-level triggers
		FieldHasInProgressTasks, FieldActiveTaskCount, FieldAllTasksStatus,
		FieldMajorityTaskType, FieldMajorityTaskStatus, FieldCompletionRatio,
		FieldDaysSinceUpdate, FieldProjectStatus, FieldHasTasksWithStatus,
	}
}

// ValidTriggerOperations returns all valid trigger operation options.
func ValidTriggerOperations() []TriggerOperation {
	return []TriggerOperation{OpEquals, OpNotEquals, OpContains, OpGreaterThan, OpLessThan}
}

// ValidActionTypes returns all valid action type options.
func ValidActionTypes() []ActionType {
	return []ActionType{
		ActionSetStatus, ActionSetPriority, ActionSetSection,
		ActionAddTag, ActionRemoveTag,
		ActionSetDueRelative, ActionClearDue,
		ActionSetActive, ActionSetPinned,
		ActionAddComment, ActionSetType,
		// Hierarchical actions
		ActionCompleteParent, ActionBlockParent, ActionCascadeTag,
		ActionInheritTag, ActionNotifyParent,
		// New actions
		ActionSetSnooze, ActionCreateFollowup, ActionMoveToProject, ActionSetRecurrence,
		// Project-level actions
		ActionSetProjectStatus,
	}
}

// IsEventTrigger returns true if the trigger field requires change detection.
func IsEventTrigger(field TriggerField) bool {
	switch field {
	case FieldStatusChanged, FieldSectionChanged, FieldPriorityChanged,
		FieldTagAdded, FieldTagRemoved, FieldCompleted, FieldReopened,
		FieldDueDateChanged, FieldTypeChanged, FieldRecurrenceCompleted, FieldStreakBroken:
		return true
	}
	return false
}

// IsHierarchicalTrigger returns true if the trigger requires parent/child data.
func IsHierarchicalTrigger(field TriggerField) bool {
	switch field {
	case FieldParentStatus, FieldParentType, FieldParentTags, FieldLevel,
		FieldHasChildren, FieldChildrenProgress, FieldAllChildrenDone,
		FieldIsBlocked, FieldBlockerCount, FieldDependencyCount:
		return true
	}
	return false
}

// IsHierarchicalAction returns true if the action operates on parent/children.
func IsHierarchicalAction(actionType ActionType) bool {
	switch actionType {
	case ActionCompleteParent, ActionBlockParent, ActionCascadeTag,
		ActionInheritTag, ActionNotifyParent:
		return true
	}
	return false
}

// IsProjectTrigger returns true if the trigger field is a project-level aggregate field.
func IsProjectTrigger(field TriggerField) bool {
	switch field {
	case FieldHasInProgressTasks, FieldActiveTaskCount, FieldAllTasksStatus,
		FieldMajorityTaskType, FieldMajorityTaskStatus, FieldCompletionRatio,
		FieldDaysSinceUpdate, FieldProjectStatus, FieldHasTasksWithStatus:
		return true
	}
	return false
}

// MatchesScope checks whether a task falls within the rule's scope.
// A nil scope matches everything. Non-empty fields use AND logic.
func (r *WorkflowRule) MatchesScope(task *models.Task, projectName, projectID string) bool {
	if r.Scope == nil {
		return true
	}

	if len(r.Scope.Projects) > 0 {
		matched := false
		for _, p := range r.Scope.Projects {
			if strings.EqualFold(p, projectName) || p == projectID {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	if len(r.Scope.Types) > 0 {
		matched := false
		taskType := string(task.Type)
		for _, t := range r.Scope.Types {
			if strings.EqualFold(t, taskType) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	if len(r.Scope.Sections) > 0 {
		matched := false
		for _, s := range r.Scope.Sections {
			if strings.EqualFold(s, task.Section) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// validateScope checks that the rule scope contains valid values.
func (r *WorkflowRule) validateScope() error {
	if r.Scope == nil {
		return nil
	}

	for _, t := range r.Scope.Types {
		if !models.TaskType(strings.ToLower(t)).IsValid() {
			return fmt.Errorf("invalid task type in scope: %s", t)
		}
	}

	for _, p := range r.Scope.Projects {
		if strings.TrimSpace(p) == "" {
			return fmt.Errorf("empty project name in scope")
		}
	}

	for _, s := range r.Scope.Sections {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("empty section name in scope")
		}
	}

	return nil
}

// validateLevel checks that triggers and actions match the rule's level.
func (r *WorkflowRule) validateLevel() error {
	level := r.GetLevel()

	// Collect all leaf triggers (from trigger group or flat list)
	var triggers []RuleTrigger
	if r.TriggerGroup != nil {
		triggers = collectTriggers(r.TriggerGroup)
	} else {
		triggers = r.GetTriggers()
	}

	if level == RuleLevelProject {
		for _, t := range triggers {
			if !IsProjectTrigger(t.Field) {
				return fmt.Errorf("project-level rule %q has task-level trigger field %q", r.Name, t.Field)
			}
		}
		for _, a := range r.Actions {
			if a.ActionType != ActionSetProjectStatus {
				return fmt.Errorf("project-level rule %q has non-project action %q", r.Name, a.ActionType)
			}
		}
	} else {
		for _, t := range triggers {
			if IsProjectTrigger(t.Field) {
				return fmt.Errorf("task-level rule %q has project-level trigger field %q", r.Name, t.Field)
			}
		}
		for _, a := range r.Actions {
			if a.ActionType == ActionSetProjectStatus {
				return fmt.Errorf("task-level rule %q has project-level action %q", r.Name, a.ActionType)
			}
		}
	}
	return nil
}

// collectTriggers recursively collects all leaf triggers from a trigger group.
func collectTriggers(group *TriggerGroup) []RuleTrigger {
	var result []RuleTrigger
	result = append(result, group.Triggers...)
	for i := range group.Groups {
		result = append(result, collectTriggers(&group.Groups[i])...)
	}
	return result
}
