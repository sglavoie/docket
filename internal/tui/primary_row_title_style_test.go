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
		assertPlanningMarkerColumnRemoved(t, line, m.rows[rowIndex], task)
	}
}

func TestOutlinerChromeUsesCompactLeftMargins(t *testing.T) {
	m := NewOutlinerModel(nil, nil, nil)

	section := ansi.Strip(m.renderOutlinerSectionHeaderRow(outlinerRow{
		IsSectionHeader: true,
		SectionName:     "Ready",
		SectionCount:    1,
	}, false, 120, 0))
	if strings.HasPrefix(section, " ") {
		t.Fatalf("section header retained a left gutter: %q", section)
	}

	project := ansi.Strip(m.renderOutlinerProjectHeaderRow(outlinerRow{
		IsProjectHeader: true,
		ProjectID:       "roadmap",
	}, false, 120, 0))
	if !strings.HasPrefix(project, " "+IconExpanded+" [roadmap]") {
		t.Fatalf("project header should retain a one-cell margin and fold indicator: %q", project)
	}
}

func newPrimaryRowStyleTask(t *testing.T, id string, shortID int, title string, priority int) *models.Task {
	t.Helper()

	task, err := models.NewGenericTask(id, shortID, title)
	if err != nil {
		t.Fatalf("NewTask(%q): %v", id, err)
	}
	task.Priority = priority
	// StateNext retains its workflow semantics without needing a separate
	// single-letter planning-marker column in the task row.
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

func assertPlanningMarkerColumnRemoved(t *testing.T, line string, row outlinerRow, task *models.Task) {
	t.Helper()

	plain := ansi.Strip(line)
	wantPrefix := RenderCursorPrefix() + row.prefix + task.State().Icon()
	if !strings.HasPrefix(plain, wantPrefix) {
		t.Fatalf("planned task row should place its state badge directly after the tree prefix; got:\n%s", plain)
	}
}
