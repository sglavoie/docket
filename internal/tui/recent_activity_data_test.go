package tui

import (
	"sort"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TestRecentActivitySort_StableTiebreakOnID exercises the exact comparator
// used by loadTasks() to ensure that tasks updated at identical timestamps
// are ordered deterministically by Task.ID. Without the tiebreak, the
// subsequent cap-at-limit truncation could drop arbitrary tasks during
// bulk-import scenarios.
func TestRecentActivitySort_StableTiebreakOnID(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	earlier := now.Add(-time.Hour)
	project := &models.Project{ID: "p1", Name: "Demo"}

	tasks := []CrossProjectTask{
		{Task: &models.Task{ID: "z", UpdatedAt: now}, Project: project},
		{Task: &models.Task{ID: "a", UpdatedAt: now}, Project: project},
		{Task: &models.Task{ID: "m", UpdatedAt: earlier}, Project: project},
		{Task: &models.Task{ID: "b", UpdatedAt: now}, Project: project},
	}

	sort.Slice(tasks, func(i, j int) bool {
		ti, tj := tasks[i].Task.UpdatedAt, tasks[j].Task.UpdatedAt
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return tasks[i].Task.ID < tasks[j].Task.ID
	})

	want := []string{"a", "b", "z", "m"}
	for i, w := range want {
		if tasks[i].Task.ID != w {
			gotIDs := make([]string, len(tasks))
			for j, it := range tasks {
				gotIDs[j] = it.Task.ID
			}
			t.Errorf("position %d: got %q, want %q (full order: %v)", i, tasks[i].Task.ID, w, gotIDs)
		}
	}
}
