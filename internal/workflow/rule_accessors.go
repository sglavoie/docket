package workflow

import "github.com/google/uuid"

// NewRule creates a new workflow rule with a generated ID.
func NewRule(name string, trigger RuleTrigger, actions []RuleAction) *WorkflowRule {
	return &WorkflowRule{
		ID:       uuid.New().String()[:8],
		Name:     name,
		Enabled:  true,
		Triggers: []RuleTrigger{trigger},
		Actions:  actions,
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
