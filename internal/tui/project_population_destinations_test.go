package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

const testParkedBoundaryID = "55555555-5555-4555-8555-555555555555"

// parkedPopulationCatalog holds one Active project, one project parked on a
// custom boundary, and one archived project.
func parkedPopulationCatalog() *config.WorkspaceCatalog {
	catalog := config.NewWorkspaceCatalog()
	catalog.Boundaries = append(catalog.Boundaries, config.ProjectBoundary{ID: testParkedBoundaryID, Name: "On Hold"})
	catalog.Projects = []config.CatalogProject{
		{ID: "active", Slug: "active", Name: "Active Work", ColorSlot: 1, BoundaryID: models.ActiveBoundaryID},
		{ID: "parked", Slug: "parked", Name: "Parked Work", ColorSlot: 2, BoundaryID: testParkedBoundaryID},
		{ID: "archived", Slug: "archived", Name: "Archived Work", ColorSlot: 3, BoundaryID: models.ActiveBoundaryID, Archived: true},
	}
	return catalog
}

func parkedPopulationApp(t *testing.T) *AppModel {
	t.Helper()
	app := newTestAppModel()
	app.runtime.SetCatalogInMemory(parkedPopulationCatalog())
	app.syncProjectCache()
	return app
}

func projectIDsOf(projects []*models.Project) []string {
	ids := make([]string, 0, len(projects))
	for _, project := range projects {
		ids = append(ids, project.ID)
	}
	return ids
}

// A parked project keeps its tasks and notes on disk, so every destination that
// can send work to it must keep listing it. Archived projects stay excluded.
func TestParkedProjectRemainsATaskMoveAndCaptureDestination(t *testing.T) {
	app := parkedPopulationApp(t)

	if got := projectIDsOf(app.availableProjects()); !slices.Equal(got, []string{"active", "parked"}) {
		t.Fatalf("availableProjects = %v, want [active parked]", got)
	}

	candidates := projectIDsOf(app.moveProjectCandidatesForSourceIDs(map[string]bool{"active": true}))
	if !slices.Equal(candidates, []string{"parked"}) {
		t.Fatalf("move destinations from active = %v, want [parked]", candidates)
	}

	preset, ok := app.captureComposerPreset(app.projectByID("active"), nil, "")
	if !ok {
		t.Fatal("captureComposerPreset did not build a preset")
	}
	if got := projectIDsOf(preset.AllProjects); !slices.Equal(got, []string{"active", "parked"}) {
		t.Fatalf("composer completion projects = %v, want [active parked]", got)
	}

	parsed, err := parseQuickCapture("%parked ship it", preset.Project, preset.AllProjects, false)
	if err != nil {
		t.Fatalf("capture into a parked project must succeed: %v", err)
	}
	if parsed.targetProjectID == nil || *parsed.targetProjectID != "parked" {
		t.Fatalf("%%project resolved to %v, want parked", parsed.targetProjectID)
	}

	if _, err := parseQuickCapture("%archived ship it", preset.Project, preset.AllProjects, false); err == nil {
		t.Fatal("a project sigil must not resolve an archived project")
	}
}

// Notes owners and the project-note command scope read the parked-inclusive
// population, so a parked project stays a note owner.
func TestNotesScopeIncludesParkedOwnersAndExcludesArchived(t *testing.T) {
	runtime := config.NewRuntimeState(t.TempDir(), config.NewDomains())
	runtime.SetCatalogInMemory(parkedPopulationCatalog())

	hub := NewManageHubModel(ManageHubDependencies{Runtime: runtime})
	if got := hub.ScopeProjectIDs(); !slices.Equal(got, []string{"active", "parked"}) {
		t.Fatalf("Notes scope project IDs = %v, want [active parked]", got)
	}
}

// A destination picker lists catalog projects whose task trees were never
// loaded. Counting them yields zero, which would read as "empty" rather than
// "not counted".
func TestDestinationPickerMarksUnloadedTaskCountUnknown(t *testing.T) {
	unloaded := &models.Project{ID: "parked", Slug: "parked", Name: "Parked Work", Context: "personal"}
	loaded := &models.Project{
		ID: "active", Slug: "active", Name: "Active Work", Context: "personal",
		Tasks: []*models.Task{{ID: "t1", Title: "One"}},
	}

	source := ListSourceFromProjects([]*models.Project{loaded, unloaded})
	if len(source.Items) != 2 {
		t.Fatalf("adapted items = %d, want 2", len(source.Items))
	}
	if got := source.Items[0].SecondaryText; !strings.Contains(got, "1 tasks") {
		t.Fatalf("loaded destination secondary = %q, want a real count", got)
	}
	if got := source.Items[1].SecondaryText; !strings.Contains(got, "unknown tasks") {
		t.Fatalf("unloaded destination secondary = %q, want an unknown count", got)
	}
	if got := source.Items[1].Preview.Fields[1].Value; got != "unknown" {
		t.Fatalf("unloaded destination preview task count = %q, want unknown", got)
	}

	if got := (ProjectPickerItem{Project: unloaded}).CountDisplayText(); got != "(unknown)" {
		t.Fatalf("unloaded picker count = %q, want (unknown)", got)
	}
	if got := (ProjectPickerItem{Project: loaded}).CountDisplayText(); got != "(1)" {
		t.Fatalf("loaded picker count = %q, want (1)", got)
	}
}
