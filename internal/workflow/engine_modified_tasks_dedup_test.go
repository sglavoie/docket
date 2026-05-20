package workflow

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestModifiedTasksNoDuplicates(t *testing.T) {
	prev := &models.Task{ID: "1", Status: models.StatusTodo, Tags: []string{}}
	curr := &models.Task{ID: "1", Status: models.StatusInProgress, Tags: []string{}}

	evalCtx := &EvaluationContext{
		Changes:  []TaskChangeContext{{Task: curr, Previous: prev}},
		AllTasks: []*models.Task{curr},
		TaskMap:  map[string]*models.Task{"1": curr},
	}

	rules := []WorkflowRule{
		{
			ID: "r1", Name: "AddTagA", Enabled: true,
			Trigger: RuleTrigger{Field: FieldStatusChanged, Operation: OpEquals, Value: "todo->in_progress"},
			Actions: []RuleAction{{ActionType: ActionAddTag, Value: "a"}},
		},
		{
			ID: "r2", Name: "AddTagB", Enabled: true,
			Trigger: RuleTrigger{Field: FieldTagAdded, Operation: OpEquals, Value: "a"},
			Actions: []RuleAction{{ActionType: ActionAddTag, Value: "b"}},
		},
	}

	result := EvaluateRulesWithContext(evalCtx, rules)
	if len(result.ModifiedTasks) != 1 {
		t.Errorf("ModifiedTasks should have 1 entry (no duplicates), got %d: %+v", len(result.ModifiedTasks), result.ModifiedTasks)
	}
}
