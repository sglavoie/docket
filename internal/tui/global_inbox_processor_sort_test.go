package tui

import (
	"sort"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TestGlobalInboxProcessor_SortStableTiebreakOnID mirrors the comparator
// used by loadAllInboxTasks() and verifies that tasks created in the same
// moment (common during bulk import / migration) are sorted deterministically
// so that the prompt-by-prompt processing order does not flip across reloads.
func TestGlobalInboxProcessor_SortStableTiebreakOnID(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	later := now.Add(time.Hour)
	p := &models.Project{ID: "p1", Name: "Demo"}

	tasks := []GlobalInboxTask{
		{Task: &models.Task{ID: "later-b", CreatedAt: later}, Project: p},
		{Task: &models.Task{ID: "now-c", CreatedAt: now}, Project: p},
		{Task: &models.Task{ID: "now-a", CreatedAt: now}, Project: p},
		{Task: &models.Task{ID: "later-a", CreatedAt: later}, Project: p},
	}

	sort.Slice(tasks, func(i, j int) bool {
		ci, cj := tasks[i].Task.CreatedAt, tasks[j].Task.CreatedAt
		if !ci.Equal(cj) {
			return ci.Before(cj)
		}
		return tasks[i].Task.ID < tasks[j].Task.ID
	})

	want := []string{"now-a", "now-c", "later-a", "later-b"}
	for i, w := range want {
		if tasks[i].Task.ID != w {
			t.Fatalf("position %d: got %q, want %q", i, tasks[i].Task.ID, w)
		}
	}
}
