package config

import (
	"testing"

	"github.com/sglavoie/tt/internal/workflow"
)

func TestAddDefaultRules_AddsNewRules(t *testing.T) {
	cfg := NewAutomationConfig()

	added := cfg.AddDefaultRules(false, false)

	if added == 0 {
		t.Error("AddDefaultRules should add at least one rule")
	}
	if !cfg.DefaultRulesInitialized {
		t.Error("DefaultRulesInitialized should be true after AddDefaultRules")
	}
}

func TestAddDefaultRules_SkipsExisting(t *testing.T) {
	cfg := NewAutomationConfig()

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
	cfg := NewAutomationConfig()

	cfg.AddDefaultRules(true, false)

	for _, rule := range cfg.WorkflowRules {
		if !rule.Enabled {
			t.Errorf("rule %s should be enabled when enableRules=true", rule.ID)
		}
	}
}

func TestAddDefaultRules_DisableRulesFlag(t *testing.T) {
	cfg := NewAutomationConfig()

	cfg.AddDefaultRules(false, false)

	for _, rule := range cfg.WorkflowRules {
		if rule.Enabled {
			t.Errorf("rule %s should be disabled when enableRules=false", rule.ID)
		}
	}
}

func TestAddDefaultRules_UseOriginalEnabled(t *testing.T) {
	cfg := NewAutomationConfig()

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
