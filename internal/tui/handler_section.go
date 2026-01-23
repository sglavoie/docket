package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
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
			m.projectTree.filter.Sections = make(map[string]bool)
			for _, section := range msg.SelectedSections {
				m.projectTree.filter.Sections[section] = true
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
			projectMap := make(map[string]*models.Project)

			for _, item := range m.pendingBulkTasks {
				if item.Task != nil && item.Project != nil {
					item.Task.Priority = msg.Priority
					item.Task.UpdatedAt = time.Now()
					projectMap[item.Project.ID] = item.Project
					count++
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
			msg.Task.Priority = msg.Priority
			msg.Task.UpdatedAt = time.Now()

			successMsg := fmt.Sprintf("Set priority %d for task #%d", msg.Priority, msg.Task.ShortID)
			return true, tea.Batch(
				startLoading("Saving..."),
				SaveProjectAsync(msg.Project, successMsg),
			)
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

			m.config.ApplyGlobalColorsToProject(msg.Project)
			successMsg := fmt.Sprintf("Moved to %s", msg.Section)
			if msg.Section == "" {
				successMsg = "Removed from section"
			}
			return true, tea.Batch(
				startLoading("Saving..."),
				SaveProjectAsync(msg.Project, successMsg),
			)
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
		m.projectTree.filter.Sections = map[string]bool{msg.Section: true}
		m.projectTree.updateVisibility()
		m.showView(ViewProjectTree, m.projectTree)
		return true, nil
	}
	return false, nil
}
