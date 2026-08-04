package config

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// populationRuntime builds an in-memory snapshot holding one pinned Active
// project, one plain Active project, one project parked on a custom boundary,
// one archived project, and the protected Unfiled project.
func populationRuntime(t *testing.T) *RuntimeState {
	t.Helper()
	catalog := NewWorkspaceCatalog()
	catalog.Boundaries = append(catalog.Boundaries, ProjectBoundary{ID: testCustomBoundaryID, Name: "On Hold"})
	catalog.SystemUnfiledProjectID = "unfiled"
	catalog.Projects = []CatalogProject{
		{ID: "unfiled", Slug: "unfiled", Name: SystemUnfiledName, ColorSlot: 1, BoundaryID: models.ActiveBoundaryID},
		{ID: "plain", Slug: "plain", Name: "Plain", ColorSlot: 2, BoundaryID: models.ActiveBoundaryID},
		{ID: "parked", Slug: "parked", Name: "Parked", ColorSlot: 3, BoundaryID: testCustomBoundaryID},
		{ID: "archived", Slug: "archived", Name: "Archived", ColorSlot: 4, BoundaryID: models.ActiveBoundaryID, Archived: true},
		{ID: "pinned", Slug: "pinned", Name: "Pinned", ColorSlot: 5, BoundaryID: models.ActiveBoundaryID, Pinned: true},
	}
	if err := catalog.Validate(); err != nil {
		t.Fatalf("population fixture must be a valid catalog: %v", err)
	}
	return NewRuntimeState(t.TempDir(), &Domains{
		Catalog:  catalog,
		Planning: NewPlanningState(),
		Machine:  NewMachineState(),
	})
}

func projectIDs(projects []*models.Project) []string {
	ids := make([]string, 0, len(projects))
	for _, project := range projects {
		ids = append(ids, project.ID)
	}
	return ids
}

func assertProjectIDs(t *testing.T, label string, got []*models.Project, want []string) {
	t.Helper()
	ids := projectIDs(got)
	if len(ids) != len(want) {
		t.Fatalf("%s = %v, want %v", label, ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("%s = %v, want %v", label, ids, want)
		}
	}
}

// AvailableProjects is what every destination that stays reachable for a parked
// project reads, so it must admit the parked project and reject only archived
// ones. Ordering is part of the contract: pinned first, protected Unfiled last.
func TestAvailableProjectsAdmitsParkedAndKeepsOrdering(t *testing.T) {
	rs := populationRuntime(t)
	assertProjectIDs(t, "AvailableProjects", rs.AvailableProjects(), []string{"pinned", "parked", "plain", "unfiled"})
}

// TaskWorkspaceProjects is the base task workspace, so a parked project must
// disappear from it while the same project remains available elsewhere.
func TestTaskWorkspaceProjectsExcludesParkedAndArchived(t *testing.T) {
	rs := populationRuntime(t)
	assertProjectIDs(t, "TaskWorkspaceProjects", rs.TaskWorkspaceProjects(), []string{"pinned", "plain", "unfiled"})
}

// A catalog written before boundaries existed carries no per-project boundary
// ID. Populations read the boundary through the catalog, which resolves an
// absent value to Active, so an unnormalized snapshot must not lose projects.
func TestTaskWorkspaceProjectsTreatsAbsentBoundaryAsActive(t *testing.T) {
	rs := NewRuntimeState(t.TempDir(), &Domains{
		Catalog: &WorkspaceCatalog{
			Projects: []CatalogProject{{ID: "legacy", Slug: "legacy", Name: "Legacy"}},
		},
		Planning: NewPlanningState(),
		Machine:  NewMachineState(),
	})
	assertProjectIDs(t, "TaskWorkspaceProjects", rs.TaskWorkspaceProjects(), []string{"legacy"})
}

func TestProjectLookupHelpersMatchTheirPopulations(t *testing.T) {
	rs := populationRuntime(t)

	if got := rs.AvailableProject("parked"); got == nil {
		t.Fatal("AvailableProject must resolve a parked project")
	}
	if got := rs.AvailableProject("archived"); got != nil {
		t.Fatalf("AvailableProject resolved an archived project: %q", got.ID)
	}
	if got := rs.TaskWorkspaceProject("parked"); got != nil {
		t.Fatalf("TaskWorkspaceProject resolved a parked project: %q", got.ID)
	}
	if got := rs.TaskWorkspaceProject("plain"); got == nil {
		t.Fatal("TaskWorkspaceProject must resolve an Active project")
	}
	if got := rs.TaskWorkspaceProject("plain-slug-that-does-not-exist"); got != nil {
		t.Fatalf("TaskWorkspaceProject resolved an absent project: %q", got.ID)
	}
	if got := rs.AvailableProject("parked"); got == nil || got.Slug != "parked" {
		t.Fatal("lookup helpers must accept a slug as well as an ID")
	}
}

func TestRuntimeDependenciesAvailableProjectsExcludesArchivedOnly(t *testing.T) {
	store := newIsolatedStore(t)
	catalog, err := store.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	catalog.Boundaries = append(catalog.Boundaries, ProjectBoundary{ID: testCustomBoundaryID, Name: "On Hold"})
	catalog.Projects = append(catalog.Projects,
		CatalogProject{ID: "parked", Slug: "parked", Name: "Parked", ColorSlot: 1, BoundaryID: testCustomBoundaryID},
		CatalogProject{ID: "archived", Slug: "archived", Name: "Archived", ColorSlot: 2, BoundaryID: models.ActiveBoundaryID, Archived: true},
	)
	if err = store.SaveCatalog(catalog); err != nil {
		t.Fatal(err)
	}

	projects, err := NewRuntimeDependencies(store).AvailableProjects(t.TempDir())
	if err != nil {
		t.Fatalf("AvailableProjects: %v", err)
	}
	assertProjectIDs(t, "RuntimeDependencies.AvailableProjects", projects, []string{"parked"})
}
