package app_test

import (
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/workflow"
)

func typeRule(id string, taskType models.TaskType, actions ...workflow.RuleAction) workflow.WorkflowRule {
	return workflow.WorkflowRule{
		ID:      id,
		Name:    id,
		Enabled: true,
		Triggers: []workflow.RuleTrigger{{
			Field:     workflow.FieldType,
			Operation: workflow.OpEquals,
			Value:     string(taskType),
		}},
		Actions: actions,
	}
}

func typeChangedRule(id, transition, tag string) workflow.WorkflowRule {
	return workflow.WorkflowRule{
		ID:      id,
		Name:    id,
		Enabled: true,
		Triggers: []workflow.RuleTrigger{{
			Field:     workflow.FieldTypeChanged,
			Operation: workflow.OpEquals,
			Value:     transition,
		}},
		Actions: []workflow.RuleAction{{ActionType: workflow.ActionAddTag, Value: tag}},
	}
}

func TestAddTaskWithWorkflowUsesStatelessCreationAndOneSnapshot(t *testing.T) {
	project := newProject("proj-a", "Alpha")
	repo := newFakeRepo(project)
	svc := newTestService(repo)
	rule := typeRule("new-task-followup", models.TaskTypeTask,
		workflow.RuleAction{ActionType: workflow.ActionAddTag, Value: "automated"},
		workflow.RuleAction{ActionType: workflow.ActionCreateFollowup, Value: "Verify new task"},
	)

	result, err := svc.AddTaskWithWorkflow(app.AddTaskRequest{
		ProjectID: project.ID,
		Title:     "Created through workflow",
	}, app.SingleTaskWorkflowOptions{
		Rules: []workflow.WorkflowRule{rule},
	})
	if err != nil {
		t.Fatalf("AddTaskWithWorkflow: %v", err)
	}
	if repo.snapshots != 1 {
		t.Fatalf("snapshots = %d, want one shared snapshot", repo.snapshots)
	}
	if len(repo.commits) != 1 || len(repo.commits[0]) != 1 {
		t.Fatalf("commits = %#v, want one grouped transaction", repo.commits)
	}
	if len(result.Tasks) != 1 || result.Tasks[0].Title != "Created through workflow" {
		t.Fatalf("tasks = %+v, want the created root", result.Tasks)
	}
	created := project.GetTaskByID(result.Tasks[0].TaskID)
	if created == nil || !created.HasTag("automated") {
		t.Fatalf("created task = %+v, want stateless rule mutation", created)
	}
	if len(project.Tasks) != 2 || !project.Tasks[1].HasTag("followup") {
		t.Fatalf("project tasks = %+v, want created task plus workflow follow-up", project.Tasks)
	}
	if result.RuleTriggerCounts[rule.ID] != 1 {
		t.Fatalf("trigger counts = %v, want rule once", result.RuleTriggerCounts)
	}
}

