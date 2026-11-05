package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/help"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/route"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// shellFeedbackText is the line the footer's feedback block would carry.
func shellFeedbackText(m *AppModel) string {
	if m.feedback.err != nil {
		return m.feedback.err.Error()
	}
	return m.feedback.message
}

// openPalette presses `:` and types query into the palette's input.
func openPalette(t *testing.T, driver *UITestDriver, query string) *help.CommandPaletteModel {
	t.Helper()
	pressMenuBarKey(t, driver, ":")
	palette := driver.App().commandPalette
	if palette == nil {
		t.Fatalf("`:` did not open the command palette:\n%s", driver.Render())
	}
	if query != "" {
		if _, err := driver.ApplyAction(UITestAction{Type: "text", Text: query}, ObserveOptions{}); err != nil {
			t.Fatal(err)
		}
		driver.drain()
	}
	return palette
}

func selectedPaletteLabel(t *testing.T, palette *help.CommandPaletteModel) string {
	t.Helper()
	item, ok := palette.Selected()
	if !ok {
		t.Fatal("the palette has no result under its cursor")
	}
	return item.Description
}

func TestCommandPaletteOpensOwnsGuidanceAndCloses(t *testing.T) {
	driver, _ := menuBarDriver(t)
	openPalette(t, driver, "")
	assertHelpScreen(t, driver, "Command Palette", "Search:", "Enter run", "Esc close", "↑/↓ choose", "Capture")
	if owner, _ := driver.App().activeGuidance(); owner != route.GuidanceOwnerModal {
		t.Fatalf("guidance owner = %v, want the palette's modal", owner)
	}
	pressMenuBarKey(t, driver, "esc")
	if driver.App().commandPalette != nil || driver.App().currentView() != presentation.ViewCrossProjectOutliner {
		t.Fatal("Esc did not close the palette back onto the Outliner")
	}

	// The Help menu's entry opens the same palette, and a row run from it acts.
	driver.dispatch(outliner.MenuBarActivateMsg{OperationID: operation.OperationCoreCommandPalette})
	driver.drain()
	if driver.App().commandPalette == nil {
		t.Fatal("Help → Command Palette did not open the palette")
	}
	if _, err := driver.ApplyAction(UITestAction{Type: "text", Text: "Go to Today"}, ObserveOptions{}); err != nil {
		t.Fatal(err)
	}
	pressMenuBarKey(t, driver, "enter")
	if out := driver.App().crossOutliner; driver.App().commandPalette != nil || out.PrimaryMode != chrome.ModeToday {
		t.Fatal("a row run from the menu-opened palette did nothing")
	}
}

func TestCommandPaletteEmptyQueryLeadsWithTheFooterLadder(t *testing.T) {
	driver, out := menuBarDriver(t)
	seatCursorOnTaskTitled(t, out, "Triage support inbox")
	palette := openPalette(t, driver, "")
	if got := selectedPaletteLabel(t, palette); got != "Capture" {
		t.Fatalf("empty palette leads with %q, want the footer's first rung", got)
	}
	pressMenuBarKey(t, driver, "down", "ctrl+n", "ctrl+p", "up")
	if got := selectedPaletteLabel(t, palette); got != "Capture" {
		t.Fatalf("up/down and Ctrl+N/Ctrl+P did not return to the head: %q", got)
	}
	if _, err := driver.ApplyAction(UITestAction{Type: "text", Text: "cmplt tsk"}, ObserveOptions{}); err != nil {
		t.Fatal(err)
	}
	driver.drain()
	if got := selectedPaletteLabel(t, palette); got != "Complete Task" {
		t.Fatalf("fuzzy query selected %q, want Complete Task", got)
	}
	assertHelpScreen(t, driver, "wd", "Target:")
}

