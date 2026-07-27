package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

// TestLoadProject_CrossProjectOrphanedDepCleaned verifies that, when a project's
// task has a cross-project dependency pointing to a project directory that no
// longer exists, the dependency is silently removed and a warning is emitted.
func TestLoadProject_CrossProjectOrphanedDepCleaned(t *testing.T) {
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	// Set up directory layout: {dir}/projects/{slug}/tasks.md
	projectsDir := filepath.Join(dir, "projects")
	projectSlug := "my-project"
	projectPath := filepath.Join(projectsDir, projectSlug)
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatal(err)
	}

	// Build a task with a dep on a non-existent cross-project task
	taskID := uuid.New().String()
	orphanedRef := "gone-project:" + uuid.New().String()
	content := "# Project: My Project\n\n" +
		"## Task: Sample Task\n" +
		"<!-- id: " + taskID + ", status: todo, priority: 1, depends: " + orphanedRef + " -->\n"

	tasksPath := filepath.Join(projectPath, "tasks.md")
	if err := os.WriteFile(tasksPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	project, err := models.NewProject(uuid.New().String(), "My Project", "")
	if err != nil {
		t.Fatal(err)
	}
	project.Slug = projectSlug
	project.Path = projectPath

	result, err := LoadProjectWithWarnings(project)
	if err != nil {
		t.Fatalf("LoadProjectWithWarnings failed: %v", err)
	}

	// The task should have no dependencies after cleanup
	task := findTaskByID(project.Tasks, taskID)
	if task == nil {
		t.Fatal("task not found in loaded project")
	}
	if len(task.DependsOn) != 0 {
		t.Errorf("DependsOn = %v, want empty (orphaned cross-project dep should be removed)", task.DependsOn)
	}

	// A WarningDataCorrection warning should have been emitted
	found := false
	for _, w := range result.Warnings {
		if w.Field == "depends" && w.TaskID == taskID {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a DataCorrection warning for orphaned cross-project dep, got none")
	}
}

// TestRoundTrip_CrossProjectRelatedToPreserved verifies that a task with a
// cross-project RelatedTo entry pointing to an existing project survives a
// full SaveProject → LoadProject round-trip without being stripped.
func TestRoundTrip_CrossProjectRelatedToPreserved(t *testing.T) {
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	projectsDir := filepath.Join(dir, "projects")

	// Create the referenced project directory so the cross-ref isn't cleaned up
	referencedSlug := "other-project"
	if err := os.MkdirAll(filepath.Join(projectsDir, referencedSlug), 0755); err != nil {
		t.Fatal(err)
	}

	// Set up the main project
	project := testutil.NewTestProject("My Project")
	project.Slug = "my-project"
	project.Path = filepath.Join(projectsDir, "my-project")

	crossRef := referencedSlug + ":" + uuid.New().String()
	task, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Task with cross-ref")
	if err != nil {
		t.Fatal(err)
	}
	task.RelatedTo = []string{crossRef}
	project.Tasks = []*models.Task{task}
	project.NextShortID = 2
	project.BuildTaskTree()

	if err := SaveProject(project); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}

	// Load into a fresh project
	loaded := testutil.NewTestProject("My Project")
	loaded.Slug = "my-project"
	loaded.Path = project.Path
	if err := LoadProject(loaded); err != nil {
		t.Fatalf("LoadProject: %v", err)
	}

	if len(loaded.Tasks) != 1 {
		t.Fatalf("expected 1 task after load, got %d", len(loaded.Tasks))
	}
	got := loaded.Tasks[0]
	if len(got.RelatedTo) != 1 || got.RelatedTo[0] != crossRef {
		t.Errorf("RelatedTo = %v, want [%s]", got.RelatedTo, crossRef)
	}
}

