package config

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/workflow"
)

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

func TestShouldEvaluateWorkflow_FalseWhenNegativeInterval(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1", Enabled: true},
	}
	val := -5
	cfg.WorkflowEvalInterval = &val

	if cfg.ShouldEvaluateWorkflow("project-1") {
		t.Error("should return false when negative interval is clamped to disabled")
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
