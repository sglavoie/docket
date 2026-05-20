package config

import (
	"testing"

	"github.com/sglavoie/tt/internal/workflow"
)

func TestGetWorkflowRules_NilSlice(t *testing.T) {
	cfg := &Config{}
	cfg.WorkflowRules = nil

	rules := cfg.GetWorkflowRules()
	if rules == nil {
		t.Error("GetWorkflowRules should return empty slice, not nil")
	}
	if len(rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(rules))
	}
}

func TestGetWorkflowRules_ReturnsAllRules(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1", Enabled: true},
		{ID: "r2", Name: "Rule 2", Enabled: false},
		{ID: "r3", Name: "Rule 3", Enabled: true},
	}

	rules := cfg.GetWorkflowRules()
	if len(rules) != 3 {
		t.Errorf("expected 3 rules, got %d", len(rules))
	}
}

func TestGetEnabledWorkflowRules_FiltersCorrectly(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1", Enabled: true},
		{ID: "r2", Name: "Rule 2", Enabled: false},
		{ID: "r3", Name: "Rule 3", Enabled: true},
		{ID: "r4", Name: "Rule 4", Enabled: false},
	}

	enabled := cfg.GetEnabledWorkflowRules()
	if len(enabled) != 2 {
		t.Errorf("expected 2 enabled rules, got %d", len(enabled))
	}

	for _, rule := range enabled {
		if !rule.Enabled {
			t.Errorf("rule %s should be enabled", rule.ID)
		}
	}
}

func TestGetEnabledWorkflowRules_EmptyWhenNoneEnabled(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1", Enabled: false},
		{ID: "r2", Name: "Rule 2", Enabled: false},
	}

	enabled := cfg.GetEnabledWorkflowRules()
	if len(enabled) != 0 {
		t.Errorf("expected 0 enabled rules, got %d", len(enabled))
	}
}

func TestGetWorkflowRulesSorted(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "c2", Name: "Zebra Custom", Source: "custom"},
		{ID: "d1", Name: "Beta Default", Source: "default"},
		{ID: "c1", Name: "Alpha Custom", Source: "custom"},
		{ID: "d2", Name: "Alpha Default", Source: "default"},
	}

	defaultRules, customRules := cfg.GetWorkflowRulesSorted()

	if len(defaultRules) != 2 {
		t.Errorf("expected 2 default rules, got %d", len(defaultRules))
	}
	if len(customRules) != 2 {
		t.Errorf("expected 2 custom rules, got %d", len(customRules))
	}

	if defaultRules[0].Name != "Alpha Default" {
		t.Errorf("expected first default rule to be 'Alpha Default', got '%s'", defaultRules[0].Name)
	}
	if defaultRules[1].Name != "Beta Default" {
		t.Errorf("expected second default rule to be 'Beta Default', got '%s'", defaultRules[1].Name)
	}
	if customRules[0].Name != "Alpha Custom" {
		t.Errorf("expected first custom rule to be 'Alpha Custom', got '%s'", customRules[0].Name)
	}
	if customRules[1].Name != "Zebra Custom" {
		t.Errorf("expected second custom rule to be 'Zebra Custom', got '%s'", customRules[1].Name)
	}
}

func TestGetWorkflowRulesSorted_EmptySlice(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = nil

	defaultRules, customRules := cfg.GetWorkflowRulesSorted()
	if len(defaultRules) != 0 {
		t.Errorf("expected 0 default rules, got %d", len(defaultRules))
	}
	if len(customRules) != 0 {
		t.Errorf("expected 0 custom rules, got %d", len(customRules))
	}
}

func TestGetWorkflowRule_FindsByID(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1"},
		{ID: "r2", Name: "Rule 2"},
		{ID: "r3", Name: "Rule 3"},
	}

	rule := cfg.GetWorkflowRule("r2")
	if rule == nil {
		t.Fatal("expected to find rule r2")
	}
	if rule.Name != "Rule 2" {
		t.Errorf("expected Name 'Rule 2', got '%s'", rule.Name)
	}
}

func TestGetWorkflowRule_ReturnsNilWhenNotFound(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1"},
	}

	rule := cfg.GetWorkflowRule("nonexistent")
	if rule != nil {
		t.Error("expected nil for nonexistent rule")
	}
}

func TestGetWorkflowRule_ReturnsPointerToOriginal(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1", Enabled: false},
	}

	rule := cfg.GetWorkflowRule("r1")
	rule.Enabled = true

	if !cfg.WorkflowRules[0].Enabled {
		t.Error("modifying returned pointer should modify original")
	}
}

func TestAddWorkflowRule(t *testing.T) {
	cfg := NewConfig()

	rule := workflow.WorkflowRule{ID: "new", Name: "New Rule", Enabled: true}
	cfg.AddWorkflowRule(rule)

	if len(cfg.WorkflowRules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(cfg.WorkflowRules))
	}
	if cfg.WorkflowRules[0].ID != "new" {
		t.Errorf("expected ID 'new', got '%s'", cfg.WorkflowRules[0].ID)
	}
}

