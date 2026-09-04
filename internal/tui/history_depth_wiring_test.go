package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
)

func TestAllNotesOpenPathsUseShellLedger(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureFresh, 100, 30)
	if err != nil {
		t.Fatalf("NewFixtureDriver: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	shell := driver.App()
	ledger := shell.deps.historyLedger()
	project := shell.projectByID(shell.runtime.SystemUnfiledProject().ID)
	assertLedger := func(name string, view presentation.PresentationID) {
		t.Helper()
		manager, ok := shell.viewNav.views.Get(view).(*notes.NotesViewModel)
		if !ok {
			t.Fatalf("%s view = %T", name, shell.viewNav.views.Get(view))
		}
		if manager.NoteLedger != ledger {
			t.Fatalf("%s received a different workspace ledger", name)
		}
	}

	shell.handleNotesMessages(notes.ShowNotesManagerMsg{Scope: route.ProjectNotesScope(project.ID)})
	assertLedger("direct project notes", presentation.ViewNotes)
	shell.handleNotesMessages(notes.ShowNotesManagerMsg{Scope: route.GlobalNotesScope()})
	assertLedger("direct global notes", presentation.ViewGlobalNotes)

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
	shell.deps = newShellDeps(nil)

	if handled, _ := shell.handleNotesMessages(notes.ShowNotesManagerMsg{Scope: route.GlobalNotesScope()}); !handled {
		t.Fatal("notes.ShowNotesManagerMsg was not handled")
	}
	manager, ok := shell.viewNav.views.Get(presentation.ViewGlobalNotes).(*notes.NotesViewModel)
	if !ok {
		t.Fatalf("global manager view = %T", shell.viewNav.views.Get(presentation.ViewGlobalNotes))
	}
	if manager.NoteLedger != nil {
		t.Fatal("isolated nil-ledger composition unexpectedly created history")
	}
	msg := manager.UndoDelete()()
	failure, ok := msg.(notes.ProjectNoteLedgerErrorMsg)
	if !ok || failure.Err == nil || failure.Err.Error() != "workspace undo ledger is unavailable" {
		t.Fatalf("nil-ledger undo = %#v, want typed unavailable error", msg)
	}

}
