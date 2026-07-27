package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func TestMaybePromptParkEscSkipsAndDoesNotReprompt(t *testing.T) {
	app, project, source, target := newParkPromptTestApp(t)
	markSessionTaskMutated(project.ID, []string{source.ID})

	continued := 0
	cmd, prompted := app.maybePromptPark(taskIdentityFor(target, project), func() tea.Cmd {
		continued++
		return nil
	})
	if !prompted {
		t.Fatal("maybePromptPark should prompt before navigating away from the mutated task")
	}
	if cmd != nil {
		t.Fatalf("prompting should not return a command, got %T", cmd())
	}
	if app.currentView() != ViewParkInput {
		t.Fatalf("currentView = %v, want ViewParkInput", app.currentView())
	}

	parkInput := app.views.Get(ViewParkInput).(*ParkInputModel)
	if !parkInput.promptMode {
		t.Fatal("smart prompt should open ParkInputModel in prompt mode")
	}
	parkInput.input.SetValue("draft note")
	_, doneCmd := parkInput.Update(syntheticSpecialKey(tea.KeyEsc))
	doneMsg, ok := doneCmd().(ParkNoteDoneMsg)
	if !ok {
		t.Fatalf("Esc emitted %T, want ParkNoteDoneMsg", doneCmd())
	}
	if !doneMsg.Skipped {
		t.Fatal("prompt-mode Esc should skip immediately")
	}

	handled, _ := app.handleTaskMessages(doneMsg)
	if !handled {
		t.Fatal("ParkNoteDoneMsg should be handled")
	}
	if continued != 1 {
		t.Fatalf("continuation calls = %d, want 1", continued)
	}
	if app.currentView() != ViewCrossProjectOutliner {
		t.Fatalf("currentView after skip = %v, want ViewCrossProjectOutliner", app.currentView())
	}
	if !wasParkPrompted(taskIdentityFor(source, project)) {
		t.Fatal("Esc skip should mark the task prompted")
	}

	_, prompted = app.maybePromptPark(taskIdentityFor(target, project), func() tea.Cmd {
		continued++
		return nil
	})
	if prompted {
		t.Fatal("task should not be prompted twice in the same TUI session")
	}
	if continued != 2 {
		t.Fatalf("continuation calls after no-prompt path = %d, want 2", continued)
	}
}

func TestMaybePromptParkSkipMatrix(t *testing.T) {
	tests := []struct {
		name   string
		target func(source, target *models.Task) TaskIdentity
		mutate func(source *models.Task, mutation sessionTaskMutation)
	}{
		{
			name: "same task target",
			target: func(source, _ *models.Task) TaskIdentity {
				return taskIdentityFor(source, source.Project)
			},
		},
		{
			name: "completed task",
			mutate: func(source *models.Task, _ sessionTaskMutation) {
				source.WorkflowState = models.StateDone
			},
		},
		{
			name: "parked since mutation",
			mutate: func(source *models.Task, mutation sessionTaskMutation) {
				parkedAt := mutation.LastMutated.Add(1)
				source.ParkedAt = &parkedAt
			},
		},
		{
			name: "already prompted",
			mutate: func(source *models.Task, mutation sessionTaskMutation) {
				markParkPrompted(TaskIdentity{ProjectID: mutation.ProjectID, TaskID: source.ID})
			},
		},
		{
			name: "missing task",
			mutate: func(source *models.Task, _ sessionTaskMutation) {
				source.Project.Tasks = nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, project, source, target := newParkPromptTestApp(t)
			markSessionTaskMutated(project.ID, []string{source.ID})
			mutation, ok := lastSessionMutation()
			if !ok {
				t.Fatal("expected test mutation")
			}
			if tt.mutate != nil {
				tt.mutate(source, mutation)
				project.BuildTaskTree()
			}

			targetIdentity := taskIdentityFor(target, project)
			if tt.target != nil {
				targetIdentity = tt.target(source, target)
			}
			continued := 0
			_, prompted := app.maybePromptPark(targetIdentity, func() tea.Cmd {
				continued++
				return nil
			})
			if prompted {
				t.Fatal("maybePromptPark should not prompt for this skip case")
			}
			if continued != 1 {
				t.Fatalf("continuation calls = %d, want 1", continued)
			}
			if app.currentView() == ViewParkInput {
				t.Fatal("skip case should not open the park input")
			}
		})
	}
}

