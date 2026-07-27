package app_test

import (
	"errors"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

func legacyMutationRule() workflow.WorkflowRule {
	return workflow.WorkflowRule{
		ID:      "legacy-mutation-rule",
		Name:    "mark legacy mutation target",
		Enabled: true,
		Triggers: []workflow.RuleTrigger{{
			Field:     workflow.FieldPriority,
			Operation: workflow.OpEquals,
			Value:     "1",
		}},
		Actions: []workflow.RuleAction{{ActionType: workflow.ActionAddTag, Value: "workflow-fired"}},
	}
}

func legacyMutationOptions() app.SingleTaskWorkflowOptions {
	return app.SingleTaskWorkflowOptions{
		Rules: []workflow.WorkflowRule{legacyMutationRule()},
	}
}

func requireLegacyCascade(t *testing.T, result app.BulkResult, taskID string) {
	t.Helper()
	if len(result.Tasks) != 1 || result.Tasks[0].TaskID != taskID {
		t.Fatalf("cascade tasks = %+v, want one result for %s", result.Tasks, taskID)
	}
	if result.RuleTriggerCounts["legacy-mutation-rule"] != 1 {
		t.Fatalf("trigger counts = %v, want legacy rule once", result.RuleTriggerCounts)
	}
	if len(result.TouchedProjectIDs) != 1 {
		t.Fatalf("touched projects = %v, want one", result.TouchedProjectIDs)
	}
}

func TestBaselineEpicWithWorkflowPreservesBaselineResult(t *testing.T) {
	epic, err := models.NewTask("epic", 1, models.TaskTypeEpic, "Release")
	if err != nil {
		t.Fatal(err)
	}
	epic.Priority = 1
	child := newTask("child", 2, "Ship")
	parentID := epic.ID
	child.ParentID = &parentID
	project := newProject("proj-a", "Alpha", epic, child)
	repo := newFakeRepo(project)

	result, err := newTestService(repo).BaselineEpicWithWorkflow(epic.ID, legacyMutationOptions())
	if err != nil {
		t.Fatalf("BaselineEpicWithWorkflow: %v", err)
	}
	if result.Baseline.ChildCount != 1 || result.Baseline.Task.TaskID != epic.ID {
		t.Fatalf("baseline = %+v, want epic with one child", result.Baseline)
	}
	if epic.EpicAttrs == nil || epic.EpicAttrs.InitialChildCount != 1 || epic.EpicAttrs.ScopeBaselinedAt == nil {
		t.Fatalf("epic attrs = %+v, want recorded baseline", epic.EpicAttrs)
	}
	if !epic.HasTag("workflow-fired") {
		t.Fatalf("epic tags = %v, want atomic workflow effect", epic.Tags)
	}
	requireLegacyCascade(t, result.Cascade, epic.ID)
	if len(repo.commits) != 1 || repo.commits[0][0].UndoAction != "Baseline Epic" {
		t.Fatalf("commits = %+v, want one Baseline Epic transaction", repo.commits)
	}
}

func TestCommentWorkflowVariantsPreserveFreshMutationDetails(t *testing.T) {
	task := newTask("task", 1, "Investigate")
	task.Priority = 1
	task.Comments = []*models.Comment{{Text: "existing"}}
	repo := newFakeRepo(newProject("proj-a", "Alpha", task))
	service := newTestService(repo)
	opts := legacyMutationOptions()

	added, err := service.AddCommentWithWorkflow(task.ID, "  new finding  ", opts)
	if err != nil {
		t.Fatalf("AddCommentWithWorkflow: %v", err)
	}
	if added.Comment.Index != 1 || added.Comment.Count != 2 || task.Comments[1].Text != "new finding" {
		t.Fatalf("add result/comments = %+v/%+v", added.Comment, task.Comments)
	}
	requireLegacyCascade(t, added.Cascade, task.ID)

	removed, err := service.RemoveCommentWithWorkflow(task.ID, 0, opts)
	if err != nil {
		t.Fatalf("RemoveCommentWithWorkflow: %v", err)
	}
	if removed.Comment.Index != 0 || removed.Comment.Count != 1 || task.Comments[0].Text != "new finding" {
		t.Fatalf("remove result/comments = %+v/%+v", removed.Comment, task.Comments)
	}

	resolved, err := service.ToggleCommentResolvedWithWorkflow(task.ID, 0, opts)
	if err != nil {
		t.Fatalf("ToggleCommentResolvedWithWorkflow: %v", err)
	}
	if !resolved.Comment.Resolved || !task.Comments[0].Resolved || resolved.Comment.Count != 1 {
		t.Fatalf("resolve result/comment = %+v/%+v", resolved.Comment, task.Comments[0])
	}

	cleared, err := service.ClearCommentsWithWorkflow(task.ID, opts)
	if err != nil {
		t.Fatalf("ClearCommentsWithWorkflow: %v", err)
	}
	if cleared.Comment.Index != -1 || cleared.Comment.Count != 1 || len(task.Comments) != 0 {
		t.Fatalf("clear result/comments = %+v/%+v", cleared.Comment, task.Comments)
	}
	if got := len(repo.commits); got != 4 {
		t.Fatalf("commits = %d, want four comment transactions", got)
	}

	before := len(repo.commits)
	if _, err := service.AddCommentWithWorkflow(task.ID, "   ", opts); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("empty comment err = %v, want ErrInvalidInput", err)
	}
	if _, err := service.RemoveCommentWithWorkflow(task.ID, 0, opts); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("stale comment index err = %v, want ErrInvalidInput", err)
	}
	if len(repo.commits) != before {
		t.Fatalf("rejected comment mutations committed: before=%d after=%d", before, len(repo.commits))
	}
}