func TestAddWorkflowRule_AppendsToExisting(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1"},
	}

	rule := workflow.WorkflowRule{ID: "r2", Name: "Rule 2"}
	cfg.AddWorkflowRule(rule)

	if len(cfg.WorkflowRules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(cfg.WorkflowRules))
	}
}

func TestUpdateWorkflowRule_UpdatesExisting(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Old Name", Enabled: false},
	}

	updated := cfg.UpdateWorkflowRule(workflow.WorkflowRule{
		ID:      "r1",
		Name:    "New Name",
		Enabled: true,
	})

	if !updated {
		t.Error("UpdateWorkflowRule should return true when rule exists")
	}
	if cfg.WorkflowRules[0].Name != "New Name" {
		t.Errorf("expected Name 'New Name', got '%s'", cfg.WorkflowRules[0].Name)
	}
	if !cfg.WorkflowRules[0].Enabled {
		t.Error("expected Enabled to be true")
	}
}

func TestUpdateWorkflowRule_ReturnsFalseWhenNotFound(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1"},
	}

	updated := cfg.UpdateWorkflowRule(workflow.WorkflowRule{ID: "nonexistent", Name: "Test"})

	if updated {
		t.Error("UpdateWorkflowRule should return false for nonexistent rule")
	}
}

func TestDeleteWorkflowRule_DeletesExisting(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1"},
		{ID: "r2", Name: "Rule 2"},
		{ID: "r3", Name: "Rule 3"},
	}

	deleted := cfg.DeleteWorkflowRule("r2")

	if !deleted {
		t.Error("DeleteWorkflowRule should return true when rule exists")
	}
	if len(cfg.WorkflowRules) != 2 {
		t.Errorf("expected 2 rules after deletion, got %d", len(cfg.WorkflowRules))
	}
	if cfg.GetWorkflowRule("r2") != nil {
		t.Error("r2 should have been deleted")
	}
}

func TestDeleteWorkflowRule_ReturnsFalseWhenNotFound(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1"},
	}

	deleted := cfg.DeleteWorkflowRule("nonexistent")

	if deleted {
		t.Error("DeleteWorkflowRule should return false for nonexistent rule")
	}
	if len(cfg.WorkflowRules) != 1 {
		t.Error("rule count should not change when deleting nonexistent rule")
	}
}

func TestToggleWorkflowRule_TogglesEnabled(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1", Enabled: false},
	}

	enabled, found := cfg.ToggleWorkflowRule("r1")
	if !found {
		t.Error("ToggleWorkflowRule should return found=true for existing rule")
	}
	if !enabled {
		t.Error("ToggleWorkflowRule should return enabled=true after toggling from false")
	}
	if !cfg.WorkflowRules[0].Enabled {
		t.Error("rule should be enabled after toggle")
	}

	enabled, found = cfg.ToggleWorkflowRule("r1")
	if !found {
		t.Error("ToggleWorkflowRule should return found=true")
	}
	if enabled {
		t.Error("ToggleWorkflowRule should return enabled=false after toggling from true")
	}
	if cfg.WorkflowRules[0].Enabled {
		t.Error("rule should be disabled after second toggle")
	}
}

func TestToggleWorkflowRule_NotFound(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1", Enabled: true},
	}

	enabled, found := cfg.ToggleWorkflowRule("nonexistent")
	if found {
		t.Error("ToggleWorkflowRule should return found=false for nonexistent rule")
	}
	if enabled {
		t.Error("ToggleWorkflowRule should return enabled=false when not found")
	}
}

func TestIncrementRuleTriggerCounts(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1", TriggerCount: 5},
		{ID: "r2", Name: "Rule 2", TriggerCount: 10},
		{ID: "r3", Name: "Rule 3", TriggerCount: 0},
	}

	counts := map[string]int{
		"r1": 3,
		"r3": 7,
	}
	cfg.IncrementRuleTriggerCounts(counts)

	if cfg.WorkflowRules[0].TriggerCount != 8 {
		t.Errorf("r1 TriggerCount: expected 8, got %d", cfg.WorkflowRules[0].TriggerCount)
	}
	if cfg.WorkflowRules[1].TriggerCount != 10 {
		t.Errorf("r2 TriggerCount should remain 10, got %d", cfg.WorkflowRules[1].TriggerCount)
	}
	if cfg.WorkflowRules[2].TriggerCount != 7 {
		t.Errorf("r3 TriggerCount: expected 7, got %d", cfg.WorkflowRules[2].TriggerCount)
	}
}

