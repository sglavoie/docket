package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/workflow"
)

// GetWorkflowRules returns all workflow rules.
func (c *Config) GetWorkflowRules() []workflow.WorkflowRule {
	if c.WorkflowRules == nil {
		return []workflow.WorkflowRule{}
	}
	return c.WorkflowRules
}

// GetEnabledWorkflowRules returns only enabled workflow rules.
func (c *Config) GetEnabledWorkflowRules() []workflow.WorkflowRule {
	var enabled []workflow.WorkflowRule
	for _, rule := range c.WorkflowRules {
		if rule.Enabled {
			enabled = append(enabled, rule)
		}
	}
	return enabled
}

// GetWorkflowRulesSorted returns rules sorted: default first, then custom.
// Within each group, rules are sorted by priority (non-zero ascending, then zero),
// then alphabetically by name within the same priority.
func (c *Config) GetWorkflowRulesSorted() (defaultRules, customRules []workflow.WorkflowRule) {
	for _, rule := range c.WorkflowRules {
		if rule.Source == "default" {
			defaultRules = append(defaultRules, rule)
		} else {
			customRules = append(customRules, rule)
		}
	}
	sortByPriorityThenName := func(rules []workflow.WorkflowRule) {
		sort.SliceStable(rules, func(i, j int) bool {
			pi, pj := rules[i].Priority, rules[j].Priority
			// Both zero: sort alphabetically
			if pi == 0 && pj == 0 {
				return rules[i].Name < rules[j].Name
			}
			// Zero sorts after non-zero
			if pi == 0 {
				return false
			}
			if pj == 0 {
				return true
			}
			// Same non-zero priority: sort alphabetically
			if pi == pj {
				return rules[i].Name < rules[j].Name
			}
			return pi < pj
		})
	}
	sortByPriorityThenName(defaultRules)
	sortByPriorityThenName(customRules)
	return defaultRules, customRules
}

// GetWorkflowRule returns a workflow rule by ID.
func (c *Config) GetWorkflowRule(id string) *workflow.WorkflowRule {
	for i := range c.WorkflowRules {
		if c.WorkflowRules[i].ID == id {
			return &c.WorkflowRules[i]
		}
	}
	return nil
}

// AddWorkflowRule adds a new workflow rule.
func (c *Config) AddWorkflowRule(rule workflow.WorkflowRule) {
	c.WorkflowRules = append(c.WorkflowRules, rule)
}

// UpdateWorkflowRule updates an existing workflow rule by ID.
// Returns true if the rule was found and updated.
func (c *Config) UpdateWorkflowRule(rule workflow.WorkflowRule) bool {
	for i := range c.WorkflowRules {
		if c.WorkflowRules[i].ID == rule.ID {
			c.WorkflowRules[i] = rule
			return true
		}
	}
	return false
}

// DeleteWorkflowRule removes a workflow rule by ID.
// Returns true if the rule was found and deleted.
func (c *Config) DeleteWorkflowRule(id string) bool {
	for i := range c.WorkflowRules {
		if c.WorkflowRules[i].ID == id {
			c.WorkflowRules = append(c.WorkflowRules[:i], c.WorkflowRules[i+1:]...)
			return true
		}
	}
	return false
}

// ToggleWorkflowRule enables or disables a workflow rule by ID.
// Returns the new enabled state, or false if the rule was not found.
func (c *Config) ToggleWorkflowRule(id string) (enabled bool, found bool) {
	for i := range c.WorkflowRules {
		if c.WorkflowRules[i].ID == id {
			c.WorkflowRules[i].Enabled = !c.WorkflowRules[i].Enabled
			return c.WorkflowRules[i].Enabled, true
		}
	}
	return false, false
}

// IncrementRuleTriggerCounts increments trigger counts for rules based on evaluation results.
func (c *Config) IncrementRuleTriggerCounts(counts map[string]int) {
	for i := range c.WorkflowRules {
		if count, ok := counts[c.WorkflowRules[i].ID]; ok {
			c.WorkflowRules[i].TriggerCount += int64(count)
		}
	}
}

// ResetRuleTriggerCount resets the trigger count for a rule by ID.
// Returns true if the rule was found.
func (c *Config) ResetRuleTriggerCount(id string) bool {
	for i := range c.WorkflowRules {
		if c.WorkflowRules[i].ID == id {
			c.WorkflowRules[i].TriggerCount = 0
			return true
		}
	}
	return false
}

