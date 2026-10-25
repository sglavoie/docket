package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// Walk the actual dropdown, including its headings, then dispatch Enter through
// the shell. This catches menus that render an action but cannot execute it.
func choosePolishTaskMenuEntry(t *testing.T, driver *UITestDriver, view *outliner.OutlinerModel, id operation.OperationID) {
	t.Helper()
	pressMenuBarKey(t, driver, "f10")
	menu := menuWithID(t, openMenuBarMenusForTest(t, view), outliner.MenuIDTask)
	for steps := 0; steps < len(menu.Entries); steps++ {
		if menu.Entries[view.MenuBar.ItemIndex].OperationID == id {
			pressMenuBarKey(t, driver, "enter")
			return
		}
		pressMenuBarKey(t, driver, "down")
	}
	t.Fatalf("Task menu did not reach %s", id)
}

func TestDailyPolishTaskMenuPlansAndOpensExistingPickers(t *testing.T) {
	driver, _, view := checklistFixtureApp(t)
	seatCursorOnTaskTitled(t, view, checklistFixturePlainTitle)
	choosePolishTaskMenuEntry(t, driver, view, operation.OperationCoreToday)
	task, project := workspaceTaskTitled(t, driver.App(), checklistFixturePlainTitle)
	if !todayContains(driver.App().domain.runtime.Planning(), taskref.TaskIdentity{ProjectID: project, TaskID: task.ID}) {
		t.Fatal("Today menu action did not commit planning membership")
	}
	pressMenuBarKey(t, driver, "u")
	seatCursorOnTaskTitled(t, view, checklistFixturePlainTitle)
	for _, tc := range []struct {
		id   operation.OperationID
		text string
	}{
		{operation.OperationTaskSetState, "Set State"},
		{operation.OperationTaskSetPriority, "Priority"},
		{operation.OperationTaskSetDueDate, "Due"},
	} {
		choosePolishTaskMenuEntry(t, driver, view, tc.id)
		if screen := driver.Observe(ObserveOptions{}).PlainScreen; !strings.Contains(screen, tc.text) || view.MenuBar.Open || driver.App().operations.ValuePicker == nil {
			t.Fatalf("%s did not open its editor:\n%s", tc.id, screen)
		}
		pressMenuBarKey(t, driver, "esc")
	}
	choosePolishTaskMenuEntry(t, driver, view, operation.OperationTaskSetState)
	pressMenuBarKey(t, driver, "/", "d", "o", "n", "e", "enter", "enter")
	task, _ = workspaceTaskTitled(t, driver.App(), checklistFixturePlainTitle)
	if task.WorkflowState != models.StateDone {
		t.Fatal("choosing Done in the menu's state picker did not finish the task")
	}
	pressMenuBarKey(t, driver, "u")
	task, _ = workspaceTaskTitled(t, driver.App(), checklistFixturePlainTitle)
	if task.WorkflowState == models.StateDone {
		t.Fatal("menu completion did not use the undo ledger")
	}
}

func TestDailyPolishTaskMenuFitsNarrowWindowsAndKeepsGroups(t *testing.T) {
	driver, view := menuBarDriver(t)
	for _, size := range [][2]int{{60, 20}, {80, 24}, {120, 40}} {
		driver.Resize(size[0], size[1])
		pressMenuBarKey(t, driver, "f10")
		menu := menuWithID(t, openMenuBarMenusForTest(t, view), outliner.MenuIDTask)
		groups := []string{}
		for _, entry := range menu.Entries {
			if entry.OperationID == "" && entry.Label != "" {
				groups = append(groups, entry.Label)
			}
		}
		if strings.Join(groups, ",") != "Plan,Status,Recovery,Details" {
			t.Fatalf("Task menu groups = %v", groups)
		}
		for range menu.Entries {
			obs := driver.Observe(ObserveOptions{})
			if len(obs.LayoutIssues) != 0 {
				t.Fatalf("menu layout at %v: %+v\n%s", size, obs.LayoutIssues, obs.PlainScreen)
			}
			pressMenuBarKey(t, driver, "down")
		}
		if view.MenuBar.ItemIndex != len(menu.Entries)-1 {
			t.Fatal("menu navigation did not reach its last action")
		}
		pressMenuBarKey(t, driver, "esc")
	}
}

func TestDailyPolishChecklistPeekOpensAndKeepsTaskActionsGuarded(t *testing.T) {
	driver, _, view := checklistFixtureApp(t)
	expandChecklistOf(t, view, checklistFixtureFoldTitle)
	seatCursorOnChecklistItem(t, view, checklistFixtureSecondItem)
	pressMenuBarKey(t, driver, "b")
	if !view.PeekMode {
		t.Fatal("b did not open Peek on the checklist item")
	}
	for _, text := range []string{checklistFixtureSecondItem, "Checklist:", "Task: " + checklistFixtureFoldTitle} {
		if screen := driver.Observe(ObserveOptions{}).PlainScreen; !strings.Contains(screen, text) {
			t.Fatalf("checklist Peek omitted %q:\n%s", text, screen)
		}
	}
	before, _ := workspaceTaskTitled(t, driver.App(), checklistFixtureFoldTitle)
	state := before.WorkflowState
	pressMenuBarKey(t, driver, "w", "d")
	after, _ := workspaceTaskTitled(t, driver.App(), checklistFixtureFoldTitle)
	if after.WorkflowState != state || after.WorkflowState == models.StateDone {
		t.Fatal("checklist Peek allowed a task mutation through the item")
	}
	pressMenuBarKey(t, driver, "b")
	if view.PeekMode {
		t.Fatal("b did not close checklist Peek")
	}
}
