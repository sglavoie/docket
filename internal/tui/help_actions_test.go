package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/help"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

func searchOutlinerHelp(t *testing.T, driver *UITestDriver, query string) {
	t.Helper()
	pressMenuBarKey(t, driver, "?", "backspace", "/")
	if _, err := driver.ApplyAction(UITestAction{Type: "text", Text: query}, ObserveOptions{}); err != nil {
		t.Fatal(err)
	}
	driver.drain()
}

func assertHelpScreen(t *testing.T, driver *UITestDriver, want ...string) {
	t.Helper()
	obs := driver.Observe(ObserveOptions{})
	if len(obs.LayoutIssues) > 0 {
		t.Fatalf("layout issues: %+v\n%s", obs.LayoutIssues, obs.PlainScreen)
	}
	for _, text := range want {
		if !strings.Contains(obs.PlainScreen, text) {
			t.Fatalf("missing %q:\n%s", text, obs.PlainScreen)
		}
	}
}

func TestHelpActionSchedulingFromSearchAtTerminalFloor(t *testing.T) {
	driver, out := menuBarDriver(t)
	driver.Resize(60, 20)
	seatCursorOnTaskTitled(t, out, "Triage support inbox")
	task, project := out.GetTaskRef()
	identity := taskref.TaskIdentity{ProjectID: project.ID, TaskID: task.ID}
	searchOutlinerHelp(t, driver, "defer")
	assertHelpScreen(t, driver, "Target:", "Triage support inbox", "Set Due, Defer, or Check-In", "Enter run", "↑/↓ choose")
	pressMenuBarKey(t, driver, "enter")
	picker := driver.App().operations.ValuePicker
	if picker == nil {
		t.Fatalf("search did not open scheduling: %T\n%s", driver.App().activeView(), driver.Render())
	}
	request := picker.ExecutionRequest()
	if request.ID != operation.OperationTaskSetDueDate || operation.CurrentTaskIdentity(request.Context) != identity {
		t.Fatalf("wrong scheduling request: %+v", request)
	}
	assertHelpScreen(t, driver, "Set Due, Defer, or Check-In", "DUE", "Tab switch field")
	pressMenuBarKey(t, driver, "tab")
	assertHelpScreen(t, driver, "DEFER", "Tab switch field")
	pressMenuBarKey(t, driver, "esc")
	if driver.App().currentView() != presentation.ViewCrossProjectOutliner || cursorTaskTitle(t, out) != task.Title {
		t.Fatal("canceling scheduling did not return to the original task")
	}
}

func TestHelpActionWorkflowPresetCommitsAndUndoes(t *testing.T) {
	driver, out := menuBarDriver(t)
	seatCursorOnTaskTitled(t, out, "Triage support inbox")
	task, project := out.GetTaskRef()
	searchOutlinerHelp(t, driver, "Complete Task")
	assertHelpScreen(t, driver, "wd", "Target:", task.Title)
	pressMenuBarKey(t, driver, "enter")
	loaded := *project
	err := storage.LoadProject(&loaded)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.GetTaskByID(task.ID).State() != models.StateDone {
		t.Fatal("running Complete Task did not commit its Done preset")
	}
	pressMenuBarKey(t, driver, "u")
	err = storage.LoadProject(&loaded)
	if err != nil || loaded.GetTaskByID(task.ID).State() != models.StateInbox {
		t.Fatalf("the normal undo did not restore Inbox: %v", err)
	}
}

func TestHelpActionBulkTagsPreservesSelectionIncludingHiddenTasks(t *testing.T) {
	driver, out := menuBarDriver(t)
	seatCursorOnTaskTitled(t, out, "Triage support inbox")
	pressMenuBarKey(t, driver, "v")
	seatCursorOnTaskTitled(t, out, "Fix redirect loop after logout")
	pressMenuBarKey(t, driver, "v", "g", "i")
	selected := out.SelectedTaskIdentities()
	if len(selected) != 2 || out.SelectionCounts().Hidden == 0 {
		t.Fatalf("fixture must retain a partly hidden selection: %+v", out.SelectionCounts())
	}
	searchOutlinerHelp(t, driver, "Set Tags & Context")
	assertHelpScreen(t, driver, "Target: 2 selected tasks (1 hidden)", "Enter run")
	pressMenuBarKey(t, driver, "enter")
	picker := driver.App().operations.ValuePicker
	if picker == nil {
		t.Fatalf("search did not open tags: %T", driver.App().activeView())
	}
	request := picker.ExecutionRequest()
	if request.ID != operation.OperationTaskSetTags || !request.Context.Selection.Visual || !slices.Equal(request.Context.Selection.Tasks, selected) {
		t.Fatalf("the original bulk selection was lost: %+v", request)
	}
}

