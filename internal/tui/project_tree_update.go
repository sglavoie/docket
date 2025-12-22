package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *ProjectTreeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	visible := m.visibleNodes()

	switch msg := msg.(type) {
	case treeRefreshMsg:
		m.buildTree()
		return m, nil

	case tea.KeyMsg:
		// Handle confirmation prompts first
		if m.confirmAction != "" {
			return m.handleConfirmation(msg)
		}

		// Handle search mode
		if m.IsSearchMode() {
			return m.handleSearchMode(msg)
		}

		// Handle tag input mode for bulk tagging
		if m.tagInputMode {
			return m.handleTagInputMode(msg)
		}

		// Handle normal key input
		return m.handleKeyMsg(msg, visible)
	}

	return m, nil
}

// handleConfirmation handles y/n confirmation prompts.
func (m *ProjectTreeModel) handleConfirmation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if m.confirmAction == "archive" {
			m.confirmAction = ""
			return m, m.archiveProject()
		} else if m.confirmAction == "unarchive" {
			m.confirmAction = ""
			return m, m.unarchiveProject()
		} else if m.confirmAction == "delete" {
			m.confirmAction = ""
			return m, m.deleteTask()
		} else if m.confirmAction == "bulk-delete" {
			m.confirmAction = ""
			return m, m.bulkDeleteTasks()
		}
	case "n", "N", "esc":
		m.confirmAction = ""
		m.deleteTarget = nil
		return m, nil
	}
	return m, nil
}

// handleSearchMode handles input while in search mode.
func (m *ProjectTreeModel) handleSearchMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	handled, cmd, valueChanged := m.HandleSearchInput(msg)
	if valueChanged {
		m.updateVisibility()
		visible := m.visibleNodes()
		m.cursor = ClampCursor(m.cursor, len(visible))
	}
	if handled {
		return m, cmd
	}
	return m, cmd
}

// handleTagInputMode handles input while in tag input mode.
func (m *ProjectTreeModel) handleTagInputMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.tagInputMode = false
		m.tagInput.Blur()
		m.tagInput.SetValue("")
		return m, nil
	case "enter":
		tag := strings.TrimSpace(m.tagInput.Value())
		m.tagInputMode = false
		m.tagInput.Blur()
		m.tagInput.SetValue("")
		if tag == "" {
			return m, nil
		}
		m.applyBulkTag(tag)
		m.clearSelection()
		return m, m.saveAndRefresh()
	default:
		var cmd tea.Cmd
		m.tagInput, cmd = m.tagInput.Update(msg)
		return m, cmd
	}
}

// handleKeyMsg dispatches key messages to appropriate handlers.
func (m *ProjectTreeModel) handleKeyMsg(msg tea.KeyMsg, visible []*TreeNode) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Special case for search activation
	if key == "/" && m.confirmAction == "" {
		return m, m.ActivateSearch()
	}

	// Special case for esc (exit multi-select or clear search)
	if key == "esc" {
		if m.multiSelectMode {
			m.clearSelection()
			return m, nil
		}
		if m.SearchQuery() != "" {
			m.ClearSearch()
			m.updateVisibility()
			return m, nil
		}
	}

	// Special case for A key (archive or deselect all)
	if key == "A" {
		handled, cmd := m.handleArchiveKey(visible)
		if handled {
			return m, cmd
		}
	}

	// Try each handler group
	if handled, cmd := m.handleNavigationKeys(key, visible); handled {
		return m, cmd
	}
	if handled, cmd := m.handleStatusKeys(key, visible); handled {
		return m, cmd
	}
	if handled, cmd := m.handleTypeKeys(key, visible); handled {
		return m, cmd
	}
	if handled, cmd := m.handleMovementKeys(key, visible); handled {
		return m, cmd
	}
	if handled, cmd := m.handleSelectionKeys(key, visible); handled {
		return m, cmd
	}
	if handled, cmd := m.handleFilterKeys(key); handled {
		return m, cmd
	}
	if handled, cmd := m.handleSortKeys(key); handled {
		return m, cmd
	}
	if handled, cmd := m.handleActionKeys(key, visible); handled {
		return m, cmd
	}

	return m, nil
}
