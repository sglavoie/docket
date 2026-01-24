package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// handleNavigationKeys handles j/k, G, arrow keys, and page navigation.
// Note: gg (go to first) is handled via the g prefix in handlePrefixKey.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleNavigationKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return true, nil
	case "down", "j":
		if m.cursor < len(visible)-1 {
			m.cursor++
		}
		return true, nil
	case "G":
		if len(visible) > 0 {
			m.cursor = len(visible) - 1
		}
		return true, nil
	case "h":
		// Cycle backward through views: Active -> Someday -> Waiting -> Inbox -> Next -> Active
		// Skip if sidebar is focused (handled by handleSidebarKeys)
		if !m.sidebar.IsFocused() {
			m.viewMode = (m.viewMode + 4) % 5
			m.updateVisibility()
			m.clampCursor()
			return true, nil
		}
	case "l":
		// Cycle forward through views: Active -> Next -> Inbox -> Waiting -> Someday -> Active
		// Skip if sidebar is focused (handled by handleSidebarKeys)
		if !m.sidebar.IsFocused() {
			m.viewMode = (m.viewMode + 1) % 5
			m.updateVisibility()
			m.clampCursor()
			return true, nil
		}
	}
	return false, nil
}

// handleSelectionKeys handles v, a, space - selection operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleSelectionKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case " ":
		if m.multiSelectMode {
			if CursorInBounds(m.cursor, len(visible)) {
				m.toggleSelection(visible[m.cursor].Task)
			}
			return true, nil
		}
		if CursorInBounds(m.cursor, len(visible)) {
			node := visible[m.cursor]
			if len(node.Task.Children) > 0 {
				node.Expanded = !node.Expanded
				m.updateVisibility()
			}
		}
		return true, nil
	case "tab":
		// Tab cycles through three states: hidden, visible/tree, visible/sidebar
		if !m.sidebar.IsEnabled() {
			// State 1 -> State 2: Show sidebar, focus tree
			m.sidebar.SetEnabled(true)
			m.sidebar.SetFocused(false)
		} else if !m.sidebar.IsFocused() {
			// State 2 -> State 3: Keep sidebar visible, focus sidebar
			m.sidebar.SetFocused(true)
			m.sidebar.SetScroll(0)
			m.sidebar.SetDependencyCursor(0)
			// Start in first non-empty dependency section
			m.sidebar.SetDependencySection(0)
			if CursorInBounds(m.cursor, len(visible)) {
				task := visible[m.cursor].Task
				if len(task.DependsOn) == 0 && len(m.project.GetBlockedByTasks(task)) > 0 {
					m.sidebar.SetDependencySection(1) // Start in "Blocking" section
				}
			}
		} else {
			// State 3 -> State 1: Hide sidebar
			m.sidebar.SetEnabled(false)
			m.sidebar.SetFocused(false)
		}
		return true, nil
	case "v":
		if m.multiSelectMode {
			m.clearSelection()
		} else {
			m.multiSelectMode = true
			if CursorInBounds(m.cursor, len(visible)) {
				m.toggleSelection(visible[m.cursor].Task)
			}
		}
		return true, nil
	case "a":
		if m.multiSelectMode {
			m.selectAllVisible()
			return true, nil
		}
		return false, nil
	case "A":
		if m.multiSelectMode {
			m.deselectAllVisible()
			return true, nil
		}
		return false, nil
	}
	return false, nil
}

// handleSidebarKeys handles sidebar navigation when sidebar has focus.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleSidebarKeys(key string, selectedTask *models.Task) (bool, tea.Cmd) {
	if !m.sidebar.IsFocused() {
		return false, nil
	}

	switch key {
	case "esc":
		m.sidebar.SetFocused(false)
		return true, nil

	case "j", "down":
		if selectedTask != nil && m.hasDependencies(selectedTask) {
			m.moveSidebarDependencyCursorDown(selectedTask)
		} else if selectedTask != nil && selectedTask.Description != "" {
			// Only scroll if there's a description to scroll
			lines := strings.Split(selectedTask.Description, "\n")
			if m.sidebar.GetScroll() < len(lines)-1 {
				m.sidebar.SetScroll(m.sidebar.GetScroll() + 1)
			}
		}
		return true, nil

	case "k", "up":
		if selectedTask != nil && m.hasDependencies(selectedTask) {
			m.moveSidebarDependencyCursorUp()
		} else {
			if m.sidebar.GetScroll() > 0 {
				m.sidebar.SetScroll(m.sidebar.GetScroll() - 1)
			}
		}
		return true, nil

	case "h", "left", "l", "right":
		if selectedTask != nil && m.hasDependencies(selectedTask) {
			m.switchSidebarDependencySection(selectedTask)
		}
		return true, nil

	case "enter":
		if selectedTask != nil {
			depTask := m.getSelectedSidebarDependency(selectedTask)
			if depTask != nil {
				m.sidebar.SetFocused(false)
				m.NavigateToTask(depTask)
			}
		}
		return true, nil
	}

	return false, nil
}

// hasDependencies returns true if the task has any dependencies.
func (m *ProjectTreeModel) hasDependencies(task *models.Task) bool {
	return len(task.DependsOn) > 0 || len(m.project.GetBlockedByTasks(task)) > 0
}

// moveSidebarDependencyCursorDown moves the dependency cursor down.
func (m *ProjectTreeModel) moveSidebarDependencyCursorDown(task *models.Task) {
	items := m.getCurrentSidebarDependencyItems(task)
	if m.sidebar.GetDependencyCursor() < len(items)-1 {
		m.sidebar.SetDependencyCursor(m.sidebar.GetDependencyCursor() + 1)
	}
}

// moveSidebarDependencyCursorUp moves the dependency cursor up.
func (m *ProjectTreeModel) moveSidebarDependencyCursorUp() {
	if m.sidebar.GetDependencyCursor() > 0 {
		m.sidebar.SetDependencyCursor(m.sidebar.GetDependencyCursor() - 1)
	}
}

// switchSidebarDependencySection switches between "Blocked by" and "Blocking" sections.
func (m *ProjectTreeModel) switchSidebarDependencySection(task *models.Task) {
	if m.sidebar.GetDependencySection() == 0 {
		m.sidebar.SetDependencySection(1)
	} else {
		m.sidebar.SetDependencySection(0)
	}
	// Reset cursor to valid position
	items := m.getCurrentSidebarDependencyItems(task)
	if m.sidebar.GetDependencyCursor() >= len(items) {
		if len(items) > 0 {
			m.sidebar.SetDependencyCursor(len(items) - 1)
		} else {
			m.sidebar.SetDependencyCursor(0)
		}
	}
}

// getCurrentSidebarDependencyItems returns the tasks in the current dependency section.
func (m *ProjectTreeModel) getCurrentSidebarDependencyItems(task *models.Task) []*models.Task {
	if m.sidebar.GetDependencySection() == 0 {
		return m.project.GetBlockingTasks(task)
	}
	return m.project.GetBlockedByTasks(task)
}

// getSelectedSidebarDependency returns the currently selected dependency task.
func (m *ProjectTreeModel) getSelectedSidebarDependency(task *models.Task) *models.Task {
	items := m.getCurrentSidebarDependencyItems(task)
	if m.sidebar.GetDependencyCursor() >= 0 && m.sidebar.GetDependencyCursor() < len(items) {
		return items[m.sidebar.GetDependencyCursor()]
	}
	return nil
}
