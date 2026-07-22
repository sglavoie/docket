package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// resolveProjectForRow returns the project that owns the given row.
// In single-project mode it returns m.project; in cross-project mode it
// resolves via crossResolveProject using the row's ProjectID.
func (m *OutlinerModel) resolveProjectForRow(row outlinerRow) *models.Project {
	if m.project != nil {
		return m.project
	}
	return m.crossResolveProject(row.ProjectID)
}

// SidebarHost interface implementation for OutlinerModel.

// SidebarProject returns the project for sidebar display.
func (m *OutlinerModel) SidebarProject() *models.Project {
	if m.project != nil {
		return m.project
	}
	if m.cursor >= 0 && m.cursor < len(m.rows) {
		row := m.rows[m.cursor]
		if row.ProjectID != "" {
			return m.crossResolveProject(row.ProjectID)
		}
	}
	return nil
}

// SidebarRuntime returns the domain runtime for sidebar resolution.
func (m *OutlinerModel) SidebarRuntime() *config.RuntimeState {
	return m.runtime
}

// SidebarGetSidebar returns the sidebar component.
func (m *OutlinerModel) SidebarGetSidebar() *SidebarComponent {
	return m.sidebar
}

// SidebarHeight returns the visible height available for sidebar content.
func (m *OutlinerModel) SidebarHeight() int {
	return m.height - CardVerticalBorderOverhead
}

// SidebarSelectedTask returns the currently selected task.
func (m *OutlinerModel) SidebarSelectedTask() *models.Task {
	if len(m.rows) > 0 && m.cursor >= 0 && m.cursor < len(m.rows) {
		return m.rows[m.cursor].task
	}
	return nil
}

// NavigateToSidebarTask navigates to a task from the sidebar. If the task is
// inside a collapsed parent, ancestors are expanded first.
func (m *OutlinerModel) NavigateToSidebarTask(task *models.Task) {
	m.navigateToSidebarTask(task)
}

func (m *OutlinerModel) navigateToSidebarTask(task *models.Task) bool {
	if task == nil {
		return false
	}
	for i, row := range m.rows {
		if row.task != nil && row.task.ID == task.ID {
			m.cursor = i
			return true
		}
	}

	if bucket := taskSectionBucket(task); bucket != "" && m.project != nil {
		delete(m.collapsedSections, bucket)
	}
	m.ensureExpandedNodes()
	for ancestor := task.Parent; ancestor != nil; ancestor = ancestor.Parent {
		m.expandedNodes[ancestor.ID] = true
	}
	m.rebuildRows()

	for i, row := range m.rows {
		if row.task != nil && row.task.ID == task.ID {
			m.cursor = i
			return true
		}
	}
	return false
}

// revealSidebarTask honors explicit task navigation even when persisted view
// state would hide the target. The initial attempt preserves the current view;
// only a genuinely hidden target resets visibility constraints.
func (m *OutlinerModel) revealSidebarTask(task *models.Task) {
	if m.navigateToSidebarTask(task) {
		return
	}

	m.zoomStack = nil
	m.filter = NewFilterState()
	m.ClearSearch()
	m.activeSavedFilter = ""
	m.activeCriteriaFilter = nil
	m.setPinnedTagTabIndex(0)
	m.smartFocus = nil
	switch {
	case task.IsTerminalState():
		m.activePerspective = PerspectiveCompleted
	case task.IsSnoozed():
		m.activePerspective = PerspectiveSnoozed
	default:
		m.activePerspective = PerspectiveAll
	}

	m.rebuildRows()
	for i, row := range m.rows {
		if row.task != nil && row.task.ID == task.ID {
			m.cursor = i
			return
		}
	}
	m.setStatus("Task is unavailable in this project")
}

// NavigateToTaskInProject navigates to a task in the cross-project outliner.
// Project identity is part of the match so duplicate task IDs in different
// projects cannot move the cursor to the wrong row.
func (m *OutlinerModel) NavigateToTaskInProject(task *models.Task, project *models.Project) {
	if task == nil {
		return
	}
	if m.project != nil {
		m.NavigateToSidebarTask(task)
		return
	}

	projectID := ""
	if project != nil {
		projectID = project.ID
	}
	if projectID == "" {
		for _, p := range m.loadedProjects {
			if p != nil && p.GetTaskByID(task.ID) == task {
				projectID = p.ID
				break
			}
		}
	}
	if projectID == "" {
		m.setStatus("Task project not found")
		return
	}

	if m.moveCursorToProjectTask(task.ID, projectID) {
		return
	}

	section := effectiveDisplaySection(task)
	delete(m.collapsedSections, "_xp:"+section)
	m.crossZoomStack = nil
	m.ensureExpandedNodes()
	for ancestor := task.Parent; ancestor != nil; ancestor = ancestor.Parent {
		m.expandedNodes[ancestor.ID] = true
	}
	m.rebuildRows()

	if !m.moveCursorToProjectTask(task.ID, projectID) {
		m.setStatus("Task is hidden by current filters")
	}
}

func (m *OutlinerModel) moveCursorToProjectTask(taskID, projectID string) bool {
	for i, row := range m.rows {
		if row.task != nil && row.ProjectID == projectID && row.task.ID == taskID {
			m.cursor = i
			return true
		}
	}
	return false
}

// UnlinkSelectedSidebarTask removes the currently selected link from the sidebar,
// routing the removal through the application-layer link coordinator (one
// ledger-undoable ChangeSet) instead of a raw save.
func (m *OutlinerModel) UnlinkSelectedSidebarTask(sourceTask *models.Task) tea.Cmd {
	section := m.sidebar.GetDependencySection()
	cursor := m.sidebar.GetDependencyCursor()
	return sidebarUnlinkCommand(m.link, m.project, sourceTask, section, cursor)
}

func (m *OutlinerModel) StatusBarLabel() string {
	if m.project != nil {
		status := fmt.Sprintf("Outliner: %s", m.project.Name)
		if m.sidebar.IsFocused() {
			status += " " + SidebarFocusBadgeStyle.Render("[SIDEBAR]")
		}
		return status
	}
	return m.crossStatusBarLabel()
}
