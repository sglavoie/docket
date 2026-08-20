package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

func TestFormatDeferDateLadder(t *testing.T) {
	now := time.Date(2026, 8, 14, 10, 30, 0, 0, time.UTC)
	cases := []struct {
		name     string
		deferred time.Time
		want     string
	}{
		{"midnight this year stays a bare day", time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC), "Sep 2"},
		{"midnight off year stays a bare day", time.Date(2027, 1, 2, 0, 0, 0, 0, time.UTC), "Jan 2 '27"},
		{"same calendar day is a clock", time.Date(2026, 8, 14, 15, 4, 0, 0, time.UTC), "15:04"},
		{"off year with a time", time.Date(2027, 1, 2, 9, 0, 0, 0, time.UTC), "Jan 2 '27 09:00"},
		{"another day this year", time.Date(2026, 8, 15, 9, 0, 0, 0, time.UTC), "Aug 15 09:00"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatDeferDate(tc.deferred, now); got != tc.want {
				t.Fatalf("formatDeferDate(%s) = %q, want %q", tc.deferred, got, tc.want)
			}
		})
	}
}

func TestPeriodicTickWakesOnlyPastTheDeferWake(t *testing.T) {
	app := typedTestApp(t)
	setTestPresentationPath(app, ViewCrossProjectOutliner)
	out := app.crossOutliner
	tickAt := time.Now()

	out.crossForceRebuild = false
	out.nextDeferWake = tickAt.Add(time.Hour)
	if handled, _ := app.handleGlobalSaveMessage(periodicSaveMsg{at: tickAt}); !handled {
		t.Fatal("periodicSaveMsg was not handled")
	}
	if out.crossForceRebuild {
		t.Fatal("a tick short of nextDeferWake forced a rebuild")
	}

	out.nextDeferWake = tickAt.Add(-time.Minute)
	if handled, _ := app.handleGlobalSaveMessage(periodicSaveMsg{at: tickAt}); !handled {
		t.Fatal("periodicSaveMsg was not handled")
	}
	if !out.crossForceRebuild {
		t.Fatal("a tick past nextDeferWake did not force a rebuild")
	}
	if !out.nextDeferWake.IsZero() {
		t.Fatal("the consumed wake clock survived the rebuild it armed")
	}
}

func TestPeriodicTickRefreshesWorkspacePulseWhenDeferredTaskWakes(t *testing.T) {
	model := typedTestApp(t)
	setTestPresentationPath(model, ViewCrossProjectOutliner)
	out := model.crossOutliner
	tickAt := time.Date(2040, time.March, 4, 9, 0, 0, 0, time.Local)
	wake := tickAt.Add(-time.Minute)
	due := tickAt.AddDate(0, 0, -3)

	deferred := primaryRowsTask("deferred-inbox", models.StateInbox)
	deferred.DeferredUntil = &wake
	deferred.DueDate = &due
	completed := primaryRowsTask("done-today", models.StateDone)
	completed.CompletedAt = &tickAt
	project := primaryRowsProject("defer-wake-pulse", deferred, completed)
	setTestAppProjects(model, project)
	installPulseRepository(t, project)

	activity, err := app.NewServiceWithArtifacts(newTestAppRepo(project), fixedCompletionActivityClock{now: tickAt}, nil, nil).CompletionActivity()
	if err != nil {
		t.Fatalf("seed completion activity: %v", err)
	}
	reader := &staticCompletionActivityReader{activity: activity}
	model.features.Activity = ActivityCoordinator{reader: reader}
	model.todayIDs = map[TaskIdentity]bool{taskIdentityFor(deferred, project): true}

	model.refreshWorkspacePulseAt(wake.Add(-time.Minute))
	pending := WorkspacePulse{Done: 1}
	if model.workspacePulse != pending {
		t.Fatalf("pending-deferral pulse = %+v, want %+v", model.workspacePulse, pending)
	}

	pushTestPresentationOverlay(model, ViewTaskComments)
	out.crossForceRebuild = false
	out.nextDeferWake = wake
	if handled, _ := model.handleGlobalSaveMessage(periodicSaveMsg{at: tickAt}); !handled {
		t.Fatal("periodicSaveMsg was not handled")
	}
	if out.crossForceRebuild || !out.nextDeferWake.Equal(wake) {
		t.Fatalf("a refused wake changed its clock: rebuild %t, wake %s", out.crossForceRebuild, out.nextDeferWake)
	}
	if model.workspacePulse != pending {
		t.Fatalf("a refused wake refreshed the pulse to %+v", model.workspacePulse)
	}

	setTestPresentationPath(model, ViewCrossProjectOutliner)
	if handled, _ := model.handleGlobalSaveMessage(periodicSaveMsg{at: tickAt}); !handled {
		t.Fatal("second periodicSaveMsg was not handled")
	}
	if !out.crossForceRebuild {
		t.Fatal("a safely consumed wake did not force the row rebuild")
	}
	if !out.nextDeferWake.IsZero() {
		t.Fatalf("the consumed wake clock survived: %s", out.nextDeferWake)
	}
	woken := WorkspacePulse{Inbox: 1, Today: 1, Overdue: 1, Done: 1}
	if model.workspacePulse != woken {
		t.Fatalf("same-day wake pulse = %+v, want %+v", model.workspacePulse, woken)
	}
}

