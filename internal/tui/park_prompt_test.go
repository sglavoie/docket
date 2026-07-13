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
	cmd, prompted := app.maybePromptPark(target.ID, func() tea.Cmd {
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
	if app.currentView() != ViewProjectTree {
		t.Fatalf("currentView after skip = %v, want ViewProjectTree", app.currentView())
	}
	if !wasParkPrompted(source.ID) {
		t.Fatal("Esc skip should mark the task prompted")
	}

	_, prompted = app.maybePromptPark(target.ID, func() tea.Cmd {
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
		target func(source, target *models.Task) string
		mutate func(source *models.Task, mutation sessionTaskMutation)
	}{
		{
			name: "same task target",
			target: func(source, _ *models.Task) string {
				return source.ID
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
			mutate: func(source *models.Task, _ sessionTaskMutation) {
				markParkPrompted(source.ID)
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

			targetID := target.ID
			if tt.target != nil {
				targetID = tt.target(source, target)
			}
			continued := 0
			_, prompted := app.maybePromptPark(targetID, func() tea.Cmd {
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
	app.config.UndoHistoryDepth = &depth
	markSessionTaskMutated(project.ID, []string{source.ID})

	continued := 0
	_, prompted := app.maybePromptPark(target.ID, func() tea.Cmd {
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

	if app.currentView() != ViewProjectTree {
		t.Fatalf("currentView after skip = %v, want ViewProjectTree", app.currentView())
	}
	selected := app.projectTree.getSelectedTask()
	if selected == nil || selected.ID != target.ID {
		t.Fatalf("selected task = %v, want target %q", selected, target.ID)
	}
}

func TestNavigateHistoryBypassesParkPrompt(t *testing.T) {
	app, project, source, target := newParkPromptTestApp(t)
	markSessionTaskMutated(project.ID, []string{source.ID})

	cmd := app.navigateToEntry(NavEntry{
		ViewType:  ViewTaskChecklist,
		ProjectID: project.ID,
		TaskID:    target.ID,
	})
	if cmd != nil {
		_ = cmd()
	}
	if app.currentView() == ViewParkInput {
		t.Fatal("nav-history reconstruction should bypass the smart park prompt")
	}
	if app.currentView() != ViewTaskChecklist {
		t.Fatalf("currentView = %v, want ViewTaskChecklist", app.currentView())
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

	app.config.CurrentProject = project.ID
	app.currentProject = project
	app.lastProjectView = ViewProjectTree
	app.projectLastView[project.ID] = ViewProjectTree
	app.projectTree = NewProjectTreeModel(project, app.config, app.configManager)
	app.projectTree.SetSize(app.width, app.height-2)
	app.views.Register(ViewProjectTree, app.projectTree)
	app.viewStack = []ViewType{ViewDashboard, ViewProjectTree}

	return app, project, source, target
}
