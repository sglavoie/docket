package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
)

// Init implements tea.Model.
func (m *CommandPaletteModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *CommandPaletteModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case CommandPaletteClearSelectionMsg:
		m.selectedTasks = make(map[string]bool)
		m.actionMode = false
		return m, nil

	case tea.KeyPressMsg:
		// Handle action mode separately
		if m.actionMode {
			return m.updateActionMode(msg)
		}
		return m.updateSearchMode(msg)
	}
	return m, nil
}

// updateSearchMode handles key events in search mode (the default mode).
func (m *CommandPaletteModel) updateSearchMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) { //nolint:gocyclo
	switch msg.String() {
	case "esc":
		return m, func() tea.Msg {
			return CloseCommandPaletteMsg{}
		}
	case "enter":
		// Handle inline task creation
		if m.inlineTaskMode && m.inlineTaskParsed != nil && m.inlineTaskParsed.title != "" {
			return m.createInlineTask()
		}
		// Handle regular item selection
		if len(m.filteredItems) > 0 && m.cursor < len(m.filteredItems) {
			return m.executeItem(m.filteredItems[m.cursor].Item)
		}
		return m, nil
	case "tab":
		// If multiple tasks are selected, show bulk actions
		if len(m.selectedTasks) > 1 {
			actions := m.getBulkActionsForSelectedTasks()
			if len(actions) > 0 {
				m.actionMode = true
				m.actionCursor = 0
				m.actionScrollOffset = 0
				m.availableActions = actions
				m.actionItem = PaletteItem{
					Name:        fmt.Sprintf("%d tasks selected", len(m.selectedTasks)),
					Description: "Bulk actions",
					Category:    ItemCategoryTasks,
				}
				return m, nil
			}
		}
		// Enter action mode for non-command items
		if len(m.filteredItems) > 0 && m.cursor < len(m.filteredItems) {
			item := m.filteredItems[m.cursor].Item
			if item.Category != ItemCategoryCommands {
				actions := m.getActionsForItem(item)
				if len(actions) > 0 {
					m.actionMode = true
					m.actionCursor = 0
					m.actionScrollOffset = 0
					m.availableActions = actions
					m.actionItem = item
					return m, nil
				}
			}
		}
		return m, nil
	case "space":
		// Toggle selection on task items (Space key)
		if len(m.filteredItems) > 0 && m.cursor < len(m.filteredItems) {
			item := m.filteredItems[m.cursor].Item
			if item.Category == ItemCategoryTasks && item.Task != nil {
				taskID := item.Task.ID
				if m.selectedTasks[taskID] {
					delete(m.selectedTasks, taskID)
				} else {
					m.selectedTasks[taskID] = true
				}
				// Move cursor down after toggling
				if m.cursor < len(m.filteredItems)-1 {
					m.cursor++
				}
				return m, nil
			}
		}
		// For non-task items, add space to search input (enables prefix filtering like ":prj test")
		m.showTips = false
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		m.filterItems()
		return m, cmd
	case "alt+up":
		// Navigate to previous search history entry
		if len(m.searchHistory) > 0 {
			if m.searchHistoryIndex < 0 {
				// Starting history navigation, go to most recent
				m.searchHistoryIndex = 0
			} else if m.searchHistoryIndex < len(m.searchHistory)-1 {
				// Go to older entry
				m.searchHistoryIndex++
			}
			if m.searchHistoryIndex < len(m.searchHistory) {
				m.searchInput.SetValue(m.searchHistory[m.searchHistoryIndex])
				m.filterItems()
			}
		}
		return m, nil
	case "alt+down":
		// Navigate to newer search history entry
		if m.searchHistoryIndex > 0 {
			m.searchHistoryIndex--
			m.searchInput.SetValue(m.searchHistory[m.searchHistoryIndex])
			m.filterItems()
		} else if m.searchHistoryIndex == 0 {
			// Exit history mode, clear input
			m.searchHistoryIndex = -1
			m.searchInput.SetValue("")
			m.filterItems()
		}
		return m, nil
	case "up", "ctrl+k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "down", "ctrl+j":
		if m.cursor < len(m.filteredItems)-1 {
			m.cursor++
		}
		return m, nil
	case "ctrl+u":
		jumpCursor(&m.cursor, len(m.filteredItems), -10)
		return m, nil
	case "ctrl+d":
		jumpCursor(&m.cursor, len(m.filteredItems), 10)
		return m, nil
	case "home":
		m.cursor = 0
		m.scrollOffset = 0
		return m, nil
	case "end":
		if len(m.filteredItems) > 0 {
			m.cursor = len(m.filteredItems) - 1
		}
		return m, nil
	case "ctrl+n":
		// Jump to next section and scroll to show it at top
		nextIdx := m.findNextSectionStart()
		if nextIdx >= 0 {
			m.cursor = nextIdx
			m.scrollAnchor = nextIdx
		}
		return m, nil
	case "ctrl+p":
		// Toggle compact view mode (hide/show preview pane)
		m.compactMode = !m.compactMode
		m.userSetCompact = true
		return m, nil
	case "ctrl+f":
		// Toggle pin on current command
		if len(m.filteredItems) > 0 && m.cursor < len(m.filteredItems) {
			item := m.filteredItems[m.cursor].Item
			if item.Category == ItemCategoryCommands && item.Command != nil {
				name := item.Command.Name
				isPinned := m.config.TogglePinnedCommand(name)
				m.pinnedCommands[name] = isPinned
				if !isPinned {
					delete(m.pinnedCommands, name)
				}
				if m.configManager != nil {
					if err := m.configManager.Save(m.config); err != nil {
						logging.Warn("failed to save config: %v", err)
					}
				}
				// Re-filter to update sort order
				m.filterItems()
			}
		}
		return m, nil
	case "ctrl+x", "ctrl+e", "ctrl+g", "ctrl+s":
		// Quick task actions (ctrl+x: complete, ctrl+e: edit, ctrl+g: glow, ctrl+s: start)
		action := strings.TrimPrefix(msg.String(), "ctrl+")
		if cmd := m.taskQuickAction(action); cmd != nil {
			return m, cmd
		}
		return m, nil
	case "ctrl+h":
		// Toggle inclusion of completed/canceled tasks
		m.includeCompleted = !m.includeCompleted
		m.allItems = m.loadAllItems()
		m.filterItems()
		return m, nil
	case "ctrl+r":
		// Reset category filter and clear persisted category
		m.searchInput.SetValue("")
		m.config.SetLastPaletteCategory("")
		if m.configManager != nil {
			if err := m.configManager.Save(m.config); err != nil {
				logging.Warn("failed to save config: %v", err)
			}
		}
		m.filterItems()
		return m, nil
	case "ctrl+b":
		// Save current search as a bookmark
		query := strings.TrimSpace(m.searchInput.Value())
		// Don't save empty queries, category prefixes alone, or inline task creation
		if query != "" && !strings.HasPrefix(query, "+") {
			_, searchQuery := m.parseQueryPrefix(query)
			if searchQuery != "" || !strings.HasPrefix(query, ":") {
				// Show save search dialog
				return m, func() tea.Msg {
					return ShowSaveSearchDialogMsg{Query: query}
				}
			}
		}
		return m, nil
	case "ctrl+o":
		// Cycle sort mode: Score -> Priority -> DueDate -> Score
		switch m.sortField {
		case SortByDefault:
			m.sortField = SortByPriority
		case SortByPriority:
			m.sortField = SortByDueDate
		case SortByDueDate:
			m.sortField = SortByDefault
		default:
			m.sortField = SortByDefault
		}
		m.filterItems()
		return m, nil
	case "e", "g", "x", "s", "r", "b":
		// Direct actions on tasks (when not actively typing a search)
		if m.isDirectActionEnabled() {
			if cmd := m.taskQuickAction(msg.String()); cmd != nil {
				return m, cmd
			}
		}
		// Fall through to default if action not applicable
		fallthrough
	default:
		key := msg.String()

		// Check if the key matches a command's shortcut (when not actively searching)
		if m.isDirectActionEnabled() && len(m.filteredItems) > 0 {
			// First check if current selection is a command with this shortcut
			if m.cursor < len(m.filteredItems) {
				item := m.filteredItems[m.cursor].Item
				if item.Category == ItemCategoryCommands && item.Command != nil {
					if item.Command.Shortcut == key {
						return m.executeItem(item)
					}
				}
			}
			// Also check visible commands (first 9) for shortcut match
			for i := 0; i < len(m.filteredItems) && i < 9; i++ {
				item := m.filteredItems[i].Item
				if item.Category == ItemCategoryCommands && item.Command != nil {
					if item.Command.Shortcut == key {
						return m.executeItem(item)
					}
				}
			}
		}

		// Handle number keys for direct jump when enabled
		if len(key) == 1 && key[0] >= '1' && key[0] <= '9' && m.isNumberJumpEnabled() {
			idx := int(key[0] - '1') // "1" -> 0, "9" -> 8
			if idx < len(m.filteredItems) {
				return m.executeItem(m.filteredItems[idx].Item)
			}
			return m, nil
		}
		// Update search input and filter items
		m.showTips = false // Hide tips on first input
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		m.filterItems()
		return m, cmd
	}
}

