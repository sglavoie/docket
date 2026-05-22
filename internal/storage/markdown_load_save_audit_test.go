package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/tterrors"
)

func newStorageAuditTask(t *testing.T, id string, shortID int, taskType models.TaskType, title string) *models.Task {
	t.Helper()

	task, err := models.NewTask(id, shortID, taskType, title)
	if err != nil {
		t.Fatalf("NewTask(%q): %v", title, err)
	}

	return task
}

func writeAuditTestProject(t *testing.T, projectPath string, project *models.Project) {
	t.Helper()

	if err := os.MkdirAll(projectPath, DirPermissions); err != nil {
		t.Fatalf("MkdirAll(%q): %v", projectPath, err)
	}

	content := WriteTasksMarkdownWithSections(project.Name, project.Tasks, project.GetSections(), project.SectionColors)
	if err := os.WriteFile(tasksFilePath(projectPath), []byte(content), FilePermissions); err != nil {
		t.Fatalf("WriteFile(tasks.md): %v", err)
	}
}

func TestLoadProjectWithWarnings_AuditsAfterNormalization(t *testing.T) {
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	projectPath := filepath.Join(dir, "projects", "load-audit-normalized")

	seed := testutil.NewTestProject("Load Audit Normalized")
	task := newStorageAuditTask(t, "task-1", 1, models.TaskTypeTask, "Needs cleanup")
	task.DependsOn = []string{"missing-local-task"}
	seed.Tasks = []*models.Task{task}

	writeAuditTestProject(t, projectPath, seed)

	loaded := testutil.NewTestProject(seed.Name)
	loaded.Path = projectPath

	result, err := LoadProjectWithWarnings(loaded)
	if err != nil {
		t.Fatalf("LoadProjectWithWarnings() error = %v, want nil", err)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("LoadProjectWithWarnings() warnings = %v, want none", result.Warnings)
	}
	if len(loaded.Tasks) != 1 {
		t.Fatalf("loaded %d tasks, want 1", len(loaded.Tasks))
	}
	if len(loaded.Tasks[0].DependsOn) != 0 {
		t.Fatalf("loaded task DependsOn = %v, want orphaned local dependency pruned", loaded.Tasks[0].DependsOn)
	}
}

func TestLoadProjectWithWarnings_FailsAuditAfterNormalization(t *testing.T) {
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	projectPath := filepath.Join(dir, "projects", "load-audit-invalid-hierarchy")

	seed := testutil.NewTestProject("Load Audit Invalid Hierarchy")
	parent := newStorageAuditTask(t, "parent-task", 1, models.TaskTypeTask, "Leaf parent")
	child := newStorageAuditTask(t, "child-story", 2, models.TaskTypeStory, "Story child")
	child.ParentID = &parent.ID
	seed.Tasks = []*models.Task{parent, child}

	writeAuditTestProject(t, projectPath, seed)

	loaded := testutil.NewTestProject(seed.Name)
	loaded.Path = projectPath

	_, err := LoadProjectWithWarnings(loaded)
	if err == nil {
		t.Fatal("LoadProjectWithWarnings() error = nil, want audit failure")
	}
	if !errors.Is(err, tterrors.ErrParseError) {
		t.Fatalf("LoadProjectWithWarnings() error = %v, want errors.Is(..., ErrParseError)", err)
	}
	if !strings.Contains(err.Error(), "project audit failed after load normalization") {
		t.Fatalf("LoadProjectWithWarnings() error = %q, want load audit context", err)
	}
	if !strings.Contains(err.Error(), "cannot be child") {
		t.Fatalf("LoadProjectWithWarnings() error = %q, want actionable hierarchy detail", err)
	}
}

func TestSaveProject_FailsAuditBeforeWrite(t *testing.T) {
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	projectPath := filepath.Join(dir, "projects", "save-audit-invalid-hierarchy")
	if err := os.MkdirAll(projectPath, DirPermissions); err != nil {
		t.Fatalf("MkdirAll(%q): %v", projectPath, err)
	}

	tasksPath := tasksFilePath(projectPath)
	original := "# Project: Save Audit Invalid Hierarchy\n\n"
	if err := os.WriteFile(tasksPath, []byte(original), FilePermissions); err != nil {
		t.Fatalf("WriteFile(tasks.md): %v", err)
	}

	project := testutil.NewTestProject("Save Audit Invalid Hierarchy")
	project.Path = projectPath

	parent := newStorageAuditTask(t, "parent-task", 1, models.TaskTypeTask, "Leaf parent")
	child := newStorageAuditTask(t, "child-story", 2, models.TaskTypeStory, "Story child")
	child.ParentID = &parent.ID
	project.Tasks = []*models.Task{parent, child}

	err := SaveProject(project)
	if err == nil {
		t.Fatal("SaveProject() error = nil, want audit failure")
	}
	if !errors.Is(err, tterrors.ErrSaveFailed) {
		t.Fatalf("SaveProject() error = %v, want errors.Is(..., ErrSaveFailed)", err)
	}
	if !strings.Contains(err.Error(), "project audit failed before save") {
		t.Fatalf("SaveProject() error = %q, want save audit context", err)
	}
	if !strings.Contains(err.Error(), "cannot be child") {
		t.Fatalf("SaveProject() error = %q, want actionable hierarchy detail", err)
	}

	got, readErr := os.ReadFile(tasksPath)
	if readErr != nil {
		t.Fatalf("ReadFile(tasks.md): %v", readErr)
	}
	if string(got) != original {
		t.Fatalf("tasks.md changed on failed save:\n--- got ---\n%s\n--- want ---\n%s", got, original)
	}
}
