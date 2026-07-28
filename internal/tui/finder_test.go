package tui

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestFinderProvidersBuildUnifiedTypedCatalog(t *testing.T) {
	registry := NewOperationRegistry()
	registerFinderTestOperation(t, registry, "open", "Open task")

	alpha := &models.Project{ID: "alpha", Name: "Alpha", Slug: "alpha", Context: "work"}
	beta := &models.Project{ID: "beta", Name: "Beta", Slug: "beta", Context: "home"}
	alphaTask := &models.Task{ID: "shared", ShortID: 1, Title: "Alpha task", WorkflowState: models.StateReady}
	betaTask := &models.Task{ID: "shared", ShortID: 2, Title: "Beta task", WorkflowState: models.StateNext}
	alpha.Tasks = []*models.Task{alphaTask}
	beta.Tasks = []*models.Task{betaTask}
	note := models.NewNote("Project note")
	mode := OutlinerModeCatalog()[0]
	scope := ContextScopeSummary{Scope: AllContextScope(), ActiveProjectCount: 2}

	finder, err := NewFinder(
		config.NewMachineState(),
		FinderOperationProvider{Registry: registry, Context: CommandContext{}},
		FinderTaskProvider{Projects: []*models.Project{beta, alpha}},
		FinderProjectProvider{Projects: []*models.Project{beta, alpha}},
		FinderNoteProvider{Notes: []FinderNote{{Note: note, Project: alpha}}}, FinderToolProvider{Tools: []FinderTool{{ID: "settings", Label: "Settings"}}}, FinderModeProvider{Modes: []OutlinerMode{mode}},
		FinderScopeProvider{Scopes: []ContextScopeSummary{scope}},
		FinderTagProvider{Tags: []TagInfo{{Name: "focus", TaskCount: 2}}},
	)
	if err != nil {
		t.Fatalf("NewFinder: %v", err)
	}

	seenCategories := make(map[ListCategoryID]bool)
	seenIDs := make(map[ListItemID]bool)
	for _, item := range finder.Items() {
		seenCategories[item.CategoryID] = true
		seenIDs[item.ID] = true
		if _, ok := item.Data.(FinderTarget); !ok {
			t.Fatalf("item %s data = %T, want FinderTarget", item.ID, item.Data)
		}
	}
	for _, category := range FinderCategories() {
		if !seenCategories[category.ID] {
			t.Errorf("missing finder category %s", category.ID)
		}
	}
	for _, id := range []ListItemID{
		QualifiedTaskListItemID(alpha.ID, alphaTask.ID),
		QualifiedTaskListItemID(beta.ID, betaTask.ID),
		QualifiedNoteListItemID(alpha.ID, note.Filename),
		"project:alpha",
	} {
		if !seenIDs[id] {
			t.Errorf("missing finder item %s", id)
		}
	}
}

func TestFinderOperationDiscoveryUsesPlacementAndApplicability(t *testing.T) {
	registry := newFinderPlacementTestRegistry(t)
	ctx := PresentationCommandContext(ViewCrossProjectOutliner)
	ctx.Capabilities.Add(CapabilitySimplifiedInput)

	items, err := (FinderOperationProvider{Registry: registry, Context: ctx}).FinderItems()
	if err != nil {
		t.Fatalf("FinderItems: %v", err)
	}
	seen := make(map[ListItemID]int, len(items))
	for _, item := range items {
		seen[item.ID]++
	}

	want := make(map[ListItemID]bool)
	for _, descriptor := range registry.All() {
		if descriptor.Placement == PlacementFinderOperation && descriptor.Applicable(ctx) {
			want[ListItemID("operation:"+string(descriptor.ID))] = true
		}
	}
	for id := range want {
		if seen[id] != 1 {
			t.Errorf("operation %s appeared %d times, want exactly once", id, seen[id])
		}
	}
	for id, count := range seen {
		if !want[id] {
			t.Errorf("Finder listed %s (%d times) despite its placement", id, count)
		}
	}
	// The unbound operation proves membership no longer depends on the key
	// grammar; the contextual and hidden ones prove placement still gates it.
	if seen["operation:finder.unbound"] != 1 {
		t.Error("Finder dropped an applicable operation that has no direct key")
	}
	for _, excluded := range []ListItemID{
		"operation:finder.tool",
		"operation:finder.contextual",
		"operation:finder.hidden",
	} {
		if seen[excluded] > 0 {
			t.Errorf("Finder exposed %s", excluded)
		}
	}
}

