package app_test

import (
	"errors"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

func TestReconcilePlanningStateSanitizesAndIsIdempotent(t *testing.T) {
	planning := config.NewPlanningState()
	planning.Next = []config.ProjectTaskLocator{
		{TaskID: "next", ProjectID: "p"},
		{TaskID: "next", ProjectID: "p"},
		{TaskID: "missing", ProjectID: "p"},
	}
	planning.Today = []config.ProjectTaskLocator{
		{TaskID: "done", ProjectID: "p"},
		{TaskID: "ready", ProjectID: "p"},
		{TaskID: "ready", ProjectID: "p"},
		{TaskID: "missing", ProjectID: "p"},
	}
	req := app.PlanningReconcileRequest{
		Complete: true,
		Targets: []app.PlanningReconcileTarget{
			{TaskID: "inbox", ProjectID: "p", State: models.StateInbox},
			{TaskID: "ready", ProjectID: "p", State: models.StateBacklog},
			{TaskID: "next", ProjectID: "p", State: models.StateBacklog},
			{TaskID: "next-unindexed", ProjectID: "p", State: models.StateBacklog},
			{TaskID: "done", ProjectID: "p", State: models.StateDone},
		},
	}

	if !app.ReconcilePlanningState(planning, req) {
		t.Fatal("reconciliation reported no change")
	}
	if got := planning.GetNext(); len(got) != 1 || got[0].TaskID != "next" {
		t.Fatalf("Next = %+v, want only valid explicit membership", got)
	}
	if got := planning.Today; len(got) != 1 || got[0].TaskID != "ready" {
		t.Fatalf("DailyPlan = %+v, want only eligible ready task", got)
	}
	if app.ReconcilePlanningState(planning, req) {
		t.Fatal("second reconciliation changed an already canonical snapshot")
	}
}

// TestReconcilePlanningStateKeepsEntriesInUnavailableProjects holds a complete
// reconcile to what it actually read: a project named unavailable contributes no
// targets, and that silence must not prune the entries naming its tasks. An
// entry missing from a project that was read is still dropped.
func TestReconcilePlanningStateKeepsEntriesInUnavailableProjects(t *testing.T) {
	planning := config.NewPlanningState()
	planning.Next = []config.ProjectTaskLocator{
		{TaskID: "unread-next", ProjectID: "evicted"},
		{TaskID: "unread-next", ProjectID: "evicted"},
		{TaskID: "missing", ProjectID: "p"},
	}
	planning.Today = []config.ProjectTaskLocator{
		{TaskID: "unread-today", ProjectID: "evicted"},
		{TaskID: "ready", ProjectID: "p"},
	}
	req := app.PlanningReconcileRequest{
		Complete:    true,
		Unavailable: []string{"evicted"},
		Targets: []app.PlanningReconcileTarget{
			{TaskID: "ready", ProjectID: "p", State: models.StateBacklog},
		},
	}

	app.ReconcilePlanningState(planning, req)
	if got := planning.GetNext(); len(got) != 1 || got[0].TaskID != "unread-next" {
		t.Fatalf("Next = %+v, want the one unread entry, deduplicated", got)
	}
	if got := planning.Today; len(got) != 2 || got[0].TaskID != "unread-today" || got[1].TaskID != "ready" {
		t.Fatalf("Today = %+v, want the unread entry kept beside the read one", got)
	}
	if app.ReconcilePlanningState(planning, req) {
		t.Fatal("second reconciliation changed an already canonical snapshot")
	}
}

func TestApplyPlanningPromotesInboxToTodayAndBacklogClearsToday(t *testing.T) {
	task := newTask("task", 1, "plan me")
	if err := task.ApplyStateTransitionAt(models.StateInbox, task.UpdatedAt); err != nil {
		t.Fatal(err)
	}
	project := newProject("project", "Project", task)
	repo := newFakeRepo(project)
	planning := config.NewPlanningState()
	svc := newTestService(repo).WithPlanningArtifacts(memoryPlanningBuilder(planning))

	if _, err := svc.ApplyPlanning(app.PlanningRequest{
		Locator: app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Action:  app.PlanningAddToday,
	}); err != nil {
		t.Fatal(err)
	}
	if task.State() != models.StateBacklog {
		t.Fatalf("state = %s, want ready after Inbox-to-Today promotion", task.State())
	}
	if len(planning.Today) != 1 || planning.Today[0].TaskID != task.ID {
		t.Fatalf("DailyPlan = %+v, want promoted task", planning.Today)
	}
	if len(planning.Next) != 0 {
		t.Fatalf("FocusQueue = %+v, want empty", planning.Next)
	}

	if _, err := svc.ApplyPlanning(app.PlanningRequest{
		Locator: app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Action:  app.PlanningBacklog,
	}); err != nil {
		t.Fatal(err)
	}
	if len(planning.Today) != 0 {
		t.Fatalf("DailyPlan after Backlog = %+v, want empty", planning.Today)
	}
}

func TestApplyPlanningNextAndTerminalCleanupShareReconciler(t *testing.T) {
	task := newTask("task", 1, "next")
	project := newProject("project", "Project", task)
	repo := newFakeRepo(project)
	planning := config.NewPlanningState()
	planning.Today = []config.ProjectTaskLocator{{
		TaskID: task.ID, ProjectID: project.ID,
	}}
	svc := newTestService(repo).WithPlanningArtifacts(memoryPlanningBuilder(planning))

	if _, err := svc.ApplyPlanning(app.PlanningRequest{
		Locator: app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Action:  app.PlanningAddNext,
	}); err != nil {
		t.Fatal(err)
	}
	if task.State() != models.StateBacklog || len(planning.GetNext()) != 1 || len(planning.Today) != 0 {
		t.Fatalf("Next result: state=%s focus=%+v daily=%+v", task.State(), planning.Next, planning.Today)
	}

	if _, err := svc.Transition(app.StateRequest{
		Locator: app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Target:  models.StateDone,
	}); err != nil {
		t.Fatal(err)
	}
	if len(planning.Next) != 0 || len(planning.Today) != 0 {
		t.Fatalf("terminal cleanup left planning refs: focus=%+v daily=%+v", planning.Next, planning.Today)
	}
}

func TestUnrelatedMutationPreservesWaitingTaskInToday(t *testing.T) {
	waiting := newTask("waiting", 1, "user-chosen waiting")
	waiting.WorkflowState = models.StateWaiting
	unrelated := newTask("unrelated", 2, "edit me")
	project := newProject("project", "Project", waiting, unrelated)
	repo := newFakeRepo(project)
	planning := config.NewPlanningState()
	planning.Today = []config.ProjectTaskLocator{{
		TaskID: waiting.ID, ProjectID: project.ID,
	}}
	svc := newTestService(repo).WithPlanningArtifacts(memoryPlanningBuilder(planning))

	title := "edited"
	if _, err := svc.Edit(app.EditRequest{Locator: repo.loc(unrelated.ID), Title: &title}); err != nil {
		t.Fatalf("Edit unrelated task: %v", err)
	}

	if got := waiting.State(); got != models.StateWaiting {
		t.Fatalf("waiting task state = %s, want user-selected waiting", got)
	}
	if got := planning.Today; len(got) != 1 ||
		got[0].ProjectID != project.ID || got[0].TaskID != waiting.ID {
		t.Fatalf("Today = %+v, want waiting task preserved", got)
	}
}

func TestApplyPlanningRejectsTerminalTodayWithoutMutation(t *testing.T) {
	task := newTask("task", 1, "done")
	if err := task.ApplyStateTransitionAt(models.StateDone, task.UpdatedAt); err != nil {
		t.Fatal(err)
	}
	project := newProject("project", "Project", task)
	repo := newFakeRepo(project)
	planning := config.NewPlanningState()
	svc := newTestService(repo).WithPlanningArtifacts(memoryPlanningBuilder(planning))

	if _, err := svc.ApplyPlanning(app.PlanningRequest{
		Locator: app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Action:  app.PlanningAddToday,
	}); err == nil {
		t.Fatal("terminal Today add unexpectedly succeeded")
	}
	if len(repo.commits) != 0 || len(repo.artifactCommits) != 0 || len(planning.Today) != 0 {
		t.Fatalf("rejected add mutated state: commits=%d artifacts=%d plan=%+v", len(repo.commits), len(repo.artifactCommits), planning.Today)
	}
}

func TestApplyPlanningRollsBackTaskWhenPlanningBuildFails(t *testing.T) {
	task := newTask("task", 1, "rollback")
	project := newProject("project", "Project", task)
	repo := newFakeRepo(project)
	wantErr := errors.New("planning unavailable")
	svc := newTestService(repo).WithPlanningArtifacts(func(app.PlanningReconcileRequest) ([]storage.ArtifactChange, error) {
		return nil, wantErr
	})

	if _, err := svc.ApplyPlanning(app.PlanningRequest{
		Locator: app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Action:  app.PlanningAddNext,
	}); !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if task.State() != models.StateBacklog {
		t.Fatalf("state = %s, want ready after rollback", task.State())
	}
	if len(repo.commits) != 0 || len(repo.artifactCommits) != 0 {
		t.Fatalf("failed mutation committed: project=%d artifact=%d", len(repo.commits), len(repo.artifactCommits))
	}
}

func TestApplyPlanningBulkCommitsMixedProjectsAndPlanningAtomically(t *testing.T) {
	first := newTask("first", 1, "first")
	if err := first.ApplyStateTransitionAt(models.StateInbox, first.UpdatedAt); err != nil {
		t.Fatal(err)
	}
	second := newTask("second", 1, "second")
	projectA := newProject("project-a", "Alpha", first)
	projectB := newProject("project-b", "Beta", second)
	repo := newFakeRepo(projectA, projectB)
	planning := config.NewPlanningState()
	svc := newTestService(repo).WithPlanningArtifacts(memoryPlanningBuilder(planning))

	result, err := svc.ApplyPlanningBulk(app.PlanningBulkRequest{
		Targets: []app.TaskLocator{
			{ProjectID: projectA.ID, TaskID: first.ID},
			{ProjectID: projectB.ID, TaskID: second.ID},
		},
		Action: app.PlanningAddToday,
	})
	if err != nil {
		t.Fatalf("ApplyPlanningBulk: %v", err)
	}
	if first.State() != models.StateBacklog || second.State() != models.StateBacklog {
		t.Fatalf("states = %s/%s, want ready/ready", first.State(), second.State())
	}
	if len(planning.Today) != 2 {
		t.Fatalf("Today = %+v, want both selected tasks", planning.Today)
	}
	if len(result.Tasks) != 2 || len(repo.commits) != 1 || len(repo.commits[0]) != 2 {
		t.Fatalf("bulk result/commit = %+v commits=%+v", result, repo.commits)
	}
	artifactCount := 0
	for _, change := range repo.commits[0] {
		artifactCount += len(change.Artifacts)
	}
	if artifactCount != 1 {
		t.Fatalf("planning artifacts in grouped commit = %d, want 1", artifactCount)
	}
}

func TestApplyPlanningBulkRejectsInvalidMemberWithoutPartialMutation(t *testing.T) {
	ready := newTask("ready", 1, "ready")
	done := newTask("done", 2, "done")
	if err := done.ApplyStateTransitionAt(models.StateDone, done.UpdatedAt); err != nil {
		t.Fatal(err)
	}
	project := newProject("project", "Project", ready, done)
	repo := newFakeRepo(project)
	planning := config.NewPlanningState()
	svc := newTestService(repo).WithPlanningArtifacts(memoryPlanningBuilder(planning))

	_, err := svc.ApplyPlanningBulk(app.PlanningBulkRequest{
		Targets: []app.TaskLocator{
			{ProjectID: project.ID, TaskID: ready.ID},
			{ProjectID: project.ID, TaskID: done.ID},
		},
		Action: app.PlanningAddToday,
	})
	if err == nil {
		t.Fatal("terminal member did not reject bulk Today")
	}
	if ready.State() != models.StateBacklog || len(planning.Today) != 0 || len(repo.commits) != 0 {
		t.Fatalf("rejected bulk planning mutated data: state=%s Today=%+v commits=%d", ready.State(), planning.Today, len(repo.commits))
	}
}

func memoryPlanningBuilder(planning *config.PlanningState) app.PlanningArtifactBuilder {
	var previous []byte
	build := 0
	return func(req app.PlanningReconcileRequest) ([]storage.ArtifactChange, error) {
		if !app.ReconcilePlanningState(planning, req) {
			return nil, nil
		}
		rev := ""
		if previous != nil {
			rev = storage.RawArtifactRevision(previous)
		}
		build++
		data := []byte("planning-" + string(rune('0'+build)))
		previous = data
		return []storage.ArtifactChange{{
			Path: "mem-workspace/.tt/planning.json", Data: data, ExpectedRevision: &rev,
		}}, nil
	}
}
