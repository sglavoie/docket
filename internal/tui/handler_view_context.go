package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// handleContextMessages handles context picker and custom context messages.
// Returns true if the message was handled.
func (m *AppModel) handleContextMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowContextPickerMsg:
		m.showView(ViewContextPicker, NewContextPickerModelFromRuntime(m.runtime))
		return true, nil

	case ContextPickerDoneMsg:
		popCmd := m.popView()
		if !msg.Canceled {
			// Reload immediately only when the pop reveals the Outliner. A
			// still-covered Outliner reloads through its later ViewDidAppearMsg.
			return true, tea.Batch(popCmd, m.requestCrossProjectOutlinerReload())
		}
		return true, popCmd

	case ShowCustomContextEditorMsg:
		m.showView(ViewCustomContextEditor, NewCustomContextEditorModel(m.runtime, msg.EditName))
		return true, nil

	case CustomContextDeletedMsg:
		// The deletion was already committed to machine-local state by the picker.
		m.setSuccessStatus("Deleted custom context: " + msg.Name)
		return true, defaultStatusTimeout()

	case CustomContextSavedMsg:
		// The save was already committed to machine-local state by the editor.
		m.popView() // Pop editor view
		// Refresh context picker if it's the current view
		if m.currentView() == ViewContextPicker {
			picker := NewContextPickerModelFromRuntime(m.runtime)
			picker.SetSize(m.width, m.height-2)
			m.views.Register(ViewContextPicker, picker)
		}
		// Show status message
		if msg.IsNew {
			m.setSuccessStatus(fmt.Sprintf("Created custom context '%s' with %d projects", msg.Name, msg.ProjectCount))
		} else {
			m.setSuccessStatus(fmt.Sprintf("Updated custom context '%s' with %d projects", msg.Name, msg.ProjectCount))
		}
		return true, defaultStatusTimeout()

	case ContextRenamedMsg:
		// The picker already committed the catalog-only rename through RuntimeState.
		m.syncProjectCache()
		m.setSuccessStatus(fmt.Sprintf("Renamed '%s' to '%s' (%d projects)", msg.OldName, msg.NewName, msg.ProjectCount))
		return true, defaultStatusTimeout()
	}
	return false, nil
}
