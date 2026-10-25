package app_test

import (
	"errors"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/textgrammar"
)

func TestCaptureDestinationsApplyLifecyclePlanningAndTerminalTimestamps(t *testing.T) {
	wantState := map[app.CaptureDestination]models.WorkflowState{
		app.CaptureDestinationInbox:     models.StateInbox,
		app.CaptureDestinationBacklog:   models.StateBacklog,
		app.CaptureDestinationToday:     models.StateBacklog,
		app.CaptureDestinationNext:      models.StateBacklog,
		textgrammar.DestinationDoing:    models.StateDoing,
		textgrammar.DestinationWaiting:  models.StateWaiting,
		textgrammar.DestinationPaused:   models.StatePaused,
		textgrammar.DestinationSomeday:  models.StateSomeday,
		textgrammar.DestinationDone:     models.StateDone,
		textgrammar.DestinationCanceled: models.StateCanceled,
	}
	wantNow := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)

	for destination := range wantState {
		destination := destination
		t.Run(string(destination), func(t *testing.T) {
			project := newProject("p1", "Alpha")
			repo := newFakeRepo(project)
			planning := &config.PlanningState{}
			svc := newTestService(repo).WithPlanningArtifacts(memoryPlanningBuilder(planning))

			result, err := svc.Capture(app.CaptureRequest{
				ProjectID: project.ID, Title: "Task", Destination: destination,
			})
			if err != nil {
				t.Fatalf("Capture: %v", err)
			}
			task := project.GetTaskByID(result.TaskID)
			if task == nil || task.State() != wantState[destination] {
				t.Fatalf("task/state = %+v, want %s", task, wantState[destination])
			}
			if task.UpdatedAt != wantNow {
				t.Errorf("UpdatedAt = %v, want %v", task.UpdatedAt, wantNow)
			}
			if destination == textgrammar.DestinationDone || destination == textgrammar.DestinationCanceled {
				if task.CompletedAt == nil || *task.CompletedAt != wantNow {
					t.Errorf("CompletedAt = %v, want %v", task.CompletedAt, wantNow)
				}
			} else if task.CompletedAt != nil {
				t.Errorf("CompletedAt = %v, want nil", task.CompletedAt)
			}

			wantToday := destination == app.CaptureDestinationToday
			wantNext := destination == app.CaptureDestinationNext
			if got := planningHasLocator(planning.GetToday(), task.ID, project.ID); got != wantToday {
				t.Errorf("Today membership = %v, want %v", got, wantToday)
			}
			if got := planningHasLocator(planning.GetNext(), task.ID, project.ID); got != wantNext {
				t.Errorf("Next membership = %v, want %v", got, wantNext)
			}
			if len(repo.commits) != 1 {
				t.Fatalf("commits = %d, want one atomic commit", len(repo.commits))
			}
		})
	}
}

func TestCapturePlanningDestinationRequiresApplicationPlanningPort(t *testing.T) {
	project := newProject("p1", "Alpha")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.Capture(app.CaptureRequest{
		ProjectID: project.ID, Title: "Today", Destination: app.CaptureDestinationToday,
	})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("error = %v, want planning builder failure", err)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("commits = %d, want none", len(repo.commits))
	}
	if len(project.Tasks) != 0 || project.NextShortID != 1 {
		t.Fatalf("failed destination mutated project: tasks=%d nextShortID=%d", len(project.Tasks), project.NextShortID)
	}
}

func planningHasLocator(items []config.ProjectTaskLocator, taskID, projectID string) bool {
	for _, item := range items {
		if item.TaskID == taskID && item.ProjectID == projectID {
			return true
		}
	}
	return false
}

func TestCaptureDestinationRetiredReplacementGuidance(t *testing.T) {
	for raw, want := range map[string]string{
		"ready":       "destination $ready was removed (use $backlog)",
		"in_progress": "destination $in_progress was removed (use $doing)",
	} {
		if got := app.InvalidCaptureDestinationError(raw).Error(); got != want {
			t.Errorf("error for %q = %q, want %q", raw, got, want)
		}
	}
}

