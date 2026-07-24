package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func scopeKey(t *testing.T, name string) tea.KeyPressMsg {
	t.Helper()
	key, err := parseUITestKey(name)
	if err != nil {
		t.Fatalf("parseUITestKey(%q): %v", name, err)
	}
	return key
}

// scopeCursorOnKind moves the picker cursor onto the first entry of the given
// lifecycle kind, failing the test when no such row exists.
func scopeCursorOnKind(t *testing.T, m *ScopePickerModel, kind scopeEntryKind) {
	t.Helper()
	for i, e := range m.filtered {
		if e.kind == kind {
			m.cursor = i
			return
		}
	}
	t.Fatalf("no scope-picker entry of kind %d", kind)
}

func newScopePickerFixture() (*ScopePickerModel, *models.Project, *models.Project, storage.TrashEntry) {
	active := &models.Project{ID: "A", Name: "Alpha"}
	archived := &models.Project{ID: "B", Name: "Bravo", Archived: true}
	trash := storage.TrashEntry{EntryID: "trash-1", ProjectID: "C", Slug: "charlie", Name: "Charlie", DeletedAt: time.Unix(1000, 0)}
	m := newScopePicker([]*models.Project{active}, []*models.Project{archived}, []storage.TrashEntry{trash})
	return m, active, archived, trash
}

func TestScopePickerBuildsAllLifecycleSections(t *testing.T) {
	m, _, _, _ := newScopePickerFixture()

	kinds := map[scopeEntryKind]int{}
	for _, e := range m.entries {
		kinds[e.kind]++
	}
	for _, want := range []scopeEntryKind{scopeEntryAllProjects, scopeEntryActive, scopeEntryArchived, scopeEntryTrash} {
		if kinds[want] == 0 {
			t.Fatalf("expected scope picker to include kind %d, got %+v", want, kinds)
		}
	}
	// The synthetic All Projects scope-clear affordance must lead the list.
	if len(m.entries) == 0 || m.entries[0].kind != scopeEntryAllProjects {
		t.Fatalf("expected All Projects entry first, got %+v", m.entries)
	}
}

func TestScopePickerAllProjectsClearsScope(t *testing.T) {
	m, _, _, _ := newScopePickerFixture()
	scopeCursorOnKind(t, m, scopeEntryAllProjects)

	_, dismiss, result := m.handleKey(scopeKey(t, "enter"))
	if !dismiss {
		t.Fatal("expected All Projects selection to dismiss the picker")
	}
	apply, ok := result.(ApplyProjectScopeMsg)
	if !ok || !apply.Clear {
		t.Fatalf("expected scope-clear ApplyProjectScopeMsg, got %#v", result)
	}
}

func TestScopePickerActiveProjectScopes(t *testing.T) {
	m, active, _, _ := newScopePickerFixture()
	scopeCursorOnKind(t, m, scopeEntryActive)

	_, dismiss, result := m.handleKey(scopeKey(t, "enter"))
	apply, ok := result.(ApplyProjectScopeMsg)
	if !dismiss || !ok || apply.Clear || apply.ProjectID != active.ID {
		t.Fatalf("expected scope to %q, got dismiss=%v %#v", active.ID, dismiss, result)
	}
}

