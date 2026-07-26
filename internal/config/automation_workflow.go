package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/workflow"
)

type WorkflowEvalState struct {
	LastEvalTime time.Time `json:"last_eval_time"`
	RulesHash    string    `json:"rules_hash,omitempty"`
}

// SanitizeRetiredWorkflowActions removes persisted actions that no longer have
// runtime semantics. A rule that loses every action is removed entirely; rules
// with remaining actions retain their original ordering and metadata. It reports
// whether the Automation domain changed so callers can persist the migration.
func (a *AutomationConfig) SanitizeRetiredWorkflowActions() bool {
	if a == nil || len(a.WorkflowRules) == 0 {
		return false
	}

	changed := false
	keptRules := make([]workflow.WorkflowRule, 0, len(a.WorkflowRules))
	for _, rule := range a.WorkflowRules {
		keptActions := rule.Actions[:0]
		removedRetiredAction := false
		for _, action := range rule.Actions {
			if action.ActionType == workflow.ActionType("set_active") {
				changed = true
				removedRetiredAction = true
				continue
			}
			keptActions = append(keptActions, action)
		}
		if removedRetiredAction && len(keptActions) == 0 {
			continue
		}
		if len(keptActions) != len(rule.Actions) {
			rule.Actions = keptActions
		}
		keptRules = append(keptRules, rule)
	}
	if changed {
		a.WorkflowRules = keptRules
	}
	return changed
}

