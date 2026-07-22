package app_test

import (
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/workflow"
)

func TestBulkTransitionWithWorkflowFiresEventRuleAndCountsTriggers(t *testing.T) {
	t1 := newTask("t1", 1, "first")
	t2 := newTask("t2", 2, "second")
	projA := newProject("proj-a", "Alpha", t1, t2)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	rules := []workflow.WorkflowRule{{
		ID:      "r-done-tag",
		Name:    "Tag on completion",
		Enabled: true,
		Trigger: workflow.RuleTrigger{Field: workflow.FieldStatusChanged, Operation: workflow.OpEquals, Value: "*->done"},
		Actions: []workflow.RuleAction{{ActionType: workflow.ActionAddTag, Value: "shipped"}},
	}}

	res, err := svc.BulkTransitionWithWorkflow(app.BulkTransitionRequest{
		TaskIDs: []string{"t1", "t2"},
		Target:  models.StateDone,
		Rules:   rules,
	})
	if err != nil {
		t.Fatalf("BulkTransitionWithWorkflow: %v", err)
	}
	if t1.State() != models.StateDone || t2.State() != models.StateDone {
		t.Errorf("states = %s/%s, want done/done", t1.State(), t2.State())
	}
	for _, task := range []*models.Task{t1, t2} {
		if !task.HasTag("shipped") {
			t.Errorf("task %s missing rule-applied tag; tags = %v", task.ID, task.Tags)
		}
	}
	if res.RuleTriggerCounts["r-done-tag"] != 2 {
		t.Errorf("trigger count = %d, want 2", res.RuleTriggerCounts["r-done-tag"])
	}
	if len(repo.commits) != 1 {
		t.Errorf("Commit called %d times, want 1 (transitions and cascade share the transaction)", len(repo.commits))
	}
}

