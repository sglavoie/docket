package app_test

import (
	"errors"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/workflow"
)

type focusArtifactRecorder struct {
	calls [][]app.WorkflowFocusChange
}

func (r *focusArtifactRecorder) build(changes []app.WorkflowFocusChange) ([]storage.ArtifactChange, error) {
	r.calls = append(r.calls, append([]app.WorkflowFocusChange(nil), changes...))
	expected := "planning-revision"
	return []storage.ArtifactChange{{
		Path:             "/tmp/planning.json",
		Data:             []byte(`{"focus_queue":true}`),
		ExpectedRevision: &expected,
	}}, nil
}

func focusOnInProgressRule() workflow.WorkflowRule {
	return workflow.WorkflowRule{
		ID:      "focus-in-progress",
		Name:    "Focus in-progress work",
		Enabled: true,
		Trigger: workflow.RuleTrigger{
			Field:     workflow.FieldStatus,
			Operation: workflow.OpEquals,
			Value:     string(models.StatusInProgress),
		},
		Actions: []workflow.RuleAction{{ActionType: workflow.ActionSetActive, Value: "true"}},
	}
}

func assertCheckedFocusArtifact(t *testing.T, repo *fakeRepo) {
	t.Helper()
	if len(repo.commits) != 1 || len(repo.commits[0]) == 0 {
		t.Fatalf("commits = %#v, want one transaction", repo.commits)
	}
	artifacts := repo.commits[0][0].Artifacts
	if len(artifacts) != 1 || artifacts[0].Path != "/tmp/planning.json" {
		t.Fatalf("artifacts = %#v, want planning artifact on first changeset", artifacts)
	}
	if artifacts[0].ExpectedRevision == nil || *artifacts[0].ExpectedRevision != "planning-revision" {
		t.Fatalf("planning expected revision = %v, want planning-revision", artifacts[0].ExpectedRevision)
	}
}

func TestBulkWorkflowAccumulatesFocusChangesInCheckedTransaction(t *testing.T) {
	a := newTask("task-a", 1, "A")
	b := newTask("task-b", 1, "B")
	projA := newProject("proj-a", "Alpha", a)
	projB := newProject("proj-b", "Beta", b)
	repo := newFakeRepo(projA, projB)
	recorder := &focusArtifactRecorder{}

	res, err := newTestService(repo).BulkTransitionWithWorkflow(app.BulkTransitionRequest{
		TaskIDs:        []string{a.ID, b.ID},
		Target:         models.StateInProgress,
		Rules:          []workflow.WorkflowRule{focusOnInProgressRule()},
		FocusArtifacts: recorder.build,
	})
	if err != nil {
		t.Fatalf("BulkTransitionWithWorkflow: %v", err)
	}
	if len(recorder.calls) != 1 || len(recorder.calls[0]) != 2 {
		t.Fatalf("focus builder calls = %#v, want one call with both tasks", recorder.calls)
	}
	wantProjects := map[string]string{a.ID: projA.ID, b.ID: projB.ID}
	for _, change := range recorder.calls[0] {
		if !change.Add || change.ProjectID != wantProjects[change.TaskID] {
			t.Errorf("focus change = %+v, want add in final owning project", change)
		}
	}
	if len(res.FocusChanges) != 2 {
		t.Fatalf("result FocusChanges = %+v, want two applied changes", res.FocusChanges)
	}
	assertCheckedFocusArtifact(t, repo)
}

func TestWorkflowFocusAdditionUsesFinalProjectAfterDeferredMove(t *testing.T) {
	task := newTask("task-1", 1, "Move and focus")
	source := newProject("proj-a", "Alpha", task)
	destination := newProject("proj-b", "Beta")
	repo := newFakeRepo(source, destination)
	recorder := &focusArtifactRecorder{}
	rule := focusOnInProgressRule()
	rule.Actions = append(rule.Actions, workflow.RuleAction{
		ActionType: workflow.ActionMoveToProject,
		Value:      destination.Name,
	})

	_, err := newTestService(repo).BulkTransitionWithWorkflow(app.BulkTransitionRequest{
		TaskIDs:        []string{task.ID},
		Target:         models.StateInProgress,
		Rules:          []workflow.WorkflowRule{rule},
		FocusArtifacts: recorder.build,
	})
	if err != nil {
		t.Fatalf("BulkTransitionWithWorkflow: %v", err)
	}
	if len(recorder.calls) != 1 || len(recorder.calls[0]) != 1 {
		t.Fatalf("focus builder calls = %#v, want one change", recorder.calls)
	}
	if got := recorder.calls[0][0].ProjectID; got != destination.ID {
		t.Fatalf("focus project = %q, want final owner %q", got, destination.ID)
	}
}