func TestFinderOperationOrderLeadsWithTheDirectKeyGrammar(t *testing.T) {
	registry := newFinderPlacementTestRegistry(t)
	ctx := PresentationCommandContext(ViewCrossProjectOutliner)
	ctx.Capabilities.Add(CapabilitySimplifiedInput)

	items, err := (FinderOperationProvider{Registry: registry, Context: ctx}).FinderItems()
	if err != nil {
		t.Fatalf("FinderItems: %v", err)
	}
	want := []ListItemID{
		"operation:tui.core.capture",
		"operation:tui.core.edit",
		"operation:tui.core.start",
		"operation:tui.core.done",
		"operation:tui.core.backlog",
		"operation:tui.core.today",
		"operation:tui.core.next",
		"operation:tui.core.actions",
	}
	if len(items) < len(want) {
		t.Fatalf("Finder operations = %d, want at least %d", len(items), len(want))
	}
	for index, id := range want {
		if items[index].ID != id {
			t.Fatalf("operation %d = %s, want %s", index, items[index].ID, id)
		}
	}
	if len(want) != FinderEmptyResultLimit {
		t.Fatalf("daily loop lead = %d, want the shortlist limit %d", len(want), FinderEmptyResultLimit)
	}
}

// newFinderPlacementTestRegistry holds the canonical key grammar plus one
// operation per placement that discovery must treat differently.
func newFinderPlacementTestRegistry(t *testing.T) *OperationRegistry {
	t.Helper()
	registry := NewOperationRegistry()
	for _, descriptor := range append(coreInputOperationDescriptors(), simplifiedTaskActionDescriptors()...) {
		if err := registry.Register(descriptor); err != nil {
			t.Fatalf("register core operation %s: %v", descriptor.ID, err)
		}
	}
	placements := map[OperationID]OperationPlacement{
		"finder.unbound":    PlacementFinderOperation,
		"finder.tool":       PlacementFinderTool,
		"finder.contextual": PlacementContextual,
		"finder.hidden":     PlacementHidden,
	}
	for id, placement := range placements {
		err := registry.Register(OperationDescriptor{
			ID:                id,
			Label:             "Placement " + string(placement),
			Category:          OperationCategoryNavigation,
			Placement:         placement,
			TargetCardinality: TargetCardinalityNone,
			Risk:              OperationRiskReadOnly,
		})
		if err != nil {
			t.Fatalf("register %s: %v", id, err)
		}
	}
	if err := registry.RegisterInputStrategy(SimplifiedInputStrategy()); err != nil {
		t.Fatalf("register simplified strategy: %v", err)
	}
	return registry
}

// openFixtureFinder launches a populated workspace, opens the Finder over it,
// and types query so the caller can act on a real result row.
func openFixtureFinder(t *testing.T, query string) (*AppModel, *DiscoveryListModel) {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	app := driver.App()
	if handled, _ := app.handleSearchMessages(ShowFinderMsg{}); !handled {
		t.Fatal("ShowFinderMsg was not handled")
	}
	discovery, ok := app.views.Get(ViewFinder).(*DiscoveryListModel)
	if !ok {
		t.Fatalf("Finder view = %T, want *DiscoveryListModel", app.views.Get(ViewFinder))
	}
	for _, key := range query {
		discovery.Update(tea.KeyPressMsg{Code: key, Text: string(key)})
	}
	return app, discovery
}

