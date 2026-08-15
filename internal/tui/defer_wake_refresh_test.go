package tui

import (
	"testing"
	"time"

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

	out.crossForceRebuild = false
	out.nextDeferWake = time.Now().Add(time.Hour)
	if handled, _ := app.handleGlobalSaveMessage(periodicSaveMsg{}); !handled {
		t.Fatal("periodicSaveMsg was not handled")
	}
	if out.crossForceRebuild {
		t.Fatal("a tick short of nextDeferWake forced a rebuild")
	}

	out.nextDeferWake = time.Now().Add(-time.Minute)
	if handled, _ := app.handleGlobalSaveMessage(periodicSaveMsg{}); !handled {
		t.Fatal("periodicSaveMsg was not handled")
	}
	if !out.crossForceRebuild {
		t.Fatal("a tick past nextDeferWake did not force a rebuild")
	}
	if !out.nextDeferWake.IsZero() {
		t.Fatal("the consumed wake clock survived the rebuild it armed")
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

			if handled, _ := app.handleGlobalSaveMessage(periodicSaveMsg{}); !handled {
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
