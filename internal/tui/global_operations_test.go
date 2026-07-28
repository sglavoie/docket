package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func productionGlobalDescriptors() []OperationDescriptor {
	return append(globalToolDescriptors(), globalActionDescriptors()...)
}

// TestGlobalOperationCatalogIsRegisteredApplicableAndExecutable is the
// completeness gate for global destinations: an entry that reaches the catalog
// but is never registered, never applicable, or never reaches a route would
// advertise a destination Finder cannot actually open.
func TestGlobalOperationCatalogIsRegisteredApplicableAndExecutable(t *testing.T) {
	app := newUITestApp()
	setTestAppProjects(app, &models.Project{ID: "alpha", Name: "Alpha", Slug: "alpha"})
	context := app.operationCommandContext()

	for _, entry := range productionGlobalDescriptors() {
		descriptor, registered := app.operationRegistry.Descriptor(entry.ID)
		if !registered {
			t.Errorf("global operation %s is not registered in the production registry", entry.ID)
			continue
		}
		if !descriptor.Applicable(context) {
			t.Errorf("global operation %s is never applicable in the Outliner context", entry.ID)
			continue
		}
		result := app.operationExecutor.Execute(OperationExecutionRequest{
			ID:      entry.ID,
			Context: context,
			Source:  OperationSourceFinder,
		})
		if result.Err != nil {
			t.Errorf("execute %s: %v", entry.ID, result.Err)
			continue
		}
		if result.Message == nil {
			t.Errorf("global operation %s reached no typed route activation", entry.ID)
		}
	}
}

// TestGlobalOperationToolsUseFrozenScopeNotCursor pins the rule that a tool
// opened at base scope stays global even when the cursor sits on a project's
// task, and that project focus is what promotes it to project scope.
func TestGlobalOperationToolsUseFrozenScopeNotCursor(t *testing.T) {
	alpha := &models.Project{ID: "alpha", Name: "Alpha", Slug: "alpha"}
	beta := &models.Project{ID: "beta", Name: "Beta", Slug: "beta"}

	app := newUITestApp()
	setTestAppProjects(app, alpha, beta)
	app.crossOutliner.loadedProjects = []*models.Project{alpha, beta}

	base := app.operationCommandContext()
	if base.FocusedProjectID != "" {
		t.Fatalf("base scope FocusedProjectID = %q, want empty", base.FocusedProjectID)
	}
	base.Entity = EntityContext{ProjectID: alpha.ID, TaskID: "alpha-task"}
	base.CurrentEntity = commandContextCurrentEntity(base.Entity)

	if got := app.globalOperationMessage(operationToolNotes, base); !isType[ShowGlobalNotesMsg](got) {
		t.Errorf("base scope Notes = %T, want ShowGlobalNotesMsg", got)
	}
	if got := app.globalOperationMessage(operationToolBlockers, base); !isType[ShowGlobalBlockersMsg](got) {
		t.Errorf("base scope Blockers = %T, want ShowGlobalBlockersMsg", got)
	}
	if got := app.globalOperationMessage(operationToolActivity, base); !isType[ShowRecentActivityMsg](got) {
		t.Errorf("base scope Activity = %T, want ShowRecentActivityMsg", got)
	}
	if got, ok := app.globalOperationMessage(operationToolMetrics, base).(ShowMetricsDashboardMsg); !ok || got.Project != nil {
		t.Errorf("base scope Metrics = %#v, want global dashboard", got)
	}
	if got, ok := app.globalOperationMessage(operationToolRoadmap, base).(ShowRoadmapMsg); !ok || got.Project != nil {
		t.Errorf("base scope Roadmap = %#v, want global roadmap", got)
	}
	if got, ok := app.globalOperationMessage(operationToolScoringProfiles, base).(ShowScoreProfileManagerMsg); !ok ||
		got.ProjectID != "" {
		t.Errorf("base scope Scoring Profiles = %#v, want app scope", got)
	}

	app.crossOutliner.scopedProjectID = beta.ID
	focused := app.operationCommandContext()
	if focused.FocusedProjectID != beta.ID {
		t.Fatalf("project focus FocusedProjectID = %q, want %q", focused.FocusedProjectID, beta.ID)
	}
	if got, ok := app.globalOperationMessage(operationToolNotes, focused).(ShowNotesMsg); !ok || got.Project != beta {
		t.Errorf("project focus Notes = %#v, want project %s", got, beta.ID)
	}
	if got, ok := app.globalOperationMessage(operationToolBlockers, focused).(ShowBlockersMsg); !ok || got.Project != beta {
		t.Errorf("project focus Blockers = %#v, want project %s", got, beta.ID)
	}
	if got, ok := app.globalOperationMessage(operationToolActivity, focused).(ShowProjectRecentActivityMsg); !ok ||
		got.Project != beta {
		t.Errorf("project focus Activity = %#v, want project %s", got, beta.ID)
	}
	if got, ok := app.globalOperationMessage(operationToolMetrics, focused).(ShowMetricsDashboardMsg); !ok ||
		got.Project != beta {
		t.Errorf("project focus Metrics = %#v, want project %s", got, beta.ID)
	}
	if got, ok := app.globalOperationMessage(operationToolScoringProfiles, focused).(ShowScoreProfileManagerMsg); !ok ||
		got.ProjectID != beta.ID {
		t.Errorf("project focus Scoring Profiles = %#v, want project %s", got, beta.ID)
	}
}