// finderTaskItemID returns the first task row currently ranked in the Finder.
func finderTaskItemID(t *testing.T, discovery *DiscoveryListModel) ListItemID {
	t.Helper()
	for _, item := range discovery.surface.Items() {
		if target, ok := item.Data.(FinderTarget); ok && target.Kind == FinderItemTask {
			return item.ID
		}
	}
	t.Fatal("no task row in the Finder results")
	return ""
}

func TestFinderTabOpensTaskActionsAndPreservesFinderState(t *testing.T) {
	app, discovery := openFixtureFinder(t, "login")
	taskID := finderTaskItemID(t, discovery)
	if !discovery.surface.SetCursorToID(taskID) {
		t.Fatalf("could not put the cursor on %s", taskID)
	}
	query := discovery.searchInput.Value()
	cursor := discovery.surface.Cursor()
	viewport := discovery.surface.Viewport()

	_, cmd := discovery.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if cmd == nil {
		t.Fatal("Tab produced no command on a task row")
	}
	request, ok := cmd().(ShowFinderContextActionsMsg)
	if !ok {
		t.Fatalf("Tab produced %T, want ShowFinderContextActionsMsg", cmd())
	}
	if request.Item.ID != taskID {
		t.Fatalf("Tab targeted %s, want the selected row %s", request.Item.ID, taskID)
	}
	if handled, _ := app.handleSearchMessages(request); !handled {
		t.Fatal("ShowFinderContextActionsMsg was not handled")
	}

	sheet := app.contextActionSheet
	if sheet == nil {
		t.Fatal("Tab did not open the contextual action sheet")
	}
	if sheet.config.Host != ContextActionHostFinder {
		t.Fatalf("sheet host = %q, want %q", sheet.config.Host, ContextActionHostFinder)
	}
	if sheet.config.Completion != ContextActionCompleteCloseHost {
		t.Fatalf("sheet completion = %q, want %q", sheet.config.Completion, ContextActionCompleteCloseHost)
	}
	if sheet.config.Targets != 1 || sheet.config.Bulk {
		t.Fatalf("sheet targets = %d bulk = %v, want a single frozen task", sheet.config.Targets, sheet.config.Bulk)
	}
	if len(sheet.surface.Items()) == 0 {
		t.Fatal("Finder task sheet offers no actions")
	}
	manifest := make(map[OperationID]bool)
	for _, id := range taskOperationManifestIDs() {
		manifest[id] = true
	}
	for _, item := range sheet.surface.Items() {
		for _, id := range item.OperationIDs {
			if !manifest[id] {
				t.Errorf("sheet row %s offers %s, which is not on the task manifest", item.ID, id)
			}
		}
	}

	if got := discovery.searchInput.Value(); got != query {
		t.Errorf("Finder query = %q, want %q preserved under the sheet", got, query)
	}
	if item, ok := discovery.surface.CurrentItem(); !ok || item.ID != taskID {
		t.Errorf("Finder selection = %v/%v, want %s preserved under the sheet", item.ID, ok, taskID)
	}
	if got := discovery.surface.Cursor(); got != cursor {
		t.Errorf("Finder cursor = %d, want %d preserved under the sheet", got, cursor)
	}
	if got := discovery.surface.Viewport(); got != viewport {
		t.Errorf("Finder viewport = %+v, want %+v preserved under the sheet", got, viewport)
	}
}