func TestPeriodicTickAcrossMidnightHoldsActionableCountsUntilTheWakeIsConsumed(t *testing.T) {
	model := typedTestApp(t)
	setTestPresentationPath(model, ViewCrossProjectOutliner)
	out := model.crossOutliner
	tickAt := time.Date(2040, time.March, 5, 0, 1, 0, 0, time.Local)
	wake := tickAt.Add(-2 * time.Minute)
	beforeWake := wake.Add(-time.Hour)
	due := tickAt.AddDate(0, 0, -3)

	deferred := primaryRowsTask("deferred-inbox", models.StateInbox)
	deferred.DeferredUntil = &wake
	deferred.DueDate = &due
	firstYesterday := primaryRowsTask("done-yesterday-1", models.StateDone)
	firstYesterday.CompletedAt = &beforeWake
	secondYesterday := primaryRowsTask("done-yesterday-2", models.StateDone)
	secondYesterday.CompletedAt = &beforeWake
	doneToday := primaryRowsTask("done-today", models.StateDone)
	doneToday.CompletedAt = &tickAt
	project := primaryRowsProject("cross-midnight-wake", deferred, firstYesterday, secondYesterday, doneToday)
	setTestAppProjects(model, project)
	installPulseRepository(t, project)

	repository := newTestAppRepo(project)
	cached, err := app.NewServiceWithArtifacts(repository, fixedCompletionActivityClock{now: beforeWake}, nil, nil).CompletionActivity()
	if err != nil {
		t.Fatalf("seed previous-day activity: %v", err)
	}
	refreshed, err := app.NewServiceWithArtifacts(repository, fixedCompletionActivityClock{now: tickAt}, nil, nil).CompletionActivity()
	if err != nil {
		t.Fatalf("seed current-day activity: %v", err)
	}
	reader := &staticCompletionActivityReader{activity: cached}
	model.features.Activity = ActivityCoordinator{reader: reader}
	model.todayIDs = map[TaskIdentity]bool{taskIdentityFor(deferred, project): true}

	model.refreshWorkspacePulseAt(beforeWake)
	pending := WorkspacePulse{Done: 2}
	if model.workspacePulse != pending {
		t.Fatalf("previous-day pending-deferral pulse = %+v, want %+v", model.workspacePulse, pending)
	}
	reader.activity = refreshed

	pushTestPresentationOverlay(model, ViewTaskComments)
	out.crossForceRebuild = false
	out.nextDeferWake = wake
	if handled, _ := model.handleGlobalSaveMessage(periodicSaveMsg{at: tickAt}); !handled {
		t.Fatal("periodicSaveMsg was not handled")
	}
	if out.crossForceRebuild || !out.nextDeferWake.Equal(wake) {
		t.Fatalf("a refused wake changed its clock: rebuild %t, wake %s", out.crossForceRebuild, out.nextDeferWake)
	}
	if !model.completionActivity.IsCurrentLocalDay(tickAt) {
		t.Fatal("the cross-midnight tick did not install the new local day's activity")
	}
	frozen := WorkspacePulse{Done: 1}
	if model.workspacePulse != frozen {
		t.Fatalf("cross-midnight refused-wake pulse = %+v, want %+v", model.workspacePulse, frozen)
	}

	setTestPresentationPath(model, ViewCrossProjectOutliner)
	if handled, _ := model.handleGlobalSaveMessage(periodicSaveMsg{at: tickAt}); !handled {
		t.Fatal("second periodicSaveMsg was not handled")
	}
	if !out.crossForceRebuild {
		t.Fatal("a safely consumed wake did not force the row rebuild")
	}
	if !out.nextDeferWake.IsZero() {
		t.Fatalf("the consumed wake clock survived: %s", out.nextDeferWake)
	}
	woken := WorkspacePulse{Inbox: 1, Today: 1, Overdue: 1, Done: 1}
	if model.workspacePulse != woken {
		t.Fatalf("cross-midnight consumed-wake pulse = %+v, want %+v", model.workspacePulse, woken)
	}
}

