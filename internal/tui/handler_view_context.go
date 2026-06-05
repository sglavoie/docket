package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/logging"
)

// handleContextMessages handles context picker and custom context messages.
// Returns true if the message was handled.
func (m *AppModel) handleContextMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowContextPickerMsg:
		if msg.Dashboard != nil {
			m.showView(ViewContextPicker, NewContextPickerModel(msg.Dashboard, m.config))
		} else {
			m.showView(ViewContextPicker, NewContextPickerModelFromConfig(m.config))
		}
		return true, nil

	case ShowContextPickerForCycleMsg:
		m.showView(ViewContextPicker, NewContextPickerModelFromConfig(m.config))
		return true, nil

	case ContextPickerDoneMsg:
		m.popView()
		if !msg.Canceled {
			// Persist context filter to config - custom and regular contexts are mutually exclusive
			if msg.IsCustomContext {
				m.config.FilterCustomContext = msg.SelectedContext
				m.config.FilterContext = ""
			} else {
				m.config.FilterContext = msg.SelectedContext
				m.config.FilterCustomContext = ""
			}
			if err := m.configManager.Save(m.config); err != nil {
				logging.Warn("failed to save config: %v", err)
			}

			if msg.FromCycle {
				// Reopen the popup with refreshed items
				m.resetProjectCycleSearch()
				m.refreshProjectCycleItems()
				m.showProjectCycle = true
				m.projectCycleCursor = 0
				m.projectCycleScroll = 0
			} else if m.dashboard != nil {
				// Update dashboard filter
				if msg.IsCustomContext {
					m.dashboard.filterContext = ""
					m.dashboard.filterCustomContext = msg.SelectedContext
				} else {
					m.dashboard.filterContext = msg.SelectedContext
					m.dashboard.filterCustomContext = ""
				}
				visible := m.dashboard.visibleProjects()
				if m.dashboard.cursor >= len(visible) {
					if len(visible) > 0 {
						m.dashboard.cursor = len(visible) - 1
					} else {
						m.dashboard.cursor = 0
					}
				}
			}

			// When the cross-project outliner is in the view stack, trigger a
			// full eager-reload sequence identical to ;o activation: re-filter
			// projects by the new context, reload all tasks from disk, and
			// rebuild all rows. Cursor is saved and restored so clampCursor()
			// in crossProjectRebuildRows clamps it to the new row count.
			for _, v := range m.viewStack {
				if v == ViewCrossProjectOutliner && m.crossOutliner != nil {
					savedCursor := m.crossOutliner.cursor
					reloadCmd := m.crossOutliner.reloadCrossProjects()
					m.crossOutliner.cursor = savedCursor
					return true, reloadCmd
				}
			}
		}
		return true, nil

	case ShowCustomContextEditorMsg:
		m.showView(ViewCustomContextEditor, NewCustomContextEditorModel(m.config, msg.EditName))
		return true, nil

	case CustomContextDeletedMsg:
		// Save config after deletion
		if err := m.configManager.Save(m.config); err != nil {
			logging.Warn("failed to save config: %v", err)
		}
		// If the deleted context was active, clear the dashboard filter
		if msg.WasActive && m.dashboard != nil {
			m.dashboard.filterCustomContext = ""
			m.dashboard.Refresh(m.config)
		}
		m.setSuccessStatus("Deleted custom context: " + msg.Name)
		return true, defaultStatusTimeout()

	case CustomContextSavedMsg:
		// Save config and go back to context picker
		if err := m.configManager.Save(m.config); err != nil {
			logging.Warn("failed to save config: %v", err)
		}
		m.popView() // Pop editor view
		// Refresh context picker if it's the current view
		if m.currentView() == ViewContextPicker {
			if m.dashboard != nil {
				picker := NewContextPickerModel(m.dashboard, m.config)
				picker.SetSize(m.width, m.height-2)
				m.views.Register(ViewContextPicker, picker)
			}
		}
		// Show status message
		if msg.IsNew {
			m.setSuccessStatus(fmt.Sprintf("Created custom context '%s' with %d projects", msg.Name, msg.ProjectCount))
		} else {
			m.setSuccessStatus(fmt.Sprintf("Updated custom context '%s' with %d projects", msg.Name, msg.ProjectCount))
		}
		return true, defaultStatusTimeout()

	case ContextRenamedMsg:
		// Save config after renaming a context
		if err := m.configManager.Save(m.config); err != nil {
			logging.Warn("failed to save config: %v", err)
		}
		// Refresh dashboard if needed
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		m.setSuccessStatus(fmt.Sprintf("Renamed '%s' to '%s' (%d projects)", msg.OldName, msg.NewName, msg.ProjectCount))
		return true, defaultStatusTimeout()
	}
	return false, nil
}
