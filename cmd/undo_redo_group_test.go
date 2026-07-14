package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func TestRunUndoRestoresGroupedHistoryForConfiguredProjects(t *testing.T) {
	tmpDir := t.TempDir()
	alphaOriginal := tasksMarkdown("Alpha", "Alpha original")
	alphaChanged := tasksMarkdown("Alpha", "Alpha changed")
	betaOriginal := tasksMarkdown("Beta", "Beta original")
	betaChanged := tasksMarkdown("Beta", "Beta changed")

	alpha := newGroupedRestoreProject(t, tmpDir, "proj-alpha", "alpha", "Alpha", alphaOriginal)
	beta := newGroupedRestoreProject(t, tmpDir, "proj-beta", "beta", "Beta", betaOriginal)
	projects := []*models.Project{alpha, beta}
	recordGroupedSnapshot(t, alpha, "move", "Move task", "group-1", []string{alpha.ID, beta.ID})
	recordGroupedSnapshot(t, beta, "move", "Move task", "group-1", []string{alpha.ID, beta.ID})
	writeProjectTasks(t, alpha, alphaChanged)
	writeProjectTasks(t, beta, betaChanged)
	configureGroupedRestoreCLI(t, tmpDir, alpha.ID, projects)

	if err := runUndo(nil, nil); err != nil {
		t.Fatalf("runUndo failed: %v", err)
	}

	assertProjectTasksContain(t, alpha, "Alpha original", "Alpha changed")
	assertProjectTasksContain(t, beta, "Beta original", "Beta changed")
}

func TestRunUndoGroupedHistoryMissingProjectHasHelpfulError(t *testing.T) {
	tmpDir := t.TempDir()
	original := tasksMarkdown("Alpha", "Alpha original")
	changed := tasksMarkdown("Alpha", "Alpha changed")

	alpha := newGroupedRestoreProject(t, tmpDir, "proj-alpha", "alpha", "Alpha", original)
	recordGroupedSnapshot(t, alpha, "move", "Move task", "group-missing", []string{alpha.ID, "proj-missing"})
	writeProjectTasks(t, alpha, changed)
	configureGroupedRestoreCLI(t, tmpDir, alpha.ID, []*models.Project{alpha})

	err := runUndo(nil, nil)
	if err == nil {
		t.Fatal("expected grouped undo to fail when a grouped project is not configured")
	}
	if !strings.Contains(err.Error(), "grouped history is incomplete or diverged") {
		t.Fatalf("expected grouped-history hint, got: %v", err)
	}
	if !strings.Contains(err.Error(), "proj-missing") {
		t.Fatalf("expected missing project ID in error, got: %v", err)
	}
	assertProjectTasksContain(t, alpha, "Alpha changed", "Alpha original")
}

func TestRunRedoRestoresGroupedHistoryForConfiguredProjects(t *testing.T) {
	tmpDir := t.TempDir()
	alphaOriginal := tasksMarkdown("Alpha", "Alpha original")
	alphaChanged := tasksMarkdown("Alpha", "Alpha changed")
	betaOriginal := tasksMarkdown("Beta", "Beta original")
	betaChanged := tasksMarkdown("Beta", "Beta changed")

	alpha := newGroupedRestoreProject(t, tmpDir, "proj-alpha", "alpha", "Alpha", alphaOriginal)
	beta := newGroupedRestoreProject(t, tmpDir, "proj-beta", "beta", "Beta", betaOriginal)
	projects := []*models.Project{alpha, beta}
	recordGroupedSnapshot(t, alpha, "move", "Move task", "group-redo", []string{alpha.ID, beta.ID})
	recordGroupedSnapshot(t, beta, "move", "Move task", "group-redo", []string{alpha.ID, beta.ID})
	writeProjectTasks(t, alpha, alphaChanged)
	writeProjectTasks(t, beta, betaChanged)
	if _, err := storage.PerformUndoForProjects(alpha, projects); err != nil {
		t.Fatalf("PerformUndoForProjects failed: %v", err)
	}
	configureGroupedRestoreCLI(t, tmpDir, alpha.ID, projects)

	if err := runRedo(nil, nil); err != nil {
		t.Fatalf("runRedo failed: %v", err)
	}

	assertProjectTasksContain(t, alpha, "Alpha changed", "Alpha original")
	assertProjectTasksContain(t, beta, "Beta changed", "Beta original")
}

func tasksMarkdown(projectName, taskTitle string) string {
	return "# Project: " + projectName + "\n" +
		"<!-- next_short_id: 2 -->\n\n" +
		"## Task: " + taskTitle + "\n" +
		"<!-- id: task-1, status: todo, priority: 3 -->\n"
}

func newGroupedRestoreProject(t *testing.T, root, id, slug, name, content string) *models.Project {
	t.Helper()
	// The workspace catalog derives every project's path as <data>/projects/<slug>
	// (absolute paths are no longer persisted), so a config reload resolves the
	// project here; the on-disk tasks.md and .undo history must live at that path.
	projectPath := filepath.Join(root, "projects", slug)
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	project := &models.Project{
		ID:   id,
		Name: name,
		Slug: slug,
		Path: projectPath,
	}
	writeProjectTasks(t, project, content)
	return project
}

func writeProjectTasks(t *testing.T, project *models.Project, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(project.Path, "tasks.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write tasks file: %v", err)
	}
}

func recordGroupedSnapshot(t *testing.T, project *models.Project, action, context, groupID string, projectIDs []string) {
	t.Helper()
	err := storage.RecordUndoSnapshotWithOptions(project, storage.UndoSnapshotOptions{
		Action:          action,
		Context:         context,
		MaxDepth:        10,
		GroupID:         groupID,
		GroupProjectIDs: projectIDs,
	})
	if err != nil {
		t.Fatalf("failed to record grouped snapshot: %v", err)
	}
}

func configureGroupedRestoreCLI(t *testing.T, dataDir, currentProjectID string, projects []*models.Project) {
	t.Helper()
	cfg := newFileBackedTestRuntime(dataDir, currentProjectID, projects...)
	configPath := filepath.Join(dataDir, "config.json")
	manager := config.NewManager(configPath)
	if err := saveTestRuntime(manager, cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	oldConfig := viper.GetString("config")
	viper.Set("config", configPath)
	t.Cleanup(func() { viper.Set("config", oldConfig) })

	oldRepo := storage.DefaultRepository
	storage.DefaultRepository = storage.NewFileSystemRepository()
	t.Cleanup(func() { storage.DefaultRepository = oldRepo })
}

func assertProjectTasksContain(t *testing.T, project *models.Project, want, unwanted string) {
	t.Helper()
	contentBytes, err := os.ReadFile(filepath.Join(project.Path, "tasks.md"))
	if err != nil {
		t.Fatalf("failed to read tasks file: %v", err)
	}
	content := string(contentBytes)
	if !strings.Contains(content, want) {
		t.Fatalf("tasks file should contain %q, got:\n%s", want, content)
	}
	if strings.Contains(content, unwanted) {
		t.Fatalf("tasks file should not contain %q, got:\n%s", unwanted, content)
	}
}
