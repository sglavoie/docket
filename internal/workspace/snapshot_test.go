package workspace

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func makeTask(id string, shortID int, dependsOn ...string) *models.Task {
	return &models.Task{
		ID:        id,
		ShortID:   shortID,
		Title:     "task-" + id,
		DependsOn: dependsOn,
	}
}

func makeProject(id, slug string, tasks ...*models.Task) *models.Project {
	return &models.Project{
		ID:    id,
		Slug:  slug,
		Name:  slug,
		Tasks: tasks,
	}
}

// buildFromProjects builds a snapshot without touching the filesystem by
// injecting an in-memory loader whose projects are already populated.
func buildFromProjects(t *testing.T, projects ...*models.Project) *Snapshot {
	t.Helper()
	loader := &Loader{
		loadProject: func(*models.Project) error { return nil },
		revisionOf:  func(p *models.Project) string { return "rev-" + p.ID },
		statSource:  func(*models.Project) sourceIdentity { return sourceIdentity{exists: false} },
		cache:       make(map[string]cacheEntry),
	}
	return loader.Build(projects)
}

func TestSnapshotIndexesTasksAndRevisions(t *testing.T) {
	a := makeTask("a1", 1)
	b := makeTask("b1", 1) // same short id in a different project
	snap := buildFromProjects(t,
		makeProject("proj-a", "alpha", a),
		makeProject("proj-b", "beta", b),
	)

	if got := len(snap.Tasks()); got != 2 {
		t.Fatalf("Tasks() = %d, want 2", got)
	}
	if got := len(snap.Projects()); got != 2 {
		t.Fatalf("Projects() = %d, want 2", got)
	}

	loc, ok := snap.Lookup("a1")
	if !ok || loc.Task != a || loc.Project.ID != "proj-a" {
		t.Fatalf("Lookup(a1) = %+v, ok=%v", loc, ok)
	}
	if _, ok := snap.Lookup(""); ok {
		t.Fatal("Lookup(\"\") should never match")
	}
	if _, ok := snap.Lookup("missing"); ok {
		t.Fatal("Lookup(missing) should not match")
	}

	if got := snap.Revision("proj-a"); got != "rev-proj-a" {
		t.Fatalf("Revision(proj-a) = %q, want rev-proj-a", got)
	}
	if got := snap.Revision("nope"); got != "" {
		t.Fatalf("Revision(nope) = %q, want empty", got)
	}

	if got := snap.QualifiedRef("b1"); got != "beta:#1" {
		t.Fatalf("QualifiedRef(b1) = %q, want beta:#1", got)
	}
}

func TestSnapshotReverseDependencies(t *testing.T) {
	// c depends on a; b depends on a.
	a := makeTask("a", 1)
	b := makeTask("b", 2, "a")
	c := makeTask("c", 3, "a")
	snap := buildFromProjects(t, makeProject("p", "p", a, b, c))

	got := snap.ReverseDependencies("a")
	if len(got) != 2 {
		t.Fatalf("ReverseDependencies(a) = %v, want 2 dependents", got)
	}
	set := map[string]bool{got[0]: true, got[1]: true}
	if !set["b"] || !set["c"] {
		t.Fatalf("ReverseDependencies(a) = %v, want b and c", got)
	}
	if none := snap.ReverseDependencies("b"); none != nil {
		t.Fatalf("ReverseDependencies(b) = %v, want nil", none)
	}

	// Mutating the returned slice must not corrupt the snapshot index.
	got[0] = "mutated"
	if again := snap.ReverseDependencies("a"); again[0] == "mutated" || again[1] == "mutated" {
		t.Fatal("ReverseDependencies must return a defensive copy")
	}
}

func TestSnapshotDuplicateTaskIDKeepsFirst(t *testing.T) {
	first := makeTask("dup", 1)
	second := makeTask("dup", 9)
	snap := buildFromProjects(t,
		makeProject("proj-a", "alpha", first),
		makeProject("proj-b", "beta", second),
	)
	loc, ok := snap.Lookup("dup")
	if !ok || loc.Task != first || loc.Project.ID != "proj-a" {
		t.Fatalf("Lookup(dup) should resolve to the first-seen task, got %+v", loc)
	}
}

func TestDiagnosticString(t *testing.T) {
	d := Diagnostic{ProjectName: "alpha", Kind: DiagnosticCorrupt, Detail: "bad markdown"}
	if got := d.String(); got != "corrupt project alpha: bad markdown" {
		t.Fatalf("String() = %q", got)
	}
	d = Diagnostic{ProjectID: "id-only", Kind: DiagnosticSkipped, Detail: "unreadable"}
	if got := d.String(); got != "skipped project id-only: unreadable" {
		t.Fatalf("String() = %q", got)
	}
}
