package workflow

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
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
	FieldStatusChanged   TriggerField = "status_changed"   // Value: "old->new", "*->done", "blocked->*"
	FieldSectionChanged  TriggerField = "section_changed"  // Value: "Backlog->Active", "*->Archive"
	FieldPriorityChanged TriggerField = "priority_changed" // Value: "*->1", "3->1"
	FieldTagAdded        TriggerField = "tag_added"        // Value: tag name that was added
	FieldTagRemoved      TriggerField = "tag_removed"      // Value: tag name that was removed
	FieldCompleted       TriggerField = "completed"        // Value: "true" (fires on done/cancelled)
	FieldReopened        TriggerField = "reopened"         // Value: "true" (fires when reopened)
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
	ActionType ActionType `json:"action_type"`
	Value      string     `json:"value"`
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
	default:
		return fmt.Sprintf("%s: %s", a.ActionType, a.Value)
	}
}

// TriggerLogic defines how multiple triggers are combined.
type TriggerLogic string

const (
	LogicAnd TriggerLogic = "and" // All triggers must match (default)
	LogicOr  TriggerLogic = "or"  // Any trigger must match
)

// WorkflowRule defines an automation rule.
type WorkflowRule struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Enabled      bool          `json:"enabled"`
	Trigger      RuleTrigger   `json:"trigger"`                 // Legacy single trigger (for backward compat)
	Triggers     []RuleTrigger `json:"triggers,omitempty"`      // Multiple triggers
	TriggerLogic TriggerLogic  `json:"trigger_logic,omitempty"` // "and" or "or" (default: "and")
	Actions      []RuleAction  `json:"actions"`
	TriggerCount int64         `json:"trigger_count,omitempty"`
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

// Validate checks if the rule is properly configured.
func (r *WorkflowRule) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("rule name is required")
	}

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

	if len(r.Actions) == 0 {
		return fmt.Errorf("at least one action is required")
	}
	for i, action := range r.Actions {
		if err := action.Validate(); err != nil {
			return fmt.Errorf("invalid action %d: %w", i+1, err)
		}
	}
	return nil
}

// Summary returns a short description of the rule.
func (r *WorkflowRule) Summary() string {
	var actionStrs []string
	for _, a := range r.Actions {
		actionStrs = append(actionStrs, a.String())
	}

	triggers := r.GetTriggers()
	if len(triggers) == 0 {
		return fmt.Sprintf("(no triggers): %s", strings.Join(actionStrs, ", "))
	}
	if len(triggers) == 1 {
		return fmt.Sprintf("When %s: %s", triggers[0].String(), strings.Join(actionStrs, ", "))
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
	return fmt.Sprintf("When (%s): %s", strings.Join(triggerStrs, logic), strings.Join(actionStrs, ", "))
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
		FieldStatusChanged:   true,
		FieldSectionChanged:  true,
		FieldPriorityChanged: true,
		FieldTagAdded:        true,
		FieldTagRemoved:      true,
		FieldCompleted:       true,
		FieldReopened:        true,
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
		// Hierarchical triggers
		FieldParentStatus, FieldParentType, FieldParentTags, FieldLevel,
		FieldHasChildren, FieldChildrenProgress, FieldAllChildrenDone,
		FieldIsBlocked, FieldBlockerCount,
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
	}
}

// IsEventTrigger returns true if the trigger field requires change detection.
func IsEventTrigger(field TriggerField) bool {
	switch field {
	case FieldStatusChanged, FieldSectionChanged, FieldPriorityChanged,
		FieldTagAdded, FieldTagRemoved, FieldCompleted, FieldReopened:
		return true
	}
	return false
}

// IsHierarchicalTrigger returns true if the trigger requires parent/child data.
func IsHierarchicalTrigger(field TriggerField) bool {
	switch field {
	case FieldParentStatus, FieldParentType, FieldParentTags, FieldLevel,
		FieldHasChildren, FieldChildrenProgress, FieldAllChildrenDone,
		FieldIsBlocked, FieldBlockerCount:
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