// TestScopePickerArchivedAndTrashActions is the completion-contract coverage:
// archived rows recover (never scope) since the cross Outliner cannot load them,
// and trash rows support restore plus confirmation-gated permanent deletion —
// all through lifecycle-safe result types rather than fabricated active projects.
func TestScopePickerArchivedAndTrashActions(t *testing.T) {
	t.Run("archived enter recovers, never scopes", func(t *testing.T) {
		m, _, archived, _ := newScopePickerFixture()
		scopeCursorOnKind(t, m, scopeEntryArchived)
		_, dismiss, result := m.handleKey(scopeKey(t, "enter"))
		if _, isScope := result.(ApplyProjectScopeMsg); isScope {
			t.Fatalf("archived Enter must not emit ApplyProjectScopeMsg (unresolvable scope), got %#v", result)
		}
		req, ok := result.(ProjectArchiveRequestMsg)
		if !dismiss || !ok || req.Archive || req.Project == nil || req.Project.ID != archived.ID {
			t.Fatalf("expected unarchive recovery for %q, got dismiss=%v %#v", archived.ID, dismiss, result)
		}
	})

	t.Run("archive recovery", func(t *testing.T) {
		m, _, archived, _ := newScopePickerFixture()
		scopeCursorOnKind(t, m, scopeEntryArchived)
		_, dismiss, result := m.handleKey(scopeKey(t, "X"))
		req, ok := result.(ProjectArchiveRequestMsg)
		if !dismiss || !ok {
			t.Fatalf("expected ProjectArchiveRequestMsg, got dismiss=%v %#v", dismiss, result)
		}
		if req.Archive || req.Project == nil || req.Project.ID != archived.ID {
			t.Fatalf("expected unarchive request for %q, got %#v", archived.ID, req)
		}
	})

	t.Run("trash restore", func(t *testing.T) {
		m, _, _, trash := newScopePickerFixture()
		scopeCursorOnKind(t, m, scopeEntryTrash)
		_, dismiss, result := m.handleKey(scopeKey(t, "enter"))
		restore, ok := result.(RestoreFromTrashMsg)
		if !dismiss || !ok || restore.EntryID != trash.EntryID {
			t.Fatalf("expected restore of %q, got %#v", trash.EntryID, result)
		}
	})

	t.Run("trash permanent delete requires confirmation", func(t *testing.T) {
		m, _, _, trash := newScopePickerFixture()
		scopeCursorOnKind(t, m, scopeEntryTrash)

		// First press arms the confirmation without emitting a destructive msg.
		_, dismiss, result := m.handleKey(scopeKey(t, "x"))
		if dismiss || result != nil {
			t.Fatalf("expected arming (no dismiss/result), got dismiss=%v %#v", dismiss, result)
		}
		if !m.confirmPurge {
			t.Fatal("expected purge confirmation to be armed")
		}

		// Confirming emits the lifecycle-safe purge and dismisses.
		_, dismiss, result = m.handleKey(scopeKey(t, "y"))
		purge, ok := result.(PurgeFromTrashMsg)
		if !dismiss || !ok || purge.EntryID != trash.EntryID {
			t.Fatalf("expected purge of %q, got %#v", trash.EntryID, result)
		}
	})

	t.Run("trash permanent delete can be cancelled", func(t *testing.T) {
		m, _, _, _ := newScopePickerFixture()
		scopeCursorOnKind(t, m, scopeEntryTrash)
		m.handleKey(scopeKey(t, "x"))
		_, dismiss, result := m.handleKey(scopeKey(t, "n"))
		if dismiss || result != nil || m.confirmPurge {
			t.Fatalf("expected cancel to disarm without action, got dismiss=%v %#v armed=%v", dismiss, result, m.confirmPurge)
		}
	})
}

func TestScopePickerArchiveGuardsAreLifecycleScoped(t *testing.T) {
	// "X" (unarchive) is inert on an active project; "x" (purge) is inert off a
	// trash row — neither dismisses nor mutates.
	m, _, _, _ := newScopePickerFixture()
	scopeCursorOnKind(t, m, scopeEntryActive)
	if _, dismiss, result := m.handleKey(scopeKey(t, "X")); dismiss || result != nil {
		t.Fatalf("expected unarchive to be inert on active row, got dismiss=%v %#v", dismiss, result)
	}
	if _, dismiss, result := m.handleKey(scopeKey(t, "x")); dismiss || result != nil || m.confirmPurge {
		t.Fatalf("expected purge to be inert off trash row, got dismiss=%v %#v", dismiss, result)
	}
}

func TestScopePickerSearchFiltersEntries(t *testing.T) {
	m, _, _, _ := newScopePickerFixture()
	m.handleKey(scopeKey(t, "/"))
	m.handleKey(scopeKey(t, "c")) // "charlie" trash slug
	m.handleKey(scopeKey(t, "h"))

	var trashRows, otherProjects int
	for _, e := range m.filtered {
		switch e.kind {
		case scopeEntryTrash:
			trashRows++
		case scopeEntryActive, scopeEntryArchived:
			otherProjects++
		}
	}
	if trashRows != 1 {
		t.Fatalf("expected the charlie trash row to survive filtering, got %d", trashRows)
	}
	if otherProjects != 0 {
		t.Fatalf("expected non-matching projects filtered out, got %d", otherProjects)
	}
}

func TestApplyProjectScopePushesAndClearsOnCrossOutliner(t *testing.T) {
	app := newTestAppModel()
	app.viewStack = []PresentationID{ViewCrossProjectOutliner}
	app.crossOutliner = scopeTestOutliner(
		scopeTestProject("A", "Alpha", "1"),
		scopeTestProject("B", "Bravo", "1"),
	)

	if handled, _ := app.applyProjectScope(ApplyProjectScopeMsg{ProjectID: "A"}); !handled {
		t.Fatal("expected scope apply handled")
	}
	if app.crossOutliner.scopedProjectID != "A" {
		t.Fatalf("expected cross outliner scoped to A, got %q", app.crossOutliner.scopedProjectID)
	}

	if handled, _ := app.applyProjectScope(ApplyProjectScopeMsg{Clear: true}); !handled {
		t.Fatal("expected scope clear handled")
	}
	if app.crossOutliner.scopedProjectID != "" {
		t.Fatalf("expected scope cleared, got %q", app.crossOutliner.scopedProjectID)
	}
}

