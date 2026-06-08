package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/metrics"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// TestMetricsDashboardWeekStartPropagation verifies that the user's
// configured WeekEndDay flows through MetricsDashboardModel.computeMetrics
// into the weekly throughput bucket labels, instead of defaulting to Monday.
func TestMetricsDashboardWeekStartPropagation(t *testing.T) {
	cfg := config.NewConfig()
	cfg.WeekEndDay = "sat" // week ends Saturday → starts Sunday

	m := NewMetricsDashboardModel(cfg)
	m.timeRange = metrics.TimeRange90d // forces weekly granularity
	m.computeMetrics()

	if m.metrics == nil {
		t.Fatal("expected metrics to be computed")
	}

	want := metrics.WeekStartFromWeekEnd(cfg.GetWeekEndDay())
	if want != time.Sunday {
		t.Fatalf("test fixture broken: WeekStartFromWeekEnd(\"sat\") = %v, want Sunday", want)
	}

	expectAgg := metrics.ComputeAllAtWithWeekStart(nil, metrics.TimeRange90d, time.Now(), want)
	if len(m.metrics.Throughput.Labels) != len(expectAgg.Throughput.Labels) {
		t.Fatalf("Labels len = %d, want %d", len(m.metrics.Throughput.Labels), len(expectAgg.Throughput.Labels))
	}
	for i, lbl := range expectAgg.Throughput.Labels {
		if m.metrics.Throughput.Labels[i] != lbl {
			t.Errorf("Labels[%d] = %q, want %q (week-start %v not propagated)",
				i, m.metrics.Throughput.Labels[i], lbl, want)
		}
	}

	mondayAgg := metrics.ComputeAllAtWithWeekStart(nil, metrics.TimeRange90d, time.Now(), time.Monday)
	if len(mondayAgg.Throughput.Labels) > 0 && len(m.metrics.Throughput.Labels) > 0 &&
		m.metrics.Throughput.Labels[0] == mondayAgg.Throughput.Labels[0] {
		// First label only matches when "now" happens to fall such that
		// both week-starts produce the same first bucket boundary. We
		// only treat this as a hard failure when every label matches.
		allEqual := true
		for i := range mondayAgg.Throughput.Labels {
			if i >= len(m.metrics.Throughput.Labels) || m.metrics.Throughput.Labels[i] != mondayAgg.Throughput.Labels[i] {
				allEqual = false
				break
			}
		}
		if allEqual {
			t.Error("dashboard labels match the Monday default — week-start was not propagated")
		}
	}
}

func TestMetricsDashboardLoadsArchivedSummaries(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "archive-summary")
	project, err := models.NewProject("archive-summary", "Archive Summary", projectPath)
	if err != nil {
		t.Fatalf("new project: %v", err)
	}
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("save project: %v", err)
	}

	now := time.Now()
	startedAt := now.Add(-72 * time.Hour)
	completedAt := now.Add(-24 * time.Hour)
	archived, err := models.NewTask("archived-summary-task", 1, models.TaskTypeTask, "Archived summary task")
	if err != nil {
		t.Fatalf("new archived task: %v", err)
	}
	archived.Status = models.StatusDone
	archived.CompletedAt = &completedAt
	archived.Activity = []*models.ActivityEntry{
		{Timestamp: startedAt, Field: "status", OldValue: string(models.StatusTodo), NewValue: string(models.StatusInProgress)},
	}

	archivePath := filepath.Join(project.Path, "completed.md")
	if err := os.WriteFile(archivePath, []byte(storage.WriteArchivedTasks(project.Name, []*models.Task{archived})), storage.FilePermissions); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	if err := storage.RebuildArchiveIndex(project); err != nil {
		t.Fatalf("rebuild archive index: %v", err)
	}
	if err := os.Chmod(archivePath, 0); err != nil {
		t.Fatalf("make archive unreadable: %v", err)
	}
	defer func() {
		_ = os.Chmod(archivePath, storage.FilePermissions)
	}()

	cfg := config.NewConfig()
	cfg.Projects = []*models.Project{project}
	model := NewMetricsDashboardModel(cfg)

	if model.metrics == nil {
		t.Fatal("expected metrics to be computed")
	}
	if model.metrics.Throughput.Total != 1 {
		t.Fatalf("Throughput.Total = %d, want 1 archived task from summary index", model.metrics.Throughput.Total)
	}
	if model.metrics.CycleTime.Count != 1 {
		t.Fatalf("CycleTime.Count = %d, want 1 archived task from summary index", model.metrics.CycleTime.Count)
	}
}
