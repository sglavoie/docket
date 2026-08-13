package app_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func TestPlanningLifecycleTransitionMatrix(t *testing.T) {
	locator := config.ProjectTaskLocator{ProjectID: "project", TaskID: "task"}
	for _, target := range models.AllWorkflowStates() {
		t.Run(string(target), func(t *testing.T) {
			planning := config.NewPlanningState()
			planning.SetToday([]config.ProjectTaskLocator{locator})
			planning.SetNext([]config.ProjectTaskLocator{locator})

			changed := app.ReconcilePlanningState(planning, app.PlanningReconcileRequest{
				Targets: []app.PlanningReconcileTarget{{
					TaskID: locator.TaskID, ProjectID: locator.ProjectID,
					State: target, StateChanged: true,
				}},
			})
			if !changed {
				t.Fatal("state transition did not reconcile planning")
			}

			if target == models.StateDoing {
				if got := planning.GetToday(); !reflect.DeepEqual(got, []config.ProjectTaskLocator{locator}) {
					t.Fatalf("Today = %+v, want preserved Doing membership", got)
				}
				if got := planning.GetNext(); len(got) != 0 {
					t.Fatalf("Next = %+v, want cleared by Doing", got)
				}
				return
			}
			if len(planning.GetToday()) != 0 || len(planning.GetNext()) != 0 {
				t.Fatalf("planning after %s = Today:%+v Next:%+v, want both cleared", target, planning.GetToday(), planning.GetNext())
			}
		})
	}
}

func TestPlanningAddActionsNormalizeLifecycleAndDefer(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	deferUntil := now.AddDate(0, 0, 2)
	for _, initial := range []models.WorkflowState{
		models.StateInbox,
		models.StateBacklog,
		models.StateDoing,
		models.StateWaiting,
		models.StatePaused,
		models.StateSomeday,
	} {
		t.Run("today_"+string(initial), func(t *testing.T) {
			task := newTask("task", 1, "task")
			task.WorkflowState = initial
			task.DeferredUntil = &deferUntil
			target, changed, err := app.ApplyPlanningActionToTask(task, "project", app.PlanningAddToday, now)
			if err != nil {
				t.Fatal(err)
			}
			wantState := models.StateBacklog
			if initial == models.StateDoing {
				wantState = models.StateDoing
			}
			if task.State() != wantState || task.DeferredUntil != nil || !changed {
				t.Fatalf("Today result = state:%s deferUntil:%v changed:%t, want %s nil true", task.State(), task.DeferredUntil, changed, wantState)
			}
			if target.Today != app.PlanningTodayAdd || target.Next != app.PlanningNextRemove {
				t.Fatalf("Today directives = %+v", target)
			}
		})

		t.Run("next_"+string(initial), func(t *testing.T) {
			task := newTask("task", 1, "task")
			task.WorkflowState = initial
			task.DeferredUntil = &deferUntil
			target, changed, err := app.ApplyPlanningActionToTask(task, "project", app.PlanningAddNext, now)
			if err != nil {
				t.Fatal(err)
			}
			if task.State() != models.StateBacklog || task.DeferredUntil != nil || !changed {
				t.Fatalf("Next result = state:%s deferUntil:%v changed:%t, want backlog nil true", task.State(), task.DeferredUntil, changed)
			}
			if target.Today != app.PlanningTodayRemove || target.Next != app.PlanningNextAdd {
				t.Fatalf("Next directives = %+v", target)
			}
		})
	}
}

func TestResolvePlanningToggleActionUsesAllSelectedRule(t *testing.T) {
	a := app.TaskLocator{ProjectID: "project", TaskID: "a"}
	b := app.TaskLocator{ProjectID: "project", TaskID: "b"}

	for _, horizon := range []app.PlanningHorizon{app.PlanningHorizonToday, app.PlanningHorizonNext} {
		add, err := app.ResolvePlanningToggleAction(horizon, []app.TaskLocator{a, b}, []app.TaskLocator{a})
		if err != nil {
			t.Fatal(err)
		}
		remove, err := app.ResolvePlanningToggleAction(horizon, []app.TaskLocator{a, b}, []app.TaskLocator{b, a})
		if err != nil {
			t.Fatal(err)
		}
		wantAdd, wantRemove := app.PlanningAddToday, app.PlanningRemoveToday
		if horizon == app.PlanningHorizonNext {
			wantAdd, wantRemove = app.PlanningAddNext, app.PlanningRemoveNext
		}
		if add != wantAdd || remove != wantRemove {
			t.Fatalf("%s mixed/all actions = %s/%s, want %s/%s", horizon, add, remove, wantAdd, wantRemove)
		}
	}
}

func TestPlanningReorderCannotConferMembership(t *testing.T) {
	a := config.ProjectTaskLocator{ProjectID: "project", TaskID: "a"}
	b := config.ProjectTaskLocator{ProjectID: "project", TaskID: "b"}
	c := config.ProjectTaskLocator{ProjectID: "project", TaskID: "c"}
	planning := config.NewPlanningState()
	planning.SetNext([]config.ProjectTaskLocator{a, b})

	if !app.ReconcilePlanningState(planning, app.PlanningReconcileRequest{
		Targets: []app.PlanningReconcileTarget{
			{ProjectID: "project", TaskID: "a", State: models.StateBacklog},
			{ProjectID: "project", TaskID: "b", State: models.StateBacklog},
			{ProjectID: "project", TaskID: "c", State: models.StateBacklog},
		},
		Next: []app.TaskLocator{
			{ProjectID: c.ProjectID, TaskID: c.TaskID},
			{ProjectID: b.ProjectID, TaskID: b.TaskID},
		},
		SetNext: true,
	}) {
		t.Fatal("reorder reported no change")
	}
	if got, want := planning.GetNext(), []config.ProjectTaskLocator{b, a}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Next = %+v, want %+v", got, want)
	}
}

