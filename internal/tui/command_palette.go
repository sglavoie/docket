package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
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
		if len(m.filteredItems) > 0 && m.cursor < len(m.filteredItems) {
			return m.executeItem(m.filteredItems[m.cursor].Item)
		}
		return m, nil
	case "shift+enter":
		if len(m.filteredItems) > 0 && m.cursor < len(m.filteredItems) {
			return m.peekItem(m.filteredItems[m.cursor].Item)
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
	case "ctrl+a":
		// Toggle transient show-all only when the query is empty; with a
		// nonempty query Ctrl+A keeps its ordinary text-input meaning.
		if strings.TrimSpace(m.searchInput.Value()) == "" {
			m.showAll = !m.showAll
			m.showTips = false
			m.filterItems()
			return m, nil
		}
		m.showTips = false
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		m.filterItems()
		return m, cmd
	default:
		// Update search input and filter items
		m.showTips = false // Hide tips on first input
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		m.filterItems()
		return m, cmd
	}
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
		return m, nil
	}
}
