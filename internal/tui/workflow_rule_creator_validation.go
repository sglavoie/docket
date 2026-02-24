package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/workflow"
)

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

// saveRule validates the form and saves the workflow rule.
func (m *WorkflowRuleCreatorModel) saveRule() tea.Cmd {
	return func() tea.Msg {
		name := strings.TrimSpace(m.nameInput.Value())
		if name == "" {
			return ErrorMsg{Err: fmt.Errorf("rule name is required")}
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
				return ErrorMsg{Err: fmt.Errorf("trigger %d: %w", i+1, err)}
			}
		}

		// Build actions
		actions := make([]workflow.RuleAction, len(m.actions))
		for i, a := range m.actions {
			actions[i] = workflow.RuleAction{
				ActionType: m.actionTypes[a.typeCursor],
				Value:      strings.TrimSpace(a.valueInput.Value()),
			}
			if err := actions[i].Validate(); err != nil {
				return ErrorMsg{Err: fmt.Errorf("action %d: %w", i+1, err)}
			}
		}

		var rule *workflow.WorkflowRule
		if len(triggers) == 1 {
			rule = workflow.NewRule(name, triggers[0], actions)
		} else {
			rule = workflow.NewCompoundRule(name, triggers, m.triggerLogic, actions)
		}

		// Build scope from inputs
		rule.Scope = m.buildScopeFromInputs()

		// Validate the complete rule
		if err := rule.Validate(); err != nil {
			return ErrorMsg{Err: err}
		}

		// If editing, preserve the original ID and update
		if m.editingRule != nil {
			rule.ID = m.editingRule.ID
			rule.Enabled = m.editingRule.Enabled
			rule.TriggerCount = m.editingRule.TriggerCount
			rule.Source = m.editingRule.Source
			m.config.UpdateWorkflowRule(*rule)
		} else {
			m.config.AddWorkflowRule(*rule)
		}

		if err := m.configManager.Save(m.config); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}

		if m.editingRule != nil {
			return WorkflowRuleEditedMsg{Rule: *rule}
		}
		return WorkflowRuleCreatedMsg{Rule: *rule}
	}
}