// TestLoadProject_CrossProjectExistingDepPreserved verifies that a cross-project
// dependency pointing to an existing project directory is kept intact.
func TestLoadProject_CrossProjectExistingDepPreserved(t *testing.T) {
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	projectsDir := filepath.Join(dir, "projects")

	// Create the referenced project directory
	referencedSlug := "other-project"
	referencedPath := filepath.Join(projectsDir, referencedSlug)
	if err := os.MkdirAll(referencedPath, 0755); err != nil {
		t.Fatal(err)
	}

	// Create the main project
	projectSlug := "my-project"
	projectPath := filepath.Join(projectsDir, projectSlug)
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatal(err)
	}

	taskID := uuid.New().String()
	crossRef := referencedSlug + ":" + uuid.New().String()
	content := "# Project: My Project\n\n" +
		"## Task: Sample Task\n" +
		"<!-- id: " + taskID + ", status: todo, priority: 1, depends: " + crossRef + " -->\n"

	if err := os.WriteFile(filepath.Join(projectPath, "tasks.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	project, err := models.NewProject(uuid.New().String(), "My Project", "")
	if err != nil {
		t.Fatal(err)
	}
	project.Slug = projectSlug
	project.Path = projectPath

	if _, err := LoadProjectWithWarnings(project); err != nil {
		t.Fatalf("LoadProjectWithWarnings failed: %v", err)
	}

	task := findTaskByID(project.Tasks, taskID)
	if task == nil {
		t.Fatal("task not found in loaded project")
	}
	if len(task.DependsOn) != 1 || task.DependsOn[0] != crossRef {
		t.Errorf("DependsOn = %v, want [%s] (existing cross-project dep should be preserved)", task.DependsOn, crossRef)
	}
}

// TestLoadProject_InvalidSectionCleared verifies that non-GTD section metadata
// is cleared and a DataCorrection warning is emitted.
func TestLoadProject_InvalidSectionCleared(t *testing.T) {
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	projectPath := filepath.Join(dir, "projects", "test-project")
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatal(err)
	}

	taskID := uuid.New().String()
	content := "# Project: Test Project\n" +
		"<!-- sections: Next;Inbox -->\n\n" +
		"## Task: Task With Gone Section\n" +
		"<!-- id: " + taskID + ", status: todo, priority: 1, section: GoneSection -->\n"

	if err := os.WriteFile(filepath.Join(projectPath, "tasks.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	project, err := models.NewProject(uuid.New().String(), "Test Project", "")
	if err != nil {
		t.Fatal(err)
	}
	project.Slug = "test-project"
	project.Path = projectPath

	result, err := LoadProjectWithWarnings(project)
	if err != nil {
		t.Fatalf("LoadProjectWithWarnings failed: %v", err)
	}

	task := findTaskByID(project.Tasks, taskID)
	if task == nil {
		t.Fatal("task not found in loaded project")
	}

	if got := task.State(); got != models.StateReady {
		t.Errorf("expected task state to be ready, got %q", got)
	}

	// A DataCorrection warning about the invalid section should have been emitted.
	found := false
	for _, w := range result.Warnings {
		if w.Field == "section" && w.TaskID == taskID {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a DataCorrection warning for invalid section, got none")
	}
}

func TestLoadProject_LegacySectionMetadataUnsupported(t *testing.T) {
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	projectPath := filepath.Join(dir, "projects", "test-project")
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatal(err)
	}

	taskID := uuid.New().String()
	legacyTaskSectionField := "custom" + "_section"
	content := "# Project: Test Project\n" +
		"<!-- sections: Next;Inbox -->\n\n" +
		"## Task: Task With Both Sections\n" +
		"<!-- id: " + taskID + ", status: todo, priority: 1, section: GoneSection, " + legacyTaskSectionField + ": Active -->\n"

	if err := os.WriteFile(filepath.Join(projectPath, "tasks.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	project, err := models.NewProject(uuid.New().String(), "Test Project", "")
	if err != nil {
		t.Fatal(err)
	}
	project.Slug = "test-project"
	project.Path = projectPath

	_, err = LoadProjectWithWarnings(project)
	if err != nil {
		t.Fatalf("LoadProjectWithWarnings failed: %v", err)
	}

	task := findTaskByID(project.Tasks, taskID)
	if task == nil {
		t.Fatal("task not found in loaded project")
	}

	if got := task.State(); got != models.StateReady {
		t.Errorf("expected task state to be ready after unsupported legacy metadata correction, got %q", got)
	}
}
