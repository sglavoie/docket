package app_test

import (
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

// TestCommitProjectWithWorkflowBareMutationCommitsOnce verifies the no-rules path
// commits the caller's already-mutated project exactly once with the given undo
// label and no trigger counts.
func TestCommitProjectWithWorkflowBareMutationCommitsOnce(t *testing.T) {
	t1 := newTask("t1", 1, "first")
	t2 := newTask("t2", 2, "second")
	projA := newProject("proj-a", "Alpha", t1, t2)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	// A structural reorder the ProjectTree/Outliner view would apply in memory.
	projA.SwapTasks(t1, t2)

	res, err := svc.CommitProjectWithWorkflow(projA, app.ProjectWorkflowOptions{UndoAction: "Move Down"})
	if err != nil {
		t.Fatalf("CommitProjectWithWorkflow: %v", err)
	}
	if projA.Tasks[0].ID != "t2" || projA.Tasks[1].ID != "t1" {
		t.Errorf("committed order = %v, want [t2 t1]", []string{projA.Tasks[0].ID, projA.Tasks[1].ID})
	}
	if res.RuleTriggerCounts != nil {
		t.Errorf("RuleTriggerCounts = %v, want nil with no rules", res.RuleTriggerCounts)
	}
	if len(res.TouchedProjectIDs) != 1 || res.TouchedProjectIDs[0] != "proj-a" {
		t.Errorf("TouchedProjectIDs = %v, want [proj-a]", res.TouchedProjectIDs)
	}
	if len(repo.commits) != 1 {
		t.Errorf("Commit called %d times, want 1", len(repo.commits))
	}
	if got := repo.commits[0][0].UndoAction; got != "Move Down" {
		t.Errorf("UndoAction = %q, want Move Down", got)
	}
}

// TestCommitProjectWithWorkflowStateBasedRuleFires verifies a state-based rule
// fires against the already-mutated project.
func TestCommitProjectWithWorkflowStateBasedRuleFires(t *testing.T) {
	t1 := newTask("t1", 1, "deploy")
	projA := newProject("proj-a", "Alpha", t1)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	// View mutation: complete the task in memory, then persist with the cascade.
	t1.Complete()

	res, err := svc.CommitProjectWithWorkflow(projA, app.ProjectWorkflowOptions{
		Rules:      []workflow.WorkflowRule{stateBasedTagRule()},
		UndoAction: "Complete Task",
	})
	if err != nil {
		t.Fatalf("CommitProjectWithWorkflow: %v", err)
	}
	if !t1.HasTag("shipped") {
		t.Errorf("state-based rule did not tag the completed task; tags = %v", t1.Tags)
	}
	if res.RuleTriggerCounts["r-status-tag"] != 1 {
		t.Errorf("trigger count = %d, want 1", res.RuleTriggerCounts["r-status-tag"])
	}
}

// TestCommitProjectWithWorkflowContextualRuleFires verifies contextual evaluation:
// a status-changed rule fires only when the caller supplies pre-mutation states.
func TestCommitProjectWithWorkflowContextualRuleFires(t *testing.T) {
	t1 := newTask("t1", 1, "deploy")
	projA := newProject("proj-a", "Alpha", t1)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	prev := workflow.CaptureTaskStates(projA.Tasks)
	t1.Complete()

	if _, err := svc.CommitProjectWithWorkflow(projA, app.ProjectWorkflowOptions{
		Rules:      []workflow.WorkflowRule{contextualTagRule()},
		PrevStates: prev,
	}); err != nil {
		t.Fatalf("CommitProjectWithWorkflow: %v", err)
	}
	if !t1.HasTag("shipped") {
		t.Errorf("contextual status-changed rule did not fire; tags = %v", t1.Tags)
	}
}

// TestCommitProjectWithWorkflowInfersContext verifies that callers which mutate
// an in-memory project do not have to supply their own previous-state map.
func TestCommitProjectWithWorkflowInfersContext(t *testing.T) {
	t1 := newTask("t1", 1, "deploy")
	projA := newProject("proj-a", "Alpha", t1)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	t1.Complete()
	if _, err := svc.CommitProjectWithWorkflow(projA, app.ProjectWorkflowOptions{
		Rules: []workflow.WorkflowRule{contextualTagRule()},
	}); err != nil {
		t.Fatalf("CommitProjectWithWorkflow: %v", err)
	}
	if !t1.HasTag("shipped") {
		t.Errorf("status-changed rule did not fire with inferred context; tags = %v", t1.Tags)
	}
}

// TestCommitProjectWithWorkflowDeferredMoveCommitsOneTransaction verifies a
// move_to_project rule relocates the subtree and commits source + destination as
// one transaction touching both projects.
func TestCommitProjectWithWorkflowDeferredMoveCommitsOneTransaction(t *testing.T) {
	t1 := newTask("t1", 1, "movable")
	projA := newProject("proj-a", "Alpha", t1)
	projB := newProject("proj-b", "Beta")
	repo := newFakeRepo(projA, projB)
	svc := newTestService(repo)

	rules := []workflow.WorkflowRule{{
		ID:       "r-move-done",
		Name:     "Archive done to Beta",
		Enabled:  true,
		Triggers: []workflow.RuleTrigger{{Field: workflow.FieldState, Operation: workflow.OpEquals, Value: "done"}}, Actions: []workflow.RuleAction{{ActionType: workflow.ActionMoveToProject, Value: "Beta"}}}}

	t1.Complete()
	res, err := svc.CommitProjectWithWorkflow(projA, app.ProjectWorkflowOptions{Rules: rules})
	if err != nil {
		t.Fatalf("CommitProjectWithWorkflow: %v", err)
	}
	if projB.GetTaskByID("t1") == nil {
		t.Fatalf("task not moved to destination; projB tasks = %d", len(projB.Tasks))
	}
	if projA.GetTaskByID("t1") != nil {
		t.Errorf("task still present in source after deferred move")
	}
	if len(repo.commits) != 1 {
		t.Fatalf("Commit called %d times, want 1", len(repo.commits))
	}
	if got := len(repo.commits[0]); got != 2 {
		t.Errorf("changesets = %d, want 2 (source + destination)", got)
	}
	if len(res.TouchedProjectIDs) != 2 {
		t.Errorf("TouchedProjectIDs = %v, want two projects", res.TouchedProjectIDs)
	}
}

// TestCommitProjectWithWorkflowNilProjectRejected verifies the guard.
func TestCommitProjectWithWorkflowNilProjectRejected(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)
	if _, err := svc.CommitProjectWithWorkflow(nil, app.ProjectWorkflowOptions{}); err == nil {
		t.Fatal("expected an error for a nil project")
	}
	if len(repo.commits) != 0 {
		t.Errorf("Commit called %d times, want 0", len(repo.commits))
	}
}

// Ensure models import is referenced (used by shared harness helpers).
var _ = models.StateDone
