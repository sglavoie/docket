package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

func TestLoadArchivedTaskSummariesRebuildsMissingIndex(t *testing.T) {
	tmpDir := t.TempDir()
	project := &models.Project{Path: tmpDir, Name: "TestProj"}

	completedAt := time.Date(2026, 6, 8, 10, 30, 0, 0, time.Local)
	startedAt := completedAt.Add(-48 * time.Hour)
	task := archiveIndexTestTask("arch-story-1", 99, models.TaskTypeStory, "Indexed story", completedAt)
	task.CreatedAt = completedAt.AddDate(0, 0, -7)
	task.UpdatedAt = completedAt
	task.EstimatedDays = 3
	task.StoryAttrs = &models.StoryAttrs{StoryPoints: 5}
	task.Activity = []*models.ActivityEntry{
		{Timestamp: startedAt.Add(2 * time.Hour), Field: "status", OldValue: "blocked", NewValue: "in_progress"},
		{Timestamp: startedAt, Field: "status", OldValue: "todo", NewValue: "in_progress"},
	}
	writeArchiveIndexTestArchive(t, tmpDir, archiveFileName, []*models.Task{task})

	summaries, err := LoadArchivedTaskSummaries(project)
	if err != nil {
		t.Fatalf("LoadArchivedTaskSummaries returned error: %v", err)
	}

	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	got := summaries[0]
	if got.ID != task.ID || got.Title != task.Title || got.Type != models.TaskTypeStory {
		t.Fatalf("summary mismatch: %+v", got)
	}
	if got.ShortID != 1 {
		t.Fatalf("archive parser short ID = %d, want 1", got.ShortID)
	}
	if got.Source.FileName != archiveFileName || got.Source.HeadingLine == 0 {
		t.Fatalf("source metadata mismatch: %+v", got.Source)
	}
	if got.CompletedAt == nil || !got.CompletedAt.Equal(completedAt) {
		t.Fatalf("CompletedAt = %v, want %v", got.CompletedAt, completedAt)
	}
	if got.FirstStartedAt == nil || !got.FirstStartedAt.Equal(startedAt) {
		t.Fatalf("FirstStartedAt = %v, want %v", got.FirstStartedAt, startedAt)
	}
	if got.StoryPoints != 5 || got.EstimatedDays != 3 {
		t.Fatalf("analytics fields mismatch: %+v", got)
	}

	index := readArchiveIndexTestFile(t, project)
	if index.Version != archiveIndexVersion || len(index.Sources) != 1 || index.Sources[0].TaskCount != 1 {
		t.Fatalf("index metadata mismatch: %+v", index)
	}
}

