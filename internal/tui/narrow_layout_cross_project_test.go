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

	task, err := models.NewGenericTask("task-1", 42, "修复跨项目移动选择器的宽字符截断问题")
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

func TestCrossProjectLinkerTaskSelectionRespectsVisualWidth(t *testing.T) {
	sourceProject, err := models.NewProject("source", "source-proj", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject source: %v", err)
	}
	sourceTask, err := models.NewGenericTask("source-task", 7, "修复跨项目链接器在窄宽度下的布局")
	if err != nil {
		t.Fatalf("NewTask source: %v", err)
	}
	targetProject, err := models.NewProject("target", "target-proj", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject target: %v", err)
	}
	targetTask, err := models.NewGenericTask("target-task", 21, "修复目标任务列表在窄视图中的宽字符截断问题")
	if err != nil {
		t.Fatalf("NewTask target: %v", err)
	}
	targetTask.WorkflowState = models.StateInProgress

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
