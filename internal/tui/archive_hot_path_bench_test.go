package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/metrics"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// Archive-aware dashboard guardrail:
// MetricsDashboardModel.computeMetrics and RoadmapModel.computeData should load
// tasks.md for active work and read compact archive summaries for completed
// history. They must not fully parse completed.md on each recompute.

func BenchmarkMetricsDashboardArchiveHotPath(b *testing.B) {
	_, manager := newArchiveHotPathBenchConfig(b, 3, 50, 1500)
	model := &MetricsDashboardModel{
		configManager: manager,
		dataDir:       metricsDataDir(manager),
		isGlobal:      true,
		timeRange:     metrics.TimeRange30d,
	}

	b.ReportMetric(3, "projects")
	b.ReportMetric(4500, "archived_tasks")
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		model.computeMetrics()
		if model.metrics == nil || model.metrics.Throughput.Total == 0 {
			b.Fatal("metrics dashboard did not include archived throughput")
		}
	}
}

func BenchmarkRoadmapArchiveHotPath(b *testing.B) {
	cfg, _ := newArchiveHotPathBenchConfig(b, 3, 50, 1500)
	model := &RoadmapModel{
		config:     cfg,
		isGlobal:   true,
		zoom:       ZoomMonth,
		collapsed:  make(map[string]bool),
		labelWidth: 30,
	}

	b.ReportMetric(3, "projects")
	b.ReportMetric(4500, "archived_tasks")
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		model.computeData()
		if model.velocity <= 0 {
			b.Fatal("roadmap velocity did not include archived throughput")
		}
		if len(model.scheduled) == 0 {
			b.Fatal("roadmap did not schedule live tasks")
		}
	}
}

// newArchiveHotPathBenchConfig builds a real on-disk workspace: projects live at
// the catalog-derived layout (<dataDir>/projects/<slug>) and a manager persists
// the catalog so the domain-backed metrics dashboard resolves them via
// RuntimeDependencies().ActiveProjects. The returned cfg keeps the same in-memory
// project slice the roadmap benchmark still consumes directly.
func newArchiveHotPathBenchConfig(b *testing.B, projectCount, livePerProject, archivedPerProject int) (*config.Config, *config.Manager) {
	b.Helper()

	dataDir := b.TempDir()
	cfg := config.NewConfig()
	cfg.Projects = nil
	cfg.DataDir = dataDir
	for projectIndex := range projectCount {
		slug := fmt.Sprintf("archive-hot-path-%02d", projectIndex+1)
		projectPath := filepath.Join(dataDir, "projects", slug)
		project, err := models.NewProject("bench-"+slug, fmt.Sprintf("Archive Hot Path %02d", projectIndex+1), projectPath)
		if err != nil {
			b.Fatalf("new project: %v", err)
		}
		project.Slug = slug
		project.Path = projectPath
		project.Tasks = makeArchiveHotPathBenchTasks(b, fmt.Sprintf("live-%02d", projectIndex+1), livePerProject, false)
		if err := storage.SaveProject(project); err != nil {
			b.Fatalf("save project: %v", err)
		}
		writeArchiveHotPathBenchArchive(b, project, fmt.Sprintf("archived-%02d", projectIndex+1), archivedPerProject)
		cfg.Projects = append(cfg.Projects, project)
	}
	manager := config.NewManager(filepath.Join(dataDir, ".tt.json"))
	if err := manager.Save(cfg); err != nil {
		b.Fatalf("save config: %v", err)
	}
	return cfg, manager
}

func writeArchiveHotPathBenchArchive(b *testing.B, project *models.Project, prefix string, count int) {
	b.Helper()

	tasks := makeArchiveHotPathBenchTasks(b, prefix, count, true)
	content := storage.WriteArchivedTasks(project.Name, tasks)
	path := filepath.Join(project.Path, "completed.md")
	if err := os.WriteFile(path, []byte(content), storage.FilePermissions); err != nil {
		b.Fatalf("write archive: %v", err)
	}
	if err := storage.RebuildArchiveIndex(project); err != nil {
		b.Fatalf("rebuild archive index: %v", err)
	}
}

func makeArchiveHotPathBenchTasks(b *testing.B, prefix string, count int, completed bool) []*models.Task {
	b.Helper()

	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	tasks := make([]*models.Task, 0, count)
	description := strings.Repeat("x", 512)
	for i := range count {
		id := fmt.Sprintf("%s-%05d", prefix, i+1)
		task, err := models.NewTask(id, i+1, models.TaskTypeTask, fmt.Sprintf("Benchmark task %05d", i+1))
		if err != nil {
			b.Fatalf("new task: %v", err)
		}
		task.CreatedAt = now.AddDate(0, 0, -60).Add(time.Duration(i) * time.Minute)
		task.Description = description
		task.TaskAttrs = &models.TaskAttrs{Effort: models.EffortMedium}
		task.EstimatedDays = 1
		if completed {
			startedAt := now.AddDate(0, 0, -20+(i%20))
			completedAt := startedAt.Add(48 * time.Hour)
			task.WorkflowState = models.StateDone
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