func TestBulkTransitionLocatorsDisambiguateDuplicateTaskIDs(t *testing.T) {
	alphaTask := newTask("shared", 1, "alpha")
	betaTask := newTask("shared", 1, "beta")
	alpha := newProject("alpha", "Alpha", alphaTask)
	beta := newProject("beta", "Beta", betaTask)
	repo := newFakeRepo(alpha, beta)
	svc := newTestService(repo)
	var planningTargets []app.PlanningReconcileTarget

	_, err := svc.BulkTransitionWithWorkflow(app.BulkTransitionRequest{
		TaskLocators: []app.TaskLocator{{ProjectID: beta.ID, TaskID: betaTask.ID}},
		Target:       models.StateInProgress,
		PlanningArtifacts: func(targets []app.PlanningReconcileTarget) ([]storage.ArtifactChange, error) {
			planningTargets = append(planningTargets, targets...)
			return nil, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if alphaTask.State() == models.StateInProgress || betaTask.State() != models.StateInProgress {
		t.Fatalf("states = alpha:%s beta:%s, want only beta in progress", alphaTask.State(), betaTask.State())
	}
	if len(planningTargets) != 1 || planningTargets[0].ProjectID != beta.ID {
		t.Fatalf("planning targets = %+v, want beta/shared only", planningTargets)
	}
}

func TestBulkTransitionWithWorkflowMovesTaskInSameTransaction(t *testing.T) {
	t1 := newTask("t1", 1, "movable")
	projA := newProject("proj-a", "Alpha", t1)
	projB := newProject("proj-b", "Beta")
	repo := newFakeRepo(projA, projB)
	svc := newTestService(repo)

	rules := []workflow.WorkflowRule{{
		ID:      "r-move-done",
		Name:    "Archive done to Beta",
		Enabled: true,
		Trigger: workflow.RuleTrigger{Field: workflow.FieldStatusChanged, Operation: workflow.OpEquals, Value: "*->done"},
		Actions: []workflow.RuleAction{{ActionType: workflow.ActionMoveToProject, Value: "Beta"}},
	}}

	res, err := svc.BulkTransitionWithWorkflow(app.BulkTransitionRequest{
		TaskIDs: []string{"t1"},
		Target:  models.StateDone,
		Rules:   rules,
	})
	if err != nil {
		t.Fatalf("BulkTransitionWithWorkflow: %v", err)
	}
	if projB.GetTaskByID("t1") == nil {
		t.Fatalf("task not moved to destination project; projB tasks = %d", len(projB.Tasks))
	}
	if projA.GetTaskByID("t1") != nil {
		t.Errorf("task still present in source project after move")
	}
	if len(repo.commits) != 1 {
		t.Fatalf("Commit called %d times, want 1 (move rides the bulk transaction)", len(repo.commits))
	}
	if got := len(repo.commits[0]); got != 2 {
		t.Errorf("changesets in the transaction = %d, want 2 (source + destination)", got)
	}
	wantTouched := map[string]bool{"proj-a": true, "proj-b": true}
	for _, id := range res.TouchedProjectIDs {
		delete(wantTouched, id)
	}
	if len(wantTouched) != 0 {
		t.Errorf("TouchedProjectIDs = %v, want proj-a and proj-b", res.TouchedProjectIDs)
	}
}

func TestBulkTransitionWithWorkflowAppendsFollowupTask(t *testing.T) {
	t1 := newTask("t1", 1, "deploy")
	projA := newProject("proj-a", "Alpha", t1)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	rules := []workflow.WorkflowRule{{
		ID:      "r-followup",
		Name:    "Follow up after deploy",
		Enabled: true,
		Trigger: workflow.RuleTrigger{Field: workflow.FieldStatusChanged, Operation: workflow.OpEquals, Value: "*->done"},
		Actions: []workflow.RuleAction{{ActionType: workflow.ActionCreateFollowup, Value: "verify {task}"}},
	}}

	_, err := svc.BulkTransitionWithWorkflow(app.BulkTransitionRequest{
		TaskIDs: []string{"t1"},
		Target:  models.StateDone,
		Rules:   rules,
	})
	if err != nil {
		t.Fatalf("BulkTransitionWithWorkflow: %v", err)
	}
	if len(projA.Tasks) != 2 {
		t.Fatalf("project tasks = %d, want 2 (original + follow-up)", len(projA.Tasks))
	}
	var followup *models.Task
	for _, task := range projA.Tasks {
		if task.ID != "t1" {
			followup = task
		}
	}
	if followup == nil || followup.ShortID == 0 {
		t.Fatalf("follow-up task missing or without a fresh short ID: %+v", followup)
	}
}

func TestBulkTransitionSkipInvalidDropsInvalidAndCommitsValid(t *testing.T) {
	t1 := newTask("t1", 1, "ready")
	t2 := newTask("t2", 2, "already done")
	t2.ApplyWorkflowState(models.StateDone)
	projA := newProject("proj-a", "Alpha", t1, t2)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	res, err := svc.BulkTransitionWithWorkflow(app.BulkTransitionRequest{
		TaskIDs:     []string{"t1", "t2"},
		Target:      models.StateDone,
		SkipInvalid: true,
	})
	if err != nil {
		t.Fatalf("BulkTransitionWithWorkflow: %v", err)
	}
	if len(res.Tasks) != 1 {
		t.Fatalf("changed tasks = %d, want 1 (the already-done task is skipped)", len(res.Tasks))
	}
	if t1.State() != models.StateDone {
		t.Errorf("t1 state = %s, want done", t1.State())
	}
	if len(repo.commits) != 1 {
		t.Errorf("Commit called %d times, want 1", len(repo.commits))
	}
}

func TestBulkTransitionSkipInvalidWithNoValidTaskCommitsNothing(t *testing.T) {
	t1 := newTask("t1", 1, "already done")
	t1.ApplyWorkflowState(models.StateDone)
	projA := newProject("proj-a", "Alpha", t1)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	res, err := svc.BulkTransitionWithWorkflow(app.BulkTransitionRequest{
		TaskIDs:     []string{"t1"},
		Target:      models.StateDone,
		SkipInvalid: true,
	})
	if err != nil {
		t.Fatalf("BulkTransitionWithWorkflow: %v", err)
	}
	if len(res.Tasks) != 0 {
		t.Errorf("changed tasks = %d, want 0", len(res.Tasks))
	}
	if len(repo.commits) != 0 {
		t.Errorf("Commit called %d times, want 0 (no valid task to commit)", len(repo.commits))
	}
}

func TestBulkTransitionWithoutSkipInvalidAbortsOnInvalid(t *testing.T) {
	t1 := newTask("t1", 1, "ready")
	t2 := newTask("t2", 2, "already done")
	t2.ApplyWorkflowState(models.StateDone)
	projA := newProject("proj-a", "Alpha", t1, t2)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	_, err := svc.BulkTransitionWithWorkflow(app.BulkTransitionRequest{
		TaskIDs: []string{"t1", "t2"},
		Target:  models.StateDone,
	})
	if err == nil {
		t.Fatal("expected the batch to abort on the already-done task")
	}
	if len(repo.commits) != 0 {
		t.Errorf("Commit called %d times, want 0 (abort commits nothing)", len(repo.commits))
	}
	if t1.State() == models.StateDone {
		t.Error("t1 should not have been committed when the batch aborts")
	}
}

func TestBulkTransitionWithoutRulesKeepsPlainBehavior(t *testing.T) {
	t1 := newTask("t1", 1, "plain")
	projA := newProject("proj-a", "Alpha", t1)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	res, err := svc.BulkTransition([]string{"t1"}, models.StateInProgress)
	if err != nil {
		t.Fatalf("BulkTransition: %v", err)
	}
	if t1.State() != models.StateInProgress {
		t.Errorf("state = %s, want in_progress", t1.State())
	}
	if res.RuleTriggerCounts != nil {
		t.Errorf("RuleTriggerCounts = %v, want nil without rules", res.RuleTriggerCounts)
	}
	if len(repo.commits) != 1 {
		t.Errorf("Commit called %d times, want 1", len(repo.commits))
	}
	if got := repo.commits[0][0].UndoAction; got != "Bulk Change State" {
		t.Errorf("UndoAction = %q, want Bulk Change State", got)
	}
}
