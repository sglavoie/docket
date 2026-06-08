package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// Archive performance baseline:
// - LoadProjectLargeTasksMD measures normal active project parsing from tasks.md.
// - LoadArchivedTasksLargeCompletedMD keeps the explicit full archive parser measurable.
// - LoadArchivedTaskSummariesLargeCompletedMD measures the summary-index hot path.
// - LoadArchivedTaskSummariesRotatedArchives measures the rotated summary-index hot path.
//
// Guardrail expectation: normal task surfaces do not read completed.md, and
// archive-aware analytics use compact summaries instead of constructing full
// models.Task values from archive markdown bodies.

func BenchmarkLoadProjectLargeTasksMD(b *testing.B) {
	const taskCount = 5000

	project := newArchiveBenchProject(b, "large-active")
	project.Tasks = makeArchiveBenchTasks(b, "live", taskCount, false, 256)
	if err := SaveProject(project); err != nil {
		b.Fatalf("save project: %v", err)
	}

	reportArchiveBenchFileSize(b, filepath.Join(project.Path, tasksFileName), "tasks_md_bytes")
	b.ReportMetric(taskCount, "tasks")
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		loaded := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
		if err := LoadProject(loaded); err != nil {
			b.Fatalf("load project: %v", err)
		}
		if len(loaded.Tasks) != taskCount {
			b.Fatalf("loaded %d tasks, want %d", len(loaded.Tasks), taskCount)
		}
	}
}

func BenchmarkLoadArchivedTasksLargeCompletedMD(b *testing.B) {
	const taskCount = 5000

	project := newArchiveBenchProject(b, "large-archive")
	writeArchiveBenchFile(b, filepath.Join(project.Path, archiveFileName), project.Name, "archive", taskCount, 512)

	reportArchiveBenchFileSize(b, filepath.Join(project.Path, archiveFileName), "completed_md_bytes")
	b.ReportMetric(taskCount, "archived_tasks")
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		tasks := LoadArchivedTasks(project)
		if len(tasks) != taskCount {
			b.Fatalf("loaded %d archived tasks, want %d", len(tasks), taskCount)
		}
	}
}

func BenchmarkLoadArchivedTasksRotatedArchives(b *testing.B) {
	const (
		fileCount      = 5
		tasksPerFile   = 1000
		totalTaskCount = fileCount * tasksPerFile
	)

	project := newArchiveBenchProject(b, "rotated-archive")
	for fileIndex := range fileCount {
		name := archiveFileName
		if fileIndex > 0 {
			name = fmt.Sprintf("completed.%d.md", fileIndex)
		}
		writeArchiveBenchFile(
			b,
			filepath.Join(project.Path, name),
			project.Name,
			fmt.Sprintf("archive-%d", fileIndex),
			tasksPerFile,
			512,
		)
	}

	b.ReportMetric(totalTaskCount, "archived_tasks")
	b.ReportMetric(fileCount, "archive_files")
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		tasks := LoadArchivedTasks(project)
		if len(tasks) != totalTaskCount {
			b.Fatalf("loaded %d archived tasks, want %d", len(tasks), totalTaskCount)
		}
	}
}

func BenchmarkLoadArchivedTaskSummariesLargeCompletedMD(b *testing.B) {
	const taskCount = 5000

	project := newArchiveBenchProject(b, "large-archive-summary")
	archivePath := filepath.Join(project.Path, archiveFileName)
	writeArchiveBenchFile(b, archivePath, project.Name, "archive-summary", taskCount, 1024)
	if err := RebuildArchiveIndex(project); err != nil {
		b.Fatalf("rebuild archive index: %v", err)
	}

	reportArchiveBenchFileSize(b, archivePath, "completed_md_bytes")
	reportArchiveBenchFileSize(b, archiveIndexPath(project), "archive_index_bytes")
	b.ReportMetric(taskCount, "archived_tasks")
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		summaries, err := LoadArchivedTaskSummaries(project)
		if err != nil {
			b.Fatalf("load archive summaries: %v", err)
		}
		if len(summaries) != taskCount {
			b.Fatalf("loaded %d archive summaries, want %d", len(summaries), taskCount)
		}
	}
}