func TestFinderTabOnNonTaskRowOpensContextualActions(t *testing.T) {
	app, discovery := openFixtureFinder(t, "")
	item, ok := discovery.surface.CurrentItem()
	if !ok {
		t.Fatal("the empty-query shortlist is empty")
	}
	if target, isTarget := item.Data.(FinderTarget); !isTarget || target.Kind == FinderItemTask {
		t.Fatalf("shortlist head %s is a task row, want a non-task row", item.ID)
	}
	_, cmd := discovery.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if cmd == nil {
		t.Fatal("Tab produced no command")
	}
	request, isRequest := cmd().(ShowFinderContextActionsMsg)
	if !isRequest {
		t.Fatalf("Tab produced %T, want ShowFinderContextActionsMsg", cmd())
	}
	if handled, _ := app.handleSearchMessages(request); !handled {
		t.Fatal("ShowFinderContextActionsMsg was not handled")
	}
	sheet := app.contextActionSheet
	if sheet == nil {
		t.Fatal("Tab did not open contextual actions on a non-task row")
	}
	if sheet.config.Host != ContextActionHostFinder {
		t.Fatalf("sheet host = %q, want %q", sheet.config.Host, ContextActionHostFinder)
	}
	if len(sheet.operationIDs()) == 0 {
		t.Fatal("the non-task sheet offers no actions")
	}
	if app.currentView() != ViewFinder {
		t.Fatalf("current view = %v, want the Finder to stay open", app.currentView())
	}
}

func TestFinderTaskActionCompletionUnwindsTheFinderExactlyOnce(t *testing.T) {
	app, _ := openFixtureFinder(t, "login")
	if app.currentView() != ViewFinder {
		t.Fatalf("current view = %v, want %v", app.currentView(), ViewFinder)
	}
	released := false
	completion := ContextActionSheetCompletedMsg{
		Host: ContextActionHostFinder,
		Cmd:  func() tea.Msg { released = true; return nil },
	}
	drain := func(cmd tea.Cmd) {
		t.Helper()
		if cmd == nil {
			return
		}
		if batch, ok := cmd().(tea.BatchMsg); ok {
			for _, child := range batch {
				if child != nil {
					child()
				}
			}
		}
	}

	handled, cmd := app.handleSearchMessages(completion)
	if !handled {
		t.Fatal("ContextActionSheetCompletedMsg was not handled")
	}
	drain(cmd)
	if app.currentView() == ViewFinder {
		t.Fatal("a completed Finder action left the Finder open")
	}
	if !released {
		t.Fatal("the completed action's own command was dropped instead of released after the unwind")
	}

	view := app.currentView()
	handled, cmd = app.handleSearchMessages(completion)
	if !handled {
		t.Fatal("a repeated completion was not handled")
	}
	drain(cmd)
	if app.currentView() != view {
		t.Fatalf("a repeated completion popped again to %v, want %v", app.currentView(), view)
	}
}

