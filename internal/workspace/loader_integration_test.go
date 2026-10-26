package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

// writeProject creates a real project directory with a tasks.md containing the
// given titles, and returns a fresh *models.Project pointing at it. It mirrors
// how config.GetActiveProjects hands back a fresh (unloaded) pointer per call.
func writeProject(t *testing.T, root, id, slug string, titles ...string) func() *models.Project {
	t.Helper()
	path := filepath.Join(root, "projects", slug)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	var tasks []*models.Task
	for i, title := range titles {
		task, err := models.NewGenericTask(slug+"-"+title, i+1, title)
		if err != nil {
			t.Fatalf("NewTask: %v", err)
		}
		tasks = append(tasks, task)
	}
	project := &models.Project{ID: id, Slug: slug, Name: slug, Path: path, Tasks: tasks}
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
	return func() *models.Project {
		return &models.Project{ID: id, Slug: slug, Name: slug, Path: path}
	}
}

// TestLoaderStorageBackedCacheAvoidsReparse exercises the default storage load
// path and real filesystem stat identity: an unchanged project must not be
// reparsed on a second build, but a changed file must be.
func TestLoaderStorageBackedCacheAvoidsReparse(t *testing.T) {
	root := t.TempDir()
	p1 := writeProject(t, root, "p1", "p1", "first task")
	p2 := writeProject(t, root, "p2", "p2", "second task")

	// Wrap the real storage loader with a parse counter to observe the default
	// path (not a fake) while still using real stat-based cache invalidation.
	parsed := map[string]int{}
	loader := NewLoader()
	loader.loadProject = func(p *models.Project) (string, error) {
		parsed[p.ID]++
		return loadThroughSharedDocuments(p)
	}

	first := loader.Build([]*models.Project{p1(), p2()})
	if len(first.Tasks()) != 2 {
		t.Fatalf("first build tasks = %d, want 2", len(first.Tasks()))
	}
	if parsed["p1"] != 1 || parsed["p2"] != 1 {
		t.Fatalf("first build parses = %v, want each once", parsed)
	}
	rev1 := first.Revision("p1")
	if rev1 == "" {
		t.Fatal("expected a non-empty revision for p1")
	}

	// Second build with untouched files: nothing should be reparsed.
	second := loader.Build([]*models.Project{p1(), p2()})
	if parsed["p1"] != 1 || parsed["p2"] != 1 {
		t.Fatalf("second build reparsed: %v, want unchanged", parsed)
	}
	if loader.CacheHits() != 2 {
		t.Fatalf("second build CacheHits = %d, want 2", loader.CacheHits())
	}
	if second.Revision("p1") != rev1 {
		t.Fatalf("revision changed across cache hit: %q vs %q", second.Revision("p1"), rev1)
	}

	// Change p2 on disk (append a task → different size), leave p1 untouched.
	writeProject(t, root, "p2", "p2", "second task", "third task")
	third := loader.Build([]*models.Project{p1(), p2()})
	if parsed["p1"] != 1 {
		t.Fatalf("p1 was reparsed after only p2 changed: %v", parsed)
	}
	if parsed["p2"] != 2 {
		t.Fatalf("p2 should have been reparsed once more: %v", parsed)
	}
	if len(third.Tasks()) != 3 {
		t.Fatalf("post-edit tasks = %d, want 3", len(third.Tasks()))
	}
}

// TestLoaderReloadIsWhatTheNextBuildReturns pins Reload: the project it loads in
// place is the object the next Build hands back for an unchanged file, without
// a parse, while a later write to the file is still read.
func TestLoaderReloadIsWhatTheNextBuildReturns(t *testing.T) {
	root := t.TempDir()
	p1 := writeProject(t, root, "p1", "p1", "first task")
	loader := NewLoader()
	loader.Build([]*models.Project{p1()})

	writeProject(t, root, "p1", "p1", "first task", "second task")
	reloaded := p1()
	if err := loader.Reload(reloaded); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if len(reloaded.Tasks) != 2 {
		t.Fatalf("Reload loaded %d tasks, want the 2 on disk", len(reloaded.Tasks))
	}
	parses := loader.ParseCount()

	built := loader.Build([]*models.Project{p1()})
	if loader.ParseCount() != parses {
		t.Fatal("Build parsed again a file Reload had just read")
	}
	if got := built.Projects(); len(got) != 1 || got[0] != reloaded {
		t.Fatal("Build did not return the object Reload loaded")
	}

	writeProject(t, root, "p1", "p1", "first task", "second task", "third task")
	if after := loader.Build([]*models.Project{p1()}); len(after.Tasks()) != 3 {
		t.Fatalf("Build after a later write saw %d tasks, want 3", len(after.Tasks()))
	}
}