func TestHelpActionUnavailableAndReferenceResultsStayInHelp(t *testing.T) {
	driver, _ := menuBarDriver(t)
	driver.Resize(60, 20)
	pressMenuBarKey(t, driver, "home") // A lane heading names no task.
	searchOutlinerHelp(t, driver, "Set Due, Defer, or Check-In")
	assertHelpScreen(t, driver, "Choose a task to use this action.", "No task selected")
	pressMenuBarKey(t, driver, "enter")
	if driver.App().currentView() != presentation.ViewHelp {
		t.Fatal("an unavailable action closed Help")
	}
	pressMenuBarKey(t, driver, "esc", "backspace", "/")
	if _, err := driver.ApplyAction(UITestAction{Type: "text", Text: "@@none"}, ObserveOptions{}); err != nil {
		t.Fatal(err)
	}
	driver.drain()
	assertHelpScreen(t, driver, "Reference only:", "@@defer")
	pressMenuBarKey(t, driver, "enter")
	if driver.App().currentView() != presentation.ViewHelp {
		t.Fatal("grammar prose was executed as an action")
	}
}

func TestHelpActionRefusesChangedTargetsAndStaleDelivery(t *testing.T) {
	driver, out := menuBarDriver(t)
	seatCursorOnTaskTitled(t, out, "Triage support inbox")
	searchOutlinerHelp(t, driver, "Complete Task")
	key, err := parseUITestKey("enter")
	if err != nil {
		t.Fatal(err)
	}
	_, cmd := driver.App().helpView.Update(key)
	if cmd == nil {
		t.Fatal("Enter did not request an action")
	}
	message := cmd().(help.RunActionMsg)
	seatCursorOnTaskTitled(t, out, "Fix redirect loop after logout")
	driver.dispatch(message)
	driver.drain()
	assertHelpScreen(t, driver, "The task or selection changed.")
	if driver.App().currentView() != presentation.ViewHelp {
		t.Fatal("a changed target closed Help")
	}
	pressMenuBarKey(t, driver, "esc", "?", "?")
	reopened := driver.App().helpView
	driver.dispatch(message)
	driver.drain()
	if driver.App().currentView() != presentation.ViewHelp || driver.App().helpView != reopened {
		t.Fatal("an old action affected a newly opened Help instance")
	}
}

func TestHelpActionMenuEntriesKeepTheirEditorsAndConfirmations(t *testing.T) {
	t.Run("compressed priority opens its picker", func(t *testing.T) {
		driver, out := menuBarDriver(t)
		seatCursorOnTaskTitled(t, out, "Fix redirect loop after logout")
		searchOutlinerHelp(t, driver, "Set Priority")
		pressMenuBarKey(t, driver, "enter")
		picker := driver.App().operations.ValuePicker
		if picker == nil || picker.ExecutionRequest().ID != operation.OperationTaskSetPriority || picker.ExecutionRequest().HasValue {
			t.Fatal("the compressed priority family ran a preset instead of opening its picker")
		}
	})
	t.Run("check-in editor", func(t *testing.T) {
		driver, out := menuBarDriver(t)
		seatCursorOnTaskTitled(t, out, "Triage support inbox")
		searchOutlinerHelp(t, driver, "Set Check-In")
		assertHelpScreen(t, driver, "Space", "Set Check-In", "Triage support inbox")
		pressMenuBarKey(t, driver, "enter")
		if driver.App().operations.CheckInEditor == nil {
			t.Fatal("Set Check-In did not open its ordinary editor")
		}
	})
	t.Run("delete confirmation", func(t *testing.T) {
		driver, out := menuBarDriver(t)
		seatCursorOnTaskTitled(t, out, "Triage support inbox")
		task, project := out.GetTaskRef()
		searchOutlinerHelp(t, driver, "Delete")
		pressMenuBarKey(t, driver, "enter")
		if driver.App().renderConfirmDialogOverlay() == "" {
			t.Fatal("Delete bypassed its confirmation")
		}
		pressMenuBarKey(t, driver, "esc")
		loaded := *project
		if err := storage.LoadProject(&loaded); err != nil || loaded.GetTaskByID(task.ID) == nil {
			t.Fatalf("canceling Delete removed the task: %v", err)
		}
		searchOutlinerHelp(t, driver, "Delete")
		pressMenuBarKey(t, driver, "enter", "y")
		if err := storage.LoadProject(&loaded); err != nil || loaded.GetTaskByID(task.ID) != nil {
			t.Fatalf("confirming Delete did not remove the intended task: %v", err)
		}
		pressMenuBarKey(t, driver, "u")
		if err := storage.LoadProject(&loaded); err != nil || loaded.GetTaskByID(task.ID) == nil {
			t.Fatalf("undo did not restore the deleted task: %v", err)
		}
	})
	t.Run("project deletion keeps its target and confirmation", func(t *testing.T) {
		driver, out := menuBarDriver(t)
		onProjectHeaderCursor(t, out, func() {
			searchOutlinerHelp(t, driver, "Delete Project")
			assertHelpScreen(t, driver, "Target: Project: Rich Project", "Space", "Enter run")
			pressMenuBarKey(t, driver, "enter")
			if driver.App().renderConfirmDialogOverlay() == "" {
				t.Fatal("Delete Project bypassed its confirmation")
			}
			pressMenuBarKey(t, driver, "esc")
			if driver.App().currentView() != presentation.ViewCrossProjectOutliner {
				t.Fatal("canceling project deletion did not return to the Outliner")
			}
		})
	})
}
