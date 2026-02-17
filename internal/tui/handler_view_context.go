package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
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
			_ = m.configManager.Save(m.config)

			if msg.FromCycle {
				// Refresh the Project Cycle Manager with new context
				if cycleManager, ok := m.views.Get(ViewProjectCycleManager).(*ProjectCycleManagerModel); ok {
					cycleManager.RefreshWithContext()
				}
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
		}
		return true, nil

	case ShowCustomContextEditorMsg:
		m.showView(ViewCustomContextEditor, NewCustomContextEditorModel(m.config, msg.EditName))
		return true, nil

	case CustomContextDeletedMsg:
		// Save config after deletion
		_ = m.configManager.Save(m.config)
		// If the deleted context was active, clear the dashboard filter
		if msg.WasActive && m.dashboard != nil {
			m.dashboard.filterCustomContext = ""
			m.dashboard.Refresh(m.config)
		}
		m.statusMessage = "Deleted custom context: " + msg.Name
		return true, defaultStatusTimeout()

	case CustomContextSavedMsg:
		// Save config and go back to context picker
		_ = m.configManager.Save(m.config)
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
			m.statusMessage = fmt.Sprintf("Created custom context '%s' with %d projects", msg.Name, msg.ProjectCount)
		} else {
			m.statusMessage = fmt.Sprintf("Updated custom context '%s' with %d projects", msg.Name, msg.ProjectCount)
		}
		return true, defaultStatusTimeout()

	case ContextRenamedMsg:
		// Save config after renaming a context
		_ = m.configManager.Save(m.config)
		// Refresh dashboard if needed
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		m.statusMessage = fmt.Sprintf("Renamed '%s' to '%s' (%d projects)", msg.OldName, msg.NewName, msg.ProjectCount)
		return true, defaultStatusTimeout()
	}
	return false, nil
}