// taskQuickAction maps a single-letter action key to a task command for the
// currently selected palette item. Returns nil when the action does not apply
// (wrong item category, missing project, or status guard fails).
// Accepted keys: e (edit), g (glow), x (complete), s (start), b (block), r (reopen).
func (m *CommandPaletteModel) taskQuickAction(key string) tea.Cmd {
	if len(m.filteredItems) == 0 || m.cursor >= len(m.filteredItems) {
		return nil
	}
	item := m.filteredItems[m.cursor].Item
	if item.Category != ItemCategoryTasks || item.Task == nil {
		return nil
	}
	task := item.Task

	// Actions that only need a task (no project required)
	switch key {
	case "e":
		return func() tea.Msg {
			return CommandPaletteExecuteMsg{ResultMsg: EditTaskMsg{Task: task}}
		}
	case "g":
		return func() tea.Msg {
			return CommandPaletteExecuteMsg{ResultMsg: GlowViewMsg{Task: task}}
		}
	}

	// Remaining actions require a project and a status guard
	if item.Project == nil {
		return nil
	}
	project := item.Project

	switch key {
	case "x":
		if task.Status != models.StatusDone && task.Status != models.StatusCanceled {
			return func() tea.Msg {
				return CommandPaletteExecuteMsg{ResultMsg: CommandPaletteTaskActionMsg{
					Task: task, Project: project, Action: "complete",
				}}
			}
		}
	case "s":
		if task.Status == models.StatusTodo {
			return func() tea.Msg {
				return CommandPaletteExecuteMsg{ResultMsg: CommandPaletteTaskActionMsg{
					Task: task, Project: project, Action: "start",
				}}
			}
		}
	case "b":
		if task.Status.IsActive() && task.Status != models.StatusBlocked {
			return func() tea.Msg {
				return CommandPaletteExecuteMsg{ResultMsg: CommandPaletteTaskActionMsg{
					Task: task, Project: project, Action: "block",
				}}
			}
		}
	case "r":
		if task.Status == models.StatusDone || task.Status == models.StatusCanceled {
			return func() tea.Msg {
				return CommandPaletteExecuteMsg{ResultMsg: CommandPaletteTaskActionMsg{
					Task: task, Project: project, Action: "reopen",
				}}
			}
		}
	}
	return nil
}

