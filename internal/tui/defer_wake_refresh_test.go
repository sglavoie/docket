package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/pulse"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/taskform"
	"github.com/sglavoie/docket/internal/tui/taskref"
	"github.com/sglavoie/docket/internal/tui/valuepicker"
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
			if got := chrome.FormatDeferDate(tc.deferred, now); got != tc.want {
				t.Fatalf("chrome.FormatDeferDate(%s) = %q, want %q", tc.deferred, got, tc.want)
			}
		})
	}
}

func TestPeriodicTickWakesOnlyPastTheDeferWake(t *testing.T) {
	app := typedTestApp(t)
	setTestPresentationPath(app, presentation.ViewCrossProjectOutliner)
	out := app.crossOutliner
	tickAt := time.Now()

	out.Population.ForceRebuild = false
	out.NextDeferWake = tickAt.Add(time.Hour)
	if handled, _ := app.handleGlobalSaveMessage(status.PeriodicSaveMsg{At: tickAt}); !handled {
		t.Fatal("periodicSaveMsg was not handled")
	}
	if out.Population.ForceRebuild {
		t.Fatal("a tick short of nextDeferWake forced a rebuild")
	}

	out.NextDeferWake = tickAt.Add(-time.Minute)
	if handled, _ := app.handleGlobalSaveMessage(status.PeriodicSaveMsg{At: tickAt}); !handled {
		t.Fatal("periodicSaveMsg was not handled")
	}
	if !out.Population.ForceRebuild {
		t.Fatal("a tick past nextDeferWake did not force a rebuild")
	}
	if !out.NextDeferWake.IsZero() {
		t.Fatal("the consumed wake clock survived the rebuild it armed")
	}
}

func TestPeriodicTickRefreshesWorkspacePulseWhenDeferredTaskWakes(t *testing.T) {
	model := typedTestApp(t)
	setTestPresentationPath(model, presentation.ViewCrossProjectOutliner)
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
	model.features.Activity = coordinator.NewActivityCoordinator(reader)
	model.todayIDs = map[taskref.TaskIdentity]bool{taskref.For(deferred, project): true}

	model.refreshWorkspacePulseAt(wake.Add(-time.Minute))
	pending := pulse.WorkspacePulse{Done: 1}
	if model.workspacePulse != pending {
		t.Fatalf("pending-deferral pulse = %+v, want %+v", model.workspacePulse, pending)
	}

	pushTestPresentationOverlay(model, presentation.ViewTaskComments)
	out.Population.ForceRebuild = false
	out.NextDeferWake = wake
	if handled, _ := model.handleGlobalSaveMessage(status.PeriodicSaveMsg{At: tickAt}); !handled {
		t.Fatal("periodicSaveMsg was not handled")
	}
	if out.Population.ForceRebuild || !out.NextDeferWake.Equal(wake) {
		t.Fatalf("a refused wake changed its clock: rebuild %t, wake %s", out.Population.ForceRebuild, out.NextDeferWake)
	}
	if model.workspacePulse != pending {
		t.Fatalf("a refused wake refreshed the pulse to %+v", model.workspacePulse)
	}

	setTestPresentationPath(model, presentation.ViewCrossProjectOutliner)
	if handled, _ := model.handleGlobalSaveMessage(status.PeriodicSaveMsg{At: tickAt}); !handled {
		t.Fatal("second periodicSaveMsg was not handled")
	}
	if !out.Population.ForceRebuild {
		t.Fatal("a safely consumed wake did not force the row rebuild")
	}
	if !out.NextDeferWake.IsZero() {
		t.Fatalf("the consumed wake clock survived: %s", out.NextDeferWake)
	}
	woken := pulse.WorkspacePulse{Inbox: 1, Today: 1, Overdue: 1, Done: 1}
	if model.workspacePulse != woken {
		t.Fatalf("same-day wake pulse = %+v, want %+v", model.workspacePulse, woken)
	}
}

