package workflow

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestEventTrigger_SectionChanged(t *testing.T) {
	// A section is only meaningful for a planning-state task: under the workflow
	// state model an in_progress task collapses to StateInProgress with no
	// section, so the section-changed event is seeded with todo-status tasks
	// whose Next/Inbox sections map to distinct states.
	prevTask := &models.Task{
		ID:            "task-1",
		WorkflowState: models.StateNext,
	}
	currTask := &models.Task{
		ID:            "task-1",
		WorkflowState: models.StateInbox,
	}

	evalCtx := &EvaluationContext{
		Changes:  []TaskChangeContext{{Task: currTask, Previous: prevTask}},
		AllTasks: []*models.Task{currTask},
		TaskMap:  map[string]*models.Task{"task-1": currTask},
	}

	rules := []WorkflowRule{
		{
			ID:      "rule-1",
			Name:    "Section change",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldSectionChanged, Operation: OpEquals, Value: "Next->Inbox"},
			Actions: []RuleAction{{ActionType: ActionAddTag, Value: "moved"}},
		},
	}

	result := EvaluateRulesWithContext(evalCtx, rules)

	if len(result.ModifiedTasks) != 1 {
		t.Fatalf("expected 1 modified task, got %d", len(result.ModifiedTasks))
	}
	if !currTask.HasTag("moved") {
		t.Error("expected task to have 'moved' tag from section change trigger")
	}
}

func TestMatchesTrigger_SectionUsesSectionField(t *testing.T) {
	t.Run("contains matches GTD section", func(t *testing.T) {
		task := &models.Task{ID: "t1", WorkflowState: models.StateNext}
		trigger := RuleTrigger{Field: FieldSection, Operation: OpContains, Value: "ext"}
		if !matchesTrigger(task, trigger) {
			t.Error("'Next' contains 'ext', should match")
		}
	})

	t.Run("equals empty matches Active view task", func(t *testing.T) {
		task := &models.Task{ID: "t1", WorkflowState: models.StateReady}
		trigger := RuleTrigger{Field: FieldSection, Operation: OpEquals, Value: ""}
		if !matchesTrigger(task, trigger) {
			t.Error("task in Active view should match section equals ''")
		}
	})
}