func TestCommandPaletteRunsDirectChordPickerAndMenuOperations(t *testing.T) {
	t.Run("direct key", func(t *testing.T) {
		driver, out := menuBarDriver(t)
		openPalette(t, driver, "Go to Today")
		pressMenuBarKey(t, driver, "enter")
		if driver.App().commandPalette != nil || out.PrimaryMode != chrome.ModeToday {
			t.Fatalf("Go to Today did not run: mode %v", out.PrimaryMode)
		}
	})
	t.Run("chord preset commits and undoes", func(t *testing.T) {
		driver, out := menuBarDriver(t)
		seatCursorOnTaskTitled(t, out, "Triage support inbox")
		task, project := out.GetTaskRef()
		openPalette(t, driver, "Complete Task")
		pressMenuBarKey(t, driver, "enter")
		loaded := *project
		if err := storage.LoadProject(&loaded); err != nil || loaded.GetTaskByID(task.ID).State() != models.StateDone {
			t.Fatalf("Complete Task did not commit Done: %v", err)
		}
		pressMenuBarKey(t, driver, "u")
		if err := storage.LoadProject(&loaded); err != nil || loaded.GetTaskByID(task.ID).State() != models.StateInbox {
			t.Fatalf("the ordinary undo did not restore Inbox: %v", err)
		}
	})
	t.Run("picker keeps its target", func(t *testing.T) {
		driver, out := menuBarDriver(t)
		seatCursorOnTaskTitled(t, out, "Triage support inbox")
		task, project := out.GetTaskRef()
		openPalette(t, driver, "defer")
		pressMenuBarKey(t, driver, "enter")
		picker := driver.App().operations.ValuePicker
		if picker == nil || driver.App().commandPalette != nil {
			t.Fatal("the scheduling picker did not replace the palette")
		}
		request := picker.ExecutionRequest()
		if request.ID != operation.OperationTaskSetDueDate ||
			operation.CurrentTaskIdentity(request.Context) != (taskref.TaskIdentity{ProjectID: project.ID, TaskID: task.ID}) {
			t.Fatalf("wrong scheduling request: %+v", request)
		}
		assertHelpScreen(t, driver, "DUE", "Tab switch field")
	})
	t.Run("keyless menu operation", func(t *testing.T) {
		driver, out := menuBarDriver(t)
		seatCursorOnTaskTitled(t, out, "Triage support inbox")
		pressMenuBarKey(t, driver, "home", "h")
		if !strings.Contains(driver.Render(), "▸") {
			t.Fatalf("h on the lane heading folded nothing:\n%s", driver.Render())
		}
		openPalette(t, driver, "Clear All Folds")
		assertHelpScreen(t, driver, "F10 → View")
		pressMenuBarKey(t, driver, "enter")
		assertHelpScreen(t, driver, "All folds cleared")
	})
}

func TestCommandPaletteRefusalsMatchTheirKeys(t *testing.T) {
	t.Run("handler refusal is the key's status", func(t *testing.T) {
		driver, _ := menuBarDriver(t)
		pressMenuBarKey(t, driver, "u")
		byKey := shellFeedbackText(driver.App())
		if byKey == "" {
			t.Fatal("u on an empty ledger said nothing, so the comparison proves nothing")
		}
		pressMenuBarKey(t, driver, "esc")
		driver.App().clearStatus()
		openPalette(t, driver, "Undo")
		pressMenuBarKey(t, driver, "enter")
		if got := shellFeedbackText(driver.App()); got != byKey {
			t.Fatalf("palette Undo said %q, the key said %q", got, byKey)
		}
	})
	t.Run("unavailable row stays open with its reason", func(t *testing.T) {
		driver, _ := menuBarDriver(t)
		driver.Resize(60, 20)
		pressMenuBarKey(t, driver, "home") // A lane heading names no task.
		openPalette(t, driver, "Set Due, Defer, or Check-In")
		assertHelpScreen(t, driver, "Choose a task to use this action.")
		pressMenuBarKey(t, driver, "enter")
		if driver.App().commandPalette == nil {
			t.Fatal("Enter on an unavailable row closed the palette")
		}
	})
	t.Run("changed target and stale delivery", func(t *testing.T) {
		driver, out := menuBarDriver(t)
		seatCursorOnTaskTitled(t, out, "Triage support inbox")
		palette := openPalette(t, driver, "Complete Task")
		key, err := parseUITestKey("enter")
		if err != nil {
			t.Fatal(err)
		}
		_, _, message := palette.HandleKey(key)
		run, ok := message.(help.RunPaletteActionMsg)
		if !ok {
			t.Fatalf("Enter did not request the row: %T", message)
		}
		seatCursorOnTaskTitled(t, out, "Fix redirect loop after logout")
		driver.dispatch(run)
		driver.drain()
		assertHelpScreen(t, driver, "The task or selection changed.")
		pressMenuBarKey(t, driver, "esc", ":")
		reopened := driver.App().commandPalette
		driver.dispatch(run)
		driver.drain()
		if driver.App().commandPalette != reopened {
			t.Fatal("a request from a closed palette ran against a newer one")
		}
	})
}

