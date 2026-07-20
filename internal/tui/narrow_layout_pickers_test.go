package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestParentSelectorViewRespectsVisualWidth(t *testing.T) {
	parentA, err := models.NewTask("parent-a", 11, models.TaskTypeStory, "修复父任务选择器在窄宽度下的宽字符截断问题")
	if err != nil {
		t.Fatalf("NewTask A: %v", err)
	}
	parentB, err := models.NewTask("parent-b", 12, models.TaskTypeTask, "ASCII fallback parent title")
	if err != nil {
		t.Fatalf("NewTask B: %v", err)
	}

	m := NewParentSelectorModel([]*models.Task{parentA, parentB}, nil, false, ViewOutliner)
	m.SetSize(32, 14)
	rendered := m.View().Content

	for line := range strings.SplitSeq(rendered, "\n") {
		if lipgloss.Width(ansi.Strip(line)) > 32 {
			t.Fatalf("parent selector line exceeds width 32: %q (width=%d)\n\n%s", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)), ansi.Strip(rendered))
		}
	}
}

func TestProjectPickerViewRespectsVisualWidth(t *testing.T) {
	projectA, err := models.NewProject("proj-a", "ui-fix-sprint", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject A: %v", err)
	}
	projectA.Name = "界面修复冲刺计划项目"
	projectA.Context = "work"

	projectB, err := models.NewProject("proj-b", "ops-project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject B: %v", err)
	}
	projectB.Context = "ops"

	m := NewProjectPickerModel([]*models.Project{projectA, projectB})
	m.SetSize(32, 14)
	rendered := m.View().Content

	for line := range strings.SplitSeq(rendered, "\n") {
		if lipgloss.Width(ansi.Strip(line)) > 32 {
			t.Fatalf("project picker line exceeds width 32: %q (width=%d)\n\n%s", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)), ansi.Strip(rendered))
		}
	}
}

func TestGlobalProjectPickerRespectsTerminalWidth(t *testing.T) {
	project, err := models.NewProject("proj-a", "ui-fix-sprint", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Name = "A project name long enough to pressure the narrow picker"
	project.Context = "context-with-a-long-label"

	for _, width := range []int{32, 18} {
		picker := newGlobalProjectPicker([]*models.Project{project}, []*models.Project{project}, PickerPurposeQuickCapture, nil, nil)
		rendered := picker.render(width)
		for line := range strings.SplitSeq(rendered, "\n") {
			if got := lipgloss.Width(ansi.Strip(line)); got > width {
				t.Fatalf("global project picker line exceeds width %d: %q (width=%d)\n\n%s", width, ansi.Strip(line), got, ansi.Strip(rendered))
			}
		}
	}
}

func TestPriorityPickerViewRespectsVisualWidth(t *testing.T) {
	task, err := models.NewTask("task-1", 5, models.TaskTypeTask, "修复优先级选择器标题在窄宽度下的溢出问题")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Priority = 3

	m := NewPriorityPickerModel(task, nil)
	m.SetSize(32, 14)
	rendered := m.View().Content

	for line := range strings.SplitSeq(rendered, "\n") {
		if lipgloss.Width(ansi.Strip(line)) > 32 {
			t.Fatalf("priority picker line exceeds width 32: %q (width=%d)\n\n%s", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)), ansi.Strip(rendered))
		}
	}
}

func TestGlobalNoteMoveViewRespectsVisualWidth(t *testing.T) {
	projectA, err := models.NewProject("proj-a", "ui-fix-sprint", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject A: %v", err)
	}
	projectA.Name = "界面修复冲刺计划项目"
	projectA.Context = "work"

	projectB, err := models.NewProject("proj-b", "ops-proj", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject B: %v", err)
	}
	projectB.Context = "ops"

	m := NewGlobalNoteMoveProjectSelectorModel([]*models.Project{projectA, projectB})
	m.SetSize(32, 14)
	rendered := m.View().Content

	for line := range strings.SplitSeq(rendered, "\n") {
		if lipgloss.Width(ansi.Strip(line)) > 32 {
			t.Fatalf("global note move line exceeds width 32: %q (width=%d)\n\n%s", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)), ansi.Strip(rendered))
		}
	}
}

func TestLinkedTaskCreatorViewRespectsVisualWidth(t *testing.T) {
	project, err := models.NewProject("proj-1", "creator-proj", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	sourceTask, err := models.NewTask("task-1", 55, models.TaskTypeTask, "修复关联任务创建器在窄宽度下的宽字符标题截断问题")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}

	m := NewLinkedTaskCreatorModel(sourceTask, project, LinkTypeBlocker)
	m.SetSize(32, 14)
	rendered := m.View().Content

	for line := range strings.SplitSeq(rendered, "\n") {
		if lipgloss.Width(ansi.Strip(line)) > 32 {
			t.Fatalf("linked task creator line exceeds width 32: %q (width=%d)\n\n%s", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)), ansi.Strip(rendered))
		}
	}
}

func TestCommandPaletteSetSizeAllowsNarrowWidths(t *testing.T) {
	m := &CommandPaletteModel{searchInput: textinput.New()}
	m.SetSize(18, 10)

	innerWidth := max(18-10, 1)
	used := m.listWidth + m.previewWidth
	if m.previewWidth > 0 {
		used++
	}
	if used > innerWidth {
		t.Fatalf("palette widths exceed inner width: list=%d preview=%d used=%d inner=%d", m.listWidth, m.previewWidth, used, innerWidth)
	}
	if !m.compactMode {
		t.Fatalf("expected compact mode on narrow palette width")
	}
}
