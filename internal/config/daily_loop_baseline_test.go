package config

import (
	"encoding/json"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	tasktemplate "github.com/sglavoie/tt/internal/template"
	"github.com/sglavoie/tt/internal/workflow"
)

func baselineJSONFields(t *testing.T, value any) map[string]json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal baseline fixture: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatalf("decode baseline fixture: %v", err)
	}
	return fields
}

func requireBaselineFields(t *testing.T, fields map[string]json.RawMessage, present, absent []string) {
	t.Helper()
	for _, key := range present {
		if _, ok := fields[key]; !ok {
			t.Errorf("current JSON shape is missing %q", key)
		}
	}
	for _, key := range absent {
		if _, ok := fields[key]; ok {
			t.Errorf("current JSON shape unexpectedly contains future field %q", key)
		}
	}
}

// TestDailyLoopCurrentPlanningAndAutomationJSONShapes records each canonical
// persistence shape as its staged migration lands.
func TestDailyLoopCurrentPlanningAndAutomationJSONShapes(t *testing.T) {
	t.Run("planning memberships", func(t *testing.T) {
		fields := baselineJSONFields(t, PlanningState{
			FocusQueue: []FocusQueueItem{{TaskID: "next-task", ProjectID: "project"}},
			DailyPlan:  []DailyPlanItem{{TaskID: "today-task", ProjectID: "project"}},
		})
		requireBaselineFields(t, fields, []string{"focus_queue", "daily_plan"}, nil)
	})

	t.Run("saved filter", func(t *testing.T) {
		states, err := models.NewWorkflowStateSet(models.StateNext)
		if err != nil {
			t.Fatalf("state set: %v", err)
		}
		filter := SavedFilter{
			ID:   "filter",
			Name: "Canonical filter",
			Mode: "snapshot",
		}
		filter.SetFilterStates(states, true)
		fields := baselineJSONFields(t, filter)
		requireBaselineFields(t, fields, []string{"filter_states"}, []string{"filter_status", "filter_sections"})
	})

	t.Run("task template", func(t *testing.T) {
		fields := baselineJSONFields(t, tasktemplate.TaskTemplate{
			ID:      "template",
			Name:    "Canonical template",
			Enabled: true,
			Type:    models.TaskTypeTask,
			State:   models.StateInbox,
		})
		requireBaselineFields(t, fields, []string{"state"}, []string{"section"})
	})

	t.Run("workflow rule", func(t *testing.T) {
		fields := baselineJSONFields(t, workflow.WorkflowRule{
			ID:      "rule",
			Name:    "Canonical workflow",
			Enabled: true,
			Triggers: []workflow.RuleTrigger{{
				Field:     workflow.FieldStateChanged,
				Operation: workflow.OpEquals,
				Value:     "ready,next->in_progress",
			}},
			Actions: []workflow.RuleAction{{
				ActionType: workflow.ActionSetState,
				Value:      string(models.StateDone),
			}},
			Scope: &workflow.RuleScope{States: []models.WorkflowState{models.StateNext}},
		})
		requireBaselineFields(t, fields, []string{"triggers", "actions", "scope"}, []string{"trigger"})

		var scopeFields map[string]json.RawMessage
		if err := json.Unmarshal(fields["scope"], &scopeFields); err != nil {
			t.Fatalf("decode workflow scope: %v", err)
		}
		requireBaselineFields(t, scopeFields, []string{"states"}, []string{"sections"})
	})
}
