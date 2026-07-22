package tui

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"

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
	cfg := newTestConfig()
	m := &DashboardModel{
		runtime:  testRuntimeAt(cfg.DataDir(), cfg),
		projects: makeDashboardScrollProjects(projectCount),
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
	halfPage := HalfPage(m.homePageSize())

	m = updateDashboardKey(t, m, syntheticCtrlKey('d'))
	if m.cursor != halfPage {
		t.Fatalf("ctrl+d cursor = %d, want %d", m.cursor, halfPage)
	}

	m = updateDashboardKey(t, m, syntheticCtrlKey('u'))
	if m.cursor != 0 {
		t.Fatalf("ctrl+u cursor = %d, want 0", m.cursor)
	}
}

func TestDashboardCtrlNavigationDoesNotBypassSearchMode(t *testing.T) {
	m := newDashboardShortcutTestModel(20)
	m.cursor = 5
	_ = m.ActivateSearch()

	m = updateDashboardKey(t, m, syntheticCtrlKey('d'))
	if m.cursor != 5 {
		t.Fatalf("ctrl+d in search mode cursor = %d, want 5", m.cursor)
	}
}

func TestDashboardProjectListPersistentScrollKeepsWindowOnMoveUp(t *testing.T) {
	m := &DashboardModel{runtime: newTestConfig()}
	m.SetSize(100, 8)
	projects := makeDashboardScrollProjects(8)
	m.projects = projects

	m.cursor = 5
	_ = m.renderProjectList(projects, 5, 80)
	start := m.viewScrollOffset
	if start == 0 {
		t.Fatalf("expected dashboard project list to scroll down, offset stayed at 0")
	}

	m.cursor = 4
	_ = m.renderProjectList(projects, 5, 80)
	if m.viewScrollOffset != start {
		t.Fatalf("moving up inside viewport changed first rendered row: offset=%d want %d", m.viewScrollOffset, start)
	}
}

func TestDashboardProjectListFilterShrinkClampsScrollOffset(t *testing.T) {
	m := &DashboardModel{runtime: newTestConfig()}
	m.SetSize(100, 8)
	projects := makeDashboardScrollProjects(5)
	m.projects = projects

	m.cursor = len(projects) - 1
	m.viewScrollOffset = 99
	_ = m.renderProjectList(projects, 5, 80)

	maxVisible := m.projectListMaxVisible(5, false)
	want := max(len(projects)-maxVisible, 0)
	if m.viewScrollOffset != want {
		t.Fatalf("shrunken dashboard list offset = %d, want %d", m.viewScrollOffset, want)
	}
}
