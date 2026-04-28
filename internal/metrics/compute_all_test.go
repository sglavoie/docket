package metrics

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestComputeAll(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	done := now.AddDate(0, 0, -2)
	start := now.AddDate(0, 0, -4)

	project := &models.Project{
		ID:   "test-project",
		Name: "Test",
		Tasks: []*models.Task{
			makeTask(models.StatusDone, timePtr(done), []*models.ActivityEntry{
				{Timestamp: start, Field: "status", OldValue: "todo", NewValue: "in_progress"},
			}),
			makeTask(models.StatusTodo, nil, nil),
		},
	}

	result := ComputeAllAt([]*models.Project{project}, TimeRange30d, now)

	if result.Completion.Total != 1 {
		t.Errorf("Completion.Total = %d, want 1", result.Completion.Total)
	}
	if result.Throughput.Total != 1 {
		t.Errorf("Throughput.Total = %d, want 1", result.Throughput.Total)
	}
	if result.CycleTime.Count != 1 {
		t.Errorf("CycleTime.Count = %d, want 1", result.CycleTime.Count)
	}
	if result.TimeRange != TimeRange30d {
		t.Errorf("TimeRange = %v, want TimeRange30d", result.TimeRange)
	}
	if result.IsGlobal {
		t.Error("IsGlobal should be false for single project")
	}
}

func TestComputeAllMultiProject(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	done := now.AddDate(0, 0, -1)

	projects := []*models.Project{
		{ID: "p1", Tasks: []*models.Task{makeTask(models.StatusDone, timePtr(done), nil)}},
		{ID: "p2", Tasks: []*models.Task{makeTask(models.StatusDone, timePtr(done), nil)}},
	}

	result := ComputeAllAt(projects, TimeRange7d, now)

	if !result.IsGlobal {
		t.Error("IsGlobal should be true for multiple projects")
	}
	if result.Completion.Total != 2 {
		t.Errorf("Completion.Total = %d, want 2", result.Completion.Total)
	}
}

func TestComputeAllExtraTasks(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	t.Run("zero extra args is backward compatible", func(t *testing.T) {
		done := now.AddDate(0, 0, -1)
		project := &models.Project{
			ID:    "p1",
			Tasks: []*models.Task{makeTask(models.StatusDone, timePtr(done), nil)},
		}

		result := ComputeAllAt([]*models.Project{project}, TimeRange7d, now)

		if result.Completion.Total != 1 {
			t.Errorf("Completion.Total = %d, want 1", result.Completion.Total)
		}
		if result.Throughput.Total != 1 {
			t.Errorf("Throughput.Total = %d, want 1", result.Throughput.Total)
		}
	})

	t.Run("extra tasks counted in completion and throughput", func(t *testing.T) {
		done := now.AddDate(0, 0, -1)
		project := &models.Project{
			ID:    "p1",
			Tasks: []*models.Task{makeTask(models.StatusDone, timePtr(done), nil)},
		}

		archived := []*models.Task{
			makeTask(models.StatusDone, timePtr(done), nil),
			makeTask(models.StatusDone, timePtr(done), nil),
		}

		result := ComputeAllAt([]*models.Project{project}, TimeRange7d, now, archived)

		if result.Completion.Total != 3 {
			t.Errorf("Completion.Total = %d, want 3", result.Completion.Total)
		}
		if result.Throughput.Total != 3 {
			t.Errorf("Throughput.Total = %d, want 3", result.Throughput.Total)
		}
	})

	t.Run("extra tasks with activity contribute to cycle time", func(t *testing.T) {
		start := now.AddDate(0, 0, -5)
		done := now.AddDate(0, 0, -1)

		project := &models.Project{
			ID:    "p1",
			Tasks: []*models.Task{},
		}

		archived := []*models.Task{
			makeTask(models.StatusDone, timePtr(done), []*models.ActivityEntry{
				{Timestamp: start, Field: "status", OldValue: "todo", NewValue: "in_progress"},
			}),
		}

		result := ComputeAllAt([]*models.Project{project}, TimeRange7d, now, archived)

		if result.CycleTime.Count != 1 {
			t.Errorf("CycleTime.Count = %d, want 1", result.CycleTime.Count)
		}
	})

	t.Run("extra tasks without activity skipped for cycle time", func(t *testing.T) {
		done := now.AddDate(0, 0, -1)

		project := &models.Project{
			ID:    "p1",
			Tasks: []*models.Task{},
		}

		archived := []*models.Task{
			makeTask(models.StatusDone, timePtr(done), nil),
		}

		result := ComputeAllAt([]*models.Project{project}, TimeRange7d, now, archived)

		if result.CycleTime.Count != 0 {
			t.Errorf("CycleTime.Count = %d, want 0 (no activity)", result.CycleTime.Count)
		}
	})
}