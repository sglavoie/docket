package tui

import (
	"fmt"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func makeDashboardScrollProjects(count int) []*models.Project {
	projects := make([]*models.Project, 0, count)
	for i := 0; i < count; i++ {
		projects = append(projects, &models.Project{
			ID:      fmt.Sprintf("project-%02d", i),
			Slug:    fmt.Sprintf("project-%02d", i),
			Name:    fmt.Sprintf("Project %02d", i),
			Context: "work",
		})
	}
	return projects
}

func TestDashboardProjectListPersistentScrollKeepsWindowOnMoveUp(t *testing.T) {
	m := &DashboardModel{config: config.NewConfig()}
	m.SetSize(100, 8)
	projects := makeDashboardScrollProjects(8)
	m.projects = projects

	m.cursor = 5
	_ = m.renderProjectList(projects, 0, 80, 0)
	start := m.viewScrollOffset
	if start == 0 {
		t.Fatalf("expected dashboard project list to scroll down, offset stayed at 0")
	}

	m.cursor = 4
	_ = m.renderProjectList(projects, 0, 80, 0)
	if m.viewScrollOffset != start {
		t.Fatalf("moving up inside viewport changed first rendered row: offset=%d want %d", m.viewScrollOffset, start)
	}
}

func TestDashboardProjectListFilterShrinkClampsScrollOffset(t *testing.T) {
	m := &DashboardModel{config: config.NewConfig()}
	m.SetSize(100, 8)
	projects := makeDashboardScrollProjects(5)
	m.projects = projects

	m.cursor = len(projects) - 1
	m.viewScrollOffset = 99
	_ = m.renderProjectList(projects, 0, 80, 0)

	maxVisible := m.projectListMaxVisible(0, 0, false)
	want := max(len(projects)-maxVisible, 0)
	if m.viewScrollOffset != want {
		t.Fatalf("shrunken dashboard list offset = %d, want %d", m.viewScrollOffset, want)
	}
}

func makeGlobalSearchScrollResults(count int) []filteredSearchResult {
	project := &models.Project{Slug: "project"}
	results := make([]filteredSearchResult, 0, count)
	for i := 0; i < count; i++ {
		task := &models.Task{
			ID:      fmt.Sprintf("task-%02d", i),
			ShortID: i + 1,
			Title:   fmt.Sprintf("Task %02d", i),
			Type:    models.TaskTypeTask,
			Status:  models.StatusTodo,
		}
		results = append(results, filteredSearchResult{
			result: models.SearchResult{Task: task, Project: project},
		})
	}
	return results
}

func TestGlobalSearchPersistentScrollKeepsWindowOnMoveUp(t *testing.T) {
	m := &GlobalSearchModel{
		config:    config.NewConfig(),
		filtered:  makeGlobalSearchScrollResults(8),
		listWidth: 80,
	}

	m.cursor = 5
	_ = m.renderTaskList(3)
	start := m.scrollOffset
	if start == 0 {
		t.Fatalf("expected global search result list to scroll down, offset stayed at 0")
	}

	m.cursor = 4
	_ = m.renderTaskList(3)
	if m.scrollOffset != start {
		t.Fatalf("moving up inside viewport changed first rendered row: offset=%d want %d", m.scrollOffset, start)
	}
}

func TestGlobalSearchResultShrinkClampsScrollOffset(t *testing.T) {
	m := &GlobalSearchModel{
		config:       config.NewConfig(),
		filtered:     makeGlobalSearchScrollResults(5),
		listWidth:    80,
		cursor:       4,
		scrollOffset: 99,
	}

	_ = m.renderTaskList(3)

	const want = 2
	if m.scrollOffset != want {
		t.Fatalf("shrunken global search result offset = %d, want %d", m.scrollOffset, want)
	}
}
