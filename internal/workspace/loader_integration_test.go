package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
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
		task, err := models.NewTask(slug+"-"+title, i+1, models.TaskTypeTask, title)
		if err != nil {
			t.Fatalf("NewTask: %v", err)
		}
		tasks = append(tasks, task)
	}
	content := storage.WriteTasksMarkdownWithProjectMetadata(slug, tasks, nil, nil)
	if err := os.WriteFile(filepath.Join(path, "tasks.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
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
	loader.loadProject = func(p *models.Project) error {
		parsed[p.ID]++
		return storage.LoadProject(p)
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