func TestFinderTaskRowsCarryTheAuthoritativeActionManifest(t *testing.T) {
	project := &models.Project{ID: "alpha", Name: "Alpha", Slug: "alpha"}
	task := &models.Task{ID: "task", ShortID: 1, Title: "Task", WorkflowState: models.StateReady}
	project.Tasks = []*models.Task{task}

	items, err := (FinderTaskProvider{Projects: []*models.Project{project}}).FinderItems()
	if err != nil {
		t.Fatalf("FinderItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("task items = %d, want 1", len(items))
	}
	want := taskOperationManifestIDs()
	got := items[0].OperationIDs
	if len(got) != len(want) {
		t.Fatalf("task row actions = %d, want the full manifest of %d", len(got), len(want))
	}
	for index, id := range want {
		if got[index] != id {
			t.Fatalf("task row action %d = %s, want %s", index, got[index], id)
		}
	}
}

func TestFinderListsEachDescriptorInExactlyOneCategory(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	defer func() { _ = workspace.Cleanup() }()
	app := driver.App()
	registry := app.GetOperationRegistry()
	ctx := app.operationCommandContext()

	finder, err := NewRuntimeFinder(app.runtime, registry, ctx)
	if err != nil {
		t.Fatalf("NewRuntimeFinder: %v", err)
	}
	owners := make(map[OperationID][]ListCategoryID)
	for _, item := range finder.Items() {
		target, ok := item.Data.(FinderTarget)
		if !ok {
			continue
		}
		switch {
		case target.Kind == FinderItemOperation && target.Operation != nil:
			owners[target.Operation.ID] = append(owners[target.Operation.ID], item.CategoryID)
		case target.Kind == FinderItemTool && target.Tool != nil:
			id := OperationID(target.Tool.ID)
			owners[id] = append(owners[id], item.CategoryID)
		}
	}
	for id, categories := range owners {
		if len(categories) != 1 {
			t.Errorf("descriptor %s appears in %v, want exactly one category", id, categories)
		}
	}
	for _, descriptor := range registry.All() {
		if descriptor.Placement != PlacementFinderTool || !descriptor.Applicable(ctx) {
			continue
		}
		if categories := owners[descriptor.ID]; len(categories) != 1 || categories[0] != FinderCategoryTools {
			t.Errorf("tool %s listed under %v, want only %s", descriptor.ID, categories, FinderCategoryTools)
		}
	}
}

func TestFinderToolCatalogOmitsReplacedWorkflowSurfaces(t *testing.T) {
	retired := map[string]bool{
		"Global Inbox":  true,
		"Global Search": true,
		"Notes Search":  true,
	}
	required := map[string]bool{
		"Weekly Review": false,
		"Notes":         false,
		"Contexts":      false,
		"Blockers":      false,
		"Settings":      false,
	}
	registry := NewOperationRegistry()
	for _, descriptor := range globalToolDescriptors() {
		if err := registry.Register(descriptor); err != nil {
			t.Fatalf("register tool %s: %v", descriptor.ID, err)
		}
	}
	for _, tool := range globalToolCatalog(registry, PresentationCommandContext(ViewCrossProjectOutliner)) {
		if retired[tool.Label] {
			t.Errorf("Finder tool catalog exposes replaced surface %q", tool.Label)
		}
		if _, ok := required[tool.Label]; ok {
			required[tool.Label] = true
		}
	}
	for label, found := range required {
		if !found {
			t.Errorf("Finder tool catalog is missing %q", label)
		}
	}
}

func TestFinderEmptyQueryUsesThreeLiveRecentsThenOperationsAndCapsAtEight(t *testing.T) {
	registry := NewOperationRegistry()
	for index := range 7 {
		id := OperationID(fmt.Sprintf("operation-%d", index))
		registerFinderTestOperation(t, registry, id, fmt.Sprintf("Operation %d", index))
	}

	project := &models.Project{ID: "project", Name: "Project", Slug: "project"}
	task := &models.Task{ID: "task", ShortID: 1, Title: "Task", WorkflowState: models.StateReady}
	project.Tasks = []*models.Task{task}
	note := models.NewNote("Note")
	machine := config.NewMachineState()
	machine.SetRecentDestinations([]config.RecentDestination{
		{Kind: config.RecentDestinationTask, ProjectID: "missing", ID: "stale"},
		{Kind: config.RecentDestinationTask, ProjectID: project.ID, ID: task.ID},
		{Kind: config.RecentDestinationProject, ID: project.ID},
		{Kind: config.RecentDestinationNote, ProjectID: project.ID, ID: note.Filename},
		{Kind: config.RecentDestinationTag, ID: "fourth-live-recent"},
	})

	finder, err := NewFinder(
		machine,
		FinderOperationProvider{Registry: registry, Context: CommandContext{}},
		FinderTaskProvider{Projects: []*models.Project{project}},
		FinderProjectProvider{Projects: []*models.Project{project}},
		FinderNoteProvider{Notes: []FinderNote{{Note: note, Project: project}}}, FinderTagProvider{Tags: []TagInfo{{Name: "fourth-live-recent", TaskCount: 1}}})
	if err != nil {
		t.Fatalf("NewFinder: %v", err)
	}

	results := finder.Results("")
	if !results.Shortlist {
		t.Fatal("empty result set should be marked as a shortlist")
	}
	if got := len(results.Results); got != FinderEmptyResultLimit {
		t.Fatalf("empty results = %d, want %d", got, FinderEmptyResultLimit)
	}
	wantRecent := []ListItemID{
		QualifiedTaskListItemID(project.ID, task.ID),
		"project:project",
		QualifiedNoteListItemID(project.ID, note.Filename),
	}
	for index, want := range wantRecent {
		result := results.Results[index]
		if result.Item.ID != want || !result.Recent || result.Contextual {
			t.Errorf("result %d = %+v, want recent %s", index, result, want)
		}
	}
	for index, result := range results.Results[FinderMaxRecentDestinations:] {
		if result.Item.CategoryID != FinderCategoryOperations || !result.Contextual || result.Recent {
			t.Errorf("operation result %d = %+v", index, result)
		}
	}
}

func TestFinderTypedQuerySearchesFullCatalogWithDeterministicRanking(t *testing.T) {
	registry := NewOperationRegistry()
	registerFinderTestOperation(t, registry, "needle-operation", "Needle")
	project := &models.Project{ID: "project", Name: "Needle", Slug: "needle-project"}
	task := &models.Task{ID: "task", ShortID: 7, Title: "Needle", WorkflowState: models.StateReady}
	project.Tasks = []*models.Task{task}
	note := models.NewNote("Needle")

	finder, err := NewFinder(
		config.NewMachineState(),
		FinderTagProvider{Tags: []TagInfo{{Name: "Needle", TaskCount: 1}}}, FinderNoteProvider{Notes: []FinderNote{{Note: note, Project: project}}}, FinderProjectProvider{Projects: []*models.Project{project}},
		FinderTaskProvider{Projects: []*models.Project{project}},
		FinderOperationProvider{Registry: registry, Context: CommandContext{}},
	)
	if err != nil {
		t.Fatalf("NewFinder: %v", err)
	}

	results := finder.Search("needle")
	if results.Shortlist {
		t.Fatal("typed search must not be marked as a shortlist")
	}
	if got, want := len(results.Results), 5; got != want {
		t.Fatalf("typed results = %d, want %d: %+v", got, want, results.Results)
	}
	wantCategories := []ListCategoryID{
		FinderCategoryOperations,
		FinderCategoryTasks,
		FinderCategoryProjects,
		FinderCategoryNotes,
		FinderCategoryTags,
	}
	for index, want := range wantCategories {
		if got := results.Results[index].Item.CategoryID; got != want {
			t.Errorf("result %d category = %s, want %s", index, got, want)
		}
	}
}

func TestFinderRecordDestinationUsesProjectQualifiedMachineHistory(t *testing.T) {
	alpha := &models.Project{ID: "alpha", Name: "Alpha"}
	beta := &models.Project{ID: "beta", Name: "Beta"}
	alpha.Tasks = []*models.Task{{ID: "shared", Title: "Alpha task", WorkflowState: models.StateReady}}
	beta.Tasks = []*models.Task{{ID: "shared", Title: "Beta task", WorkflowState: models.StateReady}}
	machine := config.NewMachineState()
	finder, err := NewFinder(machine, FinderTaskProvider{Projects: []*models.Project{alpha, beta}})
	if err != nil {
		t.Fatalf("NewFinder: %v", err)
	}

	betaID := QualifiedTaskListItemID(beta.ID, "shared")
	if !finder.RecordDestination(betaID) {
		t.Fatal("RecordDestination returned false")
	}
	recents := machine.GetRecentDestinations()
	if len(recents) != 1 {
		t.Fatalf("recents = %+v, want one", recents)
	}
	if got := recents[0].QualifiedID(); got != string(betaID) {
		t.Fatalf("qualified recent = %q, want %q", got, betaID)
	}
	if finder.RecordDestination("operation:missing") {
		t.Fatal("unknown operation must not be recorded as a destination")
	}
}

func registerFinderTestOperation(
	t *testing.T,
	registry *OperationRegistry,
	id OperationID,
	label string,
) {
	t.Helper()
	err := registry.Register(OperationDescriptor{
		ID:                id,
		Label:             label,
		Category:          OperationCategoryNavigation,
		TargetCardinality: TargetCardinalityNone,
		Risk:              OperationRiskReadOnly,
	})
	if err != nil {
		t.Fatalf("register operation %s: %v", id, err)
	}
}
