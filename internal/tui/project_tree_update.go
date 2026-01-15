package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/tt/internal/models"
)

func (m *ProjectTreeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	visible := m.visibleNodes()

	switch msg := msg.(type) {
	case treeRefreshMsg:
		m.buildTree()
		m.clampCursor()
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
		count := len(m.selectedTasks)
		m.clearSelection()
		m.setUndoContext("Add Tag", fmt.Sprintf("'%s' to %d tasks", tag, count))
		return m, m.saveAndRefresh()
	default:
		var cmd tea.Cmd
		m.tagInput, cmd = m.tagInput.Update(msg)
		return m, cmd
	}
}

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *ProjectTreeModel) handlePrefixKey(key string, visible []*TreeNode) (tea.Model, tea.Cmd) {
	prefix := m.pendingPrefix
	m.pendingPrefix = "" // Always clear prefix

	if key == "esc" {
		return m, nil // Cancel prefix mode
	}

	switch prefix {
	case ",":
		// Status operations
		switch key {
		case "s":
			_, cmd := m.handleStartTask(visible)
			return m, cmd
		case "d":
			_, cmd := m.handleCompleteTask(visible)
			return m, cmd
		case "b":
			_, cmd := m.handleBlockTask(visible)
			return m, cmd
		case "x":
			_, cmd := m.handleCancelTask(visible)
			return m, cmd
		case "o":
			_, cmd := m.handleReopenTask(visible)
			return m, cmd
		}

	case "z":
		// Fold operations
		switch key {
		case "a":
			// Toggle expand/collapse current node
			if CursorInBounds(m.cursor, len(visible)) {
				node := visible[m.cursor]
				if len(node.Task.Children) > 0 {
					node.Expanded = !node.Expanded
					m.updateVisibility()
				}
			}
			return m, nil
		case "o":
			// Expand current node
			if CursorInBounds(m.cursor, len(visible)) {
				node := visible[m.cursor]
				node.Expanded = true
				m.updateVisibility()
			}
			return m, nil
		case "c":
			// Collapse current node
			if CursorInBounds(m.cursor, len(visible)) {
				node := visible[m.cursor]
				node.Expanded = false
				m.updateVisibility()
			}
			return m, nil
		case "R":
			// Expand all
			for _, node := range m.nodes {
				node.Expanded = true
			}
			m.updateVisibility()
			return m, nil
		case "M":
			// Collapse all
			for _, node := range m.nodes {
				if node.Level > 0 {
					node.Expanded = false
				}
			}
			m.updateVisibility()
			return m, nil
		case "v":
			// Focus mode (zoom into subtree)
			if CursorInBounds(m.cursor, len(visible)) {
				task := visible[m.cursor].Task
				if task.Type.CanHaveChildren() {
					m.focusedTask = task
					m.cursor = 0
					m.buildTree()
				}
			}
			return m, nil
		}

	case "g":
		// Go/section operations
		switch key {
		case "g":
			// Go to first task (Vim-style gg)
			m.cursor = 0
			return m, nil
		case "a":
			// Go to Active view
			m.viewMode = ViewModeActive
			m.updateVisibility()
			m.pruneInvisibleSelections()
			m.clampCursor()
			return m, nil
		case "b":
			// Go to Backlog view
			m.viewMode = ViewModeBacklog
			m.updateVisibility()
			m.pruneInvisibleSelections()
			m.clampCursor()
			return m, nil
		case "i":
			// Go to Inbox view
			m.viewMode = ViewModeInbox
			m.updateVisibility()
			m.pruneInvisibleSelections()
			m.clampCursor()
			return m, nil
		case "c":
			// Go to Completed view
			m.viewMode = ViewModeCompleted
			m.updateVisibility()
			m.pruneInvisibleSelections()
			m.clampCursor()
			return m, nil
		case "A":
			// Send to Active section
			_, cmd := m.sendTasksToActive(visible)
			return m, cmd
		case "B":
			// Send to Backlog section
			_, cmd := m.sendTasksToSection(models.SectionBacklog, visible)
			return m, cmd
		case "I":
			// Send to Inbox section
			_, cmd := m.sendTasksToSection(models.SectionInbox, visible)
			return m, cmd
		case "d":
			// Cycle due date filter
			m.filterDueDate = (m.filterDueDate + 1) % 5
			m.updateVisibility()
			m.pruneInvisibleSelections()
			m.clampCursor()
			return m, nil
		case "v":
			// Cycle view modes
			m.viewMode = (m.viewMode + 1) % 4
			m.updateVisibility()
			m.pruneInvisibleSelections()
			m.clampCursor()
			return m, nil
		case "X":
			// Deselect all (in multi-select mode)
			if m.multiSelectMode {
				for _, node := range m.visibleNodes() {
					delete(m.selectedTasks, node.Task.ID)
				}
			}
			return m, nil
		}
	}

	// Invalid prefix sequence - silently ignore
	return m, nil
}

// handleKeyMsg dispatches key messages to appropriate handlers.
func (m *ProjectTreeModel) handleKeyMsg(msg tea.KeyMsg, visible []*TreeNode) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle pending prefix keys first (which-key style two-key sequences)
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key, visible)
	}

	// Get selected task for sidebar navigation
	var selectedTask *models.Task
	if len(visible) > 0 && m.cursor >= 0 && m.cursor < len(visible) {
		selectedTask = visible[m.cursor].Task
	}

	// Sidebar keys have priority when sidebar is focused
	if m.sidebarFocused {
		if handled, cmd := m.handleSidebarKeys(key, selectedTask); handled {
			return m, cmd
		}
	}

	// Prefix key activation (status, fold, go/section)
	switch key {
	case ",":
		m.pendingPrefix = ","
		return m, nil
	case "z":
		m.pendingPrefix = "z"
		return m, nil
	case "g":
		m.pendingPrefix = "g"
		return m, nil
	}

	// Special case for search activation
	if key == "/" && m.confirmAction == "" {
		return m, m.ActivateSearch()
	}

	// Special case for esc (exit multi-select, focus mode, or clear search)
	if key == "esc" {
		if m.multiSelectMode {
			m.clearSelection()
			return m, nil
		}
		if m.focusedTask != nil {
			m.focusedTask = nil
			m.buildTree()
			m.clampCursor()
			return m, nil
		}
		if m.SearchQuery() != "" {
			m.ClearSearch()
			m.updateVisibility()
			return m, nil
		}
	}

	// Special case for ctrl+a (archive project)
	if key == "ctrl+a" {
		if m.project.Archived {
			m.confirmAction = "unarchive"
		} else {
			m.confirmAction = "archive"
		}
		return m, nil
	}

	// Project cycling keys (emit messages to app level)
	switch key {
	case "`":
		return m, func() tea.Msg { return CycleProjectMsg{Forward: true} }
	case "~":
		return m, func() tea.Msg { return CycleProjectMsg{Forward: false} }
	case "@":
		return m, func() tea.Msg { return ShowProjectCycleManagerMsg{} }
	}

	// Try each handler group
	if handled, cmd := m.handleNavigationKeys(key, visible); handled {
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
	if handled, cmd := m.handleFilterBarKeys(key); handled {
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