func TestPeriodicTickAcrossMidnightHoldsActionableCountsUntilTheWakeIsConsumed(t *testing.T) {
	model := typedTestApp(t)
	setTestPresentationPath(model, presentation.ViewCrossProjectOutliner)
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
	model.features.Activity = coordinator.NewActivityCoordinator(reader)
	model.todayIDs = map[taskref.TaskIdentity]bool{taskref.For(deferred, project): true}

	model.refreshWorkspacePulseAt(beforeWake)
	pending := pulse.WorkspacePulse{Done: 2}
	if model.workspacePulse != pending {
		t.Fatalf("previous-day pending-deferral pulse = %+v, want %+v", model.workspacePulse, pending)
	}
	reader.activity = refreshed

	pushTestPresentationOverlay(model, presentation.ViewTaskComments)
	out.Population.ForceRebuild = false
	out.NextDeferWake = wake
	if handled, _ := model.handleGlobalSaveMessage(status.PeriodicSaveMsg{At: tickAt}); !handled {
		t.Fatal("periodicSaveMsg was not handled")
	}
	if out.Population.ForceRebuild || !out.NextDeferWake.Equal(wake) {
		t.Fatalf("a refused wake changed its clock: rebuild %t, wake %s", out.Population.ForceRebuild, out.NextDeferWake)
	}
	if !model.completionActivity.IsCurrentLocalDay(tickAt) {
		t.Fatal("the cross-midnight tick did not install the new local day's activity")
	}
	frozen := pulse.WorkspacePulse{Done: 1}
	if model.workspacePulse != frozen {
		t.Fatalf("cross-midnight refused-wake pulse = %+v, want %+v", model.workspacePulse, frozen)
	}

	setTestPresentationPath(model, presentation.ViewCrossProjectOutliner)
	if handled, _ := model.handleGlobalSaveMessage(status.PeriodicSaveMsg{At: tickAt}); !handled {
		t.Fatal("second periodicSaveMsg was not handled")
	}
	if !out.Population.ForceRebuild {
		t.Fatal("a safely consumed wake did not force the row rebuild")
	}
	if !out.NextDeferWake.IsZero() {
		t.Fatalf("the consumed wake clock survived: %s", out.NextDeferWake)
	}
	woken := pulse.WorkspacePulse{Inbox: 1, Today: 1, Overdue: 1, Done: 1}
	if model.workspacePulse != woken {
		t.Fatalf("cross-midnight consumed-wake pulse = %+v, want %+v", model.workspacePulse, woken)
	}
}

func TestDeferWakeIsDeferredWhileTheUserIsMidOperation(t *testing.T) {
	cases := []struct {
		name string
		arm  func(*AppModel)
	}{
		{"overlay", func(app *AppModel) { pushTestPresentationOverlay(app, presentation.ViewTaskComments) }},
		{"value picker", func(app *AppModel) { app.operations.ValuePicker = &valuepicker.OperationValuePickerModel{} }},
		{"check-in editor", func(app *AppModel) { app.operations.CheckInEditor = &taskform.CheckInEditorModel{} }},
		{"inline row editor", func(app *AppModel) {
			out := app.crossOutliner
			out.QuickEdit.Enter(out.Population.Projects[0].Tasks[0], false)
		}},
		{"visual selection", func(app *AppModel) {
			out := app.crossOutliner
			project := out.Population.Projects[0]
			out.ReplaceSelection([]taskref.TaskIdentity{taskref.For(project.Tasks[0], project)})
		}},
		{"ranked list", func(app *AppModel) {
			out := app.crossOutliner
			out.BeginSearchProjection()
			out.ToggleRankedList()
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := typedTestApp(t)
			setTestPresentationPath(app, presentation.ViewCrossProjectOutliner)
			out := app.crossOutliner

			tc.arm(app)
			out.Population.ForceRebuild = false
			lapsed := time.Now().Add(-time.Minute)
			out.NextDeferWake = lapsed

			if handled, _ := app.handleGlobalSaveMessage(status.PeriodicSaveMsg{At: time.Now()}); !handled {
				t.Fatal("periodicSaveMsg was not handled")
			}
			if out.Population.ForceRebuild {
				t.Fatal("a lapsed deferral rebuilt the rows under a pending interaction")
			}
			if !out.NextDeferWake.Equal(lapsed) {
				t.Fatalf("a refused wake dropped its clock: %s", out.NextDeferWake)
			}
		})
	}

	// A held query on its own is not a pending interaction. It narrows the
	// mode's own lanes and is re-answered on every rebuild, so the woken task is
	// matched against it rather than shut out by it.
	t.Run("held query alone", func(t *testing.T) {
		app := typedTestApp(t)
		setTestPresentationPath(app, presentation.ViewCrossProjectOutliner)
		out := app.crossOutliner

		out.BeginSearchProjection()
		out.Population.ForceRebuild = false
		out.NextDeferWake = time.Now().Add(-time.Minute)

		if handled, _ := app.handleGlobalSaveMessage(status.PeriodicSaveMsg{At: time.Now()}); !handled {
			t.Fatal("periodicSaveMsg was not handled")
		}
		if !out.Population.ForceRebuild {
			t.Fatal("a held query refused the wake, freezing an axis a query never owns")
		}
	})
}

