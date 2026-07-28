package tui

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestFinderResultSourcePromotesRecentAndPinnedDestinations(t *testing.T) {
	project := &models.Project{
		ID:     "project",
		Name:   "Pinned Project",
		Slug:   "pinned-project",
		Pinned: true,
	}
	task := &models.Task{
		ID:            "task",
		ShortID:       1,
		Title:         "Recent Task",
		WorkflowState: models.StateReady,
		Pinned:        true,
	}
	project.Tasks = []*models.Task{task}
	machine := config.NewMachineState()
	machine.SetRecentDestinations([]config.RecentDestination{{
		Kind:      config.RecentDestinationTask,
		ProjectID: project.ID,
		ID:        task.ID,
	}})

	finder, err := NewFinder(
		machine,
		FinderTaskProvider{Projects: []*models.Project{project}},
		FinderProjectProvider{Projects: []*models.Project{project}},
		FinderTagProvider{Tags: []TagInfo{{Name: "focus", TaskCount: 1, Pinned: true}}})
	if err != nil {
		t.Fatalf("NewFinder: %v", err)
	}

	results := finder.Results("")
	if got := len(results.Results); got != 3 {
		t.Fatalf("empty results = %d, want recent plus two pinned destinations", got)
	}
	if !results.Results[0].Recent || !results.Results[0].Pinned {
		t.Fatalf("first result = %+v, want pinned recent task", results.Results[0])
	}
	source := FinderResultListSource(results)
	if source.Items[0].CategoryID != finderResultCategoryRecent {
		t.Fatalf("recent category = %s, want %s", source.Items[0].CategoryID, finderResultCategoryRecent)
	}
	for _, item := range source.Items[1:] {
		if item.CategoryID != finderResultCategoryPinned {
			t.Fatalf("pinned result %s category = %s, want %s", item.ID, item.CategoryID, finderResultCategoryPinned)
		}
	}
}

func TestFinderOperationProviderAllowsRendererLocalShortcutReuse(t *testing.T) {
	registry := NewOperationRegistry()
	for _, descriptor := range []OperationDescriptor{
		{
			ID:                "first",
			Label:             "First",
			Category:          OperationCategoryNavigation,
			DirectKey:         "a",
			TargetCardinality: TargetCardinalityNone,
			Applicability:     func(CommandContext) bool { return true },
			Risk:              OperationRiskReadOnly,
		},
		{
			ID:                "second",
			Label:             "Second",
			Category:          OperationCategoryNavigation,
			DirectKey:         "a",
			TargetCardinality: TargetCardinalityNone,
			Applicability:     func(CommandContext) bool { return true },
			Risk:              OperationRiskReadOnly,
		},
	} {
		if err := registry.Register(descriptor); err != nil {
			t.Fatalf("register %s: %v", descriptor.ID, err)
		}
	}
	if err := registry.ValidateContext(CommandContext{}); err == nil {
		t.Fatal("active key dispatcher should still reject the collision")
	}
	items, err := (FinderOperationProvider{
		Registry: registry,
		Context:  CommandContext{},
	}).FinderItems()
	if err != nil {
		t.Fatalf("FinderItems must not reject renderer-local shortcut reuse: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("finder operations = %d, want 2", len(items))
	}
}

func TestModeDiscoveryUsesExactTenModeListSurface(t *testing.T) {
	model, err := NewModeDiscoveryModel(CommandContext{})
	if err != nil {
		t.Fatalf("NewModeDiscoveryModel: %v", err)
	}
	items := model.surface.Items()
	if len(items) != 10 {
		t.Fatalf("mode discovery items = %d, want 10", len(items))
	}
	ids := make([]ListItemID, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	for _, mode := range OutlinerModeCatalog() {
		if !slices.Contains(ids, "mode:"+ListItemID(mode.ID)) {
			t.Fatalf("canonical mode %s is missing from discovery", mode.ID)
		}
	}
}

func TestVisibleSearchBindingUsesCanonicalStateGrammar(t *testing.T) {
	strategy := SimplifiedInputStrategy()
	found := false
	for _, binding := range strategy.Bindings {
		if binding.OperationID == operationCoreVisibleSearch {
			found = slices.Contains(binding.Keys, "/")
			break
		}
	}
	if !found {
		t.Fatal("visible search is not bound to /")
	}

	parsed := ParseSearchQuery("$ready")
	if !slices.Equal(parsed.States, []models.WorkflowState{models.StateReady}) {
		t.Fatalf("$ready states = %v, want exact ready", parsed.States)
	}
	if !TaskMatchesFilters(
		&models.Task{WorkflowState: models.StateReady},
		parsed,
	) {
		t.Fatal("$ready should match ready task")
	}
	if TaskMatchesFilters(
		&models.Task{WorkflowState: models.StateInProgress},
		parsed,
	) {
		t.Fatal("$ready must not match in-progress task")
	}
}

func TestSimplifiedFinderAndModeKeysOpenDiscoveryListSurfaces(t *testing.T) {
	app := newTestAppModel()
	outliner := newWorkModeOutliner(t)
	app.crossOutliner = outliner
	app.views.Register(ViewCrossProjectOutliner, outliner)
	setTestPresentationPath(app, ViewCrossProjectOutliner)
	app.operationRegistry = NewOperationRegistry()
	app.operationExecutor = NewOperationExecutor(app.operationRegistry)
	if err := app.registerCoreInputOperations(); err != nil {
		t.Fatalf("register core operations: %v", err)
	}
	if err := app.registerTaskActionOperations(); err != nil {
		t.Fatalf("register task action operations: %v", err)
	}
	if err := app.registerFilterOperations(); err != nil {
		t.Fatalf("register filter operations: %v", err)
	}
	if err := app.operationRegistry.RegisterInputStrategy(SimplifiedInputStrategy()); err != nil {
		t.Fatalf("register simplified strategy: %v", err)
	}

	for _, test := range []struct {
		key  rune
		kind DiscoverySurfaceKind
	}{
		{key: ':', kind: DiscoverySurfaceFinder},
		{key: 'm', kind: DiscoverySurfaceModes},
	} {
		handled, cmd := app.handleSimplifiedInputKey(tea.KeyPressMsg{Code: test.key, Text: string(test.key)})
		if !handled || cmd == nil {
			t.Fatalf("%q did not dispatch discovery", test.key)
		}
		if handled, initCmd := app.handleSearchMessages(cmd()); !handled {
			t.Fatalf("%q emitted an unhandled discovery message", test.key)
		} else if initCmd != nil {
			_ = initCmd()
		}
		discovery, ok := app.views.Get(ViewFinder).(*DiscoveryListModel)
		if !ok || discovery.kind != test.kind {
			t.Fatalf("%q view = %T/%v, want DiscoveryListModel kind %v", test.key, app.views.Get(ViewFinder), discovery, test.kind)
		}
		if test.kind == DiscoverySurfaceModes && len(discovery.surface.Items()) != 10 {
			t.Fatalf("mode surface has %d items, want 10", len(discovery.surface.Items()))
		}
		app.popView()
	}
}