func TestDependencyWorkflowVariantsReuseWorkspaceValidation(t *testing.T) {
	task := newTask("task", 1, "Dependent")
	task.Priority = 1
	blockerA := newTask("blocker-a", 2, "First blocker")
	blockerB := newTask("blocker-b", 3, "Second blocker")
	project := newProject("proj-a", "Alpha", task, blockerA, blockerB)
	repo := newFakeRepo(project)
	service := newTestService(repo)
	opts := legacyMutationOptions()

	added, err := service.AddDependencyWithWorkflow(task.ID, blockerA.ID, opts)
	if err != nil {
		t.Fatalf("AddDependencyWithWorkflow: %v", err)
	}
	if !task.HasDependency(blockerA.ID) || !task.HasTag("workflow-fired") {
		t.Fatalf("dependency/tags = %v/%v, want blocker and workflow tag", task.DependsOn, task.Tags)
	}
	requireLegacyCascade(t, added, task.ID)

	if _, removeErr := service.RemoveDependencyWithWorkflow(task.ID, blockerA.ID, opts); removeErr != nil {
		t.Fatalf("RemoveDependencyWithWorkflow: %v", removeErr)
	}
	if task.HasDependency(blockerA.ID) {
		t.Fatalf("dependencies = %v, want blocker removed", task.DependsOn)
	}
	if _, addErr := service.AddDependencyWithWorkflow(task.ID, blockerA.ID, opts); addErr != nil {
		t.Fatalf("re-add first dependency: %v", addErr)
	}
	if _, addErr := service.AddDependencyWithWorkflow(task.ID, blockerB.ID, opts); addErr != nil {
		t.Fatalf("add second dependency: %v", addErr)
	}
	task.RemoveTag("workflow-fired")
	cleared, err := service.ClearDependenciesWithWorkflow(task.ID, opts)
	if err != nil {
		t.Fatalf("ClearDependenciesWithWorkflow: %v", err)
	}
	if len(task.DependsOn) != 0 {
		t.Fatalf("dependencies = %v, want empty", task.DependsOn)
	}
	requireLegacyCascade(t, cleared, task.ID)

	before := len(repo.commits)
	if _, err := service.AddDependencyWithWorkflow(task.ID, "missing", opts); !errors.Is(err, app.ErrTaskNotFound) {
		t.Fatalf("missing blocker err = %v, want ErrTaskNotFound", err)
	}
	if len(repo.commits) != before {
		t.Fatalf("invalid dependency committed: before=%d after=%d", before, len(repo.commits))
	}
}

