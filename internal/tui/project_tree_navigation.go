package tui

import (
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
		old := m.cursor
		if len(visible) > 0 {
			m.cursor = len(visible) - 1
		}
		return true, flashIfJumped(old, m.cursor, &m.cursorFlash)
	case "n":
		return true, m.SectionGoToNext(rootNodeIndices(visible))
	case "N":
		return true, m.SectionGoToPrev(rootNodeIndices(visible))
	case "h":
		if !m.sidebar.IsFocused() {
			m.clearActiveSavedFilter()
			modes := m.cyclableViewModes()
			if idx := viewModeIndex(modes, m.viewMode); idx >= 0 {
				m.viewMode = modes[(idx+len(modes)-1)%len(modes)]
			} else {
				m.viewMode = modes[len(modes)-1]
			}
			m.updateVisibility()
			m.clampCursor()
			return true, nil
		}
	case "l":
		if !m.sidebar.IsFocused() {
			m.clearActiveSavedFilter()
			modes := m.cyclableViewModes()
			if idx := viewModeIndex(modes, m.viewMode); idx >= 0 {
				m.viewMode = modes[(idx+1)%len(modes)]
			} else {
				m.viewMode = modes[0]
			}
			m.updateVisibility()
			m.clampCursor()
			return true, nil
		}
	}
	return false, nil
}

// cyclableViewModes returns the view modes that h/l should cycle through.
// Snoozed is included only when the project has snoozed tasks.
func (m *ProjectTreeModel) cyclableViewModes() []ViewMode {
	modes := []ViewMode{ViewModeActive, ViewModeNext, ViewModeInbox, ViewModeWaiting, ViewModeSomeday}
	for _, task := range m.project.Tasks {
		if task.IsSnoozed() {
			return append(modes, ViewModeSnoozed)
		}
	}
	return modes
}

func viewModeIndex(modes []ViewMode, mode ViewMode) int {
	for i, m := range modes {
		if m == mode {
			return i
		}
	}
	return -1
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
		// Tab cycles through three states: hidden -> visible/tree -> visible/sidebar -> hidden
		var task *models.Task
		if CursorInBounds(m.cursor, len(visible)) {
			task = visible[m.cursor].Task
		}
		m.sidebar.CycleState(task)
		// When entering focused state, start in first non-empty linked section
		if m.sidebar.IsFocused() && task != nil {
			sections := sidebarGetAvailableSections(m.project, task)
			if len(sections) > 0 {
				m.sidebar.SetDependencySection(sections[0])
			}
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

// rootNodeIndices returns the indices of root-level (Level == 0) nodes
// in the visible list, for use with navigateToNextSection/navigateToPrevSection.
func rootNodeIndices(visible []*TreeNode) []int {
	var indices []int
	for i, node := range visible {
		if node.Level == 0 {
			indices = append(indices, i)
		}
	}
	return indices
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
