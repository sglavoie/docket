package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestOutlinerQueuedPriorityTitlesUsePriorityStyle(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	tasks := []*models.Task{
		newPrimaryRowStyleTask(t, "outliner-p1", 1, "Outliner queued P1", 1),
		newPrimaryRowStyleTask(t, "outliner-p2", 2, "Outliner queued P2", 2),
	}

	m := NewOutlinerModel(newPrimaryRowStyleProject(t, "outliner-project", tasks), nil, nil)
	m.SetSize(120, 20)
	m.rebuildRows()

	for _, task := range tasks {
		rowIndex := findPrimaryRowStyleOutlinerRow(t, m.rows, task.ID)
		line := m.renderOutlinerTaskRow(m.rows[rowIndex], false, 120, 120, rowIndex, nil)
		assertPrimaryRowTitleStyle(t, line, task)
		assertQueueIndicatorOnly(t, line)
	}
}

func newPrimaryRowStyleTask(t *testing.T, id string, shortID int, title string, priority int) *models.Task {
	t.Helper()

	task, err := models.NewGenericTask(id, shortID, title)
	if err != nil {
		t.Fatalf("NewTask(%q): %v", id, err)
	}
	task.Priority = priority
	// Queue membership is the StateNext workflow state, so the "q" indicator
	// renders for StateNext tasks (the persisted order index is no longer the
	// source of badge membership).
	task.WorkflowState = models.StateNext
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

func assertQueueIndicatorOnly(t *testing.T, line string) {
	t.Helper()

	plain := ansi.Strip(line)
	if !strings.Contains(plain, "q ") {
		t.Fatalf("expected queue indicator in:\n%s", plain)
	}
	if strings.Contains(plain, "*") {
		t.Fatalf("queue indicator should not be accompanied by an active marker in:\n%s", plain)
	}
}