func TestCommandPaletteKeepsVisualSelection(t *testing.T) {
	driver, out := menuBarDriver(t)
	seatCursorOnTaskTitled(t, out, "Triage support inbox")
	pressMenuBarKey(t, driver, "v")
	seatCursorOnTaskTitled(t, out, "Fix redirect loop after logout")
	pressMenuBarKey(t, driver, "v", "g", "i")
	selected := out.SelectedTaskIdentities()
	if len(selected) != 2 {
		t.Fatalf("fixture must hold a two-task selection: %+v", selected)
	}
	openPalette(t, driver, "Set Tags & Context")
	assertHelpScreen(t, driver, "Target: 2 selected tasks (1 hidden)")
	pressMenuBarKey(t, driver, "enter")
	picker := driver.App().operations.ValuePicker
	if picker == nil {
		t.Fatal("the palette did not open tags")
	}
	request := picker.ExecutionRequest()
	if request.ID != operation.OperationTaskSetTags || !request.Context.Selection.Visual ||
		!slices.Equal(request.Context.Selection.Tasks, selected) {
		t.Fatalf("the visual selection was lost: %+v", request)
	}
}

func TestColonStaysTextInEveryInput(t *testing.T) {
	typeColon := func(t *testing.T, driver *UITestDriver) {
		t.Helper()
		if _, err := driver.ApplyAction(UITestAction{Type: "text", Text: "a:b"}, ObserveOptions{}); err != nil {
			t.Fatal(err)
		}
		driver.drain()
		if driver.App().commandPalette != nil {
			t.Fatal("`:` opened the palette over an input that owns it")
		}
		if !strings.Contains(driver.Render(), "a:b") {
			t.Fatalf("the colon was not typed:\n%s", driver.Render())
		}
	}
	t.Run("search", func(t *testing.T) {
		driver, _ := menuBarDriver(t)
		pressMenuBarKey(t, driver, "/")
		typeColon(t, driver)
	})
	t.Run("capture", func(t *testing.T) {
		driver, _ := menuBarDriver(t)
		pressMenuBarKey(t, driver, "a")
		typeColon(t, driver)
	})
	t.Run("text prompt", func(t *testing.T) {
		driver, out := menuBarDriver(t)
		seatCursorOnTaskTitled(t, out, "Triage support inbox")
		pressMenuBarKey(t, driver, "r")
		if driver.App().operations.TextPrompt == nil {
			t.Fatal("r did not open the resume-note prompt")
		}
		typeColon(t, driver)
	})
	t.Run("palette input", func(t *testing.T) {
		driver, _ := menuBarDriver(t)
		openPalette(t, driver, "a:b")
		if !strings.Contains(driver.Render(), "Search: a:b") {
			t.Fatalf("the colon was not typed into the palette:\n%s", driver.Render())
		}
	})
}

func TestCommandPaletteFitsTheTerminalFloor(t *testing.T) {
	driver, out := menuBarDriver(t)
	driver.Resize(chrome.MinTerminalWidth, chrome.MinTerminalHeight)
	seatCursorOnTaskTitled(t, out, "Triage support inbox")
	openPalette(t, driver, "")
	assertHelpScreen(t, driver, "Command Palette", "Enter run", "Esc close", "↑/↓ choose")
	pressMenuBarKey(t, driver, "pgdown", "pgdown", "down")
	assertHelpScreen(t, driver, "Enter run")
}

func TestHelpListsTheCommandPalette(t *testing.T) {
	driver, out := menuBarDriver(t)
	ctx := out.OperationCommandContext()
	reference := driver.App().coreHelpItemsForContext(ctx, false)
	if !slices.ContainsFunc(reference, func(item operation.HelpItem) bool {
		return item.Key == ":" && item.Description == "Command Palette"
	}) {
		t.Fatal("`?` help does not list `:` Command Palette")
	}
	actions := driver.App().outlinerActionItems(ctx)
	for _, keyless := range []operation.OperationID{
		operation.OperationCoreClearFolds, operation.OperationCoreToggleMotion, operation.OperationCoreSetUpCheckIns,
	} {
		if !slices.ContainsFunc(actions, func(item operation.HelpItem) bool {
			return item.Action != nil && item.Action.ID == keyless
		}) {
			t.Errorf("the action list omits keyless %s", keyless)
		}
	}
}
