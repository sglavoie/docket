package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestLoadCrossScopeTasks_ReturnsLoadFailureDetails(t *testing.T) {
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	project := testutil.NewTestProject("Broken Project")
	project.Path = filepath.Join(dir, "projects", project.Slug)
	if err := os.MkdirAll(project.Path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", project.Path, err)
	}

	parent, err := models.NewTask("parent-task", 1, models.TaskTypeTask, "Leaf parent")
	if err != nil {
		t.Fatal(err)
	}
	child, err := models.NewTask("child-story", 2, models.TaskTypeStory, "Story child")
	if err != nil {
		t.Fatal(err)
	}
	child.ParentID = &parent.ID
	content := storage.WriteTasksMarkdownWithProjectMetadata(project.Name, []*models.Task{parent, child}, nil, nil)
	if err := os.WriteFile(filepath.Join(project.Path, "tasks.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(tasks.md): %v", err)
	}

	cfg := newTestConfig()
	// Root the workspace at dir so the catalog-derived project path (DataDir/
	// projects/<slug>) matches where the malformed tasks.md was written.
	cfg = testRuntimeAt(dir, cfg)
	setTestProjects(cfg, []*models.Project{project})

	_, loaded, failed := LoadCrossScopeTasks(testRuntimeAt(cfg.DataDir(), cfg), NewProjectService(), CrossScopeOpts{})
	if len(loaded) != 0 {
		t.Fatalf("loaded projects = %d, want 0", len(loaded))
	}
	if len(failed) != 1 {
		t.Fatalf("failed loads = %v, want 1 failure", failed)
	}
	if failed[0].Project != project.Name {
		t.Fatalf("failed project = %q, want %q", failed[0].Project, project.Name)
	}
	if !strings.Contains(failed[0].Detail, "cannot be child") {
		t.Fatalf("failure detail = %q, want actionable load error", failed[0].Detail)
	}
}
