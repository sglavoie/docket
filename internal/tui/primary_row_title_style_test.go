package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// TestOutlinerQueuedPriorityTitlesStayNeutral is the inverse of the rule this
// file used to hold: an assigned priority reaches the row as its badge, and the
// title beside it is ordinary text at every priority.
func TestOutlinerQueuedPriorityTitlesStayNeutral(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: ""})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	tasks := []*models.Task{
		newPrimaryRowStyleTask(t, "outliner-p1", 1, "Outliner queued P1", 1),
		newPrimaryRowStyleTask(t, "outliner-p2", 2, "Outliner queued P2", 2),
	}

	m := outliner.NewOutlinerModel(newPrimaryRowStyleProject(t, "outliner-project", tasks), nil, nil)
	m.SetSize(120, 20)
	m.RebuildRows()

	for _, task := range tasks {
		rowIndex := findPrimaryRowStyleOutlinerRow(t, m.Rows, task.ID)
		line := m.RenderOutlinerTaskRow(m.Rows[rowIndex], false, 120, rowIndex, nil)
		assertPrimaryRowTitleStyle(t, line, task)
		assertPlanningMarkerColumnRemoved(t, line, m.Rows[rowIndex], task)
	}
}

func TestOutlinerChromeUsesCompactLeftMargins(t *testing.T) {
	m := outliner.NewOutlinerModel(nil, nil, nil)

	section := ansi.Strip(m.RenderOutlinerSectionHeaderRow(outliner.OutlinerRow{
		IsSectionHeader: true,
		SectionName:     "Ready",
		SectionCount:    1,
	}, false, 120, 0))
	if strings.HasPrefix(section, " ") {
		t.Fatalf("section header retained a left gutter: %q", section)
	}

	project := ansi.Strip(m.RenderOutlinerProjectHeaderRow(outliner.OutlinerRow{
		IsProjectHeader: true,
		ProjectID:       "roadmap",
	}, false, 120, 0))
	if !strings.HasPrefix(project, " "+theme.IconExpanded+" [roadmap]") {
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
	// StateBacklog retains its workflow semantics without needing a separate
	// single-letter planning-marker column in the task row.
	task.WorkflowState = models.StateBacklog
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

func findPrimaryRowStyleOutlinerRow(t *testing.T, rows []outliner.OutlinerRow, taskID string) int {
	t.Helper()

	for i, row := range rows {
		if row.Task != nil && row.Task.ID == taskID {
			return i
		}
	}
	t.Fatalf("outliner row for task %q not found", taskID)
	return -1
}

func assertPrimaryRowTitleStyle(t *testing.T, line string, task *models.Task) {
	t.Helper()

	if !strings.Contains(line, theme.NormalStyle.Render(task.Title)) {
		t.Fatalf("expected neutral title for %q in:\n%s", task.Title, ansi.Strip(line))
	}

	for name, styled := range map[string]string{
		"priority":     theme.GetTitlePriorityStyle(task.Priority).Render(task.Title),
		"active state": theme.ActiveStyle.Render(task.Title),
	} {
		if styled != task.Title && strings.Contains(line, styled) {
			t.Fatalf("%s recolored title %q in:\n%s", name, task.Title, ansi.Strip(line))
		}
	}

	// The priority the title no longer carries still reaches the row.
	if badge := chrome.RenderPriorityBadge(task.Priority); badge != "" && !strings.Contains(line, badge) {
		t.Fatalf("priority %d lost its badge in:\n%s", task.Priority, ansi.Strip(line))
	}
}

func assertPlanningMarkerColumnRemoved(t *testing.T, line string, row outliner.OutlinerRow, task *models.Task) {
	t.Helper()

	plain := ansi.Strip(line)
	wantPrefix := theme.RenderCursorPrefix() + row.Prefix + task.State().Icon()
	if !strings.HasPrefix(plain, wantPrefix) {
		t.Fatalf("planned task row should place its state badge directly after the tree prefix; got:\n%s", plain)
	}
}
