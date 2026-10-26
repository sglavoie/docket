package workspace

import (
	"testing"

	"github.com/sglavoie/docket/internal/models"
)

func makeTask(id string, shortID int) *models.Task {
	return &models.Task{
		ID:      id,
		ShortID: shortID,
		Title:   "task-" + id,
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
		loadProject: func(p *models.Project) (string, error) { return "rev-" + p.ID, nil },
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

	loc, status := snap.LookupTaskID("a1")
	if status != TaskLookupUnique || loc.Task != a || loc.Project.ID != "proj-a" {
		t.Fatalf("LookupTaskID(a1) = %+v, status=%v", loc, status)
	}
	if _, status := snap.LookupTaskID(""); status != TaskLookupMissing {
		t.Fatal("LookupTaskID(\"\") should be missing")
	}
	if _, status := snap.LookupTaskID("missing"); status != TaskLookupMissing {
		t.Fatal("LookupTaskID(missing) should be missing")
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

func TestSnapshotDuplicateTaskIDIsAmbiguous(t *testing.T) {
	first := makeTask("dup", 1)
	second := makeTask("dup", 9)
	snap := buildFromProjects(t,
		makeProject("proj-a", "alpha", first),
		makeProject("proj-b", "beta", second),
	)
	loc, status := snap.LookupTaskID("dup")
	if status != TaskLookupAmbiguous || loc != (TaskLocation{}) {
		t.Fatalf("LookupTaskID(dup) = %+v, status=%v; want empty ambiguous result", loc, status)
	}
	if got := snap.QualifiedRef("dup"); got != "" {
		t.Fatalf("QualifiedRef(dup) = %q, want empty for ambiguous id", got)
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
