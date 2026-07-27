package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/workflow"
)

// WorkflowRulesModel handles the workflow rules management view.
// The implementation is split across multiple files:
// - workflow_rules.go: core model, constructor, and shared helpers
// - workflow_rules_update.go: Update() and state mutation helpers
// - workflow_rules_view.go: View() and list rendering
// - workflow_rules_preview.go: preview rendering helpers
type WorkflowRulesModel struct {
	BaseSearchableView
	// runtime is the domain-native snapshot: workflow rules read from
	// runtime.Automation() and mutations commit through the coordinator built
	// over it (RuntimeState.CommitAutomation), which refreshes the snapshot.
	runtime *config.RuntimeState
	// coordinator routes the durable automation-domain write through the
	// secondary feature-coordinator seam (feature_coordinator_secondary.go).
	coordinator WorkflowCoordinator

	InteractiveViewState
	allRules      []workflow.WorkflowRule
	filteredRules []workflow.WorkflowRule
	defaultCount  int // Number of default rules (at start of allRules slice)

	expandedView bool
}

// NewWorkflowRulesModel creates a new workflow rules model.
func NewWorkflowRulesModel(runtime *config.RuntimeState) *WorkflowRulesModel {
	defaultRules, customRules := runtime.Automation().GetWorkflowRulesSorted()
	allRules := append(defaultRules, customRules...)
	var coordinator WorkflowCoordinator
	if runtime != nil {
		coordinator = newWorkflowCoordinator(runtime)
	}
	m := &WorkflowRulesModel{
		runtime:       runtime,
		coordinator:   coordinator,
		allRules:      allRules,
		filteredRules: allRules,
		defaultCount:  len(defaultRules),
	}
	m.SetSize(80, 24)
	m.InitSearchInput("Search rules...", 50, 30)
	return m
}

// SetSize sets the view dimensions.
func (m *WorkflowRulesModel) SetSize(width, height int) {
	m.BaseSearchableView.SetSize(width, height)
}

func (m *WorkflowRulesModel) Init() tea.Cmd {
	return nil
}

// isDefaultRule returns true if the rule with the given ID is a default rule.
func (m *WorkflowRulesModel) isDefaultRule(id string) bool {
	for i := 0; i < m.defaultCount && i < len(m.allRules); i++ {
		if m.allRules[i].ID == id {
			return true
		}
	}
	return false
}

// isWideScreen returns true if the screen is wide enough for split layout (≥81 chars)
func (m *WorkflowRulesModel) isWideScreen() bool {
	return m.Width() >= 81
}

// WorkflowRulesUpdatedMsg is sent when rules are modified.
type WorkflowRulesUpdatedMsg struct{}

// ShowWorkflowRulesMsg triggers showing the workflow rules view.
type ShowWorkflowRulesMsg struct{}

// formatRuleScope returns a human-readable summary of a RuleScope.
func formatRuleScope(s *workflow.RuleScope) string {
	if s == nil {
		return "all"
	}
	var parts []string
	if len(s.Projects) > 0 {
		parts = append(parts, "projects=["+strings.Join(s.Projects, ", ")+"]")
	}
	if len(s.Types) > 0 {
		parts = append(parts, "types=["+strings.Join(s.Types, ", ")+"]")
	}
	if len(s.States) > 0 {
		states := make([]string, 0, len(s.States))
		for _, state := range s.States {
			states = append(states, state.String())
		}
		parts = append(parts, "states=["+strings.Join(states, ", ")+"]")
	}
	if len(parts) == 0 {
		return "all"
	}
	return strings.Join(parts, ", ")
}
