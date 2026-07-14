package workspace

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

// fakeSource is a scriptable stand-in for a project's tasks.md: it tracks how
// many times each project was actually parsed and lets a test change the source
// identity to simulate an on-disk edit.
type fakeSource struct {
	parses   map[string]int
	identity map[string]sourceIdentity
	loadErr  map[string]error
}

func newFakeSource() *fakeSource {
	return &fakeSource{
		parses:   map[string]int{},
		identity: map[string]sourceIdentity{},
		loadErr:  map[string]error{},
	}
}

func (f *fakeSource) newLoader() *Loader {
	return &Loader{
		loadProject: func(p *models.Project) error {
			if err := f.loadErr[p.ID]; err != nil {
				return err
			}
			f.parses[p.ID]++
			// Simulate a parse populating tasks.
			p.Tasks = []*models.Task{makeTask(p.ID+"-t", 1)}
			return nil
		},
		revisionOf: func(p *models.Project) string { return "rev-" + p.ID },
		statSource: func(p *models.Project) sourceIdentity {
			id, ok := f.identity[p.ID]
			if !ok {
				return sourceIdentity{exists: true, size: 1, modTime: baseTime}
			}
			return id
		},
		cache: make(map[string]cacheEntry),
	}
}

var baseTime = time.Unix(1_700_000_000, 0)

func TestLoaderReusesUnchangedProjects(t *testing.T) {
	src := newFakeSource()
	loader := src.newLoader()

	// config.GetActiveProjects() hands back fresh pointers each call; model that.
	fresh := func() []*models.Project {
		return []*models.Project{makeProject("p1", "p1"), makeProject("p2", "p2")}
	}

	first := loader.Build(fresh())
	if len(first.Projects()) != 2 {
		t.Fatalf("first build loaded %d projects, want 2", len(first.Projects()))
	}
	if loader.ParseCount() != 2 {
		t.Fatalf("first build ParseCount = %d, want 2", loader.ParseCount())
	}

	// Second build with unchanged sources must not reparse anything.
	second := loader.Build(fresh())
	if loader.ParseCount() != 2 {
		t.Fatalf("second build reparsed: ParseCount = %d, want 2", loader.ParseCount())
	}
	if loader.CacheHits() != 2 {
		t.Fatalf("second build CacheHits = %d, want 2", loader.CacheHits())
	}
	if len(second.Projects()) != 2 {
		t.Fatalf("second build loaded %d projects, want 2", len(second.Projects()))
	}
	// The cached parse must still expose the loaded tasks.
	if len(second.Tasks()) != 2 {
		t.Fatalf("second build Tasks = %d, want 2", len(second.Tasks()))
	}
}

func TestLoaderReparsesChangedProjectOnly(t *testing.T) {
	src := newFakeSource()
	loader := src.newLoader()
	fresh := func() []*models.Project {
		return []*models.Project{makeProject("p1", "p1"), makeProject("p2", "p2")}
	}

	loader.Build(fresh())
	if loader.ParseCount() != 2 {
		t.Fatalf("initial ParseCount = %d, want 2", loader.ParseCount())
	}

	// Simulate p2's tasks.md changing on disk (new mod time).
	src.identity["p2"] = sourceIdentity{exists: true, size: 5, modTime: baseTime.Add(time.Minute)}

	loader.Build(fresh())
	if loader.ParseCount() != 3 {
		t.Fatalf("after edit ParseCount = %d, want 3 (only p2 reparsed)", loader.ParseCount())
	}
	if loader.CacheHits() != 1 {
		t.Fatalf("after edit CacheHits = %d, want 1 (p1 reused)", loader.CacheHits())
	}
}

func TestLoaderRepresentative36ProjectWorkspaceParsesOnlyChanges(t *testing.T) {
	src := newFakeSource()
	loader := src.newLoader()
	const projectCount = 36

	fresh := func() []*models.Project {
		projects := make([]*models.Project, 0, projectCount)
		for i := range projectCount {
			id := fmt.Sprintf("p%02d", i)
			projects = append(projects, makeProject(id, id))
		}
		return projects
	}

	loader.Build(fresh())
	if got := loader.ParseCount(); got != projectCount {
		t.Fatalf("initial ParseCount = %d, want %d", got, projectCount)
	}

	loader.Build(fresh())
	if got := loader.ParseCount(); got != projectCount {
		t.Fatalf("unchanged workspace reparsed projects: ParseCount = %d, want %d", got, projectCount)
	}
	if got := loader.CacheHits(); got != projectCount {
		t.Fatalf("unchanged workspace CacheHits = %d, want %d", got, projectCount)
	}

	changedID := "p17"
	src.identity[changedID] = sourceIdentity{exists: true, size: 2, modTime: baseTime.Add(time.Minute)}
	loader.Build(fresh())
	if got := loader.ParseCount(); got != projectCount+1 {
		t.Fatalf("one changed project ParseCount = %d, want %d", got, projectCount+1)
	}
	for id, parses := range src.parses {
		want := 1
		if id == changedID {
			want = 2
		}
		if parses != want {
			t.Fatalf("project %s parsed %d times, want %d", id, parses, want)
		}
	}
}

func TestLoaderDoesNotCacheMissingSourceFile(t *testing.T) {
	src := newFakeSource()
	loader := src.newLoader()
	// A project whose source file does not exist must not be cached, so it is
	// always loaded fresh (an empty file can be created between builds).
	src.identity["p1"] = sourceIdentity{exists: false}

	loader.Build([]*models.Project{makeProject("p1", "p1")})
	loader.Build([]*models.Project{makeProject("p1", "p1")})
	if loader.ParseCount() != 2 {
		t.Fatalf("missing-source project should always reparse: ParseCount = %d, want 2", loader.ParseCount())
	}
	if loader.CacheHits() != 0 {
		t.Fatalf("missing-source project must not hit cache: CacheHits = %d", loader.CacheHits())
	}
}

func TestLoaderRecordsDiagnosticsAndContinues(t *testing.T) {
	src := newFakeSource()
	src.loadErr["bad"] = fmt.Errorf("%w: unexpected heading", tterrors.ErrParseError)
	src.loadErr["gone"] = errors.New("permission denied")
	loader := src.newLoader()

	snap := loader.Build([]*models.Project{
		makeProject("ok", "ok"),
		makeProject("bad", "bad"),
		makeProject("gone", "gone"),
	})

	if len(snap.Projects()) != 1 || snap.Projects()[0].ID != "ok" {
		t.Fatalf("only the loadable project should be included, got %d", len(snap.Projects()))
	}
	if !snap.HasDiagnostics() || len(snap.Diagnostics()) != 2 {
		t.Fatalf("expected 2 diagnostics, got %v", snap.Diagnostics())
	}

	kinds := map[string]DiagnosticKind{}
	for _, d := range snap.Diagnostics() {
		kinds[d.ProjectID] = d.Kind
	}
	if kinds["bad"] != DiagnosticCorrupt {
		t.Fatalf("parse error should classify as corrupt, got %q", kinds["bad"])
	}
	if kinds["gone"] != DiagnosticSkipped {
		t.Fatalf("non-parse error should classify as skipped, got %q", kinds["gone"])
	}
}

func TestLoaderSkipsNilProjects(t *testing.T) {
	src := newFakeSource()
	loader := src.newLoader()
	snap := loader.Build([]*models.Project{nil, makeProject("p1", "p1"), nil})
	if len(snap.Projects()) != 1 {
		t.Fatalf("nil projects should be skipped, got %d", len(snap.Projects()))
	}
}
