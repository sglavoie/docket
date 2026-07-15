package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func setLiveUndoDepth(t *testing.T, shell *AppModel, depth int) {
	t.Helper()
	settings := shell.runtime.Settings()
	settings.UndoHistoryDepth = &depth
	shell.runtime.SetSettingsInMemory(settings)
	shell.pushView(ViewSettings)
	handled, _ := shell.handleViewSettingsMessages(SettingsSavedMsg{})
	if !handled {
		t.Fatal("SettingsSavedMsg was not handled")
	}
}

func captureHistoryTestTask(t *testing.T, shell *AppModel, projectID, title string) {
	t.Helper()
	if _, err := shell.appService.Capture(app.CaptureRequest{ProjectID: projectID, Title: title}); err != nil {
		t.Fatalf("Capture(%q): %v", title, err)
	}
}

func TestSettingsSavedAppliesHistoryDepthToServiceAndOpenNotesView(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureFresh, 100, 30)
	if err != nil {
		t.Fatalf("NewFixtureDriver: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	shell := driver.App()
	ledger := shell.deps.historyLedger()
	if ledger == nil {
		t.Fatal("production shell has no workspace ledger")
	}
	project := shell.projectByID(shell.runtime.SystemInboxProject().ID)
	if project == nil {
		t.Fatal("fresh fixture has no loaded system inbox")
	}

	// Open the view before either settings change. Its undo command must keep the
	// exact handle the long-lived application service records through.
	if handled, _ := shell.handleNotesMessages(ShowNotesMsg{Project: project}); !handled {
		t.Fatal("ShowNotesMsg was not handled")
	}
	notes, ok := shell.views.Get(ViewNotes).(*NotesViewModel)
	if !ok {
		t.Fatalf("project notes view = %T", shell.views.Get(ViewNotes))
	}
	if notes.noteLedger != ledger {
		t.Fatal("open notes view did not receive the shell ledger")
	}

	baseline, err := ledger.UndoCount(storage.GlobalScope())
	if err != nil {
		t.Fatalf("initial UndoCount: %v", err)
	}
	for i, title := range []string{"history one", "history two", "history three"} {
		captureHistoryTestTask(t, shell, project.ID, title)
		count, countErr := ledger.UndoCount(storage.GlobalScope())
		if countErr != nil {
			t.Fatalf("UndoCount after capture %d: %v", i+1, countErr)
		}
		if want := baseline + i + 1; count != want {
			t.Fatalf("UndoCount after capture %d = %d, want %d", i+1, count, want)
		}
	}

	setLiveUndoDepth(t, shell, 1)
	retainedNote := models.NewNote("history prunes")
	if _, err := shell.appService.ApplyNoteBatch(app.NoteBatchRequest{
		Operations: []app.NoteMutationRequest{{
			Kind:     app.NoteCreate,
			Location: app.ProjectNoteLocation(project.ID),
			Note:     retainedNote,
		}},
		UndoAction:  "Create Note",
		UndoContext: retainedNote.Title,
	}); err != nil {
		t.Fatalf("ApplyNoteBatch with depth 1: %v", err)
	}
	if count, countErr := ledger.UndoCount(storage.GlobalScope()); countErr != nil || count != 1 {
		t.Fatalf("UndoCount after live depth 1 = %d, err=%v, want 1", count, countErr)
	}

	setLiveUndoDepth(t, shell, 0)
	disabledNote := models.NewNote("history disabled")
	if _, err := shell.appService.ApplyNoteBatch(app.NoteBatchRequest{
		Operations: []app.NoteMutationRequest{{
			Kind:     app.NoteCreate,
			Location: app.GlobalNoteLocation(),
			Note:     disabledNote,
		}},
		UndoAction:  "Create Note",
		UndoContext: disabledNote.Title,
	}); err != nil {
		t.Fatalf("ApplyNoteBatch with disabled history: %v", err)
	}
	if count, countErr := ledger.UndoCount(storage.GlobalScope()); countErr != nil || count != 1 {
		t.Fatalf("UndoCount after live depth 0 = %d, err=%v, want retained 1", count, countErr)
	}

	msg := notes.undoDelete()()
	restored, ok := msg.(ProjectNoteLedgerResultMsg)
	if !ok {
		if failure, failed := msg.(ProjectNoteLedgerErrorMsg); failed {
			t.Fatalf("open notes view undo failed: %v", failure.Err)
		}
		t.Fatalf("open notes view undo = %T, want ProjectNoteLedgerResultMsg", msg)
	}
	if restored.Entry.Action != "Create Note" || restored.Entry.Context != retainedNote.Title {
		t.Fatalf("open notes view restored unexpected entry: %+v", restored.Entry)
	}
	if count, countErr := ledger.UndoCount(storage.GlobalScope()); countErr != nil || count != 0 {
		t.Fatalf("UndoCount after restoring retained entry = %d, err=%v, want 0", count, countErr)
	}
}

func TestAllNotesOpenPathsUseShellLedger(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureFresh, 100, 30)
	if err != nil {
		t.Fatalf("NewFixtureDriver: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	shell := driver.App()
	ledger := shell.deps.historyLedger()
	project := shell.projectByID(shell.runtime.SystemInboxProject().ID)
	assertLedger := func(name string, view ViewType) {
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

	shell.reconstructProjectView(NavEntry{ViewType: ViewNotes, ProjectID: project.ID})
	assertLedger("history project notes", ViewNotes)
	shell.reconstructGlobalView(NavEntry{ViewType: ViewGlobalNotes})
	assertLedger("history global notes", ViewGlobalNotes)
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

	setLiveUndoDepth(t, shell, 0)
}