func TestAttentionDayRefreshIsDeferredWhileTheUserIsMidOperation(t *testing.T) {
	cases := []struct {
		name string
		arm  func(*AppModel)
	}{
		{"overlay", func(app *AppModel) { pushTestPresentationOverlay(app, presentation.ViewTaskComments) }},
		{"value picker", func(app *AppModel) { app.operations.ValuePicker = &valuepicker.OperationValuePickerModel{} }},
		{"check-in editor", func(app *AppModel) { app.operations.CheckInEditor = &taskform.CheckInEditorModel{} }},
		{"inline row editor", func(app *AppModel) {
			out := app.crossOutliner
			out.QuickEdit.Enter(out.Population.Projects[0].Tasks[0], false)
		}},
		{"visual selection", func(app *AppModel) {
			out := app.crossOutliner
			project := out.Population.Projects[0]
			out.ReplaceSelection([]taskref.TaskIdentity{taskref.For(project.Tasks[0], project)})
		}},
		{"ranked list", func(app *AppModel) {
			out := app.crossOutliner
			out.BeginSearchProjection()
			out.ToggleRankedList()
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := typedTestApp(t)
			setTestPresentationPath(app, presentation.ViewCrossProjectOutliner)
			out := app.crossOutliner
			out.CrossProjectRebuildRows()
			tickAt := time.Now().AddDate(0, 0, 1)

			tc.arm(app)
			out.Population.ForceRebuild = false
			if handled, _ := app.handleGlobalSaveMessage(status.PeriodicSaveMsg{At: tickAt}); !handled {
				t.Fatal("periodicSaveMsg was not handled")
			}
			if out.Population.ForceRebuild {
				t.Fatal("day-boundary attention refresh rebuilt under a pending interaction")
			}
			if !out.TimeDrivenRefreshOwedAt(tickAt) {
				t.Fatal("refused day-boundary refresh was consumed")
			}
		})
	}
}

func TestPeriodicTickInvalidatesAttentionOnTheNextLocalDay(t *testing.T) {
	app := typedTestApp(t)
	setTestPresentationPath(app, presentation.ViewCrossProjectOutliner)
	out := app.crossOutliner
	out.CrossProjectRebuildRows()
	out.Population.ForceRebuild = false
	tickAt := time.Now().AddDate(0, 0, 1)

	if handled, _ := app.handleGlobalSaveMessage(status.PeriodicSaveMsg{At: tickAt}); !handled {
		t.Fatal("periodicSaveMsg was not handled")
	}
	if !out.Population.ForceRebuild {
		t.Fatal("next local day did not invalidate review filters, lane counts, and Peek")
	}
}

func TestNextDeferWakeSurvivesTheSearchProjectionReturn(t *testing.T) {
	app := typedTestApp(t)
	setTestPresentationPath(app, presentation.ViewCrossProjectOutliner)
	out := app.crossOutliner
	project := out.Population.Projects[0]

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

	out.Population.Invalidate()
	out.CrossProjectRebuildRows()
	if !out.NextDeferWake.Equal(wake) {
		t.Fatalf("ordinary rebuild wake = %s, want %s", out.NextDeferWake, wake)
	}

	out.NextDeferWake = time.Time{}
	out.BeginSearchProjection()
	if !out.NextDeferWake.Equal(wake) {
		t.Fatalf("search-projection rebuild wake = %s, want %s", out.NextDeferWake, wake)
	}
}
