package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// handleSectionMessages handles section-related messages.
// Returns true if the message was handled.
func (m *AppModel) handleSectionMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowSectionFilterSelectorMsg:
		if msg.ProjectTree != nil {
			m.showView(ViewSectionFilterSelector, NewSectionFilterSelectorModel(msg.ProjectTree))
		}
		return true, nil

	case SectionFilterSelectorDoneMsg:
		m.popView()
		if !msg.Canceled && m.projectTree != nil {
			m.projectTree.filter.Sections = make(map[string]bool)
			for _, section := range msg.SelectedSections {
				m.projectTree.filter.Sections[section] = true
			}
			m.projectTree.updateVisibility()
			m.projectTree.clampCursor()
		}
		return true, nil

	case ShowBulkSectionPickerMsg:
		if len(msg.Tasks) > 0 && msg.Project != nil {
			m.pendingBulkSectionTasks = msg.Tasks
			m.pendingBulkSectionProject = msg.Project
			m.showView(ViewSectionPicker, NewSectionPickerModel(msg.Tasks[0], msg.Project))
		}
		return true, nil

	case ShowSectionPickerMsg:
		if msg.Task != nil && msg.Project != nil {
			m.showView(ViewSectionPicker, NewSectionPickerModel(msg.Task, msg.Project))
		}
		return true, nil

	case ShowPriorityPickerMsg:
		if msg.BoardItem != nil {
			m.showView(ViewPriorityPicker, NewBoardItemPriorityPickerModel(msg.BoardItem))
		} else if msg.Task != nil && msg.Project != nil {
			m.showView(ViewPriorityPicker, NewPriorityPickerModel(msg.Task, msg.Project))
		}
		return true, nil

	case PriorityPickerDoneMsg:
		m.popView()
		if msg.Canceled {
			m.pendingBulkTasks = nil
			return true, nil
		}

		// Board item priority
		if msg.BoardItem != nil {
			msg.BoardItem.Priority = msg.Priority
			msg.BoardItem.UpdatedAt = time.Now()
			if err := storage.SaveBoardItem(m.config.DataDir, msg.BoardItem); err != nil {
				return true, errorCmd("Failed to save: %v", err)
			}
			return true, func() tea.Msg {
				return FocusBoardRefreshMsg{}
			}
		}

		// Check if this is a bulk operation from palette
		if len(m.pendingBulkTasks) > 0 {
			count := 0
			projectMap := make(map[string]*models.Project)

			for _, item := range m.pendingBulkTasks {
				if item.Task != nil && item.Project != nil {
					if item.Project.MutateTask(item.Task.ID, func(t *models.Task) {
						t.Priority = msg.Priority
					}) {
						projectMap[item.Project.ID] = item.Project
						count++
					}
				}
			}

			var projects []*models.Project
			for _, project := range projectMap {
				projects = append(projects, project)
			}

			m.pendingBulkTasks = nil
			successMsg := fmt.Sprintf("Set priority %d for %d tasks", msg.Priority, count)
			return true, tea.Batch(
				startLoading("Saving..."),
				SaveMultipleProjectsAsync(projects, successMsg),
			)
		}

		// Single task operation
		if msg.Task != nil && msg.Project != nil {
			if !msg.Project.MutateTask(msg.Task.ID, func(t *models.Task) {
				t.Priority = msg.Priority
			}) {
				return true, errorCmd("task no longer exists in project")
			}

			successMsg := fmt.Sprintf("Set priority %d for task #%d", msg.Priority, msg.Task.ShortID)
			return true, tea.Batch(
				startLoading("Saving..."),
				SaveProjectAsync(msg.Project, successMsg),
			)
		}
		return true, nil

	case SectionPickerDoneMsg:
		m.popView()
		if msg.Canceled {
			m.pendingBulkSectionTasks = nil
			m.pendingBulkSectionProject = nil
			return true, nil
		}

		// Check if this is a bulk operation
		if len(m.pendingBulkSectionTasks) > 0 && m.pendingBulkSectionProject != nil {
			project := m.pendingBulkSectionProject
			count := 0
			for _, task := range m.pendingBulkSectionTasks {
				if task != nil {
					if project.MutateTask(task.ID, func(t *models.Task) {
						assignSection(t, msg.Section)
					}) {
						count++
					}
				}
			}

			if msg.Section != "" && msg.Section != removeCustomSectionSentinel {
				project.EnsureSection(msg.Section)
				if globalColor := m.config.GetGlobalSectionColor(msg.Section); globalColor != "gray" {
					project.SetSectionColor(msg.Section, globalColor)
				}
			}
			m.config.ApplyGlobalColorsToProject(project)

			if m.kanban != nil {
				m.kanban.loadTasks()
			}

			m.pendingBulkSectionTasks = nil
			m.pendingBulkSectionProject = nil

			var successMsg string
			switch msg.Section {
			case "":
				successMsg = fmt.Sprintf("Removed %d tasks from GTD section", count)
			case removeCustomSectionSentinel:
				successMsg = fmt.Sprintf("Removed custom section from %d tasks", count)
			default:
				successMsg = fmt.Sprintf("Moved %d tasks to %s", count, msg.Section)
			}
			return true, tea.Batch(
				startLoading("Saving..."),
				SaveProjectAsync(project, successMsg),
			)
		}

		// Single task operation
		if msg.Task != nil && msg.Project != nil {
			if !msg.Project.MutateTask(msg.Task.ID, func(t *models.Task) {
				assignSection(t, msg.Section)
			}) {
				return true, errorCmd("task no longer exists in project")
			}

			if msg.Section != "" && msg.Section != removeCustomSectionSentinel {
				msg.Project.EnsureSection(msg.Section)
				if globalColor := m.config.GetGlobalSectionColor(msg.Section); globalColor != "gray" {
					msg.Project.SetSectionColor(msg.Section, globalColor)
				}
			}

			m.config.ApplyGlobalColorsToProject(msg.Project)

			// Refresh kanban view if open
			if m.kanban != nil {
				m.kanban.loadTasks()
			}

			var successMsg string
			switch msg.Section {
			case "":
				successMsg = "Removed from GTD section"
			case removeCustomSectionSentinel:
				successMsg = "Removed custom section"
			default:
				successMsg = fmt.Sprintf("Moved to %s", msg.Section)
			}
			return true, tea.Batch(
				startLoading("Saving..."),
				SaveProjectAsync(msg.Project, successMsg),
			)
		}
		return true, nil

	case ShowSectionManagerMsg:
		if msg.ProjectTree != nil {
			m.showView(ViewSectionManager, NewSectionManagerModel(msg.ProjectTree, nil, m.config, m.configManager, msg.StartAddMode))
		} else if msg.Project != nil {
			m.showView(ViewSectionManager, NewSectionManagerModel(nil, msg.Project, m.config, m.configManager, msg.StartAddMode))
		}
		return true, nil

	case CloseSectionManagerMsg:
		m.popView()
		return true, nil

	case GoToSectionMsg:
		m.popView()
		if m.projectTree != nil {
			tabs := m.projectTree.customSectionTabs()
			for i, tab := range tabs {
				if tab == msg.SectionName {
					m.projectTree.customSectionTab = i
					m.projectTree.updateVisibility()
					m.projectTree.clampCursor()
					break
				}
			}
		}
		return true, nil

	case SectionRenamedMsg, SectionAddedMsg, SectionRemovedMsg:
		m.popView()
		m.reloadAndRefreshProjectTree()
		return true, nil

	case CommandPaletteSectionSelectMsg:
		m.popView()
		m.switchToProject(msg.Project, false) // Don't restore state - apply specific section filter
		m.projectTree.filter.Sections = map[string]bool{msg.Section: true}
		m.projectTree.updateVisibility()
		m.showView(ViewProjectTree, m.projectTree)
		return true, nil
	}
	return false, nil
}

// assignSection sets the section on a task. GTD sections and custom sections
// are independent — assigning one does not clear the other.
func assignSection(task *models.Task, section string) {
	if models.IsGTDSection(section) || section == "" {
		task.Section = section
	} else if section == removeCustomSectionSentinel {
		task.CustomSection = ""
	} else {
		task.CustomSection = section
	}
}