// TestFinderToolRowsProjectEveryToolDescriptorExactlyOnce proves Finder derives
// its Tools category from the registry and that no descriptor can reach both
// discovery categories or leak a harness deep link.
func TestFinderToolRowsProjectEveryToolDescriptorExactlyOnce(t *testing.T) {
	app := newUITestApp()
	setTestAppProjects(app, &models.Project{ID: "alpha", Name: "Alpha", Slug: "alpha"})
	context := app.operationCommandContext()
	context.Capabilities.Add(CapabilityHarness)

	toolItems, err := (FinderToolProvider{
		Tools: globalToolCatalog(app.operationRegistry, context),
	}).FinderItems()
	if err != nil {
		t.Fatalf("tool FinderItems: %v", err)
	}
	operationItems, err := (FinderOperationProvider{
		Registry: app.operationRegistry,
		Context:  context,
	}).FinderItems()
	if err != nil {
		t.Fatalf("operation FinderItems: %v", err)
	}

	counts := make(map[ListItemID]int, len(toolItems))
	for _, item := range toolItems {
		if item.CategoryID != FinderCategoryTools {
			t.Errorf("tool row %s category = %s, want %s", item.ID, item.CategoryID, FinderCategoryTools)
		}
		counts[item.ID]++
	}
	for _, descriptor := range globalToolDescriptors() {
		switch counts[ListItemID("tool:"+string(descriptor.ID))] {
		case 1:
		case 0:
			t.Errorf("tool descriptor %s is missing from the Finder Tools category", descriptor.ID)
		default:
			t.Errorf("tool descriptor %s appears more than once in the Finder Tools category", descriptor.ID)
		}
	}

	toolLabels := make(map[string]bool, len(toolItems))
	for _, item := range toolItems {
		toolLabels[item.PrimaryText] = true
	}
	operationIDs := make(map[OperationID]bool, len(operationItems))
	for _, item := range operationItems {
		if toolLabels[item.PrimaryText] {
			t.Errorf("operation row %q duplicates a Finder tool of the same name", item.PrimaryText)
		}
		for _, id := range item.OperationIDs {
			operationIDs[id] = true
		}
	}
	for _, descriptor := range app.operationRegistry.All() {
		if descriptor.RequiredCapabilities.Has(CapabilityHarness) &&
			(operationIDs[descriptor.ID] || counts[ListItemID("tool:"+string(descriptor.ID))] > 0) {
			t.Errorf("harness operation %s leaked into Finder discovery", descriptor.ID)
		}
		if descriptor.Placement != PlacementFinderTool {
			continue
		}
		if operationIDs[descriptor.ID] {
			t.Errorf("tool descriptor %s is also listed as a Finder operation", descriptor.ID)
		}
	}
}

// TestGlobalOperationSandboxResetDelegatesToTheConfirmingRoute keeps sandbox
// reset destructive while leaving the single confirmation with the route that
// can name the exact item count.
func TestGlobalOperationSandboxResetDelegatesToTheConfirmingRoute(t *testing.T) {
	app := newUITestApp()
	descriptor, ok := app.operationRegistry.Descriptor(operationGlobalSandboxReset)
	if !ok {
		t.Fatal("sandbox reset is not registered")
	}
	if descriptor.Risk != OperationRiskDestructive {
		t.Fatalf("sandbox reset risk = %s, want %s", descriptor.Risk, OperationRiskDestructive)
	}

	result := app.operationExecutor.Execute(OperationExecutionRequest{
		ID:      operationGlobalSandboxReset,
		Context: app.operationCommandContext(),
		Source:  OperationSourceFinder,
	})
	if result.Err != nil {
		t.Fatalf("execute sandbox reset: %v", result.Err)
	}
	if result.ConfirmationRequired != nil {
		t.Fatal("sandbox reset must not raise a second generic confirmation")
	}
	if !isType[SandboxResetRequestMsg](result.Message) {
		t.Fatalf("sandbox reset message = %T, want SandboxResetRequestMsg", result.Message)
	}
}

func isType[T any](value any) bool {
	_, ok := value.(T)
	return ok
}
