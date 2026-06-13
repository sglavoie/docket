package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestRenderPinnedPopupKeepsProjectNameOnSameLine(t *testing.T) {
	app := newTestAppModel()
	app.width = 48
	app.height = 20
	app.showPinnedPopup = true

	projectA := &models.Project{ID: "pa", Name: "Alpha Project"}
	projectB := &models.Project{ID: "pb", Name: "Beta Project"}

	taskA, err := models.NewTask("ta", 1, models.TaskTypeTask, "Pinned alpha")
	if err != nil {
		t.Fatalf("NewTask alpha: %v", err)
	}
	taskB, err := models.NewTask("tb", 2, models.TaskTypeTask, "Pinned beta")
	if err != nil {
		t.Fatalf("NewTask beta: %v", err)
	}

	app.pinnedPopupItems = []pinnedTaskInfo{
		{Title: taskA.Title, ProjectName: projectA.Name, Task: taskA, Project: projectA},
		{Title: taskB.Title, ProjectName: projectB.Name, Task: taskB, Project: projectB},
	}
	app.pinnedPopupFiltered = []int{0, 1}

	rendered := ansi.Strip(app.renderPinnedPopup())
	assertPopupLineContains(t, rendered, "Pinned alpha", "Alpha Project")
	assertPopupLineContains(t, rendered, "Pinned beta", "Beta Project")
	assertPopupDoesNotContainStandaloneLine(t, rendered, "Alpha Project")
	assertPopupDoesNotContainStandaloneLine(t, rendered, "Beta Project")
}

func TestRenderPinnedPopupNonSelectedTitleUsesPriorityStyle(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	app := newTestAppModel()
	app.width = 56
	app.height = 20
	app.showPinnedPopup = true
	app.pinnedPopupCursor = 1

	project := &models.Project{ID: "ps", Name: "Styled Project"}
	taskA, err := models.NewTask("sta", 1, models.TaskTypeTask, "Pinned priority")
	if err != nil {
		t.Fatalf("NewTask priority: %v", err)
	}
	taskA.Priority = 1
	taskB, err := models.NewTask("stb", 2, models.TaskTypeTask, "Selected row")
	if err != nil {
		t.Fatalf("NewTask selected: %v", err)
	}
	taskB.Priority = 2

	app.pinnedPopupItems = []pinnedTaskInfo{
		{Title: taskA.Title, ProjectName: project.Name, Task: taskA, Project: project},
		{Title: taskB.Title, ProjectName: project.Name, Task: taskB, Project: project},
	}
	app.pinnedPopupFiltered = []int{0, 1}

	rendered := app.renderPinnedPopup()
	want := GetTitlePriorityStyle(1).Render("Pinned priority")
	if !strings.Contains(rendered, want) {
		t.Fatalf("expected priority-styled pinned popup title in:\n%s", ansi.Strip(rendered))
	}
}

func TestRenderQuickSwitchPopupKeepsProjectNameOnSameLine(t *testing.T) {
	app := newTestAppModel()
	app.width = 44
	app.height = 20
	app.showQuickSwitch = true

	projectA := &models.Project{ID: "qa", Name: "Gamma Project"}
	projectB := &models.Project{ID: "qb", Name: "Delta Project"}

	taskA, err := models.NewTask("qta", 1, models.TaskTypeTask, "Alpha")
	if err != nil {
		t.Fatalf("NewTask recent alpha: %v", err)
	}
	taskB, err := models.NewTask("qtb", 2, models.TaskTypeTask, "Beta")
	if err != nil {
		t.Fatalf("NewTask recent beta: %v", err)
	}

	app.recentTaskVisits = []RecentTaskVisitEntry{
		{Task: taskA, Project: projectA},
		{Task: taskB, Project: projectB},
	}
	app.quickSwitchFiltered = []int{0, 1}

	rendered := ansi.Strip(app.renderQuickSwitchPopup())
	assertPopupLineContains(t, rendered, "Alpha", "Gamma Project")
	assertPopupLineContains(t, rendered, "Beta", "Delta Project")
	assertPopupDoesNotContainStandaloneLine(t, rendered, "Gamma Project")
	assertPopupDoesNotContainStandaloneLine(t, rendered, "Delta Project")
}

func TestRenderPinnedPopupHandlesWideUnicodeWidths(t *testing.T) {
	app := newTestAppModel()
	app.width = 36
	app.height = 20
	app.showPinnedPopup = true

	project := &models.Project{ID: "pu", Name: "工程看板"}
	task, err := models.NewTask("tu", 1, models.TaskTypeTask, "修复界面")
	if err != nil {
		t.Fatalf("NewTask unicode pinned: %v", err)
	}

	app.pinnedPopupItems = []pinnedTaskInfo{{Title: task.Title, ProjectName: project.Name, Task: task, Project: project}}
	app.pinnedPopupFiltered = []int{0}

	rendered := ansi.Strip(app.renderPinnedPopup())
	assertPopupLineContains(t, rendered, "修复", "工程看板")
	assertPopupDoesNotContainStandaloneLine(t, rendered, "工程看板")
}

func TestRenderQuickSwitchPopupHandlesWideUnicodeWidths(t *testing.T) {
	app := newTestAppModel()
	app.width = 36
	app.height = 20
	app.showQuickSwitch = true

	project := &models.Project{ID: "qu", Name: "工程看板"}
	task, err := models.NewTask("qtu", 1, models.TaskTypeTask, "修复界面")
	if err != nil {
		t.Fatalf("NewTask unicode quick switch: %v", err)
	}

	app.recentTaskVisits = []RecentTaskVisitEntry{{Task: task, Project: project}}
	app.quickSwitchFiltered = []int{0}

	rendered := ansi.Strip(app.renderQuickSwitchPopup())
	assertPopupLineContains(t, rendered, "修复", "工程看板")
	assertPopupDoesNotContainStandaloneLine(t, rendered, "工程看板")
}

func assertPopupLineContains(t *testing.T, rendered, left, right string) {
	t.Helper()
	for line := range strings.SplitSeq(rendered, "\n") {
		if strings.Contains(line, left) {
			if !strings.Contains(line, right) {
				t.Fatalf("popup line containing %q is missing %q\n\n%s", left, right, rendered)
			}
			return
		}
	}
	t.Fatalf("popup is missing a line containing %q\n\n%s", left, rendered)
}

func assertPopupDoesNotContainStandaloneLine(t *testing.T, rendered, text string) {
	t.Helper()
	for line := range strings.SplitSeq(rendered, "\n") {
		trimmed := strings.TrimSpace(strings.Trim(line, "│"))
		if trimmed == text {
			t.Fatalf("popup still renders %q on a standalone line\n\n%s", text, rendered)
		}
	}
}