func TestLoadArchivedTaskSummariesRebuildsStaleIndex(t *testing.T) {
	tmpDir := t.TempDir()
	project := &models.Project{Path: tmpDir, Name: "TestProj"}
	completedAt := time.Date(2026, 6, 8, 10, 30, 0, 0, time.Local)

	first := archiveIndexTestTask("arch-first", 1, models.TaskTypeTask, "First archived task", completedAt)
	writeArchiveIndexTestArchive(t, tmpDir, archiveFileName, []*models.Task{first})

	summaries, err := LoadArchivedTaskSummaries(project)
	if err != nil {
		t.Fatalf("initial LoadArchivedTaskSummaries returned error: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 initial summary, got %d", len(summaries))
	}
	initialIndex := readArchiveIndexTestFile(t, project)
	initialSize := initialIndex.Sources[0].SizeBytes

	second := archiveIndexTestTask("arch-second", 2, models.TaskTypeBug, "Second archived task", completedAt.Add(time.Hour))
	appendArchiveIndexTestArchive(t, tmpDir, archiveFileName, []*models.Task{second})

	summaries, err = LoadArchivedTaskSummaries(project)
	if err != nil {
		t.Fatalf("stale LoadArchivedTaskSummaries returned error: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("expected stale index rebuild to return 2 summaries, got %d", len(summaries))
	}
	if summaries[0].ID != "arch-first" || summaries[1].ID != "arch-second" {
		t.Fatalf("unexpected summary order after rebuild: %+v", summaries)
	}

	rebuiltIndex := readArchiveIndexTestFile(t, project)
	if rebuiltIndex.Sources[0].TaskCount != 2 {
		t.Fatalf("rebuilt TaskCount = %d, want 2", rebuiltIndex.Sources[0].TaskCount)
	}
	if rebuiltIndex.Sources[0].SizeBytes == initialSize {
		t.Fatalf("expected source size to change after archive append")
	}
}

func TestLoadArchivedTaskSummariesRebuildsCorruptIndex(t *testing.T) {
	tmpDir := t.TempDir()
	project := &models.Project{Path: tmpDir, Name: "TestProj"}
	completedAt := time.Date(2026, 6, 8, 10, 30, 0, 0, time.Local)
	task := archiveIndexTestTask("arch-corrupt-index", 1, models.TaskTypeTask, "Recovered from bad JSON", completedAt)
	writeArchiveIndexTestArchive(t, tmpDir, archiveFileName, []*models.Task{task})

	if err := os.WriteFile(archiveIndexPath(project), []byte("{not-json"), FilePermissions); err != nil {
		t.Fatalf("write corrupt index: %v", err)
	}

	summaries, err := LoadArchivedTaskSummaries(project)
	if err != nil {
		t.Fatalf("LoadArchivedTaskSummaries returned error: %v", err)
	}
	if len(summaries) != 1 || summaries[0].ID != task.ID {
		t.Fatalf("unexpected summaries after corrupt index rebuild: %+v", summaries)
	}

	readArchiveIndexTestFile(t, project)
}

func TestInspectArchiveIndexReportsRepairStates(t *testing.T) {
	completedAt := time.Date(2026, 6, 8, 10, 30, 0, 0, time.Local)

	t.Run("missing index with archives", func(t *testing.T) {
		tmpDir := t.TempDir()
		project := &models.Project{Path: tmpDir, Name: "TestProj"}
		writeArchiveIndexTestArchive(t, tmpDir, archiveFileName, []*models.Task{
			archiveIndexTestTask("arch-missing-index", 1, models.TaskTypeTask, "Missing index", completedAt),
		})

		status, err := InspectArchiveIndex(project)
		if err != nil {
			t.Fatalf("InspectArchiveIndex returned error: %v", err)
		}
		if !status.NeedsRebuild || status.Issue != ArchiveIndexIssueMissing || !status.HasArchives {
			t.Fatalf("status = %+v, want missing rebuild issue with archives", status)
		}
	})

	t.Run("stale index after archive edit", func(t *testing.T) {
		tmpDir := t.TempDir()
		project := &models.Project{Path: tmpDir, Name: "TestProj"}
		writeArchiveIndexTestArchive(t, tmpDir, archiveFileName, []*models.Task{
			archiveIndexTestTask("arch-before-edit", 1, models.TaskTypeTask, "Before edit", completedAt),
		})
		if err := RebuildArchiveIndex(project); err != nil {
			t.Fatalf("RebuildArchiveIndex returned error: %v", err)
		}
		appendArchiveIndexTestArchive(t, tmpDir, archiveFileName, []*models.Task{
			archiveIndexTestTask("arch-after-edit", 2, models.TaskTypeTask, "After edit", completedAt.Add(time.Hour)),
		})

		status, err := InspectArchiveIndex(project)
		if err != nil {
			t.Fatalf("InspectArchiveIndex returned error: %v", err)
		}
		if !status.NeedsRebuild || status.Issue != ArchiveIndexIssueStale {
			t.Fatalf("status = %+v, want stale rebuild issue", status)
		}
	})

	t.Run("corrupt index", func(t *testing.T) {
		tmpDir := t.TempDir()
		project := &models.Project{Path: tmpDir, Name: "TestProj"}
		writeArchiveIndexTestArchive(t, tmpDir, archiveFileName, []*models.Task{
			archiveIndexTestTask("arch-corrupt-inspect", 1, models.TaskTypeTask, "Corrupt index", completedAt),
		})
		if err := os.WriteFile(archiveIndexPath(project), []byte("{not-json"), FilePermissions); err != nil {
			t.Fatalf("write corrupt index: %v", err)
		}

		status, err := InspectArchiveIndex(project)
		if err != nil {
			t.Fatalf("InspectArchiveIndex returned error: %v", err)
		}
		if !status.NeedsRebuild || status.Issue != ArchiveIndexIssueCorrupt || status.Detail == "" {
			t.Fatalf("status = %+v, want corrupt rebuild issue", status)
		}
	})

	t.Run("fresh index", func(t *testing.T) {
		tmpDir := t.TempDir()
		project := &models.Project{Path: tmpDir, Name: "TestProj"}
		writeArchiveIndexTestArchive(t, tmpDir, archiveFileName, []*models.Task{
			archiveIndexTestTask("arch-fresh-index", 1, models.TaskTypeTask, "Fresh index", completedAt),
		})
		if err := RebuildArchiveIndex(project); err != nil {
			t.Fatalf("RebuildArchiveIndex returned error: %v", err)
		}

		status, err := InspectArchiveIndex(project)
		if err != nil {
			t.Fatalf("InspectArchiveIndex returned error: %v", err)
		}
		if !status.Fresh || status.NeedsRebuild || status.TaskCount != 1 || status.SourceCount != 1 {
			t.Fatalf("status = %+v, want fresh index with counts", status)
		}
	})

	t.Run("no archives and no index", func(t *testing.T) {
		project := &models.Project{Path: t.TempDir(), Name: "TestProj"}

		status, err := InspectArchiveIndex(project)
		if err != nil {
			t.Fatalf("InspectArchiveIndex returned error: %v", err)
		}
		if !status.Fresh || status.NeedsRebuild || status.HasArchives {
			t.Fatalf("status = %+v, want clean project with no archives", status)
		}
	})
}

func TestLoadArchivedTaskSummariesOrdersRotatedArchives(t *testing.T) {
	tmpDir := t.TempDir()
	project := &models.Project{Path: tmpDir, Name: "TestProj"}
	completedAt := time.Date(2026, 6, 8, 10, 30, 0, 0, time.Local)

	writeArchiveIndexTestArchive(t, tmpDir, "completed.2.md", []*models.Task{
		archiveIndexTestTask("arch-rot-2", 1, models.TaskTypeTask, "Rotated 2", completedAt),
	})
	writeArchiveIndexTestArchive(t, tmpDir, archiveFileName, []*models.Task{
		archiveIndexTestTask("arch-main", 1, models.TaskTypeTask, "Main", completedAt),
	})
	writeArchiveIndexTestArchive(t, tmpDir, "completed.1.md", []*models.Task{
		archiveIndexTestTask("arch-rot-1", 1, models.TaskTypeTask, "Rotated 1", completedAt),
	})

	summaries, err := LoadArchivedTaskSummaries(project)
	if err != nil {
		t.Fatalf("LoadArchivedTaskSummaries returned error: %v", err)
	}

	gotIDs := archiveIndexSummaryIDs(summaries)
	wantIDs := []string{"arch-main", "arch-rot-1", "arch-rot-2"}
	if strings.Join(gotIDs, ",") != strings.Join(wantIDs, ",") {
		t.Fatalf("summary order = %v, want %v", gotIDs, wantIDs)
	}

	index := readArchiveIndexTestFile(t, project)
	gotSources := []string{index.Sources[0].FileName, index.Sources[1].FileName, index.Sources[2].FileName}
	wantSources := []string{archiveFileName, "completed.1.md", "completed.2.md"}
	if strings.Join(gotSources, ",") != strings.Join(wantSources, ",") {
		t.Fatalf("source order = %v, want %v", gotSources, wantSources)
	}
}

func TestRebuildArchiveIndexSkipsCorruptArchivePredictably(t *testing.T) {
	tmpDir := t.TempDir()
	project := &models.Project{Path: tmpDir, Name: "TestProj"}
	completedAt := time.Date(2026, 6, 8, 10, 30, 0, 0, time.Local)

	writeArchiveIndexTestArchive(t, tmpDir, archiveFileName, []*models.Task{
		archiveIndexTestTask("arch-good", 1, models.TaskTypeTask, "Good archive", completedAt),
	})
	badContent := "### Task: Bad archive\n" + strings.Repeat("x", scannerMaxBufSize+1)
	if err := os.WriteFile(filepath.Join(tmpDir, "completed.1.md"), []byte(badContent), FilePermissions); err != nil {
		t.Fatalf("write corrupt archive: %v", err)
	}

	summaries, err := LoadArchivedTaskSummaries(project)
	if err != nil {
		t.Fatalf("LoadArchivedTaskSummaries returned error: %v", err)
	}
	if len(summaries) != 1 || summaries[0].ID != "arch-good" {
		t.Fatalf("unexpected summaries with corrupt archive present: %+v", summaries)
	}

	index := readArchiveIndexTestFile(t, project)
	if len(index.Sources) != 2 {
		t.Fatalf("expected 2 source records, got %d", len(index.Sources))
	}
	if index.Sources[1].FileName != "completed.1.md" || index.Sources[1].TaskCount != 0 {
		t.Fatalf("corrupt source metadata mismatch: %+v", index.Sources[1])
	}
}

func TestFindArchivedTaskUsesSummarySource(t *testing.T) {
	tmpDir := t.TempDir()
	project := &models.Project{Path: tmpDir, Name: "TestProj"}
	completedAt := time.Date(2026, 6, 8, 10, 30, 0, 0, time.Local)

	mainTask := archiveIndexTestTask("arch-main-source", 1, models.TaskTypeTask, "Main source", completedAt)
	mainTask.Description = "main description"
	rotatedTask := archiveIndexTestTask("arch-rotated-source", 1, models.TaskTypeBug, "Rotated source", completedAt)
	rotatedTask.Description = "rotated description"
	writeArchiveIndexTestArchive(t, tmpDir, archiveFileName, []*models.Task{mainTask})
	writeArchiveIndexTestArchive(t, tmpDir, "completed.1.md", []*models.Task{rotatedTask})

	found, err := FindArchivedTask(project, "ARCH-ROTATED")
	if err != nil {
		t.Fatalf("FindArchivedTask by uppercase prefix returned error: %v", err)
	}
	if found.ID != rotatedTask.ID || found.Description != rotatedTask.Description {
		t.Fatalf("found task mismatch: %+v", found)
	}

	if _, err := FindArchivedTask(project, "1"); !errors.Is(err, tterrors.ErrAmbiguousIdentifier) {
		t.Fatalf("FindArchivedTask duplicate short ID error = %v, want ambiguous identifier", err)
	}
}

func TestFindArchivedTaskReportsDuplicateFullIDs(t *testing.T) {
	tmpDir := t.TempDir()
	project := &models.Project{Path: tmpDir, Name: "TestProj"}
	completedAt := time.Date(2026, 6, 8, 10, 30, 0, 0, time.Local)

	first := archiveIndexTestTask("arch-duplicate-full-id", 10, models.TaskTypeTask, "Duplicate one", completedAt)
	second := archiveIndexTestTask("arch-duplicate-full-id", 11, models.TaskTypeBug, "Duplicate two", completedAt)
	writeArchiveIndexTestArchive(t, tmpDir, archiveFileName, []*models.Task{first})
	writeArchiveIndexTestArchive(t, tmpDir, "completed.1.md", []*models.Task{second})

	_, err := FindArchivedTask(project, "arch-duplicate-full-id")
	if !errors.Is(err, tterrors.ErrAmbiguousIdentifier) {
		t.Fatalf("FindArchivedTask duplicate full ID error = %v, want ambiguous identifier", err)
	}
}

func archiveIndexTestTask(id string, shortID int, taskType models.TaskType, title string, completedAt time.Time) *models.Task {
	return &models.Task{
		ID:          id,
		ShortID:     shortID,
		Type:        taskType,
		Title:       title,
		Status:      models.StatusDone,
		Priority:    2,
		CreatedAt:   completedAt.AddDate(0, 0, -1),
		UpdatedAt:   completedAt,
		CompletedAt: &completedAt,
	}
}

func writeArchiveIndexTestArchive(t *testing.T, projectPath, fileName string, tasks []*models.Task) {
	t.Helper()
	content := WriteArchivedTasks("TestProj", tasks)
	if err := os.WriteFile(filepath.Join(projectPath, fileName), []byte(content), FilePermissions); err != nil {
		t.Fatalf("write %s: %v", fileName, err)
	}
}

func appendArchiveIndexTestArchive(t *testing.T, projectPath, fileName string, tasks []*models.Task) {
	t.Helper()
	content := WriteArchivedTasks("TestProj", tasks)
	f, err := os.OpenFile(filepath.Join(projectPath, fileName), os.O_APPEND|os.O_WRONLY, FilePermissions)
	if err != nil {
		t.Fatalf("open %s for append: %v", fileName, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			t.Fatalf("close %s: %v", fileName, closeErr)
		}
	}()
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("append %s: %v", fileName, err)
	}
}

func readArchiveIndexTestFile(t *testing.T, project *models.Project) ArchiveIndex {
	t.Helper()
	data, err := os.ReadFile(archiveIndexPath(project))
	if err != nil {
		t.Fatalf("read archive index: %v", err)
	}
	var index ArchiveIndex
	if err := json.Unmarshal(data, &index); err != nil {
		t.Fatalf("unmarshal archive index: %v", err)
	}
	return index
}

func archiveIndexSummaryIDs(summaries []ArchiveTaskSummary) []string {
	ids := make([]string, len(summaries))
	for i, summary := range summaries {
		ids[i] = summary.ID
	}
	return ids
}