// ResetAllRuleTriggerCounts resets trigger counts for all rules.
func (c *Config) ResetAllRuleTriggerCounts() {
	for i := range c.WorkflowRules {
		c.WorkflowRules[i].TriggerCount = 0
	}
}

// HasRuleWithID returns true if a rule with the given ID exists.
func (c *Config) HasRuleWithID(id string) bool {
	for _, rule := range c.WorkflowRules {
		if rule.ID == id {
			return true
		}
	}
	return false
}

// AddDefaultRules adds the built-in default rules that don't already exist.
// If useOriginalEnabled is true, rules keep their original Enabled state from defaults.
// Otherwise, enableRules determines the enabled state.
// Returns the number of rules added.
func (c *Config) AddDefaultRules(enableRules, useOriginalEnabled bool) int {
	defaults := workflow.GetDefaultRules()
	added := 0
	for _, rule := range defaults {
		if !c.HasRuleWithID(rule.ID) {
			if !useOriginalEnabled {
				rule.Enabled = enableRules
			}
			c.WorkflowRules = append(c.WorkflowRules, rule)
			added++
		}
	}
	c.DefaultRulesInitialized = true
	return added
}

// ResetChangedDefaultRules resets any default rules (Source == "default")
// that have been edited or deleted to their shipped values from GetDefaultRules().
// Returns counts: (updated, re-added, unchanged) and a slice of affected rule IDs.
func (c *Config) ResetChangedDefaultRules(enableRules bool) (updated, readded, unchanged int, affectedIDs []string) {
	defaults := workflow.GetDefaultRules()
	defaultMap := make(map[string]workflow.WorkflowRule)

	// Build map of shipped defaults by ID
	for _, rule := range defaults {
		defaultMap[rule.ID] = rule
	}

	// Phase 1: Update existing default rules that have changed
	for i := range c.WorkflowRules {
		existingRule := &c.WorkflowRules[i]
		if existingRule.Source != "default" {
			continue // Skip custom rules
		}

		shippedRule, exists := defaultMap[existingRule.ID]
		if !exists {
			// This default rule no longer exists in shipped set (shouldn't happen)
			continue
		}

		// Compare rule content (semantic only, ignore metadata)
		if rulesAreDifferent(existingRule, &shippedRule) {
			// Reset to shipped values, preserve Enabled state unless enableRules is true
			preservedEnabled := existingRule.Enabled
			*existingRule = shippedRule
			if !enableRules {
				existingRule.Enabled = preservedEnabled
			}
			existingRule.TriggerCount = 0 // Reset trigger count
			updated++
			affectedIDs = append(affectedIDs, existingRule.ID)
		} else {
			unchanged++
		}

		// Mark as processed
		delete(defaultMap, existingRule.ID)
	}

	// Phase 2: Re-add deleted default rules
	for _, rule := range defaultMap {
		// These are default rules that were deleted by the user
		if !enableRules {
			rule.Enabled = false // Respect original disabled state
		}
		c.WorkflowRules = append(c.WorkflowRules, rule)
		readded++
		affectedIDs = append(affectedIDs, rule.ID)
	}

	return updated, readded, unchanged, affectedIDs
}

// rulesAreDifferent compares rule content semantically (ignoring metadata).
// Returns true if the rules differ in name, priority, triggers, trigger logic, or actions.
// Compares triggers and actions by content, not by order.
func rulesAreDifferent(a, b *workflow.WorkflowRule) bool {
	// Compare name
	if a.Name != b.Name {
		return true
	}

	// Compare priority
	if a.Priority != b.Priority {
		return true
	}

	// Compare trigger logic
	if a.GetTriggerLogic() != b.GetTriggerLogic() {
		return true
	}

	// Compare triggers (order-insensitive)
	aTriggers := a.GetTriggers()
	bTriggers := b.GetTriggers()
	if len(aTriggers) != len(bTriggers) {
		return true
	}

	// Create trigger signature maps for order-insensitive comparison
	aTriggerSigs := make(map[string]int)
	for _, t := range aTriggers {
		sig := string(t.Field) + "|" + string(t.Operation) + "|" + t.Value
		aTriggerSigs[sig]++
	}
	bTriggerSigs := make(map[string]int)
	for _, t := range bTriggers {
		sig := string(t.Field) + "|" + string(t.Operation) + "|" + t.Value
		bTriggerSigs[sig]++
	}
	if len(aTriggerSigs) != len(bTriggerSigs) {
		return true
	}
	for sig, count := range aTriggerSigs {
		if bTriggerSigs[sig] != count {
			return true
		}
	}

	// Compare trigger groups
	aTG := a.TriggerGroup
	bTG := b.TriggerGroup
	if (aTG == nil) != (bTG == nil) {
		return true
	}
	if aTG != nil && bTG != nil {
		aTGJSON, _ := json.Marshal(aTG)
		bTGJSON, _ := json.Marshal(bTG)
		if string(aTGJSON) != string(bTGJSON) {
			return true
		}
	}

	// Compare actions (order-insensitive)
	if len(a.Actions) != len(b.Actions) {
		return true
	}
	aActionSigs := make(map[string]int)
	for _, act := range a.Actions {
		sig := string(act.ActionType) + "|" + act.Value + "|" + conditionSignature(act.Condition)
		aActionSigs[sig]++
	}
	bActionSigs := make(map[string]int)
	for _, act := range b.Actions {
		sig := string(act.ActionType) + "|" + act.Value + "|" + conditionSignature(act.Condition)
		bActionSigs[sig]++
	}
	if len(aActionSigs) != len(bActionSigs) {
		return true
	}
	for sig, count := range aActionSigs {
		if bActionSigs[sig] != count {
			return true
		}
	}

	return false
}

