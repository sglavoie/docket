package config

import (
	"testing"
	"time"

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

func TestAddDefaultRules_AddsNewRules(t *testing.T) {
	cfg := NewConfig()

	added := cfg.AddDefaultRules(false, false)

	if added == 0 {
		t.Error("AddDefaultRules should add at least one rule")
	}
	if !cfg.DefaultRulesInitialized {
		t.Error("DefaultRulesInitialized should be true after AddDefaultRules")
	}
}

func TestAddDefaultRules_SkipsExisting(t *testing.T) {
	cfg := NewConfig()

	defaultRules := workflow.GetDefaultRules()
	if len(defaultRules) == 0 {
		t.Skip("no default rules to test")
	}

	cfg.WorkflowRules = []workflow.WorkflowRule{defaultRules[0]}

	added := cfg.AddDefaultRules(false, false)
	expectedAdded := len(defaultRules) - 1

	if added != expectedAdded {
		t.Errorf("expected %d rules added, got %d", expectedAdded, added)
	}
}

func TestAddDefaultRules_EnableRulesFlag(t *testing.T) {
	cfg := NewConfig()

	cfg.AddDefaultRules(true, false)

	for _, rule := range cfg.WorkflowRules {
		if !rule.Enabled {
			t.Errorf("rule %s should be enabled when enableRules=true", rule.ID)
		}
	}
}

func TestAddDefaultRules_DisableRulesFlag(t *testing.T) {
	cfg := NewConfig()

	cfg.AddDefaultRules(false, false)

	for _, rule := range cfg.WorkflowRules {
		if rule.Enabled {
			t.Errorf("rule %s should be disabled when enableRules=false", rule.ID)
		}
	}
}

func TestAddDefaultRules_UseOriginalEnabled(t *testing.T) {
	cfg := NewConfig()

	cfg.AddDefaultRules(false, true)

	defaultRules := workflow.GetDefaultRules()
	defaultMap := make(map[string]bool)
	for _, r := range defaultRules {
		defaultMap[r.ID] = r.Enabled
	}

	for _, rule := range cfg.WorkflowRules {
		if expected, ok := defaultMap[rule.ID]; ok {
			if rule.Enabled != expected {
				t.Errorf("rule %s should have Enabled=%v (original), got %v", rule.ID, expected, rule.Enabled)
			}
		}
	}
}

func TestGetWorkflowEvalState_NilMap(t *testing.T) {
	cfg := &Config{}
	cfg.WorkflowEvalStates = nil

	state := cfg.GetWorkflowEvalState("project-1")
	if !state.LastEvalTime.IsZero() {
		t.Error("expected zero time for nonexistent project")
	}
	if state.RulesHash != "" {
		t.Error("expected empty hash for nonexistent project")
	}
}

func TestGetWorkflowEvalState_ReturnsCorrectState(t *testing.T) {
	cfg := NewConfig()
	evalTime := time.Now()
	cfg.WorkflowEvalStates = map[string]WorkflowEvalState{
		"project-1": {LastEvalTime: evalTime, RulesHash: "abc123"},
	}

	state := cfg.GetWorkflowEvalState("project-1")
	if !state.LastEvalTime.Equal(evalTime) {
		t.Error("LastEvalTime should match")
	}
	if state.RulesHash != "abc123" {
		t.Errorf("expected RulesHash 'abc123', got '%s'", state.RulesHash)
	}
}

func TestSetWorkflowEvalState_CreatesMapIfNil(t *testing.T) {
	cfg := &Config{}
	cfg.WorkflowEvalStates = nil

	evalTime := time.Now()
	cfg.SetWorkflowEvalState("project-1", WorkflowEvalState{
		LastEvalTime: evalTime,
		RulesHash:    "xyz789",
	})

	if cfg.WorkflowEvalStates == nil {
		t.Error("WorkflowEvalStates should be initialized")
	}
	state := cfg.WorkflowEvalStates["project-1"]
	if state.RulesHash != "xyz789" {
		t.Errorf("expected RulesHash 'xyz789', got '%s'", state.RulesHash)
	}
}

func TestSetWorkflowEvalState_OverwritesExisting(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowEvalStates = map[string]WorkflowEvalState{
		"project-1": {RulesHash: "old"},
	}

	cfg.SetWorkflowEvalState("project-1", WorkflowEvalState{RulesHash: "new"})

	if cfg.WorkflowEvalStates["project-1"].RulesHash != "new" {
		t.Error("state should be overwritten")
	}
}

func TestGetWorkflowEvalInterval_DefaultValue(t *testing.T) {
	cfg := &Config{}
	cfg.WorkflowEvalInterval = nil

	interval := cfg.GetWorkflowEvalInterval()
	if interval != 60 {
		t.Errorf("expected default interval 60, got %d", interval)
	}
}

func TestGetWorkflowEvalInterval_CustomValue(t *testing.T) {
	cfg := NewConfig()
	val := 30
	cfg.WorkflowEvalInterval = &val

	interval := cfg.GetWorkflowEvalInterval()
	if interval != 30 {
		t.Errorf("expected interval 30, got %d", interval)
	}
}

func TestGetWorkflowEvalInterval_ZeroDisabled(t *testing.T) {
	cfg := NewConfig()
	val := 0
	cfg.WorkflowEvalInterval = &val

	interval := cfg.GetWorkflowEvalInterval()
	if interval != 0 {
		t.Errorf("expected interval 0 (disabled), got %d", interval)
	}
}

func TestSetWorkflowEvalInterval(t *testing.T) {
	cfg := NewConfig()

	cfg.SetWorkflowEvalInterval(120)

	if cfg.WorkflowEvalInterval == nil {
		t.Fatal("WorkflowEvalInterval should not be nil")
	}
	if *cfg.WorkflowEvalInterval != 120 {
		t.Errorf("expected interval 120, got %d", *cfg.WorkflowEvalInterval)
	}
}

func TestComputeWorkflowRulesHash_OnlyEnabledRules(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1", Enabled: true},
		{ID: "r2", Name: "Rule 2", Enabled: false},
	}

	hash1 := cfg.ComputeWorkflowRulesHash()

	cfg.WorkflowRules[1].Enabled = true
	hash2 := cfg.ComputeWorkflowRulesHash()

	if hash1 == hash2 {
		t.Error("hash should change when enabled rules change")
	}
}

