package tui

import tea "charm.land/bubbletea/v2"

// handleSearchMessages handles Palette search and selection messages.
// Returns true if the message was handled.
func (m *AppModel) handleSearchMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case PaletteEntitySelectMsg:
		m.popView()
		if msg.Task != nil && msg.Project != nil {
			m.recordTaskVisit(msg.Task, msg.Project)
			return true, m.navigateToTaskInProject(msg.Task, msg.Project)
		}
		return true, nil

	case PalettePeekRequestMsg:
		// Open the in-app Peek overlay above the palette. The palette view stays
		// on the stack, so dismissing Peek (Esc) restores its exact state.
		if msg.Task != nil {
			m.activatePeek(msg.Task, msg.Project)
		}
		return true, nil

	case ShowCommandPaletteMsg:
		// Pass the current view (before pushing the palette) for context-aware command filtering.
		// Scope tasks to currentProject only when the invoking view is project-scoped; Dashboard
		// and cross-project views should list tasks from every project (nil = all projects).
		currentView := m.currentView()
		paletteProject := m.currentProject
		if presentationScope(currentView) == ScopeApp {
			paletteProject = nil
		}
		if review, ok := m.activeView().(*ReviewHubModel); ok && review.ProjectScope() != nil {
			paletteProject = review.ProjectScope()
		}
		palette := NewCommandPaletteModel(m.runtime, m.commandRegistry, currentView, paletteProject)
		if currentView == ViewFocusBoard {
			if planner, ok := m.activeView().(*FocusBoardModel); ok {
				palette.SetCommandContext(planner.CommandContext())
			}
			palette.suggestedCommandOrder = m.focusBoardSuggestedCommandOrder()
			palette.filterItems()
		}
		if review, ok := m.activeView().(*ReviewHubModel); ok {
			palette.SetCommandContext(review.CommandContext())
		}
		// Every TaskWorkspace-backed active view (Outliner, All Tasks, Kanban, …)
		// installs its typed command context so the palette exposes the same
		// capability-scoped Task Workspace grammar — including uppercase-X
		// deletion — that direct keys and help advertise.
		if workspace, _, ok := m.taskWorkspacePrimary(); ok && m.activeTaskWorkspace() {
			palette.SetCommandContext(workspace.CommandContext())
		}
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
			case EditTaskMsg, SetDueDateMsg, SetSnoozeDateMsg, ShowSectionPickerMsg, ShowPriorityPickerMsg:
				keepPalette = true
			case PalettePeekRequestMsg:
				// Peek activates an in-app overlay above the palette; keep the
				// palette on the stack so Esc returns to its exact search state,
				// matching the palette task-row Peek (peekItem) path.
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