// conditionSignature returns a stable string signature for an action condition.
func conditionSignature(c *workflow.RuleTrigger) string {
	if c == nil {
		return ""
	}
	return string(c.Field) + "|" + string(c.Operation) + "|" + c.Value
}

// GetWorkflowEvalState returns the evaluation state for a project.
func (c *Config) GetWorkflowEvalState(projectID string) WorkflowEvalState {
	if c.WorkflowEvalStates == nil {
		return WorkflowEvalState{}
	}
	return c.WorkflowEvalStates[projectID]
}

// SetWorkflowEvalState sets the evaluation state for a project.
func (c *Config) SetWorkflowEvalState(projectID string, state WorkflowEvalState) {
	if c.WorkflowEvalStates == nil {
		c.WorkflowEvalStates = make(map[string]WorkflowEvalState)
	}
	c.WorkflowEvalStates[projectID] = state
}

// GetWorkflowEvalInterval returns the interval in minutes between automatic evaluations.
// Returns 60 if not set, 0 means disabled.
func (c *Config) GetWorkflowEvalInterval() int {
	if c.WorkflowEvalInterval == nil {
		return 60 // Default: 60 minutes
	}
	return *c.WorkflowEvalInterval
}

// SetWorkflowEvalInterval sets the interval in minutes between automatic evaluations.
// Set to 0 to disable automatic evaluation.
func (c *Config) SetWorkflowEvalInterval(minutes int) {
	c.WorkflowEvalInterval = &minutes
}

// ComputeWorkflowRulesHash computes a hash of enabled workflow rules.
// Used to detect when rules have changed, forcing re-evaluation.
// Hashes full rule content (not just IDs) to detect in-place edits.
func (c *Config) ComputeWorkflowRulesHash() string {
	var rules []workflow.WorkflowRule
	for _, rule := range c.WorkflowRules {
		if rule.Enabled {
			rules = append(rules, rule)
		}
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })
	data, err := json.Marshal(rules)
	if err != nil {
		// Fallback to empty hash if marshaling fails (shouldn't happen with valid rules)
		return ""
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:8]) // First 8 bytes is enough
}

// ShouldEvaluateWorkflow returns true if workflow rules should be evaluated for a project.
// Returns false if no enabled rules, evaluation is disabled, or recently evaluated.
func (c *Config) ShouldEvaluateWorkflow(projectID string) bool {
	if len(c.GetEnabledWorkflowRules()) == 0 {
		return false
	}

	interval := c.GetWorkflowEvalInterval()
	if interval == 0 {
		return false // Disabled
	}

	state := c.GetWorkflowEvalState(projectID)
	currentHash := c.ComputeWorkflowRulesHash()

	// Force evaluation if rules changed
	if state.RulesHash != currentHash {
		return true
	}

	// Never evaluated
	if state.LastEvalTime.IsZero() {
		return true
	}

	// Check time interval
	elapsed := time.Since(state.LastEvalTime)
	return elapsed >= time.Duration(interval)*time.Minute
}

// InvalidateWorkflowEvalStates clears the rules hash from all eval states,
// forcing re-evaluation on next check. Called when rules are modified.
func (c *Config) InvalidateWorkflowEvalStates() {
	if c.WorkflowEvalStates == nil {
		return
	}
	for projectID, state := range c.WorkflowEvalStates {
		state.RulesHash = ""
		c.WorkflowEvalStates[projectID] = state
	}
}
