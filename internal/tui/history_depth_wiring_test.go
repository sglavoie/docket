package tui

import (
	"testing"
)

func TestAllNotesOpenPathsUseShellLedger(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureFresh, 100, 30)
	if err != nil {
		t.Fatalf("NewFixtureDriver: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	shell := driver.App()
	ledger := shell.deps.historyLedger()
	project := shell.projectByID(shell.runtime.SystemInboxProject().ID)
	assertLedger := func(name string, view PresentationID) {
		t.Helper()
		notes, ok := shell.views.Get(view).(*NotesViewModel)
		if !ok {
			t.Fatalf("%s view = %T", name, shell.views.Get(view))
		}
		if notes.noteLedger != ledger {
			t.Fatalf("%s received a different workspace ledger", name)
		}
	}

	shell.handleNotesMessages(ShowNotesMsg{Project: project})
	assertLedger("direct project notes", ViewNotes)
	shell.handleNotesMessages(ShowGlobalNotesMsg{})
	assertLedger("direct global notes", ViewGlobalNotes)

}

func TestNilShellLedgerMakesHistoryUnavailableWithoutPanic(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureFresh, 100, 30)
	if err != nil {
		t.Fatalf("NewFixtureDriver: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	shell := driver.App()
	service := newTUIAppService(workspace.DataDir, workspace.Manager.RuntimeDependencies(), nil)
	shell.appService = service
	shell.deps = newShellDeps(shell.runtime, service, workspace.Manager, nil)

	if handled, _ := shell.handleNotesMessages(ShowGlobalNotesMsg{}); !handled {
		t.Fatal("ShowGlobalNotesMsg was not handled")
	}
	notes, ok := shell.views.Get(ViewGlobalNotes).(*NotesViewModel)
	if !ok {
		t.Fatalf("global notes view = %T", shell.views.Get(ViewGlobalNotes))
	}
	if notes.noteLedger != nil {
		t.Fatal("isolated nil-ledger composition unexpectedly created history")
	}
	msg := notes.undoDelete()()
	failure, ok := msg.(ProjectNoteLedgerErrorMsg)
	if !ok || failure.Err == nil || failure.Err.Error() != "workspace undo ledger is unavailable" {
		t.Fatalf("nil-ledger undo = %#v, want typed unavailable error", msg)
	}

}
