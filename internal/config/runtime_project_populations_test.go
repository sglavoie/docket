package config

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// populationRuntime builds an in-memory snapshot holding one pinned project,
// one plain project, one project carrying the reserved parked tag, one archived
// project, and the protected Unfiled project.
func populationRuntime(t *testing.T) *RuntimeState {
	t.Helper()
	catalog := NewWorkspaceCatalog()
	catalog.SystemUnfiledProjectID = "unfiled"
	catalog.Projects = []CatalogProject{
		{ID: "unfiled", Slug: "unfiled", Name: SystemUnfiledName, ColorSlot: 1},
		{ID: "plain", Slug: "plain", Name: "Plain", ColorSlot: 2},
		{ID: "parked", Slug: "parked", Name: "Parked", ColorSlot: 3, Tags: []string{"client", models.ParkedProjectTag}},
		{ID: "archived", Slug: "archived", Name: "Archived", ColorSlot: 4, Archived: true},
		{ID: "pinned", Slug: "pinned", Name: "Pinned", ColorSlot: 5, Pinned: true},
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

// The empty tag set is legal and is what most projects carry, so an untagged
// catalog must not lose projects from the base task workspace.
func TestTaskWorkspaceProjectsAdmitsUntaggedProjects(t *testing.T) {
	rs := NewRuntimeState(t.TempDir(), &Domains{
		Catalog: &WorkspaceCatalog{
			Projects: []CatalogProject{{ID: "legacy", Slug: "legacy", Name: "Legacy"}},
		},
		Planning: NewPlanningState(),
		Machine:  NewMachineState(),
	})
	assertProjectIDs(t, "TaskWorkspaceProjects", rs.TaskWorkspaceProjects(), []string{"legacy"})
}

// Project tags are emergent, so AllProjectTags derives the entire namespace
// from the catalog. Every cataloged project contributes, archived ones
// included, because the vocabulary describes what exists rather than what a
// projection currently shows.
func TestAllProjectTagsIsTheSortedCatalogVocabulary(t *testing.T) {
	rs := NewRuntimeState(t.TempDir(), &Domains{
		Catalog: &WorkspaceCatalog{
			Projects: []CatalogProject{
				{ID: "a", Slug: "a", Name: "A", Tags: []string{"client", models.ParkedProjectTag}},
				{ID: "b", Slug: "b", Name: "B", Tags: []string{"billable", "client"}},
				{ID: "c", Slug: "c", Name: "C"},
				{ID: "d", Slug: "d", Name: "D", Archived: true, Tags: []string{"retired"}},
			},
		},
		Planning: NewPlanningState(),
		Machine:  NewMachineState(),
	})
	want := []string{"billable", "client", models.ParkedProjectTag, "retired"}
	if got := rs.AllProjectTags(); !slices.Equal(got, want) {
		t.Fatalf("AllProjectTags = %v, want %v", got, want)
	}
}

// The empty tag set is legal, so an untagged catalog yields an empty
// vocabulary instead of a defaulted one.
func TestAllProjectTagsIsEmptyForAnUntaggedCatalog(t *testing.T) {
	rs := NewRuntimeState(t.TempDir(), &Domains{
		Catalog: &WorkspaceCatalog{
			Projects: []CatalogProject{{ID: "legacy", Slug: "legacy", Name: "Legacy"}},
		},
		Planning: NewPlanningState(),
		Machine:  NewMachineState(),
	})
	if got := rs.AllProjectTags(); len(got) != 0 {
		t.Fatalf("AllProjectTags = %v, want no tags", got)
	}
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
	catalog.Projects = append(catalog.Projects,
		CatalogProject{ID: "parked", Slug: "parked", Name: "Parked", ColorSlot: 1, Tags: []string{models.ParkedProjectTag}},
		CatalogProject{ID: "archived", Slug: "archived", Name: "Archived", ColorSlot: 2, Archived: true},
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
