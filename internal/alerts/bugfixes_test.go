package alerts

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TestCheckBlockedTasks_NoActivityZeroUpdatedAtSkipped verifies that a
// blocked task with neither an activity-log entry nor a populated
// UpdatedAt does not emit a spurious "Blocked for ~700000 days" alert.
func TestCheckBlockedTasks_NoActivityZeroUpdatedAtSkipped(t *testing.T) {
	now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	task := &models.Task{
		ID:     "blk-zero",
		Title:  "blocked but zero",
		Status: models.StatusBlocked,
		// UpdatedAt intentionally zero, Activity nil
	}
	projects := []ProjectData{{
		Project: &models.Project{ID: "p1", Name: "P1"},
		Tasks:   []*models.Task{task},
	}}

	cfg := DefaultAlertConfig()
	cfg.BlockedDays = 7
	alerts := checkBlockedTasks(cfg, projects, now)
	if len(alerts) != 0 {
		t.Errorf("expected 0 blocked alerts when no activity and UpdatedAt zero, got %d (%+v)", len(alerts), alerts)
	}
}

// TestCheckProjectHealth_AllZeroUpdatedAtSkipped verifies that a project
// whose tasks all have a zero UpdatedAt does not produce a spurious
// project-health alert with an astronomically large "days ago" count.
func TestCheckProjectHealth_AllZeroUpdatedAtSkipped(t *testing.T) {
	now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	tasks := []*models.Task{
		{ID: "t1", Title: "T1", Status: models.StatusTodo}, // UpdatedAt zero
		{ID: "t2", Title: "T2", Status: models.StatusInProgress},
	}
	projects := []ProjectData{{
		Project: &models.Project{ID: "p1", Name: "P1"},
		Tasks:   tasks,
	}}

	alerts := checkProjectHealth(DefaultAlertConfig(), projects, now)
	if len(alerts) != 0 {
		t.Errorf("expected 0 project-health alerts when all task UpdatedAt are zero, got %d (%+v)", len(alerts), alerts)
	}
}
