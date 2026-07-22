package tui

import (
	"fmt"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func newRankedDateScrollTask(t *testing.T, id string, shortID int, title string) *models.Task {
	t.Helper()
	task, err := models.NewTask(id, shortID, models.TaskTypeTask, title)
	if err != nil {
		t.Fatal(err)
	}
	task.WorkflowState = models.StateReady
	task.Priority = 3
	return task
}

func TestRecentActivityPersistentScrollKeepsWindowOnMoveUp(t *testing.T) {
	cfg := newTestConfig()
	project := testutil.NewTestProject("Recent")
	m := &RecentActivityModel{
		runtime: testRuntimeAt(cfg.DataDir(), cfg),
		tabs:    TabState{Names: recentActivityTabNames},
	}
	m.SetSize(100, 10)

	now := time.Now()
	for i := range 8 {
		task := newRankedDateScrollTask(t, fmt.Sprintf("recent-%d", i), i+1, fmt.Sprintf("Recent %02d", i))
		task.UpdatedAt = now.Add(-time.Duration(i) * time.Minute)
		item := CrossProjectTask{Task: task, Project: project}
		m.tasks = append(m.tasks, item)
		m.filtered = append(m.filtered, item)
	}

	m.cursor = 4
	_ = m.View()
	start := m.viewScrollOffset
	if start == 0 {
		t.Fatalf("expected recent activity to scroll down, offset stayed at 0")
	}

	m.cursor = 3
	_ = m.View()
	if m.viewScrollOffset != start {
		t.Fatalf("moving up inside viewport changed first rendered row: offset=%d want %d", m.viewScrollOffset, start)
	}
}
