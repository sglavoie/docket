package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestScopePickerSearchCommitThenRestoreThroughAppModel drives the complete
// Ctrl+X interaction through AppModel.Update: open the picker, fuzzy-filter an
// archived project, commit the retained query, act on that filtered row, and
// confirm the lifecycle mutation. This keeps the search-state and project
// lifecycle handlers covered as one user-visible chain.
func TestScopePickerSearchCommitThenRestoreThroughAppModel(t *testing.T) {
	dataDir := t.TempDir()
	active := newSession2Project(t, dataDir, "Active")
	archived := newSession2Project(t, dataDir, "Archived")
	archived.Archive()
	app, cross := newSession3CatalogApp(t, dataDir, active, archived)
	liveArchived := app.projectByID(archived.ID)

	_, openCmd := app.Update(scopeKey(t, "ctrl+x"))
	if openCmd == nil {
		t.Fatal("Ctrl+X returned no scope-picker command")
	}
	runSession2AppCommand(t, app, openCmd)
	if app.scopePicker == nil {
		t.Fatal("Ctrl+X did not install the scope picker")
	}

	for _, key := range []string{"/", "r", "c", "h", "v", "enter"} {
		app.Update(scopeKey(t, key))
	}
	picker := app.scopePicker
	if picker == nil || picker.searching {
		t.Fatal("search commit should retain an idle scope picker")
	}
	if got := picker.input.Value(); got != "rchv" {
		t.Fatalf("committed scope query = %q, want rchv", got)
	}
	if len(picker.filtered) != 1 || picker.filtered[0].kind != scopeEntryArchived ||
		picker.filtered[0].project != liveArchived {
		t.Fatalf("committed scope result = %+v, want only archived project", picker.filtered)
	}

	_, restoreRequestCmd := app.Update(scopeKey(t, "enter"))
	if restoreRequestCmd == nil || app.scopePicker != nil {
		t.Fatal("acting on the committed archived result should close the picker and return a request")
	}
	runSession2AppCommand(t, app, restoreRequestCmd)
	if !app.projectArchiveConfirm.IsActive() || app.pendingProjectArchive != liveArchived ||
		app.pendingProjectArchiveArchive {
		t.Fatal("committed scope result did not enter the app-level restore confirmation")
	}
	if !liveArchived.Archived {
		t.Fatal("restore request mutated the project before confirmation")
	}

	_, confirmCmd := app.Update(scopeKey(t, "y"))
	if confirmCmd == nil {
		t.Fatal("confirming the scope-picker restore returned no command")
	}
	_, restoreCmd := app.Update(confirmCmd())
	if restoreCmd == nil {
		t.Fatal("confirmed scope-picker restore returned no catalog command")
	}
	_, reloadCmd := app.Update(restoreCmd())
	if reloadCmd == nil {
		t.Fatal("completed scope-picker restore returned no reload/status command")
	}
	if liveArchived.Archived || liveArchived.ArchivedAt != nil {
		t.Fatal("confirmed scope-picker restore did not update live metadata")
	}
	if app.statusMessage != "Unarchived Archived" {
		t.Fatalf("restore status = %q, want Unarchived Archived", app.statusMessage)
	}

	finishSession3CrossReload(t, app, cross)
	if session3ProjectByID(t, cross.loadedProjects, liveArchived.ID).Archived {
		t.Fatal("restored scope-picker project remained archived after reload")
	}
}

// TestProjectHeaderAdminPinThroughAppModel drives Shift+Enter and the contextual
// menu keys through the shell before feeding the emitted pin request back into
// AppModel. Lower-level menu dispatch and catalog mutation tests cannot catch a
// broken project-header route or an overlay result dropped by AppModel.
func TestProjectHeaderAdminPinThroughAppModel(t *testing.T) {
	dataDir := t.TempDir()
	alpha := newSession2Project(t, dataDir, "Alpha")
	zulu := newSession2Project(t, dataDir, "Zulu")
	app, cross := newSession3CatalogApp(t, dataDir, alpha, zulu)
	liveZulu := app.projectByID(zulu.ID)

	header := -1
	for i, row := range cross.rows {
		if row.IsProjectHeader && row.ProjectID == liveZulu.ID {
			header = i
			break
		}
	}
	if header < 0 {
		t.Fatalf("project header for %q not found in cross-project rows", liveZulu.ID)
	}
	cross.cursor = header

	if _, cmd := app.Update(scopeKey(t, "shift+enter")); cmd != nil {
		t.Fatal("opening the project admin menu should not start an async command")
	}
	if app.projectAdminMenu == nil || app.projectAdminMenu.project != liveZulu {
		t.Fatal("Shift+Enter on the Zulu header did not install its admin menu")
	}

	for app.projectAdminMenu.items[app.projectAdminMenu.cursor].action != adminActionPin {
		app.Update(scopeKey(t, "j"))
	}
	_, pinCmd := app.Update(scopeKey(t, "enter"))
	if pinCmd == nil || app.projectAdminMenu != nil {
		t.Fatal("running the header pin action should close the menu and return a request")
	}
	if liveZulu.Pinned {
		t.Fatal("header pin action mutated live metadata before AppModel handled the request")
	}

	pin := projectPinRequestFromCmd(t, pinCmd)
	_, reloadCmd := app.Update(pin)
	if reloadCmd == nil {
		t.Fatal("AppModel pin handling returned no reload/status command")
	}
	if !liveZulu.Pinned || liveZulu.PinnedAt == nil {
		t.Fatal("AppModel did not commit the project-header pin request")
	}
	if app.statusMessage != "Pinned" {
		t.Fatalf("pin status = %q, want Pinned", app.statusMessage)
	}
	finishSession3CrossReload(t, app, cross)
	if len(cross.loadedProjects) != 2 || cross.loadedProjects[0].ID != liveZulu.ID {
		t.Fatalf("pinned project order = %#v, want Zulu first", cross.loadedProjects)
	}
}

func projectPinRequestFromCmd(t *testing.T, cmd tea.Cmd) ProjectPinRequestMsg {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		if next == nil {
			continue
		}
		switch msg := next().(type) {
		case ProjectPinRequestMsg:
			return msg
		case tea.BatchMsg:
			queue = append(queue, msg...)
		}
	}
	t.Fatal("project admin command did not emit ProjectPinRequestMsg")
	return ProjectPinRequestMsg{}
}

// TestFuzzyModeSelectionThroughAppModel proves the realistic printable-key
// events from AppModel reach the inline mode picker and that a non-contiguous
// fuzzy query selects the intended mode.
func TestFuzzyModeSelectionThroughAppModel(t *testing.T) {
	app := newTestAppModel()
	cross := newWorkModeOutliner(t)
	app.crossOutliner = cross
	app.views.Register(ViewCrossProjectOutliner, cross)
	app.viewStack = []PresentationID{ViewCrossProjectOutliner}

	app.Update(scopeKey(t, "P"))
	if !cross.modePickerActive() {
		t.Fatal("P did not open the mode picker through AppModel")
	}
	for _, key := range []string{"b", "k", "l", "g"} {
		app.Update(scopeKey(t, key))
	}
	if got := cross.modePicker.query; got != "bklg" {
		t.Fatalf("mode-picker query = %q, want bklg", got)
	}

	app.Update(scopeKey(t, "enter"))
	if cross.modePickerActive() {
		t.Fatal("fuzzy mode selection did not close the picker")
	}
	if got := cross.activeOutlinerMode(); got != ModeBacklog {
		t.Fatalf("fuzzy mode selection = %q, want %q", got, ModeBacklog)
	}
}
