package tui

import (
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/config"
)

// newSavedFilterTestApp builds an AppModel rooted at a real temporary data
// directory so planning commits actually persist (the shared newTestAppModel
// harness roots the runtime at DataDir=="" where every planning commit is a
// no-op). The manager and the runtime share the same data dir so a planning
// commit is observable through both runtime.Planning() and the facade sync.
func newSavedFilterTestApp(t *testing.T) *AppModel {
	t.Helper()
	resetSessionMutations()

	dataDir := t.TempDir()
	cfg := newTestConfigAt(dataDir)
	manager := config.NewManager(filepath.Join(dataDir, ".tt.json"))
	runtime := testRuntimeAt(dataDir, cfg)

	return &AppModel{
		configManager:            manager,
		runtime:                  runtime,
		deps:                     newShellDeps(runtime, nil, manager),
		persister:                NewProjectService(),
		outlinerToggleStates:     make(map[string]*OutlinerToggleState),
		projectLastView:          make(map[string]ViewType),
		viewStack:                []ViewType{ViewDashboard},
		width:                    80,
		height:                   24,
		views:                    NewViewRegistry(),
		navHistory:               NewNavHistory(),
		projectTreeSpatialStates: make(map[string]*ProjectTreeSpatialState),
	}
}

// TestSaveFilterMsgPersistsToPlanning proves the SaveFilterMsg handler commits a
// captured filter to the planning domain (not a whole-config save), and that the
// filter is readable back through the runtime planning snapshot.
func TestSaveFilterMsgPersistsToPlanning(t *testing.T) {
	app := newSavedFilterTestApp(t)
	app.pushView(ViewSaveFilterInput)

	handled, cmd := app.handleSavedFilterMessages(SaveFilterMsg{Name: "Backend"})
	if !handled {
		t.Fatal("SaveFilterMsg should be handled")
	}
	if cmd != nil {
		if msg := cmd(); msg != nil {
			if _, isErr := msg.(ErrorMsg); isErr {
				t.Fatalf("SaveFilterMsg returned error: %+v", msg)
			}
		}
	}

	sf := app.runtime.Planning().GetSavedFilterByName("Backend")
	if sf == nil {
		t.Fatal("saved filter should be persisted in the planning domain")
	}
	if sf.Name != "Backend" {
		t.Fatalf("saved filter name = %q, want Backend", sf.Name)
	}
}

// TestSaveFilterMsgUpsertsByIdentity proves a re-save under the same name reuses
// the existing filter's ID (upsert) rather than appending a duplicate, preserving
// saved-filter identity across planning commits.
func TestSaveFilterMsgUpsertsByIdentity(t *testing.T) {
	app := newSavedFilterTestApp(t)
	app.pushView(ViewSaveFilterInput)

	app.handleSavedFilterMessages(SaveFilterMsg{Name: "Triage"})
	first := app.runtime.Planning().GetSavedFilterByName("Triage")
	if first == nil {
		t.Fatal("first save should persist")
	}
	firstID := first.ID

	app.pushView(ViewSaveFilterInput)
	app.handleSavedFilterMessages(SaveFilterMsg{Name: "Triage"})

	filters := app.runtime.Planning().GetSavedFilters()
	if len(filters) != 1 {
		t.Fatalf("saved filters = %d, want 1 (upsert by name)", len(filters))
	}
	if filters[0].ID != firstID {
		t.Fatalf("re-save changed ID from %q to %q; identity must be preserved", firstID, filters[0].ID)
	}
}

// TestDeleteSavedFilterMsgRemovesFromPlanning proves DeleteSavedFilterMsg removes
// the filter from the planning domain through a checked commit.
func TestDeleteSavedFilterMsgRemovesFromPlanning(t *testing.T) {
	app := newSavedFilterTestApp(t)

	if err := app.commitPlanningMutation(func(p *config.PlanningState) {
		p.SaveFilter(config.SavedFilter{ID: "del-me", Name: "Delete Me", Mode: "snapshot"})
	}); err != nil {
		t.Fatalf("seed save failed: %v", err)
	}
	if app.runtime.Planning().GetSavedFilter("del-me") == nil {
		t.Fatal("seed filter should exist")
	}

	handled, _ := app.handleSavedFilterMessages(DeleteSavedFilterMsg{ID: "del-me"})
	if !handled {
		t.Fatal("DeleteSavedFilterMsg should be handled")
	}
	if app.runtime.Planning().GetSavedFilter("del-me") != nil {
		t.Fatal("filter should be removed from the planning domain after delete")
	}
}

// TestToggleSavedFilterTabMsgUpdatesPlanning proves the tab toggle mutates
// ShowAsTab on the persisted planning filter.
func TestToggleSavedFilterTabMsgUpdatesPlanning(t *testing.T) {
	app := newSavedFilterTestApp(t)

	if err := app.commitPlanningMutation(func(p *config.PlanningState) {
		p.SaveFilter(config.SavedFilter{ID: "tabbable", Name: "Tabbable", Mode: "snapshot"})
	}); err != nil {
		t.Fatalf("seed save failed: %v", err)
	}

	handled, _ := app.handleSavedFilterMessages(toggleSavedFilterTabMsg{ID: "tabbable", ShowAsTab: true})
	if !handled {
		t.Fatal("toggleSavedFilterTabMsg should be handled")
	}
	sf := app.runtime.Planning().GetSavedFilter("tabbable")
	if sf == nil || !sf.ShowAsTab {
		t.Fatalf("filter ShowAsTab = %v, want true", sf)
	}

	app.handleSavedFilterMessages(toggleSavedFilterTabMsg{ID: "tabbable", ShowAsTab: false})
	sf = app.runtime.Planning().GetSavedFilter("tabbable")
	if sf == nil || sf.ShowAsTab {
		t.Fatalf("filter ShowAsTab = %v, want false", sf)
	}
}

// TestFilterBuilderSaveCommitsPlanning proves the filter builder persists a new
// smart-list (criteria mode) filter through the planning-domain commit seam.
func TestFilterBuilderSaveCommitsPlanning(t *testing.T) {
	app := newSavedFilterTestApp(t)

	builder := NewFilterBuilderModel(app.runtime)
	builder.nameInput.SetValue("High Priority")
	builder.criteria[0].fieldCursor = smartListFieldIndex("priority")
	builder.criteria[0].opCursor = 0    // equals
	builder.criteria[0].valueCursor = 0 // "1"

	cmd := builder.saveSavedFilter()
	if cmd == nil {
		t.Fatal("saveSavedFilter should return a command")
	}
	if msg := cmd(); msg != nil {
		if _, isErr := msg.(ErrorMsg); isErr {
			t.Fatalf("saveSavedFilter returned error: %+v", msg)
		}
	}

	sf := app.runtime.Planning().GetSavedFilterByName("High Priority")
	if sf == nil {
		t.Fatal("builder-created filter should persist in the planning domain")
	}
	if sf.Mode != "criteria" || len(sf.Criteria) != 1 {
		t.Fatalf("filter = %+v, want criteria mode with one criterion", sf)
	}
	if sf.Criteria[0].Field != "priority" {
		t.Fatalf("criterion field = %q, want priority", sf.Criteria[0].Field)
	}
}