func partitionAndSortWorkflowRules(all []workflow.WorkflowRule) (defaultRules, customRules []workflow.WorkflowRule) {
	for _, rule := range all {
		if rule.Source == "default" {
			defaultRules = append(defaultRules, rule)
		} else {
			customRules = append(customRules, rule)
		}
	}
	sortByPriorityThenName := func(rules []workflow.WorkflowRule) {
		sort.SliceStable(rules, func(i, j int) bool {
			pi, pj := rules[i].Priority, rules[j].Priority
			if pi == 0 && pj == 0 {
				return rules[i].Name < rules[j].Name
			}
			if pi == 0 {
				return false
			}
			if pj == 0 {
				return true
			}
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

func rulesAreDifferent(a, b *workflow.WorkflowRule) bool {
	if a.Name != b.Name ||
		a.Priority != b.Priority ||
		a.GetLevel() != b.GetLevel() ||
		a.GetTriggerLogic() != b.GetTriggerLogic() {
		return true
	}
	aTriggers := a.GetTriggers()
	bTriggers := b.GetTriggers()
	if len(aTriggers) != len(bTriggers) {
		return true
	}
	aTriggerSigs := make(map[string]int)
	for _, trigger := range aTriggers {
		aTriggerSigs[string(trigger.Field)+"|"+string(trigger.Operation)+"|"+trigger.Value]++
	}
	bTriggerSigs := make(map[string]int)
	for _, trigger := range bTriggers {
		bTriggerSigs[string(trigger.Field)+"|"+string(trigger.Operation)+"|"+trigger.Value]++
	}
	if len(aTriggerSigs) != len(bTriggerSigs) {
		return true
	}
	for sig, count := range aTriggerSigs {
		if bTriggerSigs[sig] != count {
			return true
		}
	}
	if (a.TriggerGroup == nil) != (b.TriggerGroup == nil) {
		return true
	}
	if a.TriggerGroup != nil {
		aJSON, aErr := json.Marshal(a.TriggerGroup)
		bJSON, bErr := json.Marshal(b.TriggerGroup)
		if aErr != nil || bErr != nil || string(aJSON) != string(bJSON) {
			return true
		}
	}
	if len(a.Actions) != len(b.Actions) {
		return true
	}
	aActionSigs := make(map[string]int)
	for _, action := range a.Actions {
		aActionSigs[string(action.ActionType)+"|"+action.Value+"|"+conditionSignature(action.Condition)]++
	}
	bActionSigs := make(map[string]int)
	for _, action := range b.Actions {
		bActionSigs[string(action.ActionType)+"|"+action.Value+"|"+conditionSignature(action.Condition)]++
	}
	if len(aActionSigs) != len(bActionSigs) {
		return true
	}
	for sig, count := range aActionSigs {
		if bActionSigs[sig] != count {
			return true
		}
	}
	aScopeJSON, aScopeErr := json.Marshal(a.Scope)
	bScopeJSON, bScopeErr := json.Marshal(b.Scope)
	if aScopeErr != nil || bScopeErr != nil || string(aScopeJSON) != string(bScopeJSON) {
		return true
	}
	return false
}

func conditionSignature(c *workflow.RuleTrigger) string {
	if c == nil {
		return ""
	}
	return string(c.Field) + "|" + string(c.Operation) + "|" + c.Value
}

// Workflow-rule behavior on the AutomationConfig domain aggregate. These mirror
// the equivalent *Config facade methods but depend only on the automation
// domain, so command consumers can read and mutate workflow rules through the
// narrow AutomationStore seam instead of the whole config.Config facade. They
// reuse the package-level helpers (partitionAndSortWorkflowRules,
// rulesAreDifferent) shared with the facade so both produce identical results.

// GetWorkflowRulesSorted returns rules sorted default-first, then custom, each
// group ordered by priority then name.
func (a *AutomationConfig) GetWorkflowRules() []workflow.WorkflowRule {
	if a.WorkflowRules == nil {
		return []workflow.WorkflowRule{}
	}
	return a.WorkflowRules
}

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

// IncrementRuleTriggerCounts records workflow firings on their owning
// automation-domain rules.
func (a *AutomationConfig) IncrementRuleTriggerCounts(counts map[string]int) {
	for i := range a.WorkflowRules {
		if count, ok := counts[a.WorkflowRules[i].ID]; ok {
			a.WorkflowRules[i].TriggerCount += int64(count)
		}
	}
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

func (a *AutomationConfig) GetWorkflowEvalState(projectID string) WorkflowEvalState {
	if a.WorkflowEvalStates == nil {
		return WorkflowEvalState{}
	}
	return a.WorkflowEvalStates[projectID]
}

func (a *AutomationConfig) SetWorkflowEvalState(projectID string, state WorkflowEvalState) {
	if a.WorkflowEvalStates == nil {
		a.WorkflowEvalStates = make(map[string]WorkflowEvalState)
	}
	a.WorkflowEvalStates[projectID] = state
}

func (a *AutomationConfig) GetWorkflowEvalInterval() int {
	if a.WorkflowEvalInterval == nil {
		return 60
	}
	if *a.WorkflowEvalInterval < 0 {
		return 0
	}
	return *a.WorkflowEvalInterval
}

func (a *AutomationConfig) SetWorkflowEvalInterval(minutes int) {
	if minutes < 0 {
		minutes = 0
	}
	a.WorkflowEvalInterval = &minutes
}

func (a *AutomationConfig) ComputeWorkflowRulesHash() string {
	var rules []workflow.WorkflowRule
	for _, rule := range a.WorkflowRules {
		if rule.Enabled {
			// TriggerCount is runtime telemetry, not part of the executable
			// rule definition. Including it would invalidate the evaluation
			// cache every time a rule fires.
			rule.TriggerCount = 0
			rules = append(rules, rule)
		}
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })
	data, err := json.Marshal(rules)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:8])
}

func (a *AutomationConfig) ShouldEvaluateWorkflow(projectID string) bool {
	if len(a.GetEnabledWorkflowRules()) == 0 {
		return false
	}
	interval := a.GetWorkflowEvalInterval()
	if interval == 0 {
		return false
	}
	state := a.GetWorkflowEvalState(projectID)
	if state.RulesHash != a.ComputeWorkflowRulesHash() {
		return true
	}
	if state.LastEvalTime.IsZero() {
		return true
	}
	return time.Since(state.LastEvalTime) >= time.Duration(interval)*time.Minute
}
