package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestTaskArchiveListUsesSummariesAndFilters(t *testing.T) {
	deps, project := setupArchiveAccessTest(t)
	defer deps.Cleanup()

	mayDone := archiveAccessTestTask(t, "arch-may-done-0001", 101, models.TaskTypeTask, "May completed task", time.Date(2026, 5, 10, 9, 0, 0, 0, time.Local))
	juneBug := archiveAccessTestTask(t, "arch-june-bug-0001", 102, models.TaskTypeBug, "June canceled bug", time.Date(2026, 6, 12, 14, 0, 0, 0, time.Local))
	juneBug.WorkflowState = models.StateCanceled
	juneStory := archiveAccessTestTask(t, "arch-june-story-0001", 103, models.TaskTypeStory, "June completed story", time.Date(2026, 6, 20, 14, 0, 0, 0, time.Local))
	writeArchiveAccessArchive(t, project, "completed.md", []*models.Task{mayDone, juneBug, juneStory})

	cmd := newTaskArchiveListCmd()
	mustSetFlag(t, cmd, "project", project.Slug)
	mustSetFlag(t, cmd, "type", "bug")
	mustSetFlag(t, cmd, "status", "canceled")
	mustSetFlag(t, cmd, "since", "2026-06-01")
	mustSetFlag(t, cmd, "until", "2026-06-30")
	mustSetFlag(t, cmd, "json", "true")

	stdout, err := captureArchiveAccessStdout(t, func() error {
		return runTaskArchiveList(cmd, nil)
	})
	if err != nil {
		t.Fatalf("runTaskArchiveList returned error: %v", err)
	}

	var entries []archiveSummaryJSONEntry
	if err := json.Unmarshal([]byte(stdout), &entries); err != nil {
		t.Fatalf("archive list JSON did not parse: %v\n%s", err, stdout)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 filtered archive summary, got %d: %+v", len(entries), entries)
	}
	if entries[0].ID != juneBug.ID || entries[0].Title != juneBug.Title || entries[0].Type != "bug" || entries[0].Status != "canceled" {
		t.Fatalf("unexpected archive list entry: %+v", entries[0])
	}
	if entries[0].SourceFile != "completed.md" || entries[0].SourceLine == 0 {
		t.Fatalf("expected archive source metadata, got %+v", entries[0])
	}
	if _, err := os.Stat(filepath.Join(project.Path, ".completed.index.json")); err != nil {
		t.Fatalf("archive list should create/reuse the derived summary index: %v", err)
	}
}