func TestPromptParkEnterSavesSynchronouslyBeforeContinuation(t *testing.T) {
	app, project, source, target := newParkPromptTestApp(t)
	depth := 10
	app.runtime.Settings().UndoHistoryDepth = &depth
	markSessionTaskMutated(project.ID, []string{source.ID})

	continued := 0
	_, prompted := app.maybePromptPark(taskIdentityFor(target, project), func() tea.Cmd {
		continued++
		return nil
	})
	if !prompted {
		t.Fatal("expected park prompt")
	}

	parkInput := app.views.Get(ViewParkInput).(*ParkInputModel)
	parkInput.input.SetValue("resume after review")
	_, doneCmd := parkInput.Update(syntheticSpecialKey(tea.KeyEnter))
	doneMsg := doneCmd().(ParkNoteDoneMsg)

	handled, cmd := app.handleTaskMessages(doneMsg)
	if !handled {
		t.Fatal("ParkNoteDoneMsg should be handled")
	}
	if cmd != nil {
		for _, msg := range runBulkPickerBatchCmd(t, cmd) {
			if _, ok := msg.(SaveCompleteMsg); ok {
				t.Fatal("prompt-mode Enter should use a sync save, not an async SaveCompleteMsg")
			}
		}
	}
	if continued != 1 {
		t.Fatalf("continuation calls = %d, want 1", continued)
	}

	reloaded := loadBulkPickerTestProject(t, project)
	reloadedSource := reloaded.GetTaskByID(source.ID)
	if reloadedSource == nil {
		t.Fatal("source task missing after reload")
	}
	if got := reloadedSource.ParkedNote; got != "resume after review" {
		t.Fatalf("persisted parked note = %q, want %q", got, "resume after review")
	}
}

func TestNavigateToTaskPromptsThenEscLandsOnTarget(t *testing.T) {
	app, project, source, target := newParkPromptTestApp(t)
	markSessionTaskMutated(project.ID, []string{source.ID})

	cmd := app.navigateToTaskInProject(target, project)
	if cmd != nil {
		t.Fatalf("prompt path should not return navigation command, got %T", cmd())
	}
	if app.currentView() != ViewParkInput {
		t.Fatalf("currentView = %v, want ViewParkInput", app.currentView())
	}

	parkInput := app.views.Get(ViewParkInput).(*ParkInputModel)
	_, doneCmd := parkInput.Update(syntheticSpecialKey(tea.KeyEsc))
	doneMsg := doneCmd().(ParkNoteDoneMsg)
	handled, _ := app.handleTaskMessages(doneMsg)
	if !handled {
		t.Fatal("ParkNoteDoneMsg should be handled")
	}

	if app.currentView() != ViewCrossProjectOutliner {
		t.Fatalf("currentView after skip = %v, want ViewCrossProjectOutliner", app.currentView())
	}
	if got := app.crossOutliner.scopedProjectID; got != project.ID {
		t.Fatalf("scopedProjectID after skip = %q, want %q", got, project.ID)
	}
	selected := app.crossOutliner.getCursorTask()
	if selected == nil || selected.ID != target.ID {
		t.Fatalf("selected task = %v, want target %q", selected, target.ID)
	}
}

// newParkPromptTestApp builds a disk-backed AppModel wired to the application
// service (park now commits through the workspace ledger, not the per-project
// .undo snapshot). The single project lives under a shared data dir so the
// transactional commit service's root containment accepts the park write.
func newParkPromptTestApp(t *testing.T) (*AppModel, *models.Project, *models.Task, *models.Task) {
	t.Helper()
	resetSessionMutations()
	t.Cleanup(resetSessionMutations)

	source := newBulkPickerTestTask(t, 1, "source task")
	target := newBulkPickerTestTask(t, 2, "target task")
	env := newAppBulkPickerEnv(t, []*models.Task{source, target})
	app := env.appModel()
	project := env.projects[0]
	source = project.Tasks[0]
	target = project.Tasks[1]
	// BuildTaskTree does not set Task.Project; the skip-matrix cases reach through
	// source.Project, so wire it explicitly here.
	for _, tk := range project.Tasks {
		tk.Project = project
	}

	setTestCurrentProject(app, project.ID)
	app.currentProject = project
	// The single authoritative Outliner is the cross-project instance; project
	// selection is a scope frame on it, so wire it as the sole root here.
	cross := NewOutlinerModel(nil, app.configManager, app.runtime)
	cross.loadedProjects = []*models.Project{project}
	cross.SetSize(app.width, app.height-2)
	app.crossOutliner = cross
	app.views.Register(ViewCrossProjectOutliner, cross)
	setTestPresentationPath(app, ViewDashboard, ViewCrossProjectOutliner)

	return app, project, source, target
}
