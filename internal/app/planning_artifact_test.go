package app_test

import (
	"errors"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func TestReconcilePlanningStateSanitizesAndIsIdempotent(t *testing.T) {
	planning := config.NewPlanningState()
	planning.FocusQueue = []config.FocusQueueItem{
		{TaskID: "ready", ProjectID: "p"},
		{TaskID: "next", ProjectID: "p"},
		{TaskID: "next", ProjectID: "p"},
		{TaskID: "missing", ProjectID: "p"},
	}
	planning.DailyPlan = []config.DailyPlanItem{
		{TaskID: "inbox", ProjectID: "p"},
		{TaskID: "next", ProjectID: "p"},
		{TaskID: "done", ProjectID: "p"},
		{TaskID: "ready", ProjectID: "p", PlannedDate: "2026-07-25"},
		{TaskID: "ready", ProjectID: "p", PlannedDate: "2026-07-25"},
		{TaskID: "missing", ProjectID: "p"},
	}
	req := app.PlanningReconcileRequest{
		Complete: true,
		Targets: []app.PlanningReconcileTarget{
			{TaskID: "inbox", ProjectID: "p", State: models.StateInbox},
			{TaskID: "ready", ProjectID: "p", State: models.StateReady},
			{TaskID: "next", ProjectID: "p", State: models.StateNext},
			{TaskID: "next-unindexed", ProjectID: "p", State: models.StateNext},
			{TaskID: "done", ProjectID: "p", State: models.StateDone},
		},
	}

	if !app.ReconcilePlanningState(planning, req) {
		t.Fatal("reconciliation reported no change")
	}
	if got := planning.FocusQueue; len(got) != 1 || got[0].TaskID != "next" {
		t.Fatalf("FocusQueue = %+v, want only the valid existing StateNext order entry", got)
	}
	if got := planning.DailyPlan; len(got) != 1 || got[0].TaskID != "ready" {
		t.Fatalf("DailyPlan = %+v, want only eligible ready task", got)
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
	if task.State() != models.StateReady {
		t.Fatalf("state = %s, want ready after Inbox-to-Today promotion", task.State())
	}
	if len(planning.DailyPlan) != 1 || planning.DailyPlan[0].TaskID != task.ID {
		t.Fatalf("DailyPlan = %+v, want promoted task", planning.DailyPlan)
	}
	if len(planning.FocusQueue) != 0 {
		t.Fatalf("FocusQueue = %+v, want empty", planning.FocusQueue)
	}

	if _, err := svc.ApplyPlanning(app.PlanningRequest{
		Locator: app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Action:  app.PlanningBacklog,
	}); err != nil {
		t.Fatal(err)
	}
	if len(planning.DailyPlan) != 0 {
		t.Fatalf("DailyPlan after Backlog = %+v, want empty", planning.DailyPlan)
	}
}

func TestApplyPlanningNextAndTerminalCleanupShareReconciler(t *testing.T) {
	task := newTask("task", 1, "next")
	project := newProject("project", "Project", task)
	repo := newFakeRepo(project)
	planning := config.NewPlanningState()
	planning.DailyPlan = []config.DailyPlanItem{{
		TaskID: task.ID, ProjectID: project.ID, PlannedDate: "2026-07-25",
	}}
	svc := newTestService(repo).WithPlanningArtifacts(memoryPlanningBuilder(planning))

	if _, err := svc.ApplyPlanning(app.PlanningRequest{
		Locator: app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Action:  app.PlanningAddNext,
	}); err != nil {
		t.Fatal(err)
	}
	if task.State() != models.StateNext || len(planning.FocusQueue) != 1 || len(planning.DailyPlan) != 0 {
		t.Fatalf("Next result: state=%s focus=%+v daily=%+v", task.State(), planning.FocusQueue, planning.DailyPlan)
	}

	if _, err := svc.Transition(app.StateRequest{
		Locator: app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Target:  models.StateDone,
	}); err != nil {
		t.Fatal(err)
	}
	if len(planning.FocusQueue) != 0 || len(planning.DailyPlan) != 0 {
		t.Fatalf("terminal cleanup left planning refs: focus=%+v daily=%+v", planning.FocusQueue, planning.DailyPlan)
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
	if len(repo.commits) != 0 || len(repo.artifactCommits) != 0 || len(planning.DailyPlan) != 0 {
		t.Fatalf("rejected add mutated state: commits=%d artifacts=%d plan=%+v", len(repo.commits), len(repo.artifactCommits), planning.DailyPlan)
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
	if task.State() != models.StateReady {
		t.Fatalf("state = %s, want ready after rollback", task.State())
	}
	if len(repo.commits) != 0 || len(repo.artifactCommits) != 0 {
		t.Fatalf("failed mutation committed: project=%d artifact=%d", len(repo.commits), len(repo.artifactCommits))
	}
}

func memoryPlanningBuilder(planning *config.PlanningState) app.PlanningArtifactBuilder {
	var previous []byte
	build := 0
	return func(req app.PlanningReconcileRequest) ([]storage.ArtifactChange, error) {
		if !app.ReconcilePlanningState(planning, req) {
			return nil, nil
		}
		rev := storage.RawArtifactRevision(previous)
		build++
		data := []byte("planning-" + string(rune('0'+build)))
		previous = data
		return []storage.ArtifactChange{{
			Path: "mem-workspace/.tt/planning.json", Data: data, ExpectedRevision: &rev,
		}}, nil
	}
}