func TestTaskArchiveListTextShowsExplicitArchiveEntries(t *testing.T) {
	deps, project := setupArchiveAccessTest(t)
	defer deps.Cleanup()

	task := archiveAccessTestTask(t, "arch-visible-0001", 42, models.TaskTypeIdea, "Archived idea for list", time.Date(2026, 6, 8, 10, 0, 0, 0, time.Local))
	writeArchiveAccessArchive(t, project, "completed.md", []*models.Task{task})

	cmd := newTaskArchiveListCmd()
	mustSetFlag(t, cmd, "project", project.Slug)

	stdout, err := captureArchiveAccessStdout(t, func() error {
		return runTaskArchiveList(cmd, nil)
	})
	if err != nil {
		t.Fatalf("runTaskArchiveList returned error: %v", err)
	}
	if !strings.Contains(stdout, "Archived tasks in Test Project") {
		t.Fatalf("archive list output missing project heading:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Archived idea for list") || !strings.Contains(stdout, "completed.md:") {
		t.Fatalf("archive list output missing task/source details:\n%s", stdout)
	}
}

func TestTaskArchiveShowFindsArchivedTaskByPrefix(t *testing.T) {
	deps, project := setupArchiveAccessTest(t)
	defer deps.Cleanup()

	task := archiveAccessTestTask(t, "arch-show-detail-0001", 7, models.TaskTypeTask, "Archived show target", time.Date(2026, 6, 8, 10, 0, 0, 0, time.Local))
	task.Description = "Details only available from the archive body."
	writeArchiveAccessArchive(t, project, "completed.md", []*models.Task{task})

	cmd := newTaskArchiveShowCmd()
	mustSetFlag(t, cmd, "project", project.Slug)

	stdout, err := captureArchiveAccessStdout(t, func() error {
		return runTaskArchiveShow(cmd, []string{"ARCH-SHOW"})
	})
	if err != nil {
		t.Fatalf("runTaskArchiveShow returned error: %v", err)
	}
	if !strings.Contains(stdout, "Archived show target") || !strings.Contains(stdout, "Details only available from the archive body.") {
		t.Fatalf("archive show output missing full archived details:\n%s", stdout)
	}
}

func TestSearchDoesNotIncludeCompletedArchiveFiles(t *testing.T) {
	deps, project := setupArchiveAccessTest(t)
	defer deps.Cleanup()

	archived := archiveAccessTestTask(t, "arch-search-hidden-0001", 11, models.TaskTypeTask, "archived-only-search-term", time.Date(2026, 6, 8, 10, 0, 0, 0, time.Local))
	writeArchiveAccessArchive(t, project, "completed.md", []*models.Task{archived})

	stdout, err := captureArchiveAccessStdout(t, func() error {
		return runSearch(nil, []string{"archived-only-search-term"})
	})
	if err != nil {
		t.Fatalf("runSearch returned error: %v", err)
	}
	if strings.Contains(stdout, "Found 1 task") || !strings.Contains(stdout, "No tasks found matching") {
		t.Fatalf("normal search should not include completed archive files:\n%s", stdout)
	}
}

func setupArchiveAccessTest(t *testing.T) (*testutil.TestDeps, *models.Project) {
	t.Helper()
	resetArchiveAccessVars()

	deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")

	viper.Set("config", deps.ConfigPath)
	t.Cleanup(func() { viper.Set("config", "") })

	manager := config.NewManager(deps.ConfigPath)
	if err := manager.Save(deps.Config); err != nil {
		t.Fatalf("save test config: %v", err)
	}

	if err := os.MkdirAll(project.Path, storage.DirPermissions); err != nil {
		t.Fatalf("create project dir: %v", err)
	}

	oldStorageRepo := storage.DefaultRepository
	storage.DefaultRepository = deps.Repository
	t.Cleanup(func() { storage.DefaultRepository = oldStorageRepo })

	return deps, project
}

func resetArchiveAccessVars() {
	archiveListType = ""
	archiveListStatus = ""
	archiveListSince = ""
	archiveListUntil = ""
	archiveShowNoGlow = false
	searchIncludeArchived = false
	searchLimit = 0
}

func archiveAccessTestTask(t *testing.T, id string, shortID int, taskType models.TaskType, title string, completedAt time.Time) *models.Task {
	t.Helper()
	task, err := models.NewTask(id, shortID, taskType, title)
	if err != nil {
		t.Fatalf("new archive access task: %v", err)
	}
	task.WorkflowState = models.StateDone
	task.Priority = 2
	task.CreatedAt = completedAt.AddDate(0, 0, -2)
	task.UpdatedAt = completedAt
	task.CompletedAt = &completedAt
	return task
}

func writeArchiveAccessArchive(t *testing.T, project *models.Project, fileName string, tasks []*models.Task) {
	t.Helper()
	content := storage.WriteArchivedTasks(project.Name, tasks)
	if err := os.WriteFile(filepath.Join(project.Path, fileName), []byte(content), storage.FilePermissions); err != nil {
		t.Fatalf("write archive %s: %v", fileName, err)
	}
}

func captureArchiveAccessStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	os.Stdout = w

	runErr := fn()

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return buf.String(), runErr
}

func mustSetFlag(t *testing.T, cmd *cobra.Command, name, value string) {
	t.Helper()
	if err := cmd.Flags().Set(name, value); err != nil {
		t.Fatalf("set --%s=%s: %v", name, value, err)
	}
}
