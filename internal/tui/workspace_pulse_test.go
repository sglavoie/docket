package tui

import (
	"errors"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

type selectivePulseRepository struct {
	*testutil.MockRepository
	failProjectID string
}

type fixedCompletionActivityClock struct{ now time.Time }

func (c fixedCompletionActivityClock) Now() time.Time { return c.now }

func (r *selectivePulseRepository) Load(project *models.Project) error {
	if project != nil && project.ID == r.failProjectID {
		return errors.New("injected workspace pulse load failure")
	}
	return r.MockRepository.Load(project)
}

func installPulseRepository(t *testing.T, projects ...*models.Project) *testutil.MockRepository {
	t.Helper()
	repository := testutil.NewMockRepository()
	for _, project := range projects {
		repository.Projects[project.ID] = project
	}
	previous := storage.DefaultRepository
	storage.DefaultRepository = repository
	t.Cleanup(func() { storage.DefaultRepository = previous })
	return repository
}

func TestWorkspacePulseCountsEligibleFlatTasksAtInjectedTime(t *testing.T) {
	now := time.Date(2026, time.August, 6, 15, 30, 0, 0, time.FixedZone("test", -6*60*60))
	yesterday := now.AddDate(0, 0, -1)
	tomorrow := now.AddDate(0, 0, 1)
	parentID := "parent"

	parent := primaryRowsTask(parentID, models.StateInbox)
	child := primaryRowsTask("child", models.StateInbox)
	child.ParentID = &parentID
	today := primaryRowsTask("today", models.StateBacklog)
	overdue := primaryRowsTask("overdue", models.StateBacklog)
	overdue.DueDate = &yesterday
	sameDayDue := primaryRowsTask("same-day", models.StateBacklog)
	sameDayDue.DueDate = &now
	terminal := primaryRowsTask("terminal", models.StateDone)
	terminal.DueDate = &yesterday
	deferred := primaryRowsTask("deferred", models.StateInbox)
	deferred.DueDate = &yesterday
	deferred.DeferredUntil = &tomorrow

	active := primaryRowsProject("active", parent, child, today, overdue, sameDayDue, terminal, deferred)
	nilTasks := primaryRowsProject("nil-tasks", nil)
	parked := primaryRowsProject("parked", primaryRowsTask("parked-inbox", models.StateInbox))
	parked.Tags = []string{models.ParkedProjectTag}
	archived := primaryRowsProject("archived", primaryRowsTask("archived-inbox", models.StateInbox))
	archived.Archived = true
	runtime := testConfigWithProjects(active, nilTasks, parked, archived)
	installPulseRepository(t, active, nilTasks, parked, archived)

	todayIDs := map[taskref.TaskIdentity]bool{
		taskref.For(today, active):                     true,
		taskref.For(terminal, active):                  true,
		taskref.For(deferred, active):                  true,
		{ProjectID: active.ID, TaskID: "stale"}:        true,
		{ProjectID: parked.ID, TaskID: "parked-inbox"}: true,
	}
	pulse := outliner.ComputeWorkspacePulse(runtime, todayIDs, now)

	if pulse.Inbox != 2 {
		t.Errorf("Inbox = %d, want parent and nested child exactly once", pulse.Inbox)
	}
	if pulse.Today != 1 {
		t.Errorf("Today = %d, want only the eligible loaded planning identity", pulse.Today)
	}
	if pulse.Overdue != 1 {
		t.Errorf("Overdue = %d, want only the actionable overdue task", pulse.Overdue)
	}
}

func TestWorkspacePulsePreservesPartialLoadResults(t *testing.T) {
	now := time.Date(2026, time.August, 6, 12, 0, 0, 0, time.UTC)
	loaded := primaryRowsProject("loaded", primaryRowsTask("loaded-inbox", models.StateInbox))
	failed := primaryRowsProject("failed", primaryRowsTask("failed-inbox", models.StateInbox))
	runtime := testConfigWithProjects(loaded, failed)
	base := installPulseRepository(t, loaded, failed)
	storage.DefaultRepository = &selectivePulseRepository{MockRepository: base, failProjectID: failed.ID}

	pulse := outliner.ComputeWorkspacePulse(runtime, nil, now)
	if pulse.Inbox != 1 {
		t.Fatalf("Inbox = %d, want the successfully loaded project to survive a peer load failure", pulse.Inbox)
	}
}

func TestWorkspacePulseNilRuntimeAndRefreshSeam(t *testing.T) {
	now := time.Date(2026, time.August, 6, 12, 0, 0, 0, time.UTC)
	if zero := outliner.ComputeWorkspacePulse(nil, nil, now); zero != (pulse.WorkspacePulse{}) {
		t.Fatalf("nil runtime pulse = %+v, want zero", zero)
	}

	project := primaryRowsProject("refresh", primaryRowsTask("inbox", models.StateInbox))
	runtime := testConfigWithProjects(project)
	installPulseRepository(t, project)
	shell := &AppModel{domain: domainController{runtime: runtime}}
	shell.refreshWorkspacePulseAt(now)
	if shell.workspacePulse.Inbox != 1 {
		t.Fatalf("refreshed Inbox = %d, want 1", shell.workspacePulse.Inbox)
	}
}

func TestWorkspacePulseRefreshUsesSameDayCompletionActivity(t *testing.T) {
	now := time.Now()
	done := primaryRowsTask("done-today", models.StateDone)
	done.CompletedAt = &now
	project := primaryRowsProject("completed", done)
	runtime := testConfigWithProjects(project)
	installPulseRepository(t, project)
	service := app.NewServiceWithArtifacts(newTestAppRepo(project), nil, nil, nil)
	shell := &AppModel{
		domain: domainController{runtime: runtime},
		features: coordinator.FeatureCoordinators{
			Activity: coordinator.NewActivityCoordinator(service),
		},
	}

	shell.refreshWorkspacePulseAt(now)

	if shell.workspacePulse.Done != 1 {
		t.Fatalf("refreshed Done = %d, want the current-local-day completion count", shell.workspacePulse.Done)
	}
}

func TestPeriodicTickRefreshesActivityAcrossIdleLocalMidnight(t *testing.T) {
	model := typedTestApp(t)
	tickAt := time.Date(2040, time.January, 2, 0, 1, 0, 0, time.Local)
	yesterday := tickAt.AddDate(0, 0, -1)
	firstYesterday := primaryRowsTask("done-yesterday-1", models.StateDone)
	firstYesterday.CompletedAt = &yesterday
	secondYesterday := primaryRowsTask("done-yesterday-2", models.StateDone)
	secondYesterday.CompletedAt = &yesterday
	doneToday := primaryRowsTask("done-today", models.StateDone)
	doneToday.CompletedAt = &tickAt
	project := primaryRowsProject("idle-midnight", firstYesterday, secondYesterday, doneToday)
	installPulseRepository(t, project)
	repository := newTestAppRepo(project)
	cached, err := app.NewServiceWithArtifacts(repository, fixedCompletionActivityClock{now: yesterday}, nil, nil).CompletionActivity()
	if err != nil {
		t.Fatalf("seed previous-day activity: %v", err)
	}
	if cached.CompletedToday() != 2 {
		t.Fatalf("seeded previous-day count = %d, want 2", cached.CompletedToday())
	}
	refreshed, err := app.NewServiceWithArtifacts(repository, fixedCompletionActivityClock{now: tickAt}, nil, nil).CompletionActivity()
	if err != nil {
		t.Fatalf("seed current-day activity: %v", err)
	}
	reader := &staticCompletionActivityReader{activity: refreshed}
	model.features.Activity = coordinator.NewActivityCoordinator(reader)
	model.completionActivity = cached
	model.workspacePulse.Done = cached.CompletedToday()
	model.crossOutliner.Population.ForceRebuild = false
	model.crossOutliner.NextDeferWake = tickAt.Add(-time.Minute)

	if handled, _ := model.handleGlobalSaveMessage(status.PeriodicSaveMsg{At: tickAt}); !handled {
		t.Fatal("periodicSaveMsg was not handled")
	}
	if reader.calls != 1 {
		t.Fatalf("completion activity queries = %d, want one cross-day refresh", reader.calls)
	}
	if !model.completionActivity.IsCurrentLocalDay(tickAt) {
		t.Fatal("periodic tick did not install the current-day activity")
	}
	if model.workspacePulse.Done != 1 {
		t.Fatalf("Done = %d, want the new local day's count rather than the previous day's 2", model.workspacePulse.Done)
	}
	if !model.crossOutliner.Population.ForceRebuild {
		t.Fatal("periodic tick did not use its supplied timestamp for the defer wake")
	}

	if handled, _ := model.handleGlobalSaveMessage(status.PeriodicSaveMsg{At: tickAt.Add(time.Hour)}); !handled {
		t.Fatal("second periodicSaveMsg was not handled")
	}
	if reader.calls != 1 {
		t.Fatalf("same-day periodic activity queries = %d, want cached snapshot reuse", reader.calls)
	}
	if model.workspacePulse.Done != 1 {
		t.Fatalf("same-day Done = %d, want the refreshed count to stand", model.workspacePulse.Done)
	}
}

func TestWorkspacePulseRefreshPreservesValidSameDayActivityOnFailure(t *testing.T) {
	now := time.Date(2026, time.August, 19, 12, 0, 0, 0, time.UTC)
	done := primaryRowsTask("done-today", models.StateDone)
	done.CompletedAt = &now
	project := primaryRowsProject("same-day-fallback", done)
	runtime := testConfigWithProjects(project)
	installPulseRepository(t, project)
	service := app.NewServiceWithArtifacts(newTestAppRepo(project), fixedCompletionActivityClock{now: now}, nil, nil)
	cached, err := service.CompletionActivity()
	if err != nil {
		t.Fatalf("seed completion activity: %v", err)
	}
	reader := &staticCompletionActivityReader{err: errors.New("injected activity failure")}
	shell := &AppModel{
		domain:             domainController{runtime: runtime},
		completionActivity: cached,
		features: coordinator.FeatureCoordinators{
			Activity: coordinator.NewActivityCoordinator(reader),
		},
	}

	shell.refreshWorkspacePulseAt(now.Add(time.Hour))

	if shell.workspacePulse.Done != 1 {
		t.Fatalf("Done = %d, want valid same-day cached count", shell.workspacePulse.Done)
	}
	if !shell.completionActivity.IsCurrentLocalDay(now) || !shell.completionActivity.HasHistory() {
		t.Fatal("same-day query failure discarded the valid cached activity")
	}
}

func TestWorkspacePulseRefreshClearsCrossDayActivityOnFailureAndRetries(t *testing.T) {
	yesterday := time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC)
	now := yesterday.AddDate(0, 0, 1)
	done := primaryRowsTask("done-yesterday", models.StateDone)
	done.CompletedAt = &yesterday
	inbox := primaryRowsTask("open-inbox", models.StateInbox)
	project := primaryRowsProject("cross-day-fallback", done, inbox)
	runtime := testConfigWithProjects(project)
	installPulseRepository(t, project)
	service := app.NewServiceWithArtifacts(newTestAppRepo(project), fixedCompletionActivityClock{now: yesterday}, nil, nil)
	cached, err := service.CompletionActivity()
	if err != nil {
		t.Fatalf("seed completion activity: %v", err)
	}
	reader := &staticCompletionActivityReader{err: errors.New("injected activity failure")}
	shell := &AppModel{
		domain:             domainController{runtime: runtime},
		completionActivity: cached,
		features: coordinator.FeatureCoordinators{
			Activity: coordinator.NewActivityCoordinator(reader),
		},
	}

	shell.refreshWorkspacePulseAt(now)

	if shell.workspacePulse.Inbox != 1 {
		t.Fatalf("Inbox = %d, want newly computed open-work pulse", shell.workspacePulse.Inbox)
	}
	if shell.workspacePulse.Done != 0 {
		t.Fatalf("Done = %d, want zero after cross-day query failure", shell.workspacePulse.Done)
	}
	if !shell.completionActivity.AsOf.IsZero() || shell.completionActivity.HasHistory() || shell.completionActivity.CompletedToday() != 0 {
		t.Fatalf("cross-day failure retained stale activity: %+v", shell.completionActivity)
	}

	shell.refreshWorkspacePulseAt(now.Add(time.Minute))
	if reader.calls != 2 {
		t.Fatalf("completion activity queries = %d, want retry after stale cache clear", reader.calls)
	}
}