func TestComputeWorkflowRulesHash_Deterministic(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1", Enabled: true},
		{ID: "r2", Name: "Rule 2", Enabled: true},
	}

	hash1 := cfg.ComputeWorkflowRulesHash()
	hash2 := cfg.ComputeWorkflowRulesHash()

	if hash1 != hash2 {
		t.Error("hash should be deterministic for same rules")
	}
}

func TestComputeWorkflowRulesHash_OrderIndependent(t *testing.T) {
	cfg1 := NewConfig()
	cfg1.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1", Enabled: true},
		{ID: "r2", Name: "Rule 2", Enabled: true},
	}

	cfg2 := NewConfig()
	cfg2.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r2", Name: "Rule 2", Enabled: true},
		{ID: "r1", Name: "Rule 1", Enabled: true},
	}

	hash1 := cfg1.ComputeWorkflowRulesHash()
	hash2 := cfg2.ComputeWorkflowRulesHash()

	if hash1 != hash2 {
		t.Error("hash should be the same regardless of rule order")
	}
}

func TestComputeWorkflowRulesHash_EmptyWhenNoEnabledRules(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1", Enabled: false},
	}

	hash := cfg.ComputeWorkflowRulesHash()
	if hash == "" {
		t.Error("hash should not be empty even with no enabled rules (empty array marshals)")
	}
}