func TestCaptureManyCommitsMixedDestinationsAndPlanningOnce(t *testing.T) {
	project := newProject("p1", "Alpha")
	repo := newFakeRepo(project)
	planning := &config.PlanningState{}
	svc := newTestService(repo).WithPlanningArtifacts(memoryPlanningBuilder(planning))

	results, err := svc.CaptureMany([]app.CaptureRequest{
		{SourceLine: 1, ProjectID: project.ID, Title: "Today", Destination: app.CaptureDestinationToday},
		{SourceLine: 2, ProjectID: project.ID, Title: "Next", Destination: app.CaptureDestinationNext},
		{SourceLine: 3, ProjectID: project.ID, Title: "Done", Destination: textgrammar.DestinationDone},
	})
	if err != nil {
		t.Fatalf("CaptureMany: %v", err)
	}
	if len(results) != 3 || len(repo.commits) != 1 {
		t.Fatalf("results/commits = %d/%d, want 3/1", len(results), len(repo.commits))
	}
	if !planningHasLocator(planning.GetToday(), results[0].TaskID, project.ID) ||
		!planningHasLocator(planning.GetNext(), results[1].TaskID, project.ID) {
		t.Fatalf("planning = Today:%+v Next:%+v", planning.GetToday(), planning.GetNext())
	}
	done := project.GetTaskByID(results[2].TaskID)
	if done == nil || done.State() != models.StateDone || done.CompletedAt == nil {
		t.Fatalf("done capture = %+v", done)
	}
}

func TestExpandedAddDestinationUsesSameAtomicPlanningInvariant(t *testing.T) {
	project := newProject("p1", "Alpha")
	repo := newFakeRepo(project)
	planning := &config.PlanningState{}
	svc := newTestService(repo).WithPlanningArtifacts(memoryPlanningBuilder(planning))

	result, err := svc.AddTask(app.AddTaskRequest{
		ProjectID: project.ID, Title: "Expanded", Destination: app.CaptureDestinationToday,
	})
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}
	task := project.GetTaskByID(result.TaskID)
	if task == nil || task.State() != models.StateBacklog || !planningHasLocator(planning.GetToday(), task.ID, project.ID) {
		t.Fatalf("expanded destination task/planning = %+v / %+v", task, planning.GetToday())
	}
	if len(repo.commits) != 1 {
		t.Fatalf("commits = %d, want one", len(repo.commits))
	}
}

func TestExpandedPlanningDestinationFailureRollsBackProject(t *testing.T) {
	project := newProject("p1", "Alpha")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.AddTaskWithChildren(app.AddTaskRequest{
		ProjectID: project.ID, Title: "Expanded", Destination: app.CaptureDestinationNext,
		Subtasks: []app.AddSubtaskRequest{{Title: "Child"}},
	})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
	if len(project.Tasks) != 0 || project.NextShortID != 1 || len(repo.commits) != 0 {
		t.Fatalf("failed expanded destination was partial: tasks=%d nextShortID=%d commits=%d", len(project.Tasks), project.NextShortID, len(repo.commits))
	}
}

// A deferral and a check-in are written after the lifecycle initialization, so
// every clock a capture stamps must come from the one service instant: a state
// clock read later than the touch time is what tt doctor reports as a task that
// changed state after it was last touched.
func TestCaptureStampsEveryCreationClockFromOneInstant(t *testing.T) {
	wantNow := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	wake := wantNow.Add(72 * time.Hour)
	checkIn := time.Date(2026, 7, 13, 0, 0, 0, 0, time.Local)
	cases := map[string]app.CaptureRequest{
		"deferred": {Title: "Deferred", DeferSet: true, Defer: &wake},
		"check-in": {
			Title:       "Checked in",
			Destination: textgrammar.DestinationWaiting,
			CheckIn:     models.CheckInSchedule{Next: &checkIn},
		},
		"backlog": {Title: "Plain", Destination: app.CaptureDestinationBacklog},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			project := newProject("p1", "Alpha")
			svc := newTestService(newFakeRepo(project))
			req.ProjectID = project.ID
			result, err := svc.Capture(req)
			if err != nil {
				t.Fatalf("Capture: %v", err)
			}
			task := project.GetTaskByID(result.TaskID)
			for label, got := range map[string]time.Time{
				"CreatedAt":      task.CreatedAt,
				"StateChangedAt": task.StateChangedAt,
				"UpdatedAt":      task.UpdatedAt,
			} {
				if !got.Equal(wantNow) {
					t.Errorf("%s = %v, want %v", label, got, wantNow)
				}
			}
		})
	}
}
