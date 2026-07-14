package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestGlobalSearchTaskRowHighlightUsesPriorityTitleStyle(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	task, err := models.NewTask("search-style", 42, models.TaskTypeTask, "Alpha highlight")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Priority = 1
	project := &models.Project{Slug: "proj", Name: "Project"}
	m := &GlobalSearchModel{
		runtime:   newTestConfig(),
		listWidth: 100,
		cursor:    1,
	}

	line := m.renderTaskSearchRow(0, filteredSearchResult{
		result: models.SearchResult{Task: task, Project: project},
		// Highlight "A"; the remaining title characters should keep the
		// priority title style rather than falling back to NormalStyle.
		titlePositions: []int{0},
		matchSource:    "title",
	}, nil)

	if !strings.Contains(line, HighlightStyle.Render("A")) {
		t.Fatalf("expected highlighted title match in:\n%s", ansi.Strip(line))
	}
	if !strings.Contains(line, GetTitlePriorityStyle(task.Priority).Render("l")) {
		t.Fatalf("expected non-highlighted title text to keep priority style in:\n%s", ansi.Strip(line))
	}
}

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
