package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/workflow"
)

// buildTriggerGroupFromForm builds a TriggerGroup from the form state.
// The root group uses triggerLogic. Sub-groups use their own logic.
func (m *WorkflowRuleCreatorModel) buildTriggerGroupFromForm(triggers []workflow.RuleTrigger) *workflow.TriggerGroup {
	root := &workflow.TriggerGroup{
		Logic: m.triggerLogic,
	}

	// Separate ungrouped and grouped triggers
	grouped := make(map[int][]workflow.RuleTrigger)
	for i, t := range m.triggers {
		if t.groupIndex < 0 {
			root.Triggers = append(root.Triggers, triggers[i])
		} else {
			grouped[t.groupIndex] = append(grouped[t.groupIndex], triggers[i])
		}
	}

	// Build sub-groups
	for gi := 0; gi < len(m.triggerGroups); gi++ {
		members := grouped[gi]
		if len(members) == 0 {
			continue
		}
		sub := workflow.TriggerGroup{
			Logic:    m.triggerGroups[gi].logic,
			Triggers: members,
		}
		root.Groups = append(root.Groups, sub)
	}

	return root
}

// buildScopeFromInputs parses the scope text inputs into a RuleScope.
// Returns nil if all inputs are empty.
func (m *WorkflowRuleCreatorModel) buildScopeFromInputs() *workflow.RuleScope {
	projects := parseCommaSeparatedInput(m.scopeProjectsInput.Value())
	types := parseCommaSeparatedInput(m.scopeTypesInput.Value())
	sections := parseCommaSeparatedInput(m.scopeSectionsInput.Value())

	if len(projects) == 0 && len(types) == 0 && len(sections) == 0 {
		return nil
	}
	return &workflow.RuleScope{
		Projects: projects,
		Types:    types,
		Sections: sections,
	}
}

// parseCommaSeparatedInput splits a comma-separated string into trimmed, non-empty values.
func parseCommaSeparatedInput(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// buildRuleFromForm constructs a WorkflowRule from the current form state.
// Returns the rule and any validation error.
func (m *WorkflowRuleCreatorModel) buildRuleFromForm() (*workflow.WorkflowRule, error) {
	name := strings.TrimSpace(m.nameInput.Value())
	if name == "" {
		return nil, fmt.Errorf("rule name is required")
	}

	// Build triggers
	triggers := make([]workflow.RuleTrigger, len(m.triggers))
	for i, t := range m.triggers {
		triggers[i] = workflow.RuleTrigger{
			Field:     m.triggerFields[t.fieldCursor],
			Operation: m.triggerOps[t.opCursor],
			Value:     strings.TrimSpace(t.valueInput.Value()),
		}
		if err := triggers[i].Validate(); err != nil {
			return nil, fmt.Errorf("trigger %d: %w", i+1, err)
		}
	}

	// Build actions
	actions := make([]workflow.RuleAction, len(m.actions))
	for i, a := range m.actions {
		actions[i] = workflow.RuleAction{
			ActionType: m.actionTypes[a.typeCursor],
			Value:      strings.TrimSpace(a.valueInput.Value()),
		}
		// Build condition if enabled
		if a.hasCondition {
			actions[i].Condition = &workflow.RuleTrigger{
				Field:     m.triggerFields[a.condFieldCursor],
				Operation: m.triggerOps[a.condOpCursor],
				Value:     strings.TrimSpace(a.condValueInput.Value()),
			}
		}
		if err := actions[i].Validate(); err != nil {
			return nil, fmt.Errorf("action %d: %w", i+1, err)
		}
	}

	// Parse priority
	priority := 0
	if v := strings.TrimSpace(m.priorityInput.Value()); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil || p < 0 || p > 99 {
			return nil, fmt.Errorf("priority must be 0-99")
		}
		priority = p
	}

	var rule *workflow.WorkflowRule
	if len(m.triggerGroups) > 0 {
		// Build nested TriggerGroup
		tg := m.buildTriggerGroupFromForm(triggers)
		rule = workflow.NewCompoundRule(name, nil, m.triggerLogic, actions)
		rule.TriggerGroup = tg
		rule.Triggers = nil
		rule.Trigger = workflow.RuleTrigger{}
	} else if len(triggers) == 1 {
		rule = workflow.NewRule(name, triggers[0], actions)
	} else {
		rule = workflow.NewCompoundRule(name, triggers, m.triggerLogic, actions)
	}
	rule.Priority = priority

	// Build scope from inputs
	rule.Scope = m.buildScopeFromInputs()

	// Validate the complete rule
	if err := rule.Validate(); err != nil {
		return nil, err
	}

	return rule, nil
}

// saveRule validates the form and saves the workflow rule. The rule is applied
// to a cloned automation snapshot and persisted through the coordinator over the
// runtime, which commits only the automation domain (no whole-config save) and
// refreshes the in-memory snapshot on success.
func (m *WorkflowRuleCreatorModel) saveRule() tea.Cmd {
	rule, err := m.buildRuleFromForm()
	if err != nil {
		return func() tea.Msg { return ErrorMsg{Err: err} }
	}

	automation := cloneAutomationConfig(m.runtime.Automation())

	// If editing, preserve the original ID and update
	isEdit := m.editingRule != nil
	if isEdit {
		rule.ID = m.editingRule.ID
		rule.Enabled = m.editingRule.Enabled
		rule.TriggerCount = m.editingRule.TriggerCount
		rule.Source = m.editingRule.Source
		automation.UpdateWorkflowRule(*rule)
	} else {
		automation.AddWorkflowRule(*rule)
	}

	savedRule := *rule
	return m.coordinator.SaveThen(automation, func() tea.Msg {
		if isEdit {
			return WorkflowRuleEditedMsg{Rule: savedRule}
		}
		return WorkflowRuleCreatedMsg{Rule: savedRule}
	})
}
