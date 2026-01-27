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
			// Start in first non-empty linked section
			if CursorInBounds(m.cursor, len(visible)) {
				task := visible[m.cursor].Task
				sections := m.getAvailableSections(task)
				if len(sections) > 0 {
					m.sidebar.SetDependencySection(sections[0])
				} else {
					m.sidebar.SetDependencySection(0)
				}
			} else {
				m.sidebar.SetDependencySection(0)
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
		if selectedTask != nil && m.hasLinkedTasks(selectedTask) {
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
		if selectedTask != nil && m.hasLinkedTasks(selectedTask) {
			m.moveSidebarDependencyCursorUp()
		} else {
			if m.sidebar.GetScroll() > 0 {
				m.sidebar.SetScroll(m.sidebar.GetScroll() - 1)
			}
		}
		return true, nil

	case "h", "left", "l", "right":
		if selectedTask != nil && m.hasLinkedTasks(selectedTask) {
			m.switchSidebarDependencySection(selectedTask)
		}
		return true, nil

	case "enter":
		if selectedTask != nil {
			linkedTask := m.getSelectedSidebarLinkedTask(selectedTask)
			if linkedTask != nil {
				m.sidebar.SetFocused(false)
				m.NavigateToTask(linkedTask)
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

// hasLinkedTasks returns true if the task has any linked tasks (dependencies or related).
func (m *ProjectTreeModel) hasLinkedTasks(task *models.Task) bool {
	return len(task.DependsOn) > 0 || len(m.project.GetBlockedByTasks(task)) > 0 || len(task.RelatedTo) > 0
}

// getRelatedTasks returns the tasks that are related to the given task.
func (m *ProjectTreeModel) getRelatedTasks(task *models.Task) []*models.Task {
	var related []*models.Task
	for _, relID := range task.RelatedTo {
		if t := m.project.GetTaskByID(relID); t != nil {
			related = append(related, t)
		}
	}
	return related
}

// getAvailableSections returns the non-empty section indices for a task.
// Section 0 = Blocked by, 1 = Blocking, 2 = Related
func (m *ProjectTreeModel) getAvailableSections(task *models.Task) []int {
	var sections []int
	if len(task.DependsOn) > 0 {
		sections = append(sections, 0)
	}
	if len(m.project.GetBlockedByTasks(task)) > 0 {
		sections = append(sections, 1)
	}
	if len(task.RelatedTo) > 0 {
		sections = append(sections, 2)
	}
	return sections
}

// moveSidebarDependencyCursorDown moves the dependency cursor down.
func (m *ProjectTreeModel) moveSidebarDependencyCursorDown(task *models.Task) {
	items := m.getCurrentSidebarLinkedItems(task)
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

// switchSidebarDependencySection cycles through sections (0=Blocked by, 1=Blocking, 2=Related).
func (m *ProjectTreeModel) switchSidebarDependencySection(task *models.Task) {
	sections := m.getAvailableSections(task)
	if len(sections) <= 1 {
		return
	}

	// Find current section index and move to next
	currentIdx := 0
	for i, sec := range sections {
		if sec == m.sidebar.GetDependencySection() {
			currentIdx = i
			break
		}
	}
	nextIdx := (currentIdx + 1) % len(sections)
	m.sidebar.SetDependencySection(sections[nextIdx])
	m.sidebar.SetDependencyCursor(0)
}

// getCurrentSidebarLinkedItems returns the tasks in the current linked section.
func (m *ProjectTreeModel) getCurrentSidebarLinkedItems(task *models.Task) []*models.Task {
	switch m.sidebar.GetDependencySection() {
	case 0:
		return m.project.GetBlockingTasks(task)
	case 1:
		return m.project.GetBlockedByTasks(task)
	case 2:
		return m.getRelatedTasks(task)
	}
	return nil
}

// getSelectedSidebarLinkedTask returns the currently selected linked task.
func (m *ProjectTreeModel) getSelectedSidebarLinkedTask(task *models.Task) *models.Task {
	items := m.getCurrentSidebarLinkedItems(task)
	if m.sidebar.GetDependencyCursor() >= 0 && m.sidebar.GetDependencyCursor() < len(items) {
		return items[m.sidebar.GetDependencyCursor()]
	}
	return nil
}

// JumpToTask moves the cursor to the specified task in the visible list.
func (m *ProjectTreeModel) JumpToTask(task *models.Task) {
	if task == nil {
		return
	}
	m.buildTree()
	visible := m.visibleNodes()
	for i, node := range visible {
		if node.Task.ID == task.ID {
			m.cursor = i
			return
		}
	}
}