// TestArchivedScopeLoadsOrRejectsConsistently is the FR12-003 integration
// coverage. Selecting an archived row (driven through AppModel) performs a
// lifecycle recovery, never emitting a scope the cross Outliner cannot load; a
// stray scope request for an unloaded project is refused at the apply seam and
// stays refused across a reload; and an active scope loads and remains stable
// across reload — so no archived entry ever enters a scope that a later reload
// classifies as missing and unwinds.
func TestArchivedScopeLoadsOrRejectsConsistently(t *testing.T) {
	active := scopeTestProject("A", "Alpha", "1", "2")
	archived := &models.Project{ID: "B", Name: "Bravo", Archived: true}

	newFixture := func() *AppModel {
		app := newTestAppModel()
		app.viewStack = []PresentationID{ViewCrossProjectOutliner}
		oc := scopeTestOutliner(active)
		oc.runtime = app.runtime
		app.crossOutliner = oc
		return app
	}

	t.Run("archived enter recovers, never scopes", func(t *testing.T) {
		app := newFixture()
		app.scopePicker = newScopePicker([]*models.Project{active}, []*models.Project{archived}, nil)
		scopeCursorOnKind(t, app.scopePicker, scopeEntryArchived)

		// Drive Enter on the archived row through AppModel's overlay dispatcher.
		_, cmd := app.Update(scopeKey(t, "enter"))
		if app.scopePicker != nil {
			t.Fatal("expected the scope picker to dismiss after Enter")
		}
		if cmd == nil {
			t.Fatal("expected a command from the archived Enter selection")
		}
		msg := cmd()
		if _, isScope := msg.(ApplyProjectScopeMsg); isScope {
			t.Fatalf("archived Enter must not emit ApplyProjectScopeMsg, got %#v", msg)
		}
		req, ok := msg.(ProjectArchiveRequestMsg)
		if !ok || req.Archive || req.Project == nil || req.Project.ID != archived.ID {
			t.Fatalf("expected unarchive recovery for %q, got %#v", archived.ID, msg)
		}
		// Routing the recovery back through AppModel only opens a confirmation;
		// it must never scope the cross Outliner.
		app.Update(msg)
		if app.crossOutliner.scopedProjectID != "" {
			t.Fatalf("archived recovery must not scope the Outliner, got %q", app.crossOutliner.scopedProjectID)
		}
	})

	t.Run("stray archived scope apply is rejected and stays clear across reload", func(t *testing.T) {
		app := newFixture()
		// A scope request for the unloaded archived project must be refused.
		if handled, _ := app.applyProjectScope(ApplyProjectScopeMsg{ProjectID: archived.ID}); !handled {
			t.Fatal("expected the scope message consumed")
		}
		if app.crossOutliner.scopedProjectID != "" {
			t.Fatalf("expected scope refused for unloaded project, got %q", app.crossOutliner.scopedProjectID)
		}
		// A reload must leave the base all-projects scope intact — nothing to
		// unwind, active project still rendered.
		app.crossOutliner.handleCrossProjectsLoaded(crossProjectsLoadedMsg{
			projects:   []*models.Project{active},
			generation: app.crossOutliner.crossLoadGen,
		})
		if app.crossOutliner.scopedProjectID != "" {
			t.Fatalf("expected scope still clear after reload, got %q", app.crossOutliner.scopedProjectID)
		}
		if ids := scopeVisibleProjectIDs(app.crossOutliner); !ids["A"] {
			t.Fatalf("expected active project A rendered after reload, got %v", ids)
		}
	})

	t.Run("active scope loads and stays stable across reload", func(t *testing.T) {
		app := newFixture()
		if handled, _ := app.applyProjectScope(ApplyProjectScopeMsg{ProjectID: active.ID}); !handled {
			t.Fatal("expected the scope message consumed")
		}
		if app.crossOutliner.scopedProjectID != active.ID {
			t.Fatalf("expected scope %q, got %q", active.ID, app.crossOutliner.scopedProjectID)
		}
		if ids := scopeVisibleProjectIDs(app.crossOutliner); !ids["A"] {
			t.Fatalf("expected scoped project A rendered, got %v", ids)
		}
		// Reloading with the same active set must keep the scope in place.
		app.crossOutliner.handleCrossProjectsLoaded(crossProjectsLoadedMsg{
			projects:   []*models.Project{active},
			generation: app.crossOutliner.crossLoadGen,
		})
		if app.crossOutliner.scopedProjectID != active.ID {
			t.Fatalf("expected scope %q stable across reload, got %q", active.ID, app.crossOutliner.scopedProjectID)
		}
		if ids := scopeVisibleProjectIDs(app.crossOutliner); !ids["A"] {
			t.Fatalf("expected scoped project A rendered after reload, got %v", ids)
		}
	})
}

func TestApplyProjectScopeOutsideOutlinerIsNoop(t *testing.T) {
	app := newTestAppModel() // viewStack is ViewDashboard
	handled, _ := app.applyProjectScope(ApplyProjectScopeMsg{ProjectID: "A"})
	if !handled {
		t.Fatal("expected message consumed even when not in the Outliner")
	}
}
