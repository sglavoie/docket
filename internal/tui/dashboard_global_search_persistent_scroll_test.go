package tui

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

func makeDashboardScrollProjects(count int) []*models.Project {
	projects := make([]*models.Project, 0, count)
	for i := range count {
		projects = append(projects, &models.Project{
			ID:      fmt.Sprintf("project-%02d", i),
			Slug:    fmt.Sprintf("project-%02d", i),
			Name:    fmt.Sprintf("Project %02d", i),
			Context: "work",
		})
	}
	return projects
}

func newDashboardShortcutTestModel(projectCount int) *DashboardModel {
	cfg := config.NewConfig()
	cfg.SetInfoPanelMode(config.InfoPanelOff)
	m := &DashboardModel{
		config:                cfg,
		projects:              makeDashboardScrollProjects(projectCount),
		dashboardCompactCards: true,
	}
	m.InitSearchInput("Search projects...", 100, 40)
	m.SetSize(100, 12)
	m.Dispatcher = keymap.NewDispatcher(m.buildKeyMap())
	return m
}

func updateDashboardKey(t *testing.T, m *DashboardModel, key tea.KeyPressMsg) *DashboardModel {
	t.Helper()
	model, _ := m.Update(key)
	dashboard, ok := model.(*DashboardModel)
	if !ok {
		t.Fatalf("Update returned %T, want *DashboardModel", model)
	}
	return dashboard
}

func TestDashboardCtrlDUHalfPageNavigation(t *testing.T) {
	m := newDashboardShortcutTestModel(20)
	halfPage := HalfPage(m.projectListPageSize())

	m = updateDashboardKey(t, m, syntheticCtrlKey('d'))
	if m.cursor != halfPage {
		t.Fatalf("ctrl+d cursor = %d, want %d", m.cursor, halfPage)
	}

	m = updateDashboardKey(t, m, syntheticCtrlKey('u'))
	if m.cursor != 0 {
		t.Fatalf("ctrl+u cursor = %d, want 0", m.cursor)
	}
}

func TestDashboardCtrlBFPageNavigationAndClamp(t *testing.T) {
	m := newDashboardShortcutTestModel(20)
	pageSize := m.projectListPageSize()

	m = updateDashboardKey(t, m, syntheticCtrlKey('f'))
	if m.cursor != pageSize {
		t.Fatalf("ctrl+f cursor = %d, want %d", m.cursor, pageSize)
	}

	m = updateDashboardKey(t, m, syntheticCtrlKey('b'))
	if m.cursor != 0 {
		t.Fatalf("ctrl+b cursor = %d, want 0", m.cursor)
	}

	m.cursor = len(m.projects) - 2
	m = updateDashboardKey(t, m, syntheticCtrlKey('f'))
	if want := len(m.projects) - 1; m.cursor != want {
		t.Fatalf("ctrl+f at end cursor = %d, want %d", m.cursor, want)
	}

	m.cursor = 1
	m = updateDashboardKey(t, m, syntheticCtrlKey('b'))
	if m.cursor != 0 {
		t.Fatalf("ctrl+b near start cursor = %d, want 0", m.cursor)
	}
}

func TestDashboardCtrlNavigationDoesNotBypassSearchMode(t *testing.T) {
	m := newDashboardShortcutTestModel(20)
	m.cursor = 5
	_ = m.ActivateSearch()

	m = updateDashboardKey(t, m, syntheticCtrlKey('f'))
	if m.cursor != 5 {
		t.Fatalf("ctrl+f in search mode cursor = %d, want 5", m.cursor)
	}
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
	for i := range count {
		task := &models.Task{
			ID:            fmt.Sprintf("task-%02d", i),
			ShortID:       i + 1,
			Title:         fmt.Sprintf("Task %02d", i),
			Type:          models.TaskTypeTask,
			WorkflowState: models.StateReady,
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
