package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestDependencyViewerHeaderUsesPriorityTitleStyle(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	task, err := models.NewTask("dependency-header", 7, models.TaskTypeTask, "Dependency Header")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Priority = 1
	m := &DependencyViewerModel{
		task:   task,
		width:  80,
		height: 20,
	}

	rendered := m.View().Content
	want := GetTitlePriorityStyle(task.Priority).Render(task.Title)
	if !strings.Contains(rendered, want) {
		t.Fatalf("expected priority-styled dependency header title in:\n%s", ansi.Strip(rendered))
	}
}
