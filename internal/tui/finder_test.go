package tui

import (
	"fmt"
	"testing"

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
	filter := config.SavedFilter{ID: "filter", Name: "My filter", Mode: "snapshot"}
	mode := OutlinerModeCatalog()[0]
	scope := ContextScopeSummary{Scope: AllContextScope(), ActiveProjectCount: 2}

	finder, err := NewFinder(
		config.NewMachineState(),
		FinderOperationProvider{Registry: registry, Context: CommandContext{}},
		FinderTaskProvider{Projects: []*models.Project{beta, alpha}},
		FinderProjectProvider{Projects: []*models.Project{beta, alpha}},
		FinderNoteProvider{Notes: []FinderNote{{Note: note, Project: alpha}}}, FinderToolProvider{Tools: []FinderTool{{ID: "settings", Label: "Settings"}}}, FinderModeProvider{Modes: []OutlinerMode{mode}},
		FinderScopeProvider{Scopes: []ContextScopeSummary{scope}},
		FinderTagProvider{Tags: []TagInfo{{Name: "focus", TaskCount: 2}}}, FinderSavedFilterProvider{Filters: []config.SavedFilter{filter}},
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
		"saved-filter:filter",
	} {
		if !seenIDs[id] {
			t.Errorf("missing finder item %s", id)
		}
	}
}

func TestFinderSimplifiedContextExcludesDormantCompatibilityOperations(t *testing.T) {
	registry := NewOperationRegistry()
	for _, descriptor := range coreInputOperationDescriptors() {
		if err := registry.Register(descriptor); err != nil {
			t.Fatalf("register core operation %s: %v", descriptor.ID, err)
		}
	}
	registerFinderTestOperation(t, registry, "legacy-command", "Global Inbox Processor")
	if err := registry.RegisterInputStrategy(SimplifiedInputStrategy()); err != nil {
		t.Fatalf("register simplified strategy: %v", err)
	}

	ctx := PresentationCommandContext(ViewCrossProjectOutliner)
	ctx.Capabilities.Add(CapabilitySimplifiedInput)
	items, err := (FinderOperationProvider{Registry: registry, Context: ctx}).FinderItems()
	if err != nil {
		t.Fatalf("FinderItems: %v", err)
	}
	if got, want := len(items), len(SimplifiedInputStrategy().Bindings); got != want {
		t.Fatalf("simplified Finder operations = %d, want %d", got, want)
	}
	for _, item := range items {
		if item.PrimaryText == "Global Inbox Processor" || item.ID == "operation:legacy-command" {
			t.Fatalf("Finder exposed dormant compatibility operation: %+v", item)
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
	for _, tool := range finderTools() {
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
		FinderSavedFilterProvider{Filters: []config.SavedFilter{{ID: "needle", Name: "Needle"}}}, FinderTagProvider{Tags: []TagInfo{{Name: "Needle", TaskCount: 1}}}, FinderNoteProvider{Notes: []FinderNote{{Note: note, Project: project}}}, FinderProjectProvider{Projects: []*models.Project{project}},
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
	if got, want := len(results.Results), 6; got != want {
		t.Fatalf("typed results = %d, want %d: %+v", got, want, results.Results)
	}
	wantCategories := []ListCategoryID{
		FinderCategoryOperations,
		FinderCategoryTasks,
		FinderCategoryProjects,
		FinderCategoryNotes,
		FinderCategoryTags,
		FinderCategorySavedFilters,
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
