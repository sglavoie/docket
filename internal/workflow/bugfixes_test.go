package workflow

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TestApplyHierarchicalAction_RecordsParentInModifiedTasks ensures that a
// rule mutating a parent via ActionCompleteParent surfaces the parent in
// result.ModifiedTasks. Without this, the caller would silently lose the
// parent's status change at persist time.
func TestApplyHierarchicalAction_RecordsParentInModifiedTasks(t *testing.T) {
	parent := &models.Task{ID: "p", Status: models.StatusTodo}
	child := &models.Task{ID: "c", Status: models.StatusDone, Parent: parent}
	parent.Children = []*models.Task{child}

	evalCtx := &EvaluationContext{
		Changes: []TaskChangeContext{
			{Task: child, Previous: &models.Task{ID: "c", Status: models.StatusInProgress}},
		},
		AllTasks: []*models.Task{parent, child},
		TaskMap:  map[string]*models.Task{"p": parent, "c": child},
	}

	rules := []WorkflowRule{
		{
			ID:      "r1",
			Name:    "Complete parent when child completes",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldCompleted, Operation: OpEquals, Value: "true"},
			Actions: []RuleAction{{ActionType: ActionCompleteParent}},
		},
	}

	result := EvaluateRulesWithContext(evalCtx, rules)
	if !parent.IsComplete() {
		t.Fatalf("expected parent to be completed, got status %q", parent.Status)
	}
	if !containsTask(result.ModifiedTasks, parent) {
		t.Errorf("expected parent in ModifiedTasks, got %d entries", len(result.ModifiedTasks))
	}
}

// TestApplyHierarchicalAction_BlockParentRecorded verifies the parent is
// surfaced as modified when ActionBlockParent fires.
func TestApplyHierarchicalAction_BlockParentRecorded(t *testing.T) {
	parent := &models.Task{ID: "p", Status: models.StatusTodo}
	child := &models.Task{ID: "c", Status: models.StatusBlocked, Parent: parent}
	parent.Children = []*models.Task{child}

	evalCtx := &EvaluationContext{
		AllTasks: []*models.Task{parent, child},
		TaskMap:  map[string]*models.Task{"p": parent, "c": child},
	}

	result := &EvaluationResult{}
	modified := applyHierarchicalAction(child, RuleAction{ActionType: ActionBlockParent}, evalCtx, result)
	if !modified {
		t.Fatal("expected modified")
	}
	if parent.Status != models.StatusBlocked {
		t.Fatalf("expected parent blocked, got %q", parent.Status)
	}
	if !containsTask(result.ModifiedTasks, parent) {
		t.Error("expected blocked parent in ModifiedTasks")
	}
}

// TestApplyAction_SetRecurrenceUnchangedNoOp verifies that setting the same
// recurrence pattern that is already on the task does not report the task as
// modified. Reporting spurious modifications causes recursive re-evaluation
// and may re-trigger downstream rules with no actual state change.
func TestApplyAction_SetRecurrenceUnchangedNoOp(t *testing.T) {
	existing := models.ParseRecurrence("daily")
	if existing == nil {
		t.Fatal("ParseRecurrence(daily) returned nil")
	}
	task := &models.Task{ID: "1", Recurrence: existing}

	modified := applyAction(task, RuleAction{ActionType: ActionSetRecurrence, Value: "daily"})
	if modified {
		t.Error("expected applyAction to return false when recurrence pattern is unchanged")
	}
}

// TestApplyAction_SetSnoozeUnchangedNoOp verifies that re-asserting the same
// snooze duration on a task whose SnoozedUntil already points at that target
// time does not re-mark the task as modified.
func TestApplyAction_SetSnoozeUnchangedNoOp(t *testing.T) {
	pre := time.Now().AddDate(0, 0, 3)
	task := &models.Task{ID: "1", SnoozedUntil: &pre}

	modified := applyAction(task, RuleAction{ActionType: ActionSetSnooze, Value: "+3d"})
	if modified {
		t.Error("expected applyAction to return false when snooze target is essentially unchanged")
	}
}

// TestEvaluation_ConditionalRuleReFiresAfterRecursiveRound verifies that a
// rule whose action conditions are not satisfied initially is NOT marked as
// permanently applied. If a later rule's mutation triggers a recursive
// re-evaluation that makes the condition true, the deferred rule must be
// able to fire on the recursive pass.
func TestEvaluation_ConditionalRuleReFiresAfterRecursiveRound(t *testing.T) {
	// Task starts as status=todo, no tag "ready".
	// Rule A: priority=1. When status=todo, set status=done IF tag has "ready".
	// Rule B: priority=2. When status=todo, add tag "ready".
	//
	// Without the fix, A is marked applied on pass 1 (even though the
	// condition was unmet) so it cannot fire on the recursive pass kicked
	// off by B adding the tag — status remains todo.
	task := &models.Task{
		ID:     "task-1",
		Status: models.StatusTodo,
		Tags:   []string{},
	}

	rules := []WorkflowRule{
		{
			ID:       "rule-A",
			Name:     "Complete when ready",
			Enabled:  true,
			Priority: 1,
			Trigger:  RuleTrigger{Field: FieldStatus, Operation: OpEquals, Value: "todo"},
			Actions: []RuleAction{
				{
					ActionType: ActionSetStatus,
					Value:      string(models.StatusDone),
					Condition:  &RuleTrigger{Field: FieldTags, Operation: OpContains, Value: "ready"},
				},
			},
		},
		{
			ID:       "rule-B",
			Name:     "Add ready tag",
			Enabled:  true,
			Priority: 2,
			Trigger:  RuleTrigger{Field: FieldStatus, Operation: OpEquals, Value: "todo"},
			Actions:  []RuleAction{{ActionType: ActionAddTag, Value: "ready"}},
		},
	}

	evalCtx := &EvaluationContext{
		Changes:  []TaskChangeContext{{Task: task, Previous: nil}},
		AllTasks: []*models.Task{task},
		TaskMap:  map[string]*models.Task{"task-1": task},
	}

	_ = EvaluateRulesWithContext(evalCtx, rules)

	if !task.HasTag("ready") {
		t.Fatalf("expected tag 'ready' to be added by rule B, got tags: %v", task.Tags)
	}
	if task.Status != models.StatusDone {
		t.Errorf("expected status=done after rule A's condition became true via recursive re-eval, got %s", task.Status)
	}
}
