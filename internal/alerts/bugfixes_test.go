package alerts

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

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
