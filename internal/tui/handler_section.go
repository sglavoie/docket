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
		if !msg.Cancelled && m.projectTree != nil {
			m.projectTree.filterSections = make(map[string]bool)
			for _, section := range msg.SelectedSections {
				m.projectTree.filterSections[section] = true
			}
			m.projectTree.updateVisibility()
			m.projectTree.clampCursor()
		}
		return true, nil

	case ShowSectionPickerMsg:
		if msg.Task != nil && msg.Project != nil {
			m.showView(ViewSectionPicker, NewSectionPickerModel(msg.Task, msg.Project))
		}
		return true, nil

	case ShowPriorityPickerMsg:
		if msg.Task != nil && msg.Project != nil {
			m.showView(ViewPriorityPicker, NewPriorityPickerModel(msg.Task, msg.Project))
		}
		return true, nil

	case PriorityPickerDoneMsg:
		m.popView()
		if msg.Cancelled {
			m.pendingBulkTasks = nil
			return true, nil
		}

		// Check if this is a bulk operation from palette
		if len(m.pendingBulkTasks) > 0 {
			count := 0
			projectTasks := make(map[string][]*models.Task)
			projectMap := make(map[string]*models.Project)

			for _, item := range m.pendingBulkTasks {
				if item.Task != nil && item.Project != nil {
					item.Task.Priority = msg.Priority
					item.Task.UpdatedAt = time.Now()
					projectTasks[item.Project.ID] = append(projectTasks[item.Project.ID], item.Task)
					projectMap[item.Project.ID] = item.Project
					count++
				}
			}

			// Save each affected project
			for projectID, project := range projectMap {
				if len(projectTasks[projectID]) > 0 {
					if err := storage.BackupProject(project); err != nil {
						m.pendingBulkTasks = nil
						return true, errorCmd("failed to create backup: %w", err)
					}
					if err := storage.SaveProject(project); err != nil {
						m.pendingBulkTasks = nil
						return true, errorCmd("failed to save: %w", err)
					}
					storage.LoadProject(project)
				}
			}

			m.refreshProjectTree()
			m.statusMessage = fmt.Sprintf("Set priority %d for %d tasks", msg.Priority, count)
			m.pendingBulkTasks = nil
			return true, statusTimeout(2 * time.Second)
		}

		// Single task operation
		if msg.Task != nil && msg.Project != nil {
			msg.Task.Priority = msg.Priority
			msg.Task.UpdatedAt = time.Now()

			if err := storage.BackupProject(msg.Project); err != nil {
				return true, errorCmd("failed to create backup: %w", err)
			}
			if err := storage.SaveProject(msg.Project); err != nil {
				return true, errorCmd("failed to save: %w", err)
			}

			storage.LoadProject(msg.Project)
			m.refreshProjectTree()
			m.statusMessage = fmt.Sprintf("Set priority %d for task #%d", msg.Priority, msg.Task.ShortID)
			return true, statusTimeout(2 * time.Second)
		}
		return true, nil

	case SectionPickerDoneMsg:
		m.popView()
		if !msg.Cancelled && msg.Task != nil && msg.Project != nil {
			msg.Task.Section = msg.Section
			msg.Task.UpdatedAt = time.Now()

			if msg.Section != "" {
				msg.Project.EnsureSection(msg.Section)
				if globalColor := m.config.GetGlobalSectionColor(msg.Section); globalColor != "gray" {
					msg.Project.SetSectionColor(msg.Section, globalColor)
				}
			}

			if err := storage.BackupProject(msg.Project); err != nil {
				return true, errorCmd("failed to create backup: %w", err)
			}
			if err := storage.SaveProject(msg.Project); err != nil {
				return true, errorCmd("failed to save: %w", err)
			}

			storage.LoadProject(msg.Project)
			m.config.ApplyGlobalColorsToProject(msg.Project)
			m.refreshProjectTree()
		}
		return true, nil

	case ShowSectionManagerMsg:
		if msg.ProjectTree != nil {
			m.showView(ViewSectionManager, NewSectionManagerModel(msg.ProjectTree, m.config, m.configManager, msg.StartAddMode))
		}
		return true, nil

	case CloseSectionManagerMsg:
		m.popView()
		return true, nil

	case SectionRenamedMsg, SectionAddedMsg, SectionRemovedMsg:
		m.popView()
		m.reloadAndRefreshProjectTree()
		return true, nil

	case CommandPaletteSectionSelectMsg:
		m.popView()
		m.switchToProject(msg.Project, false) // Don't restore state - apply specific section filter
		m.projectTree.filterSections = map[string]bool{msg.Section: true}
		m.projectTree.updateVisibility()
		m.showView(ViewProjectTree, m.projectTree)
		return true, nil
	}
	return false, nil
}
