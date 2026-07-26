package cmd

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

func TestFormatGitStyleDiffUsesCanonicalStateVocabulary(t *testing.T) {
	before := &workflow.WorkflowRule{
		Name:     "State route",
		Enabled:  true,
		Triggers: []workflow.RuleTrigger{{Field: workflow.FieldState, Operation: workflow.OpEquals, Value: "ready"}},
		Actions:  []workflow.RuleAction{{ActionType: workflow.ActionSetState, Value: "next"}},
		Scope:    &workflow.RuleScope{States: []models.WorkflowState{models.StateReady}},
	}
	after := &workflow.WorkflowRule{
		Name:     "State route",
		Enabled:  true,
		Triggers: []workflow.RuleTrigger{{Field: workflow.FieldState, Operation: workflow.OpIn, Value: "ready,next"}},
		Actions:  []workflow.RuleAction{{ActionType: workflow.ActionSetState, Value: "in_progress"}},
		Scope:    &workflow.RuleScope{States: []models.WorkflowState{models.StateReady, models.StateNext}},
	}

	diff := formatGitStyleDiff(before, after)
	for _, want := range []string{
		"-   0. state = ready",
		"+   0. state in ready,next",
		"-   0. set state to next",
		"+   0. set state to in_progress",
		"- Scope: states=[ready]",
		"+ Scope: states=[ready, next]",
	} {
		if !strings.Contains(diff, want) {
			t.Errorf("diff missing %q:\n%s", want, diff)
		}
	}
	if strings.Contains(diff, "status") || strings.Contains(diff, "section") {
		t.Fatalf("canonical workflow diff leaked legacy lifecycle vocabulary:\n%s", diff)
	}
}
