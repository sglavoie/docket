package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func TestTaskArchiveRebuildIndexRepairsMissingIndex(t *testing.T) {
	deps, project := setupArchiveAccessTest(t)
	defer deps.Cleanup()

	task := archiveAccessTestTask(t, "arch-rebuild-index-0001", 21, models.TaskTypeTask, "Rebuild index target", time.Date(2026, 6, 8, 10, 0, 0, 0, time.Local))
	writeArchiveAccessArchive(t, project, "completed.md", []*models.Task{task})

	cmd := newTaskArchiveRebuildIndexCmd()
	mustSetFlag(t, cmd, "project", project.Slug)

	stdout, err := captureArchiveAccessStdout(t, func() error {
		return runTaskArchiveRebuildIndex(cmd, nil)
	})
	if err != nil {
		t.Fatalf("runTaskArchiveRebuildIndex returned error: %v", err)
	}
	if !strings.Contains(stdout, "Rebuilt archive index") || !strings.Contains(stdout, "1 task") {
		t.Fatalf("archive rebuild output missing summary:\n%s", stdout)
	}

	status, err := storage.InspectArchiveIndex(project)
	if err != nil {
		t.Fatalf("InspectArchiveIndex returned error: %v", err)
	}
	if !status.Fresh || status.NeedsRebuild || status.TaskCount != 1 {
		t.Fatalf("status after rebuild = %+v, want fresh index with 1 task", status)
	}
}

func TestTaskArchiveRebuildIndexRepairsCorruptIndex(t *testing.T) {
	deps, project := setupArchiveAccessTest(t)
	defer deps.Cleanup()

	task := archiveAccessTestTask(t, "arch-corrupt-rebuild-0001", 22, models.TaskTypeBug, "Corrupt rebuild target", time.Date(2026, 6, 8, 10, 0, 0, 0, time.Local))
	writeArchiveAccessArchive(t, project, "completed.md", []*models.Task{task})
	if err := os.WriteFile(filepath.Join(project.Path, ".completed.index.json"), []byte("{not-json"), storage.FilePermissions); err != nil {
		t.Fatalf("write corrupt index: %v", err)
	}

	cmd := newTaskArchiveRebuildIndexCmd()
	mustSetFlag(t, cmd, "project", project.Slug)
	mustSetFlag(t, cmd, "json", "true")

	stdout, err := captureArchiveAccessStdout(t, func() error {
		return runTaskArchiveRebuildIndex(cmd, nil)
	})
	if err != nil {
		t.Fatalf("runTaskArchiveRebuildIndex returned error: %v", err)
	}
	if !strings.Contains(stdout, `"task_count": 1`) || !strings.Contains(stdout, `"rebuilt": true`) {
		t.Fatalf("archive rebuild JSON missing counts:\n%s", stdout)
	}

	status, err := storage.InspectArchiveIndex(project)
	if err != nil {
		t.Fatalf("InspectArchiveIndex returned error: %v", err)
	}
	if !status.Fresh || status.NeedsRebuild {
		t.Fatalf("status after corrupt rebuild = %+v, want fresh index", status)
	}
}

func TestTaskArchiveRebuildIndexRejectsAllWithProject(t *testing.T) {
	cmd := newTaskArchiveRebuildIndexCmd()
	mustSetFlag(t, cmd, "all", "true")
	mustSetFlag(t, cmd, "project", "test-project")

	if err := runTaskArchiveRebuildIndex(cmd, nil); err == nil || !strings.Contains(err.Error(), "--all and --project") {
		t.Fatalf("runTaskArchiveRebuildIndex error = %v, want --all/--project validation", err)
	}
}
