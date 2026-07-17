package app_test

import (
	"errors"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/workflow"
)

type projectStatusArtifactRecorder struct {
	calls [][]app.WorkflowProjectStatusChange
}

func (r *projectStatusArtifactRecorder) build(changes []app.WorkflowProjectStatusChange) ([]storage.ArtifactChange, error) {
	r.calls = append(r.calls, append([]app.WorkflowProjectStatusChange(nil), changes...))
	expected := "catalog-revision"
	return []storage.ArtifactChange{{
		Path:             "/tmp/workspace.json",
		Data:             []byte(`{"project_status":true}`),
		ExpectedRevision: &expected,
	}}, nil
}

func TestWorkflowOrchestrationGroupsEveryCascadeEffect(t *testing.T) {
	task := newTask("task-1", 1, "ship")
	source := newProject("proj-a", "Alpha", task)
	destination := newProject("proj-b", "Beta")
	repo := newFakeRepo(source, destination)
	projectStatus := &projectStatusArtifactRecorder{}

	taskRule := workflow.WorkflowRule{
		ID:      "cascade",
		Name:    "Ship cascade",
		Enabled: true,
		Trigger: workflow.RuleTrigger{Field: workflow.FieldStatus, Operation: workflow.OpEquals, Value: string(models.StatusInProgress)},
		Actions: []workflow.RuleAction{
			{ActionType: workflow.ActionCreateFollowup, Value: "verify {task}"},
			{ActionType: workflow.ActionMoveToProject, Value: destination.Name},
		},
	}
	projectRule := workflow.WorkflowRule{
		ID:       "project-active",
		Name:     "Mark active",
		Level:    workflow.RuleLevelProject,
		Enabled:  true,
		Trigger:  workflow.RuleTrigger{Field: workflow.FieldActiveTaskCount, Operation: workflow.OpGreaterThan, Value: "0"},
		Actions:  []workflow.RuleAction{{ActionType: workflow.ActionSetProjectStatus, Value: "active"}},
		Priority: 10,
	}

	result, err := newTestService(repo).StateWithWorkflow(task.ID, models.StateInProgress, app.SingleTaskWorkflowOptions{
		Rules:                  []workflow.WorkflowRule{taskRule, projectRule},
		ProjectStatusArtifacts: projectStatus.build,
	})
	if err != nil {
		t.Fatalf("StateWithWorkflow: %v", err)
	}
	if repo.snapshots != 1 {
		t.Fatalf("snapshots = %d, want one fresh workspace snapshot", repo.snapshots)
	}
	if len(repo.commits) != 1 || len(repo.commits[0]) != 2 {
		t.Fatalf("commits = %#v, want one transaction with source and destination", repo.commits)
	}
	if destination.GetTaskByID(task.ID) == nil || source.GetTaskByID(task.ID) != nil {
		t.Fatalf("task ownership source=%v destination=%v, want destination only", source.GetTaskByID(task.ID), destination.GetTaskByID(task.ID))
	}
	if len(source.Tasks) != 1 || !source.Tasks[0].HasTag("followup") {
		t.Fatalf("source tasks = %#v, want one generated follow-up", source.Tasks)
	}
	if source.StatusIcon != "active" || source.StatusIconSource != "auto" {
		t.Fatalf("source project status = %q/%q, want active/auto", source.StatusIcon, source.StatusIconSource)
	}
	if len(result.ProjectStatusChanges) != 1 || result.ProjectStatusChanges[0].ProjectID != source.ID {
		t.Fatalf("project status changes = %+v, want source project", result.ProjectStatusChanges)
	}
	if result.RuleTriggerCounts[taskRule.ID] != 1 || result.RuleTriggerCounts[projectRule.ID] != 1 {
		t.Fatalf("trigger counts = %v, want both task and project rules once", result.RuleTriggerCounts)
	}
	if len(projectStatus.calls) != 1 || len(projectStatus.calls[0]) != 1 {
		t.Fatalf("project status builder calls = %#v, want one status change", projectStatus.calls)
	}
	artifacts := repo.commits[0][0].Artifacts
	if len(artifacts) != 1 || artifacts[0].Path != "/tmp/workspace.json" {
		t.Fatalf("artifacts = %#v, want checked catalog artifact", artifacts)
	}
	for _, artifact := range artifacts {
		if artifact.ExpectedRevision == nil {
			t.Fatalf("artifact %q lacks expected revision", artifact.Path)
		}
	}
}

