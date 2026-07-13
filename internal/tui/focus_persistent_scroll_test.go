package tui

import (
	"fmt"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestFocusBoardViewPersistentScrollKeepsWindowOnMoveUp(t *testing.T) {
	project, err := models.NewProject("proj", "proj", "/tmp/test-proj")
	if err != nil {
		t.Fatal(err)
	}

	m := newTestFocusBoard()
	m.SetSize(100, 10)
	m.tabs.Active = boardTabNext

	for i := range 8 {
		task := makeBoardTask(t, fmt.Sprintf("id-%02d", i), i+1, fmt.Sprintf("Task %02d", i))
		row := boardRow{Task: task, Project: project, Source: project.Name}
		m.rows = append(m.rows, row)
		m.filtered = append(m.filtered, row)
	}

	m.cursor = 4
	_ = m.View()
	start := m.viewScrollOffset
	if start == 0 {
		t.Fatalf("expected initial render to scroll down, offset stayed at 0")
	}

	m.cursor = 3
	_ = m.View()
	if m.viewScrollOffset != start {
		t.Fatalf("moving up inside viewport changed first rendered row: offset=%d want %d", m.viewScrollOffset, start)
	}
}

func TestFocusViewPersistentScrollKeepsWindowOnMoveUp(t *testing.T) {
	project := newTestFocusProject(t, "Backlog")
	cfg := newTestFocusConfig([]*models.Project{project})
	m := newTestFocusViewModel(cfg)
	m.SetSize(100, 10)
	m.tabs.Active = focusTabByProject

	for i := range 8 {
		task := newTestFocusTask(t, fmt.Sprintf("Task %02d", i), i+1, true)
		project.Tasks = append(project.Tasks, task)
		m.tasks = append(m.tasks, FocusTask{Task: task, Project: project})
	}
	m.applyFilter()

	m.cursor = 5
	_ = m.View()
	start := m.viewScrollOffset
	if start == 0 {
		t.Fatalf("expected initial render to scroll down, offset stayed at 0")
	}

	m.cursor = 4
	_ = m.View()
	if m.viewScrollOffset != start {
		t.Fatalf("moving up inside viewport changed first rendered row: offset=%d want %d", m.viewScrollOffset, start)
	}
}

func TestProjectFocusViewPersistentScrollKeepsWindowOnMoveUp(t *testing.T) {
	cfg := testutil.NewTestConfig()
	project := testutil.NewTestProject("focus-test")
	queue := make([]config.FocusQueueItem, 0, 8)
	for i := range 8 {
		task := mustNewTestTask(project, fmt.Sprintf("Task %02d", i))
		task.Priority = i + 1
		project.Tasks = append(project.Tasks, task)
		queue = append(queue, config.FocusQueueItem{TaskID: task.ID, ProjectID: project.ID})
	}
	project.BuildTaskTree()
	cfg.SetFocusQueue(queue)

	m := NewProjectFocusViewModel(project, cfg)
	m.SetSize(100, 11)

	m.cursor = 4
	_ = m.View()
	start := m.viewScrollOffset
	if start == 0 {
		t.Fatalf("expected initial render to scroll down, offset stayed at 0")
	}

	m.cursor = 3
	_ = m.View()
	if m.viewScrollOffset != start {
		t.Fatalf("moving up inside viewport changed first rendered row: offset=%d want %d", m.viewScrollOffset, start)
	}
}
