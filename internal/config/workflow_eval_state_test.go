package config

import (
	"testing"
	"time"
)

func TestGetWorkflowEvalState_NilMap(t *testing.T) {
	cfg := &AutomationConfig{}
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
	cfg := NewAutomationConfig()
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
	cfg := &AutomationConfig{}
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
	cfg := NewAutomationConfig()
	cfg.WorkflowEvalStates = map[string]WorkflowEvalState{
		"project-1": {RulesHash: "old"},
	}

	cfg.SetWorkflowEvalState("project-1", WorkflowEvalState{RulesHash: "new"})

	if cfg.WorkflowEvalStates["project-1"].RulesHash != "new" {
		t.Error("state should be overwritten")
	}
}

func TestGetWorkflowEvalInterval_DefaultValue(t *testing.T) {
	cfg := &AutomationConfig{}
	cfg.WorkflowEvalInterval = nil

	interval := cfg.GetWorkflowEvalInterval()
	if interval != 60 {
		t.Errorf("expected default interval 60, got %d", interval)
	}
}

func TestGetWorkflowEvalInterval_CustomValue(t *testing.T) {
	cfg := NewAutomationConfig()
	val := 30
	cfg.WorkflowEvalInterval = &val

	interval := cfg.GetWorkflowEvalInterval()
	if interval != 30 {
		t.Errorf("expected interval 30, got %d", interval)
	}
}

func TestGetWorkflowEvalInterval_ZeroDisabled(t *testing.T) {
	cfg := NewAutomationConfig()
	val := 0
	cfg.WorkflowEvalInterval = &val

	interval := cfg.GetWorkflowEvalInterval()
	if interval != 0 {
		t.Errorf("expected interval 0 (disabled), got %d", interval)
	}
}

func TestGetWorkflowEvalInterval_NegativeDisabled(t *testing.T) {
	cfg := NewAutomationConfig()
	val := -5
	cfg.WorkflowEvalInterval = &val

	interval := cfg.GetWorkflowEvalInterval()
	if interval != 0 {
		t.Errorf("expected negative interval to be treated as disabled, got %d", interval)
	}
}

func TestSetWorkflowEvalInterval(t *testing.T) {
	cfg := NewAutomationConfig()

	cfg.SetWorkflowEvalInterval(120)

	if cfg.WorkflowEvalInterval == nil {
		t.Fatal("WorkflowEvalInterval should not be nil")
	}
	if *cfg.WorkflowEvalInterval != 120 {
		t.Errorf("expected interval 120, got %d", *cfg.WorkflowEvalInterval)
	}
}

func TestSetWorkflowEvalInterval_NegativeClampsToDisabled(t *testing.T) {
	cfg := NewAutomationConfig()

	cfg.SetWorkflowEvalInterval(-10)

	if cfg.WorkflowEvalInterval == nil {
		t.Fatal("WorkflowEvalInterval should not be nil")
	}
	if *cfg.WorkflowEvalInterval != 0 {
		t.Errorf("expected negative interval clamped to 0, got %d", *cfg.WorkflowEvalInterval)
	}
}
