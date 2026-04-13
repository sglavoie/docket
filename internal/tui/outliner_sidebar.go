package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
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

// SidebarConfig returns the config for sidebar display.
func (m *OutlinerModel) SidebarConfig() *config.Config {
	return m.config
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

// NavigateToSidebarTask navigates to a task from the sidebar.
// If the task is inside a collapsed parent, ancestors are expanded first.
func (m *OutlinerModel) NavigateToSidebarTask(task *models.Task) {
	for i, row := range m.rows {
		if row.task != nil && row.task.ID == task.ID {
			m.cursor = i
			return
		}
	}

	if task.Section != "" && m.project != nil {
		delete(m.collapsedSections, task.Section)
	}
	m.ensureExpandedNodes()
	for ancestor := task.Parent; ancestor != nil; ancestor = ancestor.Parent {
		m.expandedNodes[ancestor.ID] = true
	}
	m.rebuildRows()

	for i, row := range m.rows {
		if row.task != nil && row.task.ID == task.ID {
			m.cursor = i
			return
		}
	}
}

// UnlinkSelectedSidebarTask removes the currently selected link from the sidebar.
func (m *OutlinerModel) UnlinkSelectedSidebarTask(sourceTask *models.Task) tea.Cmd {
	section := m.sidebar.GetDependencySection()
	cursor := m.sidebar.GetDependencyCursor()
	persister := m.projectService

	m.setUndoContext("Unlink", sourceTask.Title)
	m.undo.TaskIDs = []string{sourceTask.ID}
	opts := m.buildWorkflowOpts(nil)
	project := m.project
	cfg := m.config
	projectID := m.project.ID

	var crossProjectRef string
	switch section {
	case 0:
		if cursor < 0 || cursor >= len(sourceTask.DependsOn) {
			return nil
		}
		sourceTask.RemoveDependency(sourceTask.DependsOn[cursor])

	case 1:
		blockedTasks := project.GetBlockedByTasks(sourceTask)
		if cursor < 0 || cursor >= len(blockedTasks) {
			return nil
		}
		blockedTasks[cursor].RemoveDependency(sourceTask.ID)

	case 2:
		if cursor < 0 || cursor >= len(sourceTask.RelatedTo) {
			return nil
		}
		ref := sourceTask.RelatedTo[cursor]
		taskRef := models.ParseTaskRef(ref)

		sourceTask.RemoveRelatedTo(ref)
		if taskRef.IsCrossProject() {
			crossProjectRef = ref
		} else if targetTask := project.GetTaskByID(taskRef.TaskID); targetTask != nil {
			targetTask.RemoveRelatedTo(sourceTask.ID)
		}
	}

	return func() tea.Msg {
		if crossProjectRef != "" && cfg != nil {
			taskRef := models.ParseTaskRef(crossProjectRef)
			targetProject := cfg.GetProject(taskRef.ProjectID)
			if targetProject != nil {
				if len(targetProject.Tasks) == 0 {
					if err := storage.LoadProject(targetProject); err != nil {
						return ErrorMsg{Err: err}
					}
				}
				if targetTask := targetProject.GetTaskByID(taskRef.TaskID); targetTask != nil {
					targetTask.RemoveRelatedTo(projectID + ":" + sourceTask.ID)
					if err := storage.BackupProject(targetProject); err != nil {
						return ErrorMsg{Err: err}
					}
					if err := storage.SaveProjectLocked(targetProject); err != nil {
						return ErrorMsg{Err: err}
					}
				}
			}
		}

		if err := persistWithWorkflow(opts); err != nil {
			return ErrorMsg{Err: err}
		}
		if err := persister.Load(project); err != nil {
			return ErrorMsg{Err: err}
		}
		return LinkRemovedMsg{SourceTask: sourceTask, Section: section}
	}
}

func (m *OutlinerModel) StatusBarLabel() string {
	if m.project != nil {
		status := fmt.Sprintf("Outliner: %s", m.project.Name) + FormatCyclePosition(m.config, m.project.ID)
		if m.sidebar.IsFocused() {
			status += " " + SidebarFocusBadgeStyle.Render("[SIDEBAR]")
		}
		return status
	}
	return m.crossStatusBarLabel()
}
