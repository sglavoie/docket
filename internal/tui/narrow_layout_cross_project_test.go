package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestProjectSelectorViewRespectsVisualWidth(t *testing.T) {
	sourceProject, err := models.NewProject("source", "Source Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject source: %v", err)
	}

	task, err := models.NewTask("task-1", 42, models.TaskTypeTask, "修复跨项目移动选择器的宽字符截断问题")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}

	projectA, err := models.NewProject("proj-a", "ui-fix-sprint", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject A: %v", err)
	}
	projectA.Name = "界面修复冲刺计划项目"
	projectA.Context = "work"

	projectB, err := models.NewProject("proj-b", "Release coordination", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject B: %v", err)
	}
	projectB.Context = "ops"

	m := NewProjectSelectorModel([]*models.Project{sourceProject, projectA, projectB}, task, sourceProject)
	m.SetSize(32, 14)
	rendered := m.View().Content

	for line := range strings.SplitSeq(rendered, "\n") {
		if lipgloss.Width(line) > 32 {
			t.Fatalf("project selector line exceeds width 32: %q (width=%d)\n\n%s", ansi.Strip(line), lipgloss.Width(line), ansi.Strip(rendered))
		}
	}

	if !strings.Contains(ansi.Strip(rendered), "界面修复") {
		t.Fatalf("expected rendered selector to include the unicode project name, got:\n%s", ansi.Strip(rendered))
	}
	if !strings.Contains(ansi.Strip(rendered), "Moving: #42") {
		t.Fatalf("expected task header in selector view, got:\n%s", ansi.Strip(rendered))
	}
}

func TestLinkedTaskSelectorRelationLineRespectsVisualWidth(t *testing.T) {
	sourceProject, err := models.NewProject("source", "source-proj", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject source: %v", err)
	}
	targetProject, err := models.NewProject("target", "target-proj", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject target: %v", err)
	}
	targetProject.Name = "界面修复冲刺计划项目"

	task, err := models.NewTask("task-1", 12, models.TaskTypeTask, "修复跨项目链接列表在窄宽度下的标题截断问题")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Status = models.StatusInProgress

	m := &LinkedTaskSelectorModel{sourceTask: task, project: sourceProject}
	m.SetSize(32, 12)

	line := "> " + m.renderRelationLine(LinkedTaskRelation{Task: task, Project: targetProject}, false)
	if lipgloss.Width(ansi.Strip(line)) > 32 {
		t.Fatalf("linked task selector line exceeds width 32: %q (width=%d)", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)))
	}
	if !strings.Contains(ansi.Strip(line), "界面修复") {
		t.Fatalf("expected cross-project label to remain visible, got %q", ansi.Strip(line))
	}
}

func TestCrossProjectLinkerTaskSelectionRespectsVisualWidth(t *testing.T) {
	sourceProject, err := models.NewProject("source", "source-proj", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject source: %v", err)
	}
	sourceTask, err := models.NewTask("source-task", 7, models.TaskTypeTask, "修复跨项目链接器在窄宽度下的布局")
	if err != nil {
		t.Fatalf("NewTask source: %v", err)
	}
	targetProject, err := models.NewProject("target", "target-proj", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject target: %v", err)
	}
	targetTask, err := models.NewTask("target-task", 21, models.TaskTypeTask, "修复目标任务列表在窄视图中的宽字符截断问题")
	if err != nil {
		t.Fatalf("NewTask target: %v", err)
	}
	targetTask.Status = models.StatusInProgress

	m := &CrossProjectLinkerModel{
		sourceTask:    sourceTask,
		sourceProject: sourceProject,
		step:          StepSelectTask,
		targetProject: targetProject,
		filteredTasks: []*models.Task{targetTask},
	}
	m.SetSize(32, 14)

	rendered := m.viewTaskSelection()
	for line := range strings.SplitSeq(strings.TrimRight(rendered, "\n"), "\n") {
		if lipgloss.Width(ansi.Strip(line)) > 32 {
			t.Fatalf("cross-project linker line exceeds width 32: %q (width=%d)\n\n%s", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)), ansi.Strip(rendered))
		}
	}
}

func TestFormatSidebarLinkedTaskRespectsVisualWidth(t *testing.T) {
	project, err := models.NewProject("proj-1", "sidebar-proj", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Name = "界面修复冲刺计划项目"
	task, err := models.NewTask("task-1", 31, models.TaskTypeTask, "修复侧边栏关联任务在窄宽度下的宽字符截断问题")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}

	line := formatSidebarLinkedTask(sidebarLinkedTask{Task: task, Project: project}, 24, true, true)
	if lipgloss.Width(ansi.Strip(line)) > 24 {
		t.Fatalf("sidebar linked task line exceeds width 24: %q (width=%d)", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)))
	}
	if !strings.Contains(ansi.Strip(line), "[界面") {
		t.Fatalf("expected project label to remain visible, got %q", ansi.Strip(line))
	}
}
