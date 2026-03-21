package tui

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func newTestProjectTreeConfig() *config.Config {
	return &config.Config{
		Projects:          []*models.Project{},
		ProjectViewStates: make(map[string]config.ProjectViewState),
		SectionColors:     make(map[string]string),
	}
}

func newTestProjectTreeProject(t *testing.T, name string, tasks []*models.Task) *models.Project {
	t.Helper()
	p, err := models.NewProject(uuid.New().String(), name, "")
	if err != nil {
		t.Fatal(err)
	}
	p.Tasks = tasks
	return p
}

func newTestProjectTreeTask(t *testing.T, title string) *models.Task {
	t.Helper()
	task, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, title)
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func TestProjectTreeView_BasicRender(t *testing.T) {
	task1 := newTestProjectTreeTask(t, "First task")
	task2 := newTestProjectTreeTask(t, "Second task")
	project := newTestProjectTreeProject(t, "Test Project", []*models.Task{task1, task2})
	cfg := newTestProjectTreeConfig()

	m := NewProjectTreeModel(project, cfg, nil)
	m.SetSize(80, 24)

	output := m.View()
	if !strings.Contains(output, "First task") {
		t.Error("expected output to contain 'First task'")
	}
	if !strings.Contains(output, "Second task") {
		t.Error("expected output to contain 'Second task'")
	}
}

func TestProjectTreeView_EmptyProject(t *testing.T) {
	project := newTestProjectTreeProject(t, "Empty Project", nil)
	cfg := newTestProjectTreeConfig()

	m := NewProjectTreeModel(project, cfg, nil)
	m.SetSize(80, 24)

	output := m.View()
	if output == "" {
		t.Error("expected non-empty output even for empty project")
	}
}

func TestProjectTreeView_CursorBounds(t *testing.T) {
	project := newTestProjectTreeProject(t, "Empty", nil)
	cfg := newTestProjectTreeConfig()

	m := NewProjectTreeModel(project, cfg, nil)
	m.cursor = 100
	m.clampCursor()

	if m.cursor != 0 {
		t.Errorf("expected cursor clamped to 0, got %d", m.cursor)
	}
}

func TestProjectTreeView_FilterState(t *testing.T) {
	project := newTestProjectTreeProject(t, "Test", nil)
	cfg := newTestProjectTreeConfig()

	m := NewProjectTreeModel(project, cfg, nil)

	if m.filter.Type != "" {
		t.Errorf("expected empty type filter, got %q", m.filter.Type)
	}
	if m.filter.Status != "" {
		t.Errorf("expected empty status filter, got %q", m.filter.Status)
	}
	if m.filter.Priority != 0 {
		t.Errorf("expected 0 priority filter, got %d", m.filter.Priority)
	}
}
