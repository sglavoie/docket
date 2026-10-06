package config

import (
	"slices"
	"strings"
	"testing"
	"time"
)

const reconcileWorkspaceID = "806a60c1-081b-420d-a4a8-7733b6201f71"

func reconcileCatalog(t *testing.T, edit func(c *WorkspaceCatalog)) []byte {
	t.Helper()
	stamp := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	c := NewWorkspaceCatalog()
	c.WorkspaceID = reconcileWorkspaceID
	c.Projects = []CatalogProject{
		{ID: "unfiled-id", Slug: "unfiled", Name: "Unfiled", CreatedAt: stamp, UpdatedAt: stamp, ColorSlot: 1},
		{ID: "p1", Slug: "alpha", Name: "Alpha", CreatedAt: stamp, UpdatedAt: stamp, ColorSlot: 2},
	}
	c.SystemUnfiledProjectID = "unfiled-id"
	if edit != nil {
		edit(c)
	}
	data, err := encodeDomainEnvelope(domainCatalog, reconcileWorkspaceID, c)
	if err != nil {
		t.Fatalf("encode catalog: %v", err)
	}
	return data
}

func decodeReconciledCatalog(t *testing.T, data []byte) *WorkspaceCatalog {
	t.Helper()
	c := &WorkspaceCatalog{}
	if _, err := decodeSyncedArtifact(domainCatalog, data, c); err != nil {
		t.Fatal(err)
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("reconciled catalog is invalid: %v", err)
	}
	return c
}

func slugOf(c *WorkspaceCatalog, id string) string {
	for _, p := range c.Projects {
		if p.ID == id {
			return p.Slug
		}
	}
	return ""
}

func newProject(id, slug string) CatalogProject {
	stamp := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	return CatalogProject{ID: id, Slug: slug, Name: slug, CreatedAt: stamp, UpdatedAt: stamp, ColorSlot: 3}
}

// TestReconcileCatalogArtifactsKeepsSlugsUnique reverts a rename onto a slug
// a concurrent creation took, and suffixes the second of two projects created
// concurrently under one slug.
func TestReconcileCatalogArtifactsKeepsSlugsUnique(t *testing.T) {
	base := reconcileCatalog(t, nil)
	a := reconcileCatalog(t, func(c *WorkspaceCatalog) {
		c.Projects[1].Slug = "gamma"
		c.Projects = append(c.Projects, newProject("p3", "delta"))
	})
	b := reconcileCatalog(t, func(c *WorkspaceCatalog) {
		c.Projects = append(c.Projects, newProject("p2", "gamma"), newProject("p4", "delta"))
	})
	merged, conflicts, err := ReconcileCatalogArtifacts(base, [][]byte{a, b})
	if err != nil {
		t.Fatal(err)
	}
	c := decodeReconciledCatalog(t, merged)
	if slugOf(c, "p1") != "alpha" || slugOf(c, "p2") != "gamma" {
		t.Fatalf("slugs p1=%q p2=%q, want the colliding rename reverted", slugOf(c, "p1"), slugOf(c, "p2"))
	}
	if slugOf(c, "p3") != "delta" || slugOf(c, "p4") != "delta-p4" {
		t.Fatalf("slugs p3=%q p4=%q, want the larger id suffixed", slugOf(c, "p3"), slugOf(c, "p4"))
	}
	if len(conflicts) != 2 {
		t.Fatalf("conflicts = %+v, want the reverted rename and the suffixed creation", conflicts)
	}
}

// TestReconcileCatalogArtifactsKeepsChangedProjectOverDeletion keeps a project
// one side renamed while the other deleted it, and restores a deleted entry.
func TestReconcileCatalogArtifactsKeepsChangedProjectOverDeletion(t *testing.T) {
	base := reconcileCatalog(t, nil)
	deleted := reconcileCatalog(t, func(c *WorkspaceCatalog) { c.Projects = c.Projects[:1] })
	renamed := reconcileCatalog(t, func(c *WorkspaceCatalog) { c.Projects[1].Name = "Alpha Prime" })
	merged, conflicts, err := ReconcileCatalogArtifacts(base, [][]byte{deleted, renamed})
	if err != nil {
		t.Fatal(err)
	}
	c := decodeReconciledCatalog(t, merged)
	if len(c.Projects) != 2 || c.Projects[1].Name != "Alpha Prime" {
		t.Fatalf("projects = %+v, want the renamed project kept", c.Projects)
	}
	if len(conflicts) != 1 || !strings.Contains(conflicts[0].Detail, "kept") {
		t.Fatalf("conflicts = %+v", conflicts)
	}

	restored, ok, err := RestoreCatalogProject(deleted, "p1", base)
	if err != nil || !ok {
		t.Fatalf("RestoreCatalogProject = %v, %v", ok, err)
	}
	if slugOf(decodeReconciledCatalog(t, restored), "p1") != "alpha" {
		t.Fatal("the deleted entry was not restored")
	}
	if again, ok, _ := RestoreCatalogProject(restored, "p1", base); ok || string(again) != string(restored) {
		t.Fatal("restoring a named project changed the catalog")
	}
}

// TestReconcilePlanningArtifactsMergesMembership merges Today and Next as set
// changes against the base and keeps one task out of both collections.
func TestReconcilePlanningArtifactsMergesMembership(t *testing.T) {
	loc := func(id string) ProjectTaskLocator { return ProjectTaskLocator{ProjectID: "p1", TaskID: id} }
	encode := func(today, next []ProjectTaskLocator) []byte {
		data, err := encodeDomainEnvelope(domainPlanning, reconcileWorkspaceID, &PlanningState{Today: today, Next: next})
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	base := encode([]ProjectTaskLocator{loc("x")}, []ProjectTaskLocator{loc("y")})
	a := encode([]ProjectTaskLocator{loc("x"), loc("y"), loc("q")}, []ProjectTaskLocator{loc("z")})
	b := encode([]ProjectTaskLocator{loc("w")}, []ProjectTaskLocator{loc("y"), loc("q")})
	merged, conflicts, err := ReconcilePlanningArtifacts(base, [][]byte{a, b})
	if err != nil {
		t.Fatal(err)
	}
	p := &PlanningState{}
	if _, err := decodeSyncedArtifact(domainPlanning, merged, p); err != nil {
		t.Fatal(err)
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("reconciled planning is invalid: %v", err)
	}
	if want := []ProjectTaskLocator{loc("y"), loc("q"), loc("w")}; !slices.Equal(p.Today, want) {
		t.Fatalf("Today = %+v, want %+v", p.Today, want)
	}
	if want := []ProjectTaskLocator{loc("z")}; !slices.Equal(p.Next, want) {
		t.Fatalf("Next = %+v, want %+v", p.Next, want)
	}
	if len(conflicts) != 1 || conflicts[0].Field != "collection" {
		t.Fatalf("conflicts = %+v, want the Today/Next collision", conflicts)
	}
}
