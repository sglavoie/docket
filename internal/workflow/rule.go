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
	ActionSetStatus   ActionType = "set_status"
	ActionSetPriority ActionType = "set_priority"
	ActionSetSection  ActionType = "set_section"
	ActionAddTag      ActionType = "add_tag"
	ActionRemoveTag   ActionType = "remove_tag"
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
	default:
		return fmt.Sprintf("%s: %s", a.ActionType, a.Value)
	}
}

// WorkflowRule defines an automation rule.
type WorkflowRule struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Enabled      bool         `json:"enabled"`
	Trigger      RuleTrigger  `json:"trigger"`
	Actions      []RuleAction `json:"actions"`
	TriggerCount int64        `json:"trigger_count,omitempty"`
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

// Validate checks if the rule is properly configured.
func (r *WorkflowRule) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("rule name is required")
	}
	if err := r.Trigger.Validate(); err != nil {
		return fmt.Errorf("invalid trigger: %w", err)
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
	return fmt.Sprintf("When %s: %s", r.Trigger.String(), strings.Join(actionStrs, ", "))
}

// Validate checks if the trigger is properly configured.
func (t RuleTrigger) Validate() error {
	validFields := map[TriggerField]bool{
		FieldStatus:   true,
		FieldPriority: true,
		FieldSection:  true,
		FieldTags:     true,
		FieldType:     true,
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
		ActionSetStatus:   true,
		ActionSetPriority: true,
		ActionSetSection:  true,
		ActionAddTag:      true,
		ActionRemoveTag:   true,
	}
	if !validActions[a.ActionType] {
		return fmt.Errorf("invalid action type: %s", a.ActionType)
	}

	// set_section with empty value means "Active" (no section)
	if a.ActionType != ActionSetSection && a.Value == "" {
		return fmt.Errorf("action value is required")
	}

	return nil
}

// ValidTriggerFields returns all valid trigger field options.
func ValidTriggerFields() []TriggerField {
	return []TriggerField{FieldStatus, FieldPriority, FieldSection, FieldTags, FieldType}
}

// ValidTriggerOperations returns all valid trigger operation options.
func ValidTriggerOperations() []TriggerOperation {
	return []TriggerOperation{OpEquals, OpNotEquals, OpContains, OpGreaterThan, OpLessThan}
}

// ValidActionTypes returns all valid action type options.
func ValidActionTypes() []ActionType {
	return []ActionType{ActionSetStatus, ActionSetPriority, ActionSetSection, ActionAddTag, ActionRemoveTag}
}