// updateActionMode handles key events when showing inline actions for an item.
func (m *CommandPaletteModel) updateActionMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc", "tab":
		// Exit action mode, return to search
		m.actionMode = false
		m.availableActions = nil
		return m, nil
	case "enter":
		// Execute the selected action
		if m.actionCursor < len(m.availableActions) {
			action := m.availableActions[m.actionCursor]
			if action.Handler != nil {
				result := action.Handler()
				return m, func() tea.Msg {
					return CommandPaletteExecuteMsg{ResultMsg: result}
				}
			}
		}
		return m, nil
	case "up", "ctrl+k":
		if m.actionCursor > 0 {
			m.actionCursor--
		}
		return m, nil
	case "down", "ctrl+j":
		if m.actionCursor < len(m.availableActions)-1 {
			m.actionCursor++
		}
		return m, nil
	default:
		// Check if the key matches an action's KeyHint for quick execution
		for _, action := range m.availableActions {
			if action.KeyHint == key && action.Handler != nil {
				result := action.Handler()
				return m, func() tea.Msg {
					return CommandPaletteExecuteMsg{ResultMsg: result}
				}
			}
		}
		// Any other key exits action mode and processes as search input
		m.actionMode = false
		m.availableActions = nil
		// Process the key as search input
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		m.filterItems()
		return m, cmd
	}
}
