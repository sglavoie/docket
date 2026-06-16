package workflow

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestEventTrigger_SectionChanged(t *testing.T) {
	prevTask := &models.Task{
		ID:      "task-1",
		Status:  models.StatusInProgress,
		Section: models.SectionNext,
	}
	currTask := &models.Task{
		ID:      "task-1",
		Status:  models.StatusInProgress,
		Section: models.SectionInbox,
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
		task := &models.Task{ID: "t1", Section: models.SectionNext}
		trigger := RuleTrigger{Field: FieldSection, Operation: OpContains, Value: "ext"}
		if !matchesTrigger(task, trigger) {
			t.Error("'Next' contains 'ext', should match")
		}
	})

	t.Run("equals empty matches Active view task", func(t *testing.T) {
		task := &models.Task{ID: "t1", Section: ""}
		trigger := RuleTrigger{Field: FieldSection, Operation: OpEquals, Value: ""}
		if !matchesTrigger(task, trigger) {
			t.Error("task in Active view should match section equals ''")
		}
	})
}
