package app_test

import (
	"errors"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

func TestApplyWorkflowsAppliesFieldMutation(t *testing.T) {
	task := newTask("t1", 1, "task")
	task.Priority = 3
	projA := newProject("proj-a", "Alpha", task)
	repo := newFakeRepo(projA)
	before := repo.revisions[projA.ID]
	svc := newTestService(repo)

	rules := []workflow.WorkflowRule{{
		ID:       "r1",
		Name:     "Escalate P3",
		Enabled:  true,
		Triggers: []workflow.RuleTrigger{{Field: workflow.FieldPriority, Operation: workflow.OpEquals, Value: "3"}}, Actions: []workflow.RuleAction{{ActionType: workflow.ActionSetPriority, Value: "1"}}}}

	res, err := svc.ApplyWorkflows("proj-a", rules, nil)
	if err != nil {
		t.Fatalf("ApplyWorkflows: %v", err)
	}
	if task.Priority != 1 {
		t.Errorf("priority = %d, want 1 after rule", task.Priority)
	}
	if len(res.ModifiedTaskIDs) != 1 || res.ModifiedTaskIDs[0] != "t1" {
		t.Errorf("ModifiedTaskIDs = %v, want [t1]", res.ModifiedTaskIDs)
	}
	if res.Revisions[projA.ID] == "" || res.Revisions[projA.ID] == before {
		t.Errorf("revision = %q, want a new revision (was %q)", res.Revisions[projA.ID], before)
	}
	if len(repo.commits) != 1 {
		t.Errorf("Commit called %d times, want 1", len(repo.commits))
	}
}

func TestApplyWorkflowsSetStateUsesServiceTransitionClock(t *testing.T) {
	task := newTask("t1", 1, "task")
	task.Priority = 1
	task.WorkflowState = models.StateReady
	projA := newProject("proj-a", "Alpha", task)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)
	rules := []workflow.WorkflowRule{{
		ID: "r-state", Name: "Queue high priority", Enabled: true,
		Triggers: []workflow.RuleTrigger{{
			Field: workflow.FieldState, Operation: workflow.OpEquals, Value: "ready",
		}},
		Actions: []workflow.RuleAction{{ActionType: workflow.ActionSetState, Value: "next"}},
	}}

	if _, err := svc.ApplyWorkflows(projA.ID, rules, nil); err != nil {
		t.Fatalf("ApplyWorkflows: %v", err)
	}
	wantTime := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	if task.State() != models.StateNext {
		t.Fatalf("state = %s, want next", task.State())
	}
	if !task.UpdatedAt.Equal(wantTime) {
		t.Fatalf("UpdatedAt = %v, want service clock %v", task.UpdatedAt, wantTime)
	}
}

func TestApplyWorkflowsSurfacesConflictWithoutCommitting(t *testing.T) {
	task := newTask("t1", 1, "task")
	task.Priority = 3
	projA := newProject("proj-a", "Alpha", task)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	rules := []workflow.WorkflowRule{
		{
			ID:       "r1",
			Name:     "To P1",
			Enabled:  true,
			Triggers: []workflow.RuleTrigger{{Field: workflow.FieldPriority, Operation: workflow.OpEquals, Value: "3"}}, Actions: []workflow.RuleAction{{ActionType: workflow.ActionSetPriority, Value: "1"}}},
		{
			ID:       "r2",
			Name:     "To P5",
			Enabled:  true,
			Triggers: []workflow.RuleTrigger{{Field: workflow.FieldPriority, Operation: workflow.OpEquals, Value: "3"}}, Actions: []workflow.RuleAction{{ActionType: workflow.ActionSetPriority, Value: "5"}}},
	}

	res, err := svc.ApplyWorkflows("proj-a", rules, nil)
	if err != nil {
		t.Fatalf("ApplyWorkflows: %v", err)
	}
	if len(res.Conflicts) != 1 {
		t.Fatalf("Conflicts = %v, want exactly one", res.Conflicts)
	}
	if res.Conflicts[0].Field != "priority" {
		t.Errorf("conflict field = %q, want priority", res.Conflicts[0].Field)
	}
	// Same-priority conflict: neither rule applied, so the field is untouched and
	// nothing is committed.
	if task.Priority != 3 {
		t.Errorf("priority = %d, want 3 (conflict left it unchanged)", task.Priority)
	}
	if len(repo.commits) != 0 {
		t.Errorf("conflicting cascade committed %d times, want 0", len(repo.commits))
	}
}

func TestApplyWorkflowsExecutesCrossProjectMoveInOneTransaction(t *testing.T) {
	task := newTask("t1", 1, "route me")
	task.WorkflowState = models.StateReady
	projA := newProject("proj-a", "Alpha", task)
	projB := newProject("proj-b", "Beta")
	repo := newFakeRepo(projA, projB)
	svc := newTestService(repo)

	rules := []workflow.WorkflowRule{{
		ID:       "r1",
		Name:     "Route ready tasks to Beta",
		Enabled:  true,
		Triggers: []workflow.RuleTrigger{{Field: workflow.FieldState, Operation: workflow.OpEquals, Value: string(models.StateReady)}}, Actions: []workflow.RuleAction{{ActionType: workflow.ActionMoveToProject, Value: "Beta"}}}}

	res, err := svc.ApplyWorkflows("proj-a", rules, map[string]*models.Task{})
	if err != nil {
		t.Fatalf("ApplyWorkflows: %v", err)
	}
	if len(res.Moves) != 1 || res.Moves[0].SourceProjectID != "proj-a" || res.Moves[0].DestProjectID != "proj-b" {
		t.Fatalf("Moves = %v, want one move to proj-b", res.Moves)
	}
	if len(projA.Tasks) != 0 {
		t.Errorf("source has %d tasks, want 0 after move", len(projA.Tasks))
	}
	if len(projB.Tasks) != 1 {
		t.Errorf("dest has %d tasks, want 1 after move", len(projB.Tasks))
	}
	// The field mutation and the cross-project move commit as one transaction.
	if len(repo.commits) != 1 {
		t.Fatalf("Commit called %d times, want 1 (atomic cascade)", len(repo.commits))
	}
	if len(repo.commits[0]) != 2 {
		t.Errorf("transaction carried %d changesets, want 2 (source + dest)", len(repo.commits[0]))
	}
}

func TestApplyWorkflowsNoRulesNoCommit(t *testing.T) {
	task := newTask("t1", 1, "task")
	projA := newProject("proj-a", "Alpha", task)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	res, err := svc.ApplyWorkflows("proj-a", nil, nil)
	if err != nil {
		t.Fatalf("ApplyWorkflows: %v", err)
	}
	if len(res.ModifiedTaskIDs) != 0 {
		t.Errorf("ModifiedTaskIDs = %v, want empty", res.ModifiedTaskIDs)
	}
	if len(repo.commits) != 0 {
		t.Errorf("no-op cascade committed %d times, want 0", len(repo.commits))
	}
}

func TestApplyWorkflowsUnknownProject(t *testing.T) {
	repo := newFakeRepo(newProject("proj-a", "Alpha"))
	svc := newTestService(repo)

	if _, err := svc.ApplyWorkflows("missing", nil, nil); !errors.Is(err, app.ErrProjectNotFound) {
		t.Fatalf("err = %v, want ErrProjectNotFound", err)
	}
}
