package config

import "github.com/sglavoie/tt/internal/workflow"

// Workflow-rule behavior on the AutomationConfig domain aggregate. These mirror
// the equivalent *Config facade methods but depend only on the automation
// domain, so command consumers can read and mutate workflow rules through the
// narrow AutomationStore seam instead of the whole config.Config facade. They
// reuse the package-level helpers (partitionAndSortWorkflowRules,
// rulesAreDifferent) shared with the facade so both produce identical results.

// GetWorkflowRulesSorted returns rules sorted default-first, then custom, each
// group ordered by priority then name.
func (a *AutomationConfig) GetWorkflowRulesSorted() (defaultRules, customRules []workflow.WorkflowRule) {
	return partitionAndSortWorkflowRules(a.WorkflowRules)
}

// GetEnabledWorkflowRules returns only enabled workflow rules.
func (a *AutomationConfig) GetEnabledWorkflowRules() []workflow.WorkflowRule {
	var enabled []workflow.WorkflowRule
	for _, rule := range a.WorkflowRules {
		if rule.Enabled {
			enabled = append(enabled, rule)
		}
	}
	return enabled
}

// GetWorkflowRule returns a pointer to the workflow rule with the given ID, or
// nil. The pointer aliases the backing slice so in-place edits are observed.
func (a *AutomationConfig) GetWorkflowRule(id string) *workflow.WorkflowRule {
	for i := range a.WorkflowRules {
		if a.WorkflowRules[i].ID == id {
			return &a.WorkflowRules[i]
		}
	}
	return nil
}

// HasRuleWithID reports whether a rule with the given ID exists.
func (a *AutomationConfig) HasRuleWithID(id string) bool {
	for _, rule := range a.WorkflowRules {
		if rule.ID == id {
			return true
		}
	}
	return false
}

// AddWorkflowRule appends a new workflow rule.
func (a *AutomationConfig) AddWorkflowRule(rule workflow.WorkflowRule) {
	a.WorkflowRules = append(a.WorkflowRules, rule)
}

// UpdateWorkflowRule replaces an existing rule by ID, returning true when found.
func (a *AutomationConfig) UpdateWorkflowRule(rule workflow.WorkflowRule) bool {
	for i := range a.WorkflowRules {
		if a.WorkflowRules[i].ID == rule.ID {
			a.WorkflowRules[i] = rule
			return true
		}
	}
	return false
}

// DeleteWorkflowRule removes a rule by ID, returning true when found.
func (a *AutomationConfig) DeleteWorkflowRule(id string) bool {
	for i := range a.WorkflowRules {
		if a.WorkflowRules[i].ID == id {
			a.WorkflowRules = append(a.WorkflowRules[:i], a.WorkflowRules[i+1:]...)
			return true
		}
	}
	return false
}

// ToggleWorkflowRule flips a rule's enabled state, returning the new state and
// whether the rule was found.
func (a *AutomationConfig) ToggleWorkflowRule(id string) (enabled bool, found bool) {
	for i := range a.WorkflowRules {
		if a.WorkflowRules[i].ID == id {
			a.WorkflowRules[i].Enabled = !a.WorkflowRules[i].Enabled
			return a.WorkflowRules[i].Enabled, true
		}
	}
	return false, false
}

// ResetRuleTriggerCount resets one rule's trigger count, returning true when
// found.
func (a *AutomationConfig) ResetRuleTriggerCount(id string) bool {
	for i := range a.WorkflowRules {
		if a.WorkflowRules[i].ID == id {
			a.WorkflowRules[i].TriggerCount = 0
			return true
		}
	}
	return false
}

// ResetAllRuleTriggerCounts resets trigger counts for all rules.
func (a *AutomationConfig) ResetAllRuleTriggerCounts() {
	for i := range a.WorkflowRules {
		a.WorkflowRules[i].TriggerCount = 0
	}
}

// AddDefaultRules adds built-in default rules that do not already exist. When
// useOriginalEnabled is true, rules keep their shipped Enabled state; otherwise
// enableRules decides. Returns the number of rules added.
func (a *AutomationConfig) AddDefaultRules(enableRules, useOriginalEnabled bool) int {
	defaults := workflow.GetDefaultRules()
	added := 0
	for _, rule := range defaults {
		if !a.HasRuleWithID(rule.ID) {
			if !useOriginalEnabled {
				rule.Enabled = enableRules
			}
			a.WorkflowRules = append(a.WorkflowRules, rule)
			added++
		}
	}
	a.DefaultRulesInitialized = true
	return added
}

// ResetChangedDefaultRules resets edited or deleted default rules (Source ==
// "default") to their shipped values, mirroring Config.ResetChangedDefaultRules.
// Returns (updated, re-added, unchanged) counts and the affected rule IDs.
func (a *AutomationConfig) ResetChangedDefaultRules(enableRules bool) (updated, readded, unchanged int, affectedIDs []string) {
	defaults := workflow.GetDefaultRules()
	defaultMap := make(map[string]workflow.WorkflowRule)
	for _, rule := range defaults {
		defaultMap[rule.ID] = rule
	}

	// Phase 1: update existing default rules that have changed.
	for i := range a.WorkflowRules {
		existingRule := &a.WorkflowRules[i]
		if existingRule.Source != "default" {
			continue
		}
		shippedRule, exists := defaultMap[existingRule.ID]
		if !exists {
			continue
		}
		if rulesAreDifferent(existingRule, &shippedRule) {
			preservedEnabled := existingRule.Enabled
			*existingRule = shippedRule
			if enableRules {
				existingRule.Enabled = true
			} else {
				existingRule.Enabled = preservedEnabled
			}
			existingRule.TriggerCount = 0
			updated++
			affectedIDs = append(affectedIDs, existingRule.ID)
		} else {
			unchanged++
		}
		delete(defaultMap, existingRule.ID)
	}

	// Phase 2: re-add deleted default rules.
	for _, rule := range defaultMap {
		rule.Enabled = enableRules
		a.WorkflowRules = append(a.WorkflowRules, rule)
		readded++
		affectedIDs = append(affectedIDs, rule.ID)
	}

	return updated, readded, unchanged, affectedIDs
}

// InvalidateWorkflowEvalStates clears the rules hash from every eval state,
// forcing re-evaluation on the next check. Called after rules change.
func (a *AutomationConfig) InvalidateWorkflowEvalStates() {
	if a.WorkflowEvalStates == nil {
		return
	}
	for projectID, state := range a.WorkflowEvalStates {
		state.RulesHash = ""
		a.WorkflowEvalStates[projectID] = state
	}
}
