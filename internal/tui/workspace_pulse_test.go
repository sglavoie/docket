package tui

import (
	"errors"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
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

	todayIDs := map[TaskIdentity]bool{
		taskIdentityFor(today, active):                 true,
		taskIdentityFor(terminal, active):              true,
		taskIdentityFor(deferred, active):              true,
		{ProjectID: active.ID, TaskID: "stale"}:        true,
		{ProjectID: parked.ID, TaskID: "parked-inbox"}: true,
	}
	pulse := ComputeWorkspacePulse(runtime, todayIDs, now)

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

	pulse := ComputeWorkspacePulse(runtime, nil, now)
	if pulse.Inbox != 1 {
		t.Fatalf("Inbox = %d, want the successfully loaded project to survive a peer load failure", pulse.Inbox)
	}
}

func TestWorkspacePulseNilRuntimeAndRefreshSeam(t *testing.T) {
	now := time.Date(2026, time.August, 6, 12, 0, 0, 0, time.UTC)
	if pulse := ComputeWorkspacePulse(nil, nil, now); pulse != (WorkspacePulse{}) {
		t.Fatalf("nil runtime pulse = %+v, want zero", pulse)
	}

	project := primaryRowsProject("refresh", primaryRowsTask("inbox", models.StateInbox))
	runtime := testConfigWithProjects(project)
	installPulseRepository(t, project)
	shell := &AppModel{runtime: runtime}
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
		runtime: runtime,
		features: FeatureCoordinators{
			Activity: ActivityCoordinator{reader: service},
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
	reader := &staticCompletionActivityReader{activity: app.CompletionActivity{
		AsOf: tickAt,
		DailyCompletions: map[time.Time]int{
			time.Date(2040, time.January, 2, 0, 0, 0, 0, time.Local): 1,
		},
	}}
	model.features.Activity = ActivityCoordinator{reader: reader}
	model.completionActivity = app.CompletionActivity{AsOf: tickAt.AddDate(0, 0, -1)}
	model.crossOutliner.crossForceRebuild = false
	model.crossOutliner.nextDeferWake = tickAt.Add(-time.Minute)

	if handled, _ := model.handleGlobalSaveMessage(periodicSaveMsg{at: tickAt}); !handled {
		t.Fatal("periodicSaveMsg was not handled")
	}
	if reader.calls != 1 {
		t.Fatalf("completion activity queries = %d, want one cross-day refresh", reader.calls)
	}
	if !model.completionActivity.IsCurrentLocalDay(tickAt) {
		t.Fatal("periodic tick did not install the current-day activity")
	}
	if !model.crossOutliner.crossForceRebuild {
		t.Fatal("periodic tick did not use its supplied timestamp for the defer wake")
	}

	if handled, _ := model.handleGlobalSaveMessage(periodicSaveMsg{at: tickAt.Add(time.Hour)}); !handled {
		t.Fatal("second periodicSaveMsg was not handled")
	}
	if reader.calls != 1 {
		t.Fatalf("same-day periodic activity queries = %d, want cached snapshot reuse", reader.calls)
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
		runtime:            runtime,
		completionActivity: cached,
		features: FeatureCoordinators{
			Activity: ActivityCoordinator{reader: reader},
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
		runtime:            runtime,
		completionActivity: cached,
		features: FeatureCoordinators{
			Activity: ActivityCoordinator{reader: reader},
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

func TestActivityCoordinatorRejectsNilReader(t *testing.T) {
	if _, err := (ActivityCoordinator{}).CompletionActivity(); !errors.Is(err, errCompletionActivityReaderUnavailable) {
		t.Fatalf("nil reader error = %v, want %v", err, errCompletionActivityReaderUnavailable)
	}
}

func TestWorkspacePulseFormattingFollowsFourStepLadder(t *testing.T) {
	pulse := WorkspacePulse{Inbox: 3, Today: 2, Overdue: 1, Done: 4}
	full := formatWorkspacePulseComponents(pulse, workspacePulseFull)
	wantStyled := MutedStyle.Render("workspace: ") +
		MutedStyle.Render("inbox 3") +
		MutedStyle.Render(" · ") +
		MutedStyle.Render("today 2") +
		MutedStyle.Render(" · ") +
		PreviewWarningStyle.Render("overdue 1") +
		MutedStyle.Render(" · ") +
		MutedStyle.Render("done 4")
	if full != wantStyled {
		t.Fatalf("full pulse ANSI styling = %q, want %q", full, wantStyled)
	}

	for _, tt := range []struct {
		level workspacePulseLevel
		want  string
	}{
		{workspacePulseFull, "workspace: inbox 3 · today 2 · overdue 1 · done 4"},
		{workspacePulseWithoutDone, "workspace: inbox 3 · today 2 · overdue 1"},
		{workspacePulseWithoutInbox, "workspace: today 2 · overdue 1"},
		{workspacePulseTodayOnly, "workspace: today 2"},
		{workspacePulseHidden, ""},
	} {
		if got := ansi.Strip(formatWorkspacePulseComponents(pulse, tt.level)); got != tt.want {
			t.Errorf("level %d pulse = %q, want %q", tt.level, got, tt.want)
		}
	}

	if got := ansi.Strip(formatWorkspacePulseComponents(WorkspacePulse{}, workspacePulseFull)); got != "workspace: today 0 · done 0" {
		t.Fatalf("zero pulse = %q, want Today and Done zeroes", got)
	}
}