func BenchmarkLoadArchivedTaskSummariesRotatedArchives(b *testing.B) {
	const (
		fileCount      = 5
		tasksPerFile   = 1000
		totalTaskCount = fileCount * tasksPerFile
	)

	project := newArchiveBenchProject(b, "rotated-archive-summary")
	for fileIndex := range fileCount {
		name := archiveFileName
		if fileIndex > 0 {
			name = fmt.Sprintf("completed.%d.md", fileIndex)
		}
		writeArchiveBenchFile(
			b,
			filepath.Join(project.Path, name),
			project.Name,
			fmt.Sprintf("archive-summary-%d", fileIndex),
			tasksPerFile,
			1024,
		)
	}
	if err := RebuildArchiveIndex(project); err != nil {
		b.Fatalf("rebuild archive index: %v", err)
	}

	reportArchiveBenchFileSize(b, archiveIndexPath(project), "archive_index_bytes")
	b.ReportMetric(totalTaskCount, "archived_tasks")
	b.ReportMetric(fileCount, "archive_files")
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		summaries, err := LoadArchivedTaskSummaries(project)
		if err != nil {
			b.Fatalf("load archive summaries: %v", err)
		}
		if len(summaries) != totalTaskCount {
			b.Fatalf("loaded %d archive summaries, want %d", len(summaries), totalTaskCount)
		}
	}
}

func newArchiveBenchProject(b *testing.B, slug string) *models.Project {
	b.Helper()

	projectPath := filepath.Join(b.TempDir(), slug)
	project, err := models.NewProject("bench-"+slug, "Bench "+slug, projectPath)
	if err != nil {
		b.Fatalf("new project: %v", err)
	}
	return project
}

func writeArchiveBenchFile(b *testing.B, path, projectName, prefix string, count, descriptionBytes int) {
	b.Helper()

	tasks := makeArchiveBenchTasks(b, prefix, count, true, descriptionBytes)
	content := WriteArchivedTasks(projectName, tasks)
	if err := os.MkdirAll(filepath.Dir(path), DirPermissions); err != nil {
		b.Fatalf("create archive dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), FilePermissions); err != nil {
		b.Fatalf("write archive: %v", err)
	}
}

func makeArchiveBenchTasks(b *testing.B, prefix string, count int, completed bool, descriptionBytes int) []*models.Task {
	b.Helper()

	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	tasks := make([]*models.Task, 0, count)
	description := strings.Repeat("x", descriptionBytes)
	for i := range count {
		id := fmt.Sprintf("%s-%05d", prefix, i+1)
		task, err := models.NewTask(id, i+1, models.TaskTypeTask, fmt.Sprintf("Benchmark task %05d", i+1))
		if err != nil {
			b.Fatalf("new task: %v", err)
		}
		task.CreatedAt = now.AddDate(0, 0, -60).Add(time.Duration(i) * time.Minute)
		task.UpdatedAt = task.CreatedAt.Add(24 * time.Hour)
		task.Description = description
		task.TaskAttrs = &models.TaskAttrs{Effort: models.EffortMedium}
		if completed {
			startedAt := now.AddDate(0, 0, -20+(i%10))
			completedAt := startedAt.Add(48 * time.Hour)
			task.Status = models.StatusDone
			task.CompletedAt = &completedAt
			task.Activity = []*models.ActivityEntry{
				{Timestamp: startedAt, Field: "status", OldValue: string(models.StatusTodo), NewValue: string(models.StatusInProgress)},
				{Timestamp: completedAt, Field: "status", OldValue: string(models.StatusInProgress), NewValue: string(models.StatusDone)},
			}
		}
		tasks = append(tasks, task)
	}
	return tasks
}

func reportArchiveBenchFileSize(b *testing.B, path, unit string) {
	b.Helper()

	info, err := os.Stat(path)
	if err != nil {
		b.Fatalf("stat %s: %v", path, err)
	}
	b.ReportMetric(float64(info.Size()), unit)
}
