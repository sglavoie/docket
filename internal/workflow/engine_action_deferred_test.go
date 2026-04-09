package workflow

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestApplyAction_CreateFollowup(t *testing.T) {
	task := &models.Task{ID: "original-123", Title: "Original task", RelatedTo: []string{}}
	result := &EvaluationResult{
		AppliedRules:      make(map[string][]string),
		RuleTriggerCounts: make(map[string]int),
	}
	action := RuleAction{ActionType: ActionCreateFollowup, Value: "Follow up on original"}

	modified := applyCreateFollowup(task, action, result)

	if !modified {
		t.Error("expected applyCreateFollowup to return true")
	}
	if len(result.NewFollowupTasks) != 1 {
		t.Fatalf("expected 1 follow-up task, got %d", len(result.NewFollowupTasks))
	}

	followup := result.NewFollowupTasks[0]
	if followup.Title != "Follow up on original" {
		t.Errorf("expected title %q, got %q", "Follow up on original", followup.Title)
	}
	if followup.Status != models.StatusTodo {
		t.Errorf("expected status todo, got %s", followup.Status)
	}
	if followup.Priority != models.PriorityDefault {
		t.Errorf("expected priority %d, got %d", models.PriorityDefault, followup.Priority)
	}
	if !followup.HasTag("followup") {
		t.Error("expected follow-up task to have 'followup' tag")
	}

	foundOriginalInFollowup := false
	for _, id := range followup.RelatedTo {
		if id == task.ID {
			foundOriginalInFollowup = true
		}
	}
	if !foundOriginalInFollowup {
		t.Error("expected follow-up to have RelatedTo pointing to original task")
	}

	foundFollowupInOriginal := false
	for _, id := range task.RelatedTo {
		if id == followup.ID {
			foundFollowupInOriginal = true
		}
	}
	if !foundFollowupInOriginal {
		t.Error("expected original task to have RelatedTo pointing to follow-up")
	}
}

func TestApplyAction_MoveToProject(t *testing.T) {
	task := &models.Task{ID: "task-1", Title: "Test task", Section: "Backlog"}
	result := &EvaluationResult{
		AppliedRules:      make(map[string][]string),
		RuleTriggerCounts: make(map[string]int),
	}
	action := RuleAction{ActionType: ActionMoveToProject, Value: "DestProject"}

	modified := applyMoveToProject(task, action, result)

	if !modified {
		t.Error("expected applyMoveToProject to return true")
	}
	if len(result.DeferredMoves) != 1 {
		t.Fatalf("expected 1 deferred move, got %d", len(result.DeferredMoves))
	}
	move := result.DeferredMoves[0]
	if move.TaskID != "task-1" {
		t.Errorf("expected TaskID %q, got %q", "task-1", move.TaskID)
	}
	if move.Destination != "DestProject" {
		t.Errorf("expected Destination %q, got %q", "DestProject", move.Destination)
	}
	if task.Section != "Backlog" {
		t.Error("expected task section to remain unchanged")
	}
}
