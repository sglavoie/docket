package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// SidebarProject returns the project for sidebar display.
func (m *ProjectTreeModel) SidebarProject() *models.Project {
	return m.project
}

// SidebarRuntime returns the domain runtime for sidebar resolution.
func (m *ProjectTreeModel) SidebarRuntime() *config.RuntimeState {
	return m.runtime
}

// SidebarGetSidebar returns the sidebar component.
func (m *ProjectTreeModel) SidebarGetSidebar() *SidebarComponent {
	return m.sidebar
}

// SidebarHeight returns the visible height available for sidebar content.
func (m *ProjectTreeModel) SidebarHeight() int {
	return m.height - CardVerticalBorderOverhead
}

// SidebarSelectedTask returns the currently selected task.
func (m *ProjectTreeModel) SidebarSelectedTask() *models.Task {
	visible := m.visibleNodes()
	if len(visible) > 0 && m.cursor >= 0 && m.cursor < len(visible) {
		return visible[m.cursor].Task
	}
	return nil
}

// NavigateToSidebarTask navigates to a task from the sidebar.
func (m *ProjectTreeModel) NavigateToSidebarTask(task *models.Task) {
	_ = m.NavigateToTask(task)
}

// UnlinkSelectedSidebarTask removes the currently selected link from the sidebar,
// routing the removal through the application-layer link coordinator (one
// ledger-undoable ChangeSet) instead of a raw save.
func (m *ProjectTreeModel) UnlinkSelectedSidebarTask(sourceTask *models.Task) tea.Cmd {
	section := m.sidebar.GetDependencySection()
	cursor := m.sidebar.GetDependencyCursor()
	return sidebarUnlinkCommand(m.link, m.project, sourceTask, section, cursor)
}
