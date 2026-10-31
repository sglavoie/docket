package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/history"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/presentation"
)

func openHistoryFromMenu(t *testing.T, driver *UITestDriver) *history.Model {
	t.Helper()
	pressMenuBarKey(t, driver, "f10", "right", "down", "down", "enter")
	view, ok := driver.App().activeView().(*history.Model)
	if !ok {
		t.Fatalf("menu did not open history: %T\n%s", driver.App().activeView(), driver.Render())
	}
	return view
}

func TestChangeHistoryBrowsesDetailsAndRestoresNextAction(t *testing.T) {
	driver, out := menuBarDriver(t)
	pressMenuBarKey(t, driver, "!", "1", "!", "2")
	cursor := out.Cursor
	view := openHistoryFromMenu(t, driver)
	if screen := driver.Render(); !strings.Contains(screen, "Next undo:") || !strings.Contains(screen, "Nothing to redo") {
		t.Fatalf("recovery preview missing:\n%s", screen)
	}
	// Inspect the older transaction; u must still undo the newest one.
	pressMenuBarKey(t, driver, "end", "enter")
	if screen := driver.Render(); !strings.Contains(screen, "Priority: 0 → 1") {
		t.Fatalf("before/after details missing:\n%s", screen)
	}
	pressMenuBarKey(t, driver, "u")
	if driver.App().activeView() != view || !strings.Contains(driver.Render(), "Next redo:") {
		t.Fatalf("undo dismissed or failed to refresh history:\n%s", driver.Render())
	}
	ledger, err := driver.App().workspaceHistoryLedger().History(storage.GlobalScope())
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Undo) != 1 || len(ledger.Redo) != 1 {
		t.Fatalf("selected row changed undo target: %+v", ledger)
	}
	pressMenuBarKey(t, driver, "ctrl+r")
	ledger, err = driver.App().workspaceHistoryLedger().History(storage.GlobalScope())
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Undo) != 2 || len(ledger.Redo) != 0 {
		t.Fatalf("redo did not restore one transaction: %+v", ledger)
	}
	pressMenuBarKey(t, driver, "esc", "esc")
	if driver.App().currentView() != presentation.ViewCrossProjectOutliner || out.Cursor != cursor {
		t.Fatal("closing history changed the original view")
	}
}

func TestChangeHistoryRefusesStaleRequestAndKeepsNewerEdit(t *testing.T) {
	driver, out := menuBarDriver(t)
	pressMenuBarKey(t, driver, "!", "1")
	view := openHistoryFromMenu(t, driver)
	ledger := driver.App().workspaceHistoryLedger()
	entry, _, err := ledger.SelectNewestUndo(storage.GlobalScope())
	if err != nil {
		t.Fatal(err)
	}
	ctx := out.OperationCommandContext()
	identity := operation.CurrentTaskIdentity(ctx)
	title := "Newer edit while history was open"
	_, err = driver.App().domain.service.Edit(app.EditRequest{Locator: app.TaskLocator{ProjectID: identity.ProjectID, TaskID: identity.TaskID}, Title: &title})
	if err != nil {
		t.Fatal(err)
	}
	driver.dispatch(history.RestoreRequest{Source: view, Operation: operation.OperationCoreUndo, TransactionID: entry.TransactionID})
	driver.drain()
	current, _, err := ledger.SelectNewestUndo(storage.GlobalScope())
	if err != nil {
		t.Fatal(err)
	}
	if current.TransactionID == entry.TransactionID {
		t.Fatal("stale request restored a different action")
	}
	pressMenuBarKey(t, driver, "home", "enter")
	if screen := driver.Render(); !strings.Contains(screen, title) || driver.App().activeView() != view {
		t.Fatalf("stale refusal did not refresh the preview:\n%s", screen)
	}
}

func TestChangeHistoryEmptyHelpAndNarrowLayout(t *testing.T) {
	driver, out := menuBarDriver(t)
	cursor := out.Cursor
	openHistoryFromMenu(t, driver)
	if !strings.Contains(driver.Render(), "No retained changes yet") {
		t.Fatalf("empty state missing:\n%s", driver.Render())
	}
	pressMenuBarKey(t, driver, "?", "end", "esc")
	if driver.App().currentView() != presentation.ViewChangeHistory {
		t.Fatal("help escape dismissed history")
	}
	for _, size := range [][2]int{{80, 24}, {60, 20}, {120, 30}, {80, 24}} {
		if _, err := driver.ApplyAction(UITestAction{Type: "resize", Width: size[0], Height: size[1]}, ObserveOptions{}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(driver.Render(), "Change History") {
			t.Fatal("resize hid the view instead of testing its layout")
		}
		if _, err := driver.ApplyAction(UITestAction{Type: "assert", Assert: "no_layout_issues"}, ObserveOptions{}); err != nil {
			t.Fatalf("%v\n%s", err, driver.Render())
		}
	}
	pressMenuBarKey(t, driver, "u", "ctrl+r", "esc")
	if driver.App().currentView() != presentation.ViewCrossProjectOutliner || out.Cursor != cursor {
		t.Fatal("empty history moved the outline")
	}
}

func TestChangeHistoryIgnoresRecoveryFromClosedBrowser(t *testing.T) {
	driver, _ := menuBarDriver(t)
	pressMenuBarKey(t, driver, "!", "1")
	old := openHistoryFromMenu(t, driver)
	ledger := driver.App().workspaceHistoryLedger()
	entry, ok, err := ledger.SelectNewestUndo(storage.GlobalScope())
	if err != nil || !ok {
		t.Fatalf("missing fixture transaction: %v", err)
	}
	pressMenuBarKey(t, driver, "esc")
	current := openHistoryFromMenu(t, driver)
	driver.dispatch(history.RestoreRequest{Source: old, Operation: operation.OperationCoreUndo, TransactionID: entry.TransactionID})
	driver.drain()
	after, ok, err := ledger.SelectNewestUndo(storage.GlobalScope())
	if err != nil || !ok || after.TransactionID != entry.TransactionID || driver.App().activeView() != current {
		t.Fatalf("closed browser restored data or replaced the current browser: %+v, %v", after, err)
	}
}
