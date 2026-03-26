package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// SidebarProject returns the project for sidebar display.
func (m *ProjectTreeModel) SidebarProject() *models.Project {
	return m.project
}

// SidebarConfig returns the config for sidebar display.
func (m *ProjectTreeModel) SidebarConfig() *config.Config {
	return m.config
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

// UnlinkSelectedSidebarTask removes the currently selected link from the sidebar.
func (m *ProjectTreeModel) UnlinkSelectedSidebarTask(sourceTask *models.Task) tea.Cmd {
	section := m.sidebar.GetDependencySection()
	cursor := m.sidebar.GetDependencyCursor()

	return func() tea.Msg {
		switch section {
		case 0: // Blocked by - remove from sourceTask.DependsOn
			if cursor < 0 || cursor >= len(sourceTask.DependsOn) {
				return nil
			}
			ref := sourceTask.DependsOn[cursor]
			sourceTask.RemoveDependency(ref)

			if err := m.projectService.Backup(m.project); err != nil {
				return ErrorMsg{Err: err}
			}
			if err := m.projectService.Save(m.project); err != nil {
				return ErrorMsg{Err: err}
			}

		case 1: // Blocking - remove from the other task's DependsOn
			blockedTasks := m.project.GetBlockedByTasks(sourceTask)
			if cursor < 0 || cursor >= len(blockedTasks) {
				return nil
			}
			blockedTask := blockedTasks[cursor]
			blockedTask.RemoveDependency(sourceTask.ID)

			if err := m.projectService.Backup(m.project); err != nil {
				return ErrorMsg{Err: err}
			}
			if err := m.projectService.Save(m.project); err != nil {
				return ErrorMsg{Err: err}
			}

		case 2: // Related - bidirectional, remove from both
			if cursor < 0 || cursor >= len(sourceTask.RelatedTo) {
				return nil
			}
			ref := sourceTask.RelatedTo[cursor]
			taskRef := models.ParseTaskRef(ref)

			sourceTask.RemoveRelatedTo(ref)

			if taskRef.IsCrossProject() {
				if m.config != nil {
					targetProject := m.config.GetProject(taskRef.ProjectID)
					if targetProject != nil {
						if len(targetProject.Tasks) == 0 {
							if err := storage.LoadProject(targetProject); err != nil {
								return ErrorMsg{Err: err}
							}
						}
						if targetTask := targetProject.GetTaskByID(taskRef.TaskID); targetTask != nil {
							reverseRef := m.project.ID + ":" + sourceTask.ID
							targetTask.RemoveRelatedTo(reverseRef)

							if err := storage.BackupProject(targetProject); err != nil {
								return ErrorMsg{Err: err}
							}
							if err := storage.SaveProjectLocked(targetProject); err != nil {
								return ErrorMsg{Err: err}
							}
						}
					}
				}
			} else {
				if targetTask := m.project.GetTaskByID(taskRef.TaskID); targetTask != nil {
					targetTask.RemoveRelatedTo(sourceTask.ID)
				}
			}

			if err := m.projectService.Backup(m.project); err != nil {
				return ErrorMsg{Err: err}
			}
			if err := m.projectService.Save(m.project); err != nil {
				return ErrorMsg{Err: err}
			}
		}

		if err := m.projectService.Load(m.project); err != nil {
			return ErrorMsg{Err: err}
		}
		return LinkRemovedMsg{SourceTask: sourceTask, Section: section}
	}
}