func TestComputeWorkflowRulesHash_DetectsContentChanges(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1", Enabled: true},
	}

	hash1 := cfg.ComputeWorkflowRulesHash()

	cfg.WorkflowRules[0].Name = "Rule 1 Modified"
	hash2 := cfg.ComputeWorkflowRulesHash()

	if hash1 == hash2 {
		t.Error("hash should change when rule content changes")
	}
}

func TestShouldEvaluateWorkflow_FalseWhenNoEnabledRules(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1", Enabled: false},
	}

	if cfg.ShouldEvaluateWorkflow("project-1") {
		t.Error("should return false when no enabled rules")
	}
}

func TestShouldEvaluateWorkflow_FalseWhenDisabled(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1", Enabled: true},
	}
	val := 0
	cfg.WorkflowEvalInterval = &val

	if cfg.ShouldEvaluateWorkflow("project-1") {
		t.Error("should return false when interval is 0 (disabled)")
	}
}

func TestShouldEvaluateWorkflow_TrueWhenRulesChanged(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1", Enabled: true},
	}
	cfg.WorkflowEvalStates = map[string]WorkflowEvalState{
		"project-1": {
			LastEvalTime: time.Now(),
			RulesHash:    "old-hash",
		},
	}

	if !cfg.ShouldEvaluateWorkflow("project-1") {
		t.Error("should return true when rules hash differs")
	}
}

func TestShouldEvaluateWorkflow_TrueWhenNeverEvaluated(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1", Enabled: true},
	}
	currentHash := cfg.ComputeWorkflowRulesHash()
	cfg.WorkflowEvalStates = map[string]WorkflowEvalState{
		"project-1": {
			LastEvalTime: time.Time{},
			RulesHash:    currentHash,
		},
	}

	if !cfg.ShouldEvaluateWorkflow("project-1") {
		t.Error("should return true when never evaluated (zero time)")
	}
}

func TestShouldEvaluateWorkflow_TrueWhenIntervalElapsed(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1", Enabled: true},
	}
	val := 1
	cfg.WorkflowEvalInterval = &val
	currentHash := cfg.ComputeWorkflowRulesHash()
	cfg.WorkflowEvalStates = map[string]WorkflowEvalState{
		"project-1": {
			LastEvalTime: time.Now().Add(-2 * time.Minute),
			RulesHash:    currentHash,
		},
	}

	if !cfg.ShouldEvaluateWorkflow("project-1") {
		t.Error("should return true when interval has elapsed")
	}
}

func TestShouldEvaluateWorkflow_FalseWhenRecentlyEvaluated(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1", Enabled: true},
	}
	val := 60
	cfg.WorkflowEvalInterval = &val
	currentHash := cfg.ComputeWorkflowRulesHash()
	cfg.WorkflowEvalStates = map[string]WorkflowEvalState{
		"project-1": {
			LastEvalTime: time.Now().Add(-1 * time.Minute),
			RulesHash:    currentHash,
		},
	}

	if cfg.ShouldEvaluateWorkflow("project-1") {
		t.Error("should return false when recently evaluated within interval")
	}
}

func TestInvalidateWorkflowEvalStates(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowEvalStates = map[string]WorkflowEvalState{
		"project-1": {LastEvalTime: time.Now(), RulesHash: "hash1"},
		"project-2": {LastEvalTime: time.Now(), RulesHash: "hash2"},
	}

	cfg.InvalidateWorkflowEvalStates()

	for projectID, state := range cfg.WorkflowEvalStates {
		if state.RulesHash != "" {
			t.Errorf("project %s should have empty RulesHash after invalidation", projectID)
		}
		if state.LastEvalTime.IsZero() {
			t.Errorf("project %s should retain LastEvalTime after invalidation", projectID)
		}
	}
}

func TestInvalidateWorkflowEvalStates_NilMap(t *testing.T) {
	cfg := &Config{}
	cfg.WorkflowEvalStates = nil

	cfg.InvalidateWorkflowEvalStates()

	if cfg.WorkflowEvalStates != nil {
		t.Error("should not create map when nil")
	}
}
