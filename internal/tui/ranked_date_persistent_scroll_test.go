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
	task.Status = models.StatusTodo
	task.Priority = 3
	return task
}

func TestPriorityViewPersistentScrollUsesRenderedLineIndex(t *testing.T) {
	cfg := testutil.NewTestConfig()
	project := testutil.NewTestProject("Priority")
	m := &PriorityViewModel{
		ViewBundle:         ViewBundle{config: cfg},
		project:            project,
		waitingOnCollapsed: true,
		tabs:               TabState{Names: priorityTabNames},
	}
	m.SetSize(100, 10)

	now := time.Now()
	for i := 0; i < 4; i++ {
		task := newRankedDateScrollTask(t, fmt.Sprintf("overdue-%d", i), i+1, fmt.Sprintf("Overdue %02d", i))
		due := now.AddDate(0, 0, -1)
		task.DueDate = &due
		pt := &PriorityTask{Task: task, RelativeDue: FormatRelativeDue(task.DueDate, false)}
		m.overdue = append(m.overdue, pt)
		project.Tasks = append(project.Tasks, task)
	}
	for i := 0; i < 4; i++ {
		task := newRankedDateScrollTask(t, fmt.Sprintf("today-%d", i), i+5, fmt.Sprintf("Today %02d", i))
		due := now
		task.DueDate = &due
		pt := &PriorityTask{Task: task, RelativeDue: FormatRelativeDue(task.DueDate, false)}
		m.today = append(m.today, pt)
		project.Tasks = append(project.Tasks, task)
	}
	m.totalTasks = len(m.overdue) + len(m.today)

	m.cursor = 5
	_ = m.View()
	start := m.viewScrollOffset
	if start == 0 {
		t.Fatalf("expected grouped priority view to scroll down, offset stayed at 0")
	}

	m.cursor = 4
	_ = m.View()
	if m.viewScrollOffset != start {
		t.Fatalf("moving up inside viewport changed first rendered row: offset=%d want %d", m.viewScrollOffset, start)
	}
}

func TestDueDateOverviewPersistentScrollUsesRenderedLineIndex(t *testing.T) {
	cfg := testutil.NewTestConfig()
	project := testutil.NewTestProject("Due Dates")
	m := &DueDateOverviewModel{
		ViewBundle: ViewBundle{config: cfg},
		tabs:       TabState{Names: dueDateTabNames},
	}
	m.SetSize(100, 11)

	now := time.Now()
	for i := 0; i < 4; i++ {
		task := newRankedDateScrollTask(t, fmt.Sprintf("due-overdue-%d", i), i+1, fmt.Sprintf("Overdue %02d", i))
		due := now.AddDate(0, 0, -1)
		task.DueDate = &due
		m.overdue = append(m.overdue, CrossProjectTask{Task: task, Project: project})
	}
	for i := 0; i < 4; i++ {
		task := newRankedDateScrollTask(t, fmt.Sprintf("due-today-%d", i), i+5, fmt.Sprintf("Today %02d", i))
		due := now
		task.DueDate = &due
		m.today = append(m.today, CrossProjectTask{Task: task, Project: project})
	}
	m.recalcTotalTasks()
	m.viewScrollOffset = 0

	m.cursor = 5
	_ = m.View()
	start := m.viewScrollOffset
	if start == 0 {
		t.Fatalf("expected grouped due-date view to scroll down, offset stayed at 0")
	}

	m.cursor = 4
	_ = m.View()
	if m.viewScrollOffset != start {
		t.Fatalf("moving up inside viewport changed first rendered row: offset=%d want %d", m.viewScrollOffset, start)
	}
}

func TestRecentActivityPersistentScrollKeepsWindowOnMoveUp(t *testing.T) {
	cfg := testutil.NewTestConfig()
	project := testutil.NewTestProject("Recent")
	m := &RecentActivityModel{
		config: cfg,
		tabs:   TabState{Names: recentActivityTabNames},
	}
	m.SetSize(100, 10)

	now := time.Now()
	for i := 0; i < 8; i++ {
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
