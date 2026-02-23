package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// handleKeyMsg handles keyboard input in normal mode.
func (m *FocusBoardModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle pending prefix keys (which-key style)
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// Handle confirmation dialog before other key processing
	if m.confirm.IsActive() {
		return m.handleConfirmKey(key)
	}

	// Navigation
	if handleListNavigation(key, &m.cursor, len(m.filtered), m.Height()) {
		return m, nil
	}

	// Activate prefix keys: F always, ,!g gated on item, mc gated on item
	if prefix, ok := activateSecondaryPrefix(key, ",!gF", "mc", m.getRowAtCursor() != nil); ok {
		m.pendingPrefix = prefix
		return m, nil
	}

	switch key {
	case "esc", "q":
		if m.HasActiveFilters() {
			m.ClearAllFilters()
			m.applyFilter()
			m.clampCursor()
			m.statusMessage = "Filters cleared"
			return m, nil
		}
		return m, func() tea.Msg { return CloseFocusBoardMsg{} }

	// Search
	case "/":
		m.searchInput.SetValue(m.searchQuery)
		return m, m.ActivateSearch()

	// Tab navigation
	case "tab", "]", "l":
		CycleTab(&m.activeTab, len(focusBoardTabNames), 1)
		m.applyFilter()
		m.clampCursor()
		return m, nil
	case "shift+tab", "[", "h":
		CycleTab(&m.activeTab, len(focusBoardTabNames), -1)
		m.applyFilter()
		m.clampCursor()
		return m, nil

	// Quick-add action
	case "a":
		m.addMode = true
		m.addInput.SetValue("")
		m.addInput.Focus()
		return m, textinput.Blink

	// Quick-edit title
	case "E":
		if row := m.getRowAtCursor(); row != nil && row.isAction() {
			m.editMode = true
			m.editIdx = m.cursor
			m.editInput.SetValue(row.Task.Title)
			m.editInput.Focus()
			return m, textinput.Blink
		}

	// Mark done
	case "x":
		return m.markDone()

	// Dismiss (cancel)
	case "X":
		return m.dismiss()

	// Toggle done/open
	case "d":
		return m.toggleDone()

	// Set priority
	case "p":
		if row := m.getRowAtCursor(); row != nil {
			return m, func() tea.Msg {
				return ShowPriorityPickerMsg{Task: row.Task, Project: row.Project}
			}
		}

	// Open URL / navigate to task
	case "enter":
		return m.handleEnter()

	// Open URL directly
	case "o":
		if row := m.getRowAtCursor(); row != nil && row.Task.URL != "" {
			return m, openBrowserCmd(row.Task.URL)
		}

	// Edit task
	case "e":
		if row := m.getRowAtCursor(); row != nil {
			return m, func() tea.Msg {
				return EditTaskMsg{Task: row.Task, Project: row.Project}
			}
		}

	// Delete permanently
	case "D":
		if row := m.getRowAtCursor(); row != nil {
			if row.isAction() {
				m.confirm.Show("delete", fmt.Sprintf("Delete action #%d? (y/n)", row.Task.ShortID))
				m.statusMessage = m.confirm.Prompt()
				return m, nil
			}
			m.statusMessage = "Use project tree to delete non-action tasks"
		}
		return m, nil

	// Sort keys (1-5 for sort field, 0 to reset)
	case "1", "2", "3", "4", "5", "0":
		handleSortKey(key, &m.sort, func() {
			m.applyFilter()
			m.clampCursor()
			m.statusMessage = "Sort reset"
		}, func(field SortField) {
			m.applyFilter()
			m.clampCursor()
			direction := "↑"
			if !m.sort.Ascending {
				direction = "↓"
			}
			m.statusMessage = fmt.Sprintf("Sort: %s %s", field, direction)
		})
		return m, nil

	// Assign to project (move task to another project)
	case "P":
		if row := m.getRowAtCursor(); row != nil && row.Project != nil {
			return m, func() tea.Msg {
				return ShowProjectSelectorForMoveMsg{Task: row.Task, SourceProject: row.Project}
			}
		}
		return m, nil

	// Score editor
	case "S":
		if row := m.getRowAtCursor(); row != nil {
			return m, func() tea.Msg {
				return ShowScoreEditorMsg{Task: row.Task, Project: row.Project}
			}
		}
		return m, nil

	// Help
	case "?":
		return m, helpCmd(ViewFocusBoard, "Focus Board", m.GetHelpItems())
	}

	return m, nil
}

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *FocusBoardModel) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
	prefix := m.pendingPrefix
	m.pendingPrefix = ""

	if key == "esc" {
		return m, nil
	}

	if model, cmd, handled := dispatchCommonPrefix(prefix, key, m, nil); handled {
		return model, cmd
	}

	return m, nil
}

// handleConfirmKey handles key input while the confirmation dialog is active.
func (m *FocusBoardModel) handleConfirmKey(key string) (tea.Model, tea.Cmd) {
	if confirmed, handled := m.HandleConfirmKey(key); handled {
		if confirmed {
			return m.deleteItem()
		}
		return m, nil
	}
	return m, nil
}

// GetHelpItems returns help items for this view.
func (m *FocusBoardModel) GetHelpItems() []HelpItem {
	return []HelpItem{
		{Key: "a", Description: "Quick-add action"},
		{Key: "x", Description: "Mark done"},
		{Key: "X", Description: "Dismiss (cancel)"},
		{Key: "d", Description: "Toggle done/open"},
		{Key: "D", Description: "Delete action permanently"},
		{Key: "p", Description: "Set priority"},
		{Key: ",", Description: "Status prefix (,t/,i/,d/,b)"},
		{Key: "!", Description: "Priority prefix (!1-!5)"},
		{Key: "g", Description: "Goto prefix (gg/gp)"},
		{Key: "m", Description: "Move section (mn/ms/md/mr/mw)"},
		{Key: "c", Description: "Linked task prefix"},
		{Key: "e", Description: "Edit task"},
		{Key: "E", Description: "Quick-edit title"},
		{Key: "P", Description: "Assign project"},
		{Key: "S", Description: "Edit score dimensions"},
		{Key: "o", Description: "Open URL"},
		{Key: "enter", Description: "Navigate to task / open URL"},
		{Key: "tab/[/]", Description: "Switch tabs"},
		{Key: "F", Description: "Filter (F-prefix)"},
		{Key: "/", Description: "Search"},
		{Key: "Fx", Description: "Clear filters"},
		{Key: "1-5", Description: "Sort by field (pri/due/new/sts/scr)"},
		{Key: "0", Description: "Reset sort"},
		{Key: "j/k", Description: "Navigate list"},
		{Key: "q/esc", Description: "Close (clears filters first)"},
	}
}
