package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestProjectTreeActivePriorityTitlesUsePriorityStyle(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	tasks := []*models.Task{
		newPrimaryRowStyleTask(t, "tree-p1", 1, "Project tree active P1", 1),
		newPrimaryRowStyleTask(t, "tree-p2", 2, "Project tree active P2", 2),
	}
	m := NewProjectTreeModel(newPrimaryRowStyleProject(t, "tree-project", tasks), nil, nil)
	// Queue membership replaces the deprecated Active flag for the row marker.
	m.queueIDs = map[string]bool{}
	for _, task := range tasks {
		m.queueIDs[task.ID] = true
	}
	for _, task := range tasks {
		line := m.renderNodeWithWidth(m.findNodeByTaskID(task.ID), false, false, 120)
		assertPrimaryRowTitleStyle(t, line, task)
		assertActiveMarkerVisible(t, line)
	}
}

func TestOutlinerActivePriorityTitlesUsePriorityStyle(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	tasks := []*models.Task{
		newPrimaryRowStyleTask(t, "outliner-p1", 1, "Outliner active P1", 1),
		newPrimaryRowStyleTask(t, "outliner-p2", 2, "Outliner active P2", 2),
	}

	m := NewOutlinerModel(newPrimaryRowStyleProject(t, "outliner-project", tasks), nil, nil)
	// The active marker now reflects Focus Queue membership, not the legacy
	// Active flag.
	m.queueIDs = map[string]bool{}
	for _, task := range tasks {
		m.queueIDs[task.ID] = true
	}
	m.SetSize(120, 20)
	m.rebuildRows()

	for _, task := range tasks {
		rowIndex := findPrimaryRowStyleOutlinerRow(t, m.rows, task.ID)
		line := m.renderOutlinerTaskRow(m.rows[rowIndex], false, 120, 120, rowIndex, nil)
		assertPrimaryRowTitleStyle(t, line, task)
		assertActiveMarkerVisible(t, line)
	}
}

func newPrimaryRowStyleTask(t *testing.T, id string, shortID int, title string, priority int) *models.Task {
	t.Helper()

	task, err := models.NewTask(id, shortID, models.TaskTypeTask, title)
	if err != nil {
		t.Fatalf("NewTask(%q): %v", id, err)
	}
	task.Priority = priority
	return task
}

func newPrimaryRowStyleProject(t *testing.T, id string, tasks []*models.Task) *models.Project {
	t.Helper()

	project, err := models.NewProject(id, "Primary Row Style", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject(%q): %v", id, err)
	}
	project.Tasks = tasks
	project.BuildTaskTree()
	return project
}

func findPrimaryRowStyleOutlinerRow(t *testing.T, rows []outlinerRow, taskID string) int {
	t.Helper()

	for i, row := range rows {
		if row.task != nil && row.task.ID == taskID {
			return i
		}
	}
	t.Fatalf("outliner row for task %q not found", taskID)
	return -1
}

func assertPrimaryRowTitleStyle(t *testing.T, line string, task *models.Task) {
	t.Helper()

	priorityTitle := GetTitlePriorityStyle(task.Priority).Render(task.Title)
	if !strings.Contains(line, priorityTitle) {
		t.Fatalf("expected priority-styled title for %q in:\n%s", task.Title, ansi.Strip(line))
	}

	activeTitle := ActiveStyle.Render(task.Title)
	if activeTitle != priorityTitle && strings.Contains(line, activeTitle) {
		t.Fatalf("active state should not recolor title %q in:\n%s", task.Title, ansi.Strip(line))
	}
}

func assertActiveMarkerVisible(t *testing.T, line string) {
	t.Helper()

	if !strings.Contains(ansi.Strip(line), "*") {
		t.Fatalf("expected active marker in:\n%s", ansi.Strip(line))
	}
}
