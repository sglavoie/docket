package tui

import tea "github.com/charmbracelet/bubbletea"

// handleSearchMessages handles search-related messages (global search, command palette, saved searches).
// Returns true if the message was handled.
func (m *AppModel) handleSearchMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowGlobalSearchMsg:
		m.globalSearch = NewGlobalSearchModel(m.config)
		m.showView(ViewGlobalSearch, m.globalSearch)
		return true, m.globalSearch.Init()

	case CloseGlobalSearchMsg:
		m.popView()
		return true, nil

	case GlobalSearchSelectMsg:
		m.popView()
		if msg.Task != nil && msg.Project != nil {
			m.switchToProject(msg.Project, true)
			m.projectTree.NavigateToTask(msg.Task)
			m.showView(ViewProjectTree, m.projectTree)
		}
		return true, nil

	case ShowCommandPaletteMsg:
		// Pass the current view (before pushing the palette) for context-aware command filtering
		// Pass the current project to filter tasks (nil = show all projects' tasks)
		currentView := m.currentView()
		palette := NewCommandPaletteModel(m.config, m.configManager, m.commandRegistry, currentView, m.currentProject)
		m.showView(ViewCommandPalette, palette)
		return true, palette.Init()

	case CloseCommandPaletteMsg:
		m.popView()
		return true, nil

	case CommandPaletteExecuteMsg:
		// Check if the inner message opens a modal view that should return to the palette
		keepPalette := false
		if msg.ResultMsg != nil {
			switch msg.ResultMsg.(type) {
			case EditTaskMsg, SetDueDateMsg, ShowSectionPickerMsg, ShowPriorityPickerMsg, GlowViewMsg:
				keepPalette = true
			}
		}
		// Only close command palette for non-modal actions
		if !keepPalette {
			m.popView()
		}
		// Process the inner message through handlers
		if msg.ResultMsg != nil {
			return m.processMessage(msg.ResultMsg)
		}
		return true, nil

	case ShowSaveSearchDialogMsg:
		// Show a simple text input dialog for the search name
		m.showView(ViewSaveSearchInput, NewSaveSearchInputModel(msg.Query))
		return true, nil

	case SaveSearchMsg:
		// Save the search to config
		m.config.SaveSearch(msg.Name, msg.Query)
		if err := m.configManager.Save(m.config); err != nil {
			return true, errorCmd("failed to save search: %w", err)
		}
		// Pop the input dialog
		m.popView()
		// Refresh the command palette if it's still showing
		if m.currentView() == ViewCommandPalette {
			if palette, ok := m.views.Get(ViewCommandPalette).(*CommandPaletteModel); ok {
				palette.allItems = palette.loadAllItems()
				palette.filterItems()
			}
		}
		return true, nil

	case DeleteSavedSearchMsg:
		// Delete the saved search from config
		m.config.DeleteSavedSearch(msg.Name)
		if err := m.configManager.Save(m.config); err != nil {
			return true, errorCmd("failed to delete saved search: %w", err)
		}
		// Refresh the command palette if it's showing
		if m.currentView() == ViewCommandPalette {
			if palette, ok := m.views.Get(ViewCommandPalette).(*CommandPaletteModel); ok {
				palette.allItems = palette.loadAllItems()
				palette.filterItems()
			}
		}
		return true, nil

	case ExecuteSavedSearchMsg:
		// Execute a saved search by setting the query in the palette
		if m.currentView() == ViewCommandPalette {
			if palette, ok := m.views.Get(ViewCommandPalette).(*CommandPaletteModel); ok {
				palette.searchInput.SetValue(msg.Query)
				palette.filterItems()
			}
		}
		return true, nil
	}
	return false, nil
}
