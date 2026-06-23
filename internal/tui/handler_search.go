package tui

import tea "charm.land/bubbletea/v2"

// handleSearchMessages handles search-related messages (global search, command palette, saved searches).
// Returns true if the message was handled.
func (m *AppModel) handleSearchMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowGlobalSearchMsg:
		m.globalSearch = NewGlobalSearchModel(m.config, m.configManager)
		m.showView(ViewGlobalSearch, m.globalSearch)
		return true, m.globalSearch.Init()

	case GlobalSearchSelectMsg:
		m.popView()
		if msg.Task != nil && msg.Project != nil {
			m.recordTaskVisit(msg.Task, msg.Project)
			return true, m.navigateToTaskInProject(msg.Task, msg.Project)
		}
		return true, nil

	case ShowCommandPaletteMsg:
		// Pass the current view (before pushing the palette) for context-aware command filtering.
		// Scope tasks to currentProject only when the invoking view is project-scoped; Dashboard
		// and cross-project views should list tasks from every project (nil = all projects).
		currentView := m.currentView()
		paletteProject := m.currentProject
		if currentView == ViewDashboard || isGlobalNavView(currentView) {
			paletteProject = nil
		}
		palette := NewCommandPaletteModel(m.config, m.configManager, m.commandRegistry, currentView, paletteProject)
		// Apply pre-filter if specified (overrides last category)
		if msg.PreFilter != "" {
			palette.searchInput.SetValue(msg.PreFilter)
			palette.showTips = false
			palette.filterItems()
		}
		return m.showViewInit(ViewCommandPalette, palette)

	case CommandPaletteExecuteMsg:
		// Check if the inner message opens a modal view that should return to the palette
		keepPalette := false
		if msg.ResultMsg != nil {
			switch msg.ResultMsg.(type) {
			case EditTaskMsg, SetDueDateMsg, SetSnoozeDateMsg, ShowSectionPickerMsg, ShowPriorityPickerMsg, GlowViewMsg:
				keepPalette = true
			}
		}
		// Only close command palette for non-modal actions. popView() can return
		// a cmd (e.g. ViewDidAppearMsg when revealing the cross-project outliner);
		// preserve it so the revealed view still gets its eager-reload signal.
		var popCmd tea.Cmd
		if !keepPalette {
			popCmd = m.popView()
		}
		var resultCmd tea.Cmd
		if msg.ResultMsg != nil {
			if cmdMsg, ok := msg.ResultMsg.(PaletteCommandResultMsg); ok {
				// Unwrap a command-carrying result so the handler's tea.Cmd runs
				// after the palette closes instead of being routed as a Msg.
				resultCmd = cmdMsg.Cmd
			} else {
				_, resultCmd = m.processMessage(msg.ResultMsg)
			}
		}
		return true, batchPaletteExecuteCmds(popCmd, resultCmd)

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

// batchPaletteExecuteCmds combines the popView cmd and the palette result cmd,
// dropping nils so a single non-nil cmd is returned without wrapping it in a
// degenerate Batch.
func batchPaletteExecuteCmds(popCmd, resultCmd tea.Cmd) tea.Cmd {
	switch {
	case popCmd == nil && resultCmd == nil:
		return nil
	case popCmd == nil:
		return resultCmd
	case resultCmd == nil:
		return popCmd
	default:
		return tea.Batch(popCmd, resultCmd)
	}
}
