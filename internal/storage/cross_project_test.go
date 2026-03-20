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

	project := models.NewProject(uuid.New().String(), "My Project", "")
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

	project := models.NewProject(uuid.New().String(), "My Project", "")
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