func TestApplyTemplateWithWorkflowGroupsAutomationAndCrossProjectEffects(t *testing.T) {
	source := newProject("proj-a", "Alpha")
	destination := newProject("proj-b", "Beta")
	repo := newFakeRepo(source, destination)
	svc := newTestService(repo)
	automationRevision := "automation-revision"
	automationArtifact := storage.ArtifactChange{
		Path:             "/tmp/automation.json",
		Data:             []byte(`{"template_use_count":1}`),
		ExpectedRevision: &automationRevision,
	}
	rule := typeRule("epic-template-cascade", models.TaskTypeEpic,
		workflow.RuleAction{ActionType: workflow.ActionCreateFollowup, Value: "Verify template"},
		workflow.RuleAction{ActionType: workflow.ActionMoveToProject, Value: destination.Name},
	)

	result, err := svc.ApplyTemplateWithWorkflow(app.ApplyTemplateRequest{
		AddTaskRequest: app.AddTaskRequest{
			ProjectID: source.ID,
			Title:     "Release epic",
			Type:      models.TaskTypeEpic,
			Artifacts: []storage.ArtifactChange{automationArtifact},
		},
		TemplateName: "Release",
		Children:     []app.TemplateChildSpec{{Title: "Ship it"}},
	}, app.SingleTaskWorkflowOptions{
		Rules: []workflow.WorkflowRule{rule},
	})
	if err != nil {
		t.Fatalf("ApplyTemplateWithWorkflow: %v", err)
	}
	if repo.snapshots != 1 {
		t.Fatalf("snapshots = %d, want one shared snapshot", repo.snapshots)
	}
	if len(repo.commits) != 1 || len(repo.commits[0]) != 2 {
		t.Fatalf("commits = %#v, want one source+destination transaction", repo.commits)
	}
	artifacts := repo.commits[0][0].Artifacts
	if len(artifacts) != 1 || artifacts[0].Path != "/tmp/automation.json" {
		t.Fatalf("artifacts = %#v, want automation artifact", artifacts)
	}
	if result.Template.SubtasksCreated != 1 {
		t.Fatalf("template result = %+v, want one child", result.Template)
	}
	if result.Template.Task.ProjectID != destination.ID || result.Cascade.Tasks[0].ProjectID != destination.ID {
		t.Fatalf("template/cascade task owners = %q/%q, want final destination %q", result.Template.Task.ProjectID, result.Cascade.Tasks[0].ProjectID, destination.ID)
	}
	if destination.GetTaskByID(result.Template.Task.TaskID) == nil {
		t.Fatal("template root did not move to the destination")
	}
	if len(source.Tasks) != 1 || !source.Tasks[0].HasTag("followup") {
		t.Fatalf("source tasks = %+v, want the workflow follow-up", source.Tasks)
	}
}

func TestEditWithWorkflowAlwaysUsesTruePreviousType(t *testing.T) {
	task := typedTask("task-1", 1, models.TaskTypeTask, "Edit me")
	project := newProject("proj-a", "Alpha", task)
	repo := newFakeRepo(project)
	newType := models.TaskTypeBug
	rule := typeChangedRule("edit-type-transition", "task->bug", "edited-type")

	result, err := newTestService(repo).EditWithWorkflow(app.EditRequest{
		TaskID: task.ID,
		Type:   &newType,
	}, app.SingleTaskWorkflowOptions{
		Rules: []workflow.WorkflowRule{rule},
	})
	if err != nil {
		t.Fatalf("EditWithWorkflow: %v", err)
	}
	if task.Type != models.TaskTypeBug || !task.HasTag("edited-type") {
		t.Fatalf("task type/tags = %s/%v, want contextual edit rule", task.Type, task.Tags)
	}
	if result.RuleTriggerCounts[rule.ID] != 1 {
		t.Fatalf("trigger counts = %v, want contextual rule once", result.RuleTriggerCounts)
	}
}

func TestChangeTaskTypeWithWorkflowAlwaysUsesTruePreviousType(t *testing.T) {
	task := typedTask("task-1", 1, models.TaskTypeTask, "Promote me")
	project := newProject("proj-a", "Alpha", task)
	repo := newFakeRepo(project)
	rule := typeChangedRule("promote-transition", "task->story", "promoted")

	result, err := newTestService(repo).ChangeTaskTypeWithWorkflow(task.ID, app.PromoteType, app.SingleTaskWorkflowOptions{
		Rules: []workflow.WorkflowRule{rule},
	})
	if err != nil {
		t.Fatalf("ChangeTaskTypeWithWorkflow: %v", err)
	}
	if task.Type != models.TaskTypeStory || !task.HasTag("promoted") {
		t.Fatalf("task type/tags = %s/%v, want contextual type-change rule", task.Type, task.Tags)
	}
	if result.RuleTriggerCounts[rule.ID] != 1 {
		t.Fatalf("trigger counts = %v, want contextual rule once", result.RuleTriggerCounts)
	}
}