func TestDeferCommitsLifecycleAndPlanningTogether(t *testing.T) {
	task := newTask("task", 1, "planned")
	task.WorkflowState = models.StateDoing
	project := newProject("project", "Project", task)
	repo := newFakeRepo(project)
	planning := config.NewPlanningState()
	planning.SetToday([]config.ProjectTaskLocator{{ProjectID: project.ID, TaskID: task.ID}})
	svc := newTestService(repo).WithPlanningArtifacts(memoryPlanningBuilder(planning))
	until := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)

	if _, err := svc.DeferByLocator(app.TaskLocator{ProjectID: project.ID, TaskID: task.ID}, &until); err != nil {
		t.Fatal(err)
	}
	if task.State() != models.StateBacklog || task.DeferredUntil == nil || !task.DeferredUntil.Equal(until) {
		t.Fatalf("task after deferUntil = state:%s deferUntil:%v", task.State(), task.DeferredUntil)
	}
	if len(planning.GetToday()) != 0 || len(planning.GetNext()) != 0 {
		t.Fatalf("planning after deferUntil = Today:%+v Next:%+v", planning.GetToday(), planning.GetNext())
	}
	if len(repo.commits) != 1 || len(repo.commits[0]) != 1 || len(repo.commits[0][0].Artifacts) != 1 {
		t.Fatalf("deferUntil commits = %+v, want one project-plus-planning transaction", repo.commits)
	}
}

func TestDeleteOwnsSubtreePlanningCleanupAndRollback(t *testing.T) {
	t.Run("complete subtree cleanup", func(t *testing.T) {
		parent := newTask("parent", 1, "parent")
		child := newTask("child", 2, "child")
		child.ParentID = &parent.ID
		kept := newTask("kept", 3, "kept")
		project := newProject("project", "Project", parent, child, kept)
		repo := newFakeRepo(project)
		planning := config.NewPlanningState()
		planning.SetToday([]config.ProjectTaskLocator{
			{ProjectID: project.ID, TaskID: parent.ID},
			{ProjectID: project.ID, TaskID: kept.ID},
		})
		planning.SetNext([]config.ProjectTaskLocator{{ProjectID: project.ID, TaskID: child.ID}})
		svc := newTestService(repo).WithPlanningArtifacts(memoryPlanningBuilder(planning))

		if _, err := svc.Delete(app.DeleteRequest{TaskID: parent.ID, Confirm: true}); err != nil {
			t.Fatal(err)
		}
		if got := planning.GetToday(); !reflect.DeepEqual(got, []config.ProjectTaskLocator{{ProjectID: project.ID, TaskID: kept.ID}}) {
			t.Fatalf("Today after delete = %+v", got)
		}
		if len(planning.GetNext()) != 0 {
			t.Fatalf("Next after delete = %+v", planning.GetNext())
		}
		if len(repo.commits) != 1 || len(repo.commits[0][0].Artifacts) != 1 {
			t.Fatalf("delete did not carry one planning artifact: %+v", repo.commits)
		}
	})

	t.Run("planning build failure leaves subtree intact", func(t *testing.T) {
		parent := newTask("parent", 1, "parent")
		child := newTask("child", 2, "child")
		child.ParentID = &parent.ID
		project := newProject("project", "Project", parent, child)
		repo := newFakeRepo(project)
		wantErr := errors.New("planning unavailable")
		svc := newTestService(repo).WithPlanningArtifacts(func(app.PlanningReconcileRequest) ([]storage.ArtifactChange, error) {
			return nil, wantErr
		})

		_, err := svc.Delete(app.DeleteRequest{TaskID: parent.ID, Confirm: true})
		if !errors.Is(err, wantErr) {
			t.Fatalf("Delete error = %v, want %v", err, wantErr)
		}
		if project.GetTaskByID(parent.ID) == nil || project.GetTaskByID(child.ID) == nil || len(repo.commits) != 0 {
			t.Fatalf("failed delete mutated project: tasks=%+v commits=%d", project.Tasks, len(repo.commits))
		}
	})

	t.Run("commit failure restores subtree", func(t *testing.T) {
		parent := newTask("parent", 1, "parent")
		child := newTask("child", 2, "child")
		child.ParentID = &parent.ID
		project := newProject("project", "Project", parent, child)
		repo := newFakeRepo(project)
		repo.commitErr = errors.New("commit failed")
		svc := newTestService(repo)

		if _, err := svc.Delete(app.DeleteRequest{TaskID: parent.ID, Confirm: true}); err == nil {
			t.Fatal("Delete unexpectedly succeeded")
		}
		if project.GetTaskByID(parent.ID) == nil || project.GetTaskByID(child.ID) == nil || len(repo.commits) != 0 {
			t.Fatalf("failed commit did not restore subtree: tasks=%+v commits=%d", project.Tasks, len(repo.commits))
		}
	})
}