func TestResetRuleTriggerCount(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1", TriggerCount: 100},
		{ID: "r2", Name: "Rule 2", TriggerCount: 50},
	}

	found := cfg.ResetRuleTriggerCount("r1")
	if !found {
		t.Error("ResetRuleTriggerCount should return true for existing rule")
	}
	if cfg.WorkflowRules[0].TriggerCount != 0 {
		t.Errorf("r1 TriggerCount should be 0, got %d", cfg.WorkflowRules[0].TriggerCount)
	}
	if cfg.WorkflowRules[1].TriggerCount != 50 {
		t.Errorf("r2 TriggerCount should remain 50, got %d", cfg.WorkflowRules[1].TriggerCount)
	}
}

func TestResetRuleTriggerCount_NotFound(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1", TriggerCount: 100},
	}

	found := cfg.ResetRuleTriggerCount("nonexistent")
	if found {
		t.Error("ResetRuleTriggerCount should return false for nonexistent rule")
	}
}

func TestResetAllRuleTriggerCounts(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1", TriggerCount: 100},
		{ID: "r2", Name: "Rule 2", TriggerCount: 50},
		{ID: "r3", Name: "Rule 3", TriggerCount: 25},
	}

	cfg.ResetAllRuleTriggerCounts()

	for _, rule := range cfg.WorkflowRules {
		if rule.TriggerCount != 0 {
			t.Errorf("rule %s TriggerCount should be 0, got %d", rule.ID, rule.TriggerCount)
		}
	}
}

func TestHasRuleWithID(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1"},
		{ID: "r2", Name: "Rule 2"},
	}

	if !cfg.HasRuleWithID("r1") {
		t.Error("HasRuleWithID should return true for existing rule")
	}
	if !cfg.HasRuleWithID("r2") {
		t.Error("HasRuleWithID should return true for existing rule")
	}
	if cfg.HasRuleWithID("nonexistent") {
		t.Error("HasRuleWithID should return false for nonexistent rule")
	}
}

// TestResetChangedDefaultRules_EnableRulesTrueAppliesToAll verifies that the
// enableRules flag forces every affected rule into the enabled state, both
// for updated rules whose content drifted and for re-added rules that the
// user had deleted. Previously, enableRules=true only suppressed the
// "preserve" path for updates and left re-added rules with their shipped
// (mostly disabled) state, so `--enable` silently failed to enable them.
func TestResetChangedDefaultRules_EnableRulesTrueAppliesToAll(t *testing.T) {
	cfg := NewConfig()

	// Seed with the first two default rules to exercise both phases:
	// rule 0 has its content drifted (will be updated),
	// rule 1 is omitted entirely (will be re-added in phase 2).
	defaults := []workflow.WorkflowRule{}
	for _, r := range workflowDefaultsCopy() {
		defaults = append(defaults, r)
		if len(defaults) == 1 {
			break
		}
	}
	if len(defaults) == 0 {
		t.Fatal("no default rules available")
	}
	drifted := defaults[0]
	drifted.Name = "modified-by-user"
	drifted.Enabled = false
	cfg.WorkflowRules = []workflow.WorkflowRule{drifted}

	updated, readded, _, _ := cfg.ResetChangedDefaultRules(true)
	if updated < 1 {
		t.Errorf("expected at least 1 updated rule, got %d", updated)
	}
	if readded < 1 {
		t.Errorf("expected at least 1 re-added rule, got %d", readded)
	}
	for _, r := range cfg.WorkflowRules {
		if r.Source != "default" {
			continue
		}
		if !r.Enabled {
			t.Errorf("rule %q expected enabled after enableRules=true, got disabled", r.ID)
		}
	}
}

// TestResetChangedDefaultRules_EnableRulesFalsePreservesUserState verifies
// that enableRules=false keeps the user's enabled state for drifted rules
// and disables newly re-added rules (matching the original semantics).
func TestResetChangedDefaultRules_EnableRulesFalsePreservesUserState(t *testing.T) {
	cfg := NewConfig()

	// Pick the first default rule, mark it user-enabled but with drifted content.
	defs := workflowDefaultsCopy()
	if len(defs) == 0 {
		t.Fatal("no default rules available")
	}
	drifted := defs[0]
	drifted.Name = "user-renamed"
	drifted.Enabled = true
	cfg.WorkflowRules = []workflow.WorkflowRule{drifted}

	cfg.ResetChangedDefaultRules(false)

	var found bool
	for _, r := range cfg.WorkflowRules {
		if r.ID == drifted.ID {
			found = true
			if !r.Enabled {
				t.Error("drifted rule should preserve user-enabled state when enableRules=false")
			}
		} else if r.Source == "default" && r.Enabled {
			// Re-added defaults should be disabled.
			t.Errorf("re-added rule %q should be disabled when enableRules=false", r.ID)
		}
	}
	if !found {
		t.Error("drifted rule missing from result")
	}
}

// workflowDefaultsCopy returns a fresh copy of the shipped default rules so
// tests can mutate them without affecting the global default set.
func workflowDefaultsCopy() []workflow.WorkflowRule {
	src := workflow.GetDefaultRules()
	out := make([]workflow.WorkflowRule, len(src))
	copy(out, src)
	return out
}