func TestTypeAttributeWorkflowVariantsShareValidation(t *testing.T) {
	task := newTask("task", 1, "Implement")
	task.Priority = 1
	repo := newFakeRepo(newProject("proj-a", "Alpha", task))
	service := newTestService(repo)
	opts := legacyMutationOptions()

	applied, err := service.ApplyTypeAttrsWithWorkflow(task.ID, models.TypeAttrValues{TaskEffort: "small"}, opts)
	if err != nil {
		t.Fatalf("ApplyTypeAttrsWithWorkflow: %v", err)
	}
	if task.TaskAttrs == nil || task.TaskAttrs.Effort != models.EffortSmall {
		t.Fatalf("task attrs = %+v, want small effort", task.TaskAttrs)
	}
	requireLegacyCascade(t, applied, task.ID)

	task.RemoveTag("workflow-fired")
	cleared, err := service.ClearTypeAttrsWithWorkflow(task.ID, opts)
	if err != nil {
		t.Fatalf("ClearTypeAttrsWithWorkflow: %v", err)
	}
	if task.TaskAttrs != nil {
		t.Fatalf("task attrs = %+v, want cleared", task.TaskAttrs)
	}
	requireLegacyCascade(t, cleared, task.ID)

	before := len(repo.commits)
	if _, err := service.ApplyTypeAttrsWithWorkflow(task.ID, models.TypeAttrValues{TaskEffort: "gigantic"}, opts); err == nil {
		t.Fatal("expected invalid task effort to be rejected")
	}
	if len(repo.commits) != before {
		t.Fatalf("invalid attributes committed: before=%d after=%d", before, len(repo.commits))
	}
}

func TestParkAndDimWorkflowVariants(t *testing.T) {
	task := newTask("task", 1, "Pause work")
	task.Priority = 1
	repo := newFakeRepo(newProject("proj-a", "Alpha", task))
	service := newTestService(repo)
	opts := legacyMutationOptions()

	if result, err := service.ParkWithWorkflow(task.ID, "waiting on review", opts); err != nil {
		t.Fatalf("ParkWithWorkflow: %v", err)
	} else {
		requireLegacyCascade(t, result, task.ID)
	}
	if !task.IsParked() || task.ParkedNote != "waiting on review" {
		t.Fatalf("parked state = %v/%q", task.IsParked(), task.ParkedNote)
	}
	if _, err := service.UnparkWithWorkflow(task.ID, opts); err != nil {
		t.Fatalf("UnparkWithWorkflow: %v", err)
	}
	if task.IsParked() {
		t.Fatal("task remained parked")
	}
	if _, err := service.ToggleDimmedWithWorkflow(task.ID, opts); err != nil {
		t.Fatalf("ToggleDimmedWithWorkflow: %v", err)
	}
	if !task.Dimmed {
		t.Fatal("task was not dimmed")
	}
	if _, err := service.UndimWithWorkflow(task.ID, opts); err != nil {
		t.Fatalf("UndimWithWorkflow: %v", err)
	}
	if task.Dimmed {
		t.Fatal("task remained dimmed")
	}
	if got := len(repo.commits); got != 4 {
		t.Fatalf("commits = %d, want park/unpark/dim/undim", got)
	}
}