func TestBulkWorkflowReturnsNormalizedConflicts(t *testing.T) {
	task := newTask("task-1", 1, "conflicted")
	project := newProject("proj-a", "Alpha", task)
	repo := newFakeRepo(project)
	rules := []workflow.WorkflowRule{
		{
			ID:      "priority-one",
			Name:    "Set P1",
			Enabled: true,
			Trigger: workflow.RuleTrigger{Field: workflow.FieldStatus, Operation: workflow.OpEquals, Value: string(models.StatusInProgress)},
			Actions: []workflow.RuleAction{{ActionType: workflow.ActionSetPriority, Value: "1"}},
		},
		{
			ID:      "priority-five",
			Name:    "Set P5",
			Enabled: true,
			Trigger: workflow.RuleTrigger{Field: workflow.FieldStatus, Operation: workflow.OpEquals, Value: string(models.StatusInProgress)},
			Actions: []workflow.RuleAction{{ActionType: workflow.ActionSetPriority, Value: "5"}},
		},
	}

	result, err := newTestService(repo).BulkTransitionWithWorkflow(app.BulkTransitionRequest{
		TaskIDs: []string{task.ID},
		Target:  models.StateInProgress,
		Rules:   rules,
	})
	if err != nil {
		t.Fatalf("BulkTransitionWithWorkflow: %v", err)
	}
	if len(result.Conflicts) != 1 {
		t.Fatalf("conflicts = %+v, want one normalized conflict", result.Conflicts)
	}
	conflict := result.Conflicts[0]
	if conflict.TaskID != task.ID || conflict.TaskTitle != task.Title || conflict.Field != "priority" {
		t.Fatalf("conflict = %+v, want task identity and priority field", conflict)
	}
	if len(conflict.Rules) != 2 || conflict.Rules[0] != "Set P1" || conflict.Rules[1] != "Set P5" {
		t.Fatalf("conflict rules = %v, want both presentation-neutral rule names", conflict.Rules)
	}
	if task.Priority != models.PriorityDefault {
		t.Fatalf("priority = %d, want conflict to leave it unchanged", task.Priority)
	}
}

func TestWorkflowProjectStatusFailsClosedWithoutCatalogArtifact(t *testing.T) {
	task := newTask("task-1", 1, "status")
	project := newProject("proj-a", "Alpha", task)
	repo := newFakeRepo(project)
	rule := workflow.WorkflowRule{
		ID:      "project-active",
		Name:    "Mark active",
		Level:   workflow.RuleLevelProject,
		Enabled: true,
		Trigger: workflow.RuleTrigger{Field: workflow.FieldActiveTaskCount, Operation: workflow.OpGreaterThan, Value: "0"},
		Actions: []workflow.RuleAction{{ActionType: workflow.ActionSetProjectStatus, Value: "active"}},
	}

	_, err := newTestService(repo).SetPriorityWithWorkflow(task.ID, 1, app.SingleTaskWorkflowOptions{Rules: []workflow.WorkflowRule{rule}})
	if !errors.Is(err, app.ErrWorkflowProjectStatusPersistenceUnavailable) {
		t.Fatalf("err = %v, want ErrWorkflowProjectStatusPersistenceUnavailable", err)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("commits = %d, want no project write without catalog artifact", len(repo.commits))
	}
}

func TestSingleTaskWorkflowReportsRecurringInstancesFromGroupedCommit(t *testing.T) {
	task := newTask("task-1", 1, "daily")
	task.Recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}
	project := newProject("proj-a", "Alpha", task)
	repo := newFakeRepo(project)

	result, err := newTestService(repo).StateWithWorkflow(task.ID, models.StateDone, app.SingleTaskWorkflowOptions{})
	if err != nil {
		t.Fatalf("StateWithWorkflow: %v", err)
	}
	if len(result.Tasks) != 1 || result.Tasks[0].RecurringGenerated != 1 {
		t.Fatalf("task result = %+v, want one generated recurrence", result.Tasks)
	}
	if len(project.Tasks) != 2 {
		t.Fatalf("project tasks = %d, want completed task plus recurring instance", len(project.Tasks))
	}
}

func TestWorkflowRulesSeeLegacyPreEvaluationReconciliation(t *testing.T) {
	parent := newTask("parent", 1, "parent")
	parent.ApplyWorkflowState(models.StateDone)
	child := newTask("child", 2, "child")
	parentID := parent.ID
	child.ParentID = &parentID
	project := newProject("proj-a", "Alpha", parent, child)
	repo := newFakeRepo(project)
	rule := workflow.WorkflowRule{
		ID:      "tag-reopened",
		Name:    "Tag reopened parent",
		Enabled: true,
		Trigger: workflow.RuleTrigger{Field: workflow.FieldStatus, Operation: workflow.OpEquals, Value: string(models.StatusInProgress)},
		Actions: []workflow.RuleAction{{ActionType: workflow.ActionAddTag, Value: "reconciled"}},
	}

	_, err := newTestService(repo).SetPriorityWithWorkflow(child.ID, 1, app.SingleTaskWorkflowOptions{Rules: []workflow.WorkflowRule{rule}})
	if err != nil {
		t.Fatalf("SetPriorityWithWorkflow: %v", err)
	}
	if parent.State() != models.StateInProgress || !parent.HasTag("reconciled") {
		t.Fatalf("parent state/tags = %s/%v, want pre-rule reconcile then rule tag", parent.State(), parent.Tags)
	}
}