func TestSingleTaskWorkflowFocusChangeFailsClosedWithoutArtifactBuilder(t *testing.T) {
	task := newTask("task-1", 1, "Task")
	project := newProject("proj-a", "Alpha", task)
	repo := newFakeRepo(project)

	_, err := newTestService(repo).StateWithWorkflow(task.ID, models.StateInProgress, app.SingleTaskWorkflowOptions{
		Rules: []workflow.WorkflowRule{focusOnInProgressRule()},
	})
	if !errors.Is(err, app.ErrWorkflowFocusPersistenceUnavailable) {
		t.Fatalf("err = %v, want ErrWorkflowFocusPersistenceUnavailable", err)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("commits = %d, want zero when planning persistence is unavailable", len(repo.commits))
	}
}

func TestSingleTaskWorkflowCommitsFocusArtifactWithTask(t *testing.T) {
	task := newTask("task-1", 1, "Task")
	project := newProject("proj-a", "Alpha", task)
	repo := newFakeRepo(project)
	recorder := &focusArtifactRecorder{}

	res, err := newTestService(repo).StateWithWorkflow(task.ID, models.StateInProgress, app.SingleTaskWorkflowOptions{
		Rules:          []workflow.WorkflowRule{focusOnInProgressRule()},
		FocusArtifacts: recorder.build,
	})
	if err != nil {
		t.Fatalf("StateWithWorkflow: %v", err)
	}
	if len(res.FocusChanges) != 1 || res.FocusChanges[0].ProjectID != project.ID {
		t.Fatalf("FocusChanges = %+v, want task in proj-a", res.FocusChanges)
	}
	assertCheckedFocusArtifact(t, repo)
}

func TestProjectWorkflowCommitsFocusArtifactWithProjectMutation(t *testing.T) {
	task := newTask("task-1", 1, "Task")
	project := newProject("proj-a", "Alpha", task)
	repo := newFakeRepo(project)
	recorder := &focusArtifactRecorder{}
	task.ApplyWorkflowState(models.StateInProgress)

	res, err := newTestService(repo).CommitProjectWithWorkflow(project, app.ProjectWorkflowOptions{
		Rules:          []workflow.WorkflowRule{focusOnInProgressRule()},
		FocusArtifacts: recorder.build,
	})
	if err != nil {
		t.Fatalf("CommitProjectWithWorkflow: %v", err)
	}
	if len(res.FocusChanges) != 1 || res.FocusChanges[0].TaskID != task.ID {
		t.Fatalf("FocusChanges = %+v, want task-1", res.FocusChanges)
	}
	assertCheckedFocusArtifact(t, repo)
}

func TestApplyWorkflowsCommitsFocusArtifactAtomically(t *testing.T) {
	task := newTask("task-1", 1, "Task")
	task.ApplyWorkflowState(models.StateInProgress)
	project := newProject("proj-a", "Alpha", task)
	repo := newFakeRepo(project)
	recorder := &focusArtifactRecorder{}

	res, err := newTestService(repo).ApplyWorkflows(
		project.ID,
		[]workflow.WorkflowRule{focusOnInProgressRule()},
		nil,
		recorder.build,
	)
	if err != nil {
		t.Fatalf("ApplyWorkflows: %v", err)
	}
	if len(res.FocusChanges) != 1 || !res.FocusChanges[0].Add {
		t.Fatalf("FocusChanges = %+v, want one add", res.FocusChanges)
	}
	assertCheckedFocusArtifact(t, repo)
}