func TestGuardedSnoozeWithWorkflowPreservesCLIAndTUIGates(t *testing.T) {
	until := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	task := newTask("task", 1, "Active work")
	task.Priority = 1
	task.ApplyWorkflowState(models.StateInProgress)
	repo := newFakeRepo(newProject("proj-a", "Alpha", task))
	service := newTestService(repo)
	opts := legacyMutationOptions()

	if _, err := service.GuardedSnoozeWithWorkflow(task.ID, &until, opts); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("guarded in-progress snooze err = %v, want ErrInvalidInput", err)
	}
	if task.SnoozedUntil != nil || task.HasTag("workflow-fired") || len(repo.commits) != 0 {
		t.Fatalf("rejected guarded snooze mutated task: snooze=%v tags=%v commits=%d", task.SnoozedUntil, task.Tags, len(repo.commits))
	}

	// The secondary-view variant deliberately remains gate-free.
	result, err := service.SnoozeWithWorkflow(task.ID, &until, opts)
	if err != nil {
		t.Fatalf("gate-free SnoozeWithWorkflow: %v", err)
	}
	if task.SnoozedUntil == nil || !task.SnoozedUntil.Equal(until) || !task.HasTag("workflow-fired") {
		t.Fatalf("gate-free snooze/task tags = %v/%v", task.SnoozedUntil, task.Tags)
	}
	requireLegacyCascade(t, result, task.ID)

	done := newTask("done", 2, "Finished")
	done.Priority = 1
	done.ApplyWorkflowState(models.StateDone)
	doneRepo := newFakeRepo(newProject("proj-done", "Done", done))
	if _, err := newTestService(doneRepo).GuardedSnoozeWithWorkflow(done.ID, &until, opts); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("guarded terminal snooze err = %v, want ErrInvalidInput", err)
	}
	if done.SnoozedUntil != nil || done.HasTag("workflow-fired") || len(doneRepo.commits) != 0 {
		t.Fatalf("terminal snooze mutated task: snooze=%v tags=%v commits=%d", done.SnoozedUntil, done.Tags, len(doneRepo.commits))
	}
}

func TestLegacyWorkflowNoOpsSkipRulesAndLedger(t *testing.T) {
	task := newTask("task", 1, "Already clear")
	task.Priority = 1
	project := newProject("proj-a", "Alpha", task)
	repo := newFakeRepo(project)
	service := newTestService(repo)
	opts := legacyMutationOptions()
	originalUpdatedAt := task.UpdatedAt

	comments, err := service.ClearCommentsWithWorkflow(task.ID, opts)
	if err != nil {
		t.Fatalf("ClearCommentsWithWorkflow no-op: %v", err)
	}
	if comments.Comment.Count != 0 || len(comments.Cascade.Tasks) != 1 {
		t.Fatalf("comment no-op result = %+v", comments)
	}
	results := []app.BulkResult{comments.Cascade}
	for name, operation := range map[string]func() (app.BulkResult, error){
		"clear dependencies": func() (app.BulkResult, error) {
			return service.ClearDependenciesWithWorkflow(task.ID, opts)
		},
		"unpark": func() (app.BulkResult, error) {
			return service.UnparkWithWorkflow(task.ID, opts)
		},
		"undim": func() (app.BulkResult, error) {
			return service.UndimWithWorkflow(task.ID, opts)
		},
		"clear snooze": func() (app.BulkResult, error) {
			return service.GuardedSnoozeWithWorkflow(task.ID, nil, opts)
		},
	} {
		result, err := operation()
		if err != nil {
			t.Fatalf("%s no-op: %v", name, err)
		}
		results = append(results, result)
	}

	for _, result := range results {
		if len(result.Tasks) != 1 || result.Tasks[0].TaskID != task.ID {
			t.Fatalf("no-op task result = %+v, want current task", result.Tasks)
		}
		if result.RuleTriggerCounts != nil || len(result.TouchedProjectIDs) != 0 {
			t.Fatalf("no-op cascade = %+v, want no rules or touched projects", result)
		}
	}
	if task.HasTag("workflow-fired") {
		t.Fatalf("no-op unexpectedly evaluated rule; tags = %v", task.Tags)
	}
	if !task.UpdatedAt.Equal(originalUpdatedAt) {
		t.Fatalf("UpdatedAt changed on no-op: got %v want %v", task.UpdatedAt, originalUpdatedAt)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("no-op operations committed %d transactions, want none", len(repo.commits))
	}
}