func TestDeferWakeIsDeferredWhileTheUserIsMidOperation(t *testing.T) {
	cases := []struct {
		name string
		arm  func(*AppModel)
	}{
		{"overlay", func(app *AppModel) { pushTestPresentationOverlay(app, ViewTaskComments) }},
		{"value picker", func(app *AppModel) { app.operationValuePicker = &OperationValuePickerModel{} }},
		{"inline row editor", func(app *AppModel) {
			out := app.crossOutliner
			out.quickEdit.Enter(out.loadedProjects[0].Tasks[0], false)
		}},
		{"visual selection", func(app *AppModel) {
			out := app.crossOutliner
			project := out.loadedProjects[0]
			out.ReplaceSelection([]TaskIdentity{taskIdentityFor(project.Tasks[0], project)})
		}},
		{"search projection", func(app *AppModel) { app.crossOutliner.beginSearchProjection() }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := typedTestApp(t)
			setTestPresentationPath(app, ViewCrossProjectOutliner)
			out := app.crossOutliner

			tc.arm(app)
			out.crossForceRebuild = false
			lapsed := time.Now().Add(-time.Minute)
			out.nextDeferWake = lapsed

			if handled, _ := app.handleGlobalSaveMessage(periodicSaveMsg{at: time.Now()}); !handled {
				t.Fatal("periodicSaveMsg was not handled")
			}
			if out.crossForceRebuild {
				t.Fatal("a lapsed deferral rebuilt the rows under a pending interaction")
			}
			if !out.nextDeferWake.Equal(lapsed) {
				t.Fatalf("a refused wake dropped its clock: %s", out.nextDeferWake)
			}
		})
	}
}

func TestNextDeferWakeSurvivesTheSearchProjectionReturn(t *testing.T) {
	app := typedTestApp(t)
	setTestPresentationPath(app, ViewCrossProjectOutliner)
	out := app.crossOutliner
	project := out.loadedProjects[0]

	wake := time.Now().Add(2 * time.Hour).Round(0)
	project.Tasks = append(project.Tasks, &models.Task{
		ID:            "deferred-task",
		ShortID:       project.NextShortID,
		Title:         "Deferred task",
		WorkflowState: models.StateBacklog,
		DeferredUntil: &wake,
	})
	project.NextShortID++
	project.BuildTaskTree()

	out.crossForceRebuild = true
	out.crossProjectRebuildRows()
	if !out.nextDeferWake.Equal(wake) {
		t.Fatalf("ordinary rebuild wake = %s, want %s", out.nextDeferWake, wake)
	}

	out.nextDeferWake = time.Time{}
	out.beginSearchProjection()
	if !out.nextDeferWake.Equal(wake) {
		t.Fatalf("search-projection rebuild wake = %s, want %s", out.nextDeferWake, wake)
	}
}
