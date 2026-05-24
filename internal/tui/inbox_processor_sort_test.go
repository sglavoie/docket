package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TestInboxProcessor_SortStableTiebreakOnID mirrors
// TestGlobalInboxProcessor_SortStableTiebreakOnID but exercises the
// single-project getInboxTasksForModel function. Tasks created at the same
// timestamp must sort by ID so the inbox order does not flip across reloads.
func TestInboxProcessor_SortStableTiebreakOnID(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	later := now.Add(time.Hour)

	mkInbox := func(id string, ts time.Time) *models.Task {
		return &models.Task{
			ID:        id,
			CreatedAt: ts,
			Section:   models.SectionInbox,
			Type:      models.TaskTypeTask,
			Status:    models.StatusTodo,
		}
	}

	project := &models.Project{
		ID:   "p1",
		Slug: "p1",
		Name: "Demo",
		Tasks: []*models.Task{
			mkInbox("later-b", later),
			mkInbox("now-c", now),
			mkInbox("now-a", now),
			mkInbox("later-a", later),
		},
	}

	tasks := getInboxTasksForModel(project)

	want := []string{"now-a", "now-c", "later-a", "later-b"}
	if len(tasks) != len(want) {
		t.Fatalf("got %d tasks, want %d", len(tasks), len(want))
	}
	for i, w := range want {
		if tasks[i].ID != w {
			t.Fatalf("position %d: got %q, want %q", i, tasks[i].ID, w)
		}
	}
}
