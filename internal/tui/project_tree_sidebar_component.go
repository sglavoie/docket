package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// SidebarComponent encapsulates sidebar state and functionality.
type SidebarComponent struct {
	enabled           bool // Toggle sidebar on/off
	focused           bool // When true, sidebar has focus for scrolling
	scroll            int  // Scroll offset for sidebar content
	dependencyCursor  int  // Cursor position within active dependency section
	dependencySection int  // 0 = "Blocked by", 1 = "Blocking"
}

// NewSidebarComponent creates a new sidebar component.
func NewSidebarComponent() *SidebarComponent {
	return &SidebarComponent{
		enabled:           true, // Sidebar on by default
		focused:           false,
		scroll:            0,
		dependencyCursor:  0,
		dependencySection: 0,
	}
}

// IsEnabled returns whether the sidebar is visible.
func (s *SidebarComponent) IsEnabled() bool {
	return s.enabled
}

// IsFocused returns whether the sidebar has focus.
func (s *SidebarComponent) IsFocused() bool {
	return s.focused
}

// SetEnabled sets the sidebar visibility.
func (s *SidebarComponent) SetEnabled(enabled bool) {
	s.enabled = enabled
}

// SetFocused sets the sidebar focus state.
func (s *SidebarComponent) SetFocused(focused bool) {
	s.focused = focused
}

// GetScroll returns the scroll offset.
func (s *SidebarComponent) GetScroll() int {
	return s.scroll
}

// SetScroll sets the scroll offset.
func (s *SidebarComponent) SetScroll(scroll int) {
	s.scroll = scroll
}

// GetDependencyCursor returns the dependency cursor position.
func (s *SidebarComponent) GetDependencyCursor() int {
	return s.dependencyCursor
}

// SetDependencyCursor sets the dependency cursor position.
func (s *SidebarComponent) SetDependencyCursor(cursor int) {
	s.dependencyCursor = cursor
}

// GetDependencySection returns the active dependency section (0=Blocked by, 1=Blocking).
func (s *SidebarComponent) GetDependencySection() int {
	return s.dependencySection
}

// SetDependencySection sets the active dependency section.
func (s *SidebarComponent) SetDependencySection(section int) {
	s.dependencySection = section
}

// ResetScroll resets the scroll position to the top.
func (s *SidebarComponent) ResetScroll() {
	s.scroll = 0
	s.dependencyCursor = 0
	s.dependencySection = 0
}

// Focus focuses the sidebar and resets scroll position.
func (s *SidebarComponent) Focus() {
	s.focused = true
	s.scroll = 0
	s.dependencyCursor = 0
	s.dependencySection = 0
}

// Blur removes focus from the sidebar.
func (s *SidebarComponent) Blur() {
	s.focused = false
}

// Toggle toggles the sidebar visibility.
func (s *SidebarComponent) Toggle() {
	s.enabled = !s.enabled
}

// CycleState cycles through sidebar states: hidden -> visible/tree -> visible/sidebar -> hidden.
// Returns true if the cycle should stop and handle the event elsewhere.
func (s *SidebarComponent) CycleState(task *models.Task) bool {
	if !s.enabled {
		// State 1 -> State 2: Show sidebar, focus tree
		s.enabled = true
		s.focused = false
		return false
	} else if !s.focused {
		// State 2 -> State 3: Keep sidebar visible, focus sidebar
		s.focused = true
		s.scroll = 0
		s.dependencyCursor = 0
		// If task has blocking tasks, start in "Blocking" section
		if task != nil && len(task.DependsOn) == 0 {
			s.dependencySection = 1 // Start in "Blocking" section
		} else {
			s.dependencySection = 0
		}
		return false
	} else {
		// State 3 -> State 1: Hide sidebar
		s.enabled = false
		s.focused = false
		return false
	}
}

// ShouldCaptureEsc returns true when the sidebar needs esc for internal handling.
func (s *SidebarComponent) ShouldCaptureEsc() bool {
	return s.focused
}

// HasTextInput returns false - sidebar doesn't have text input.
func (s *SidebarComponent) HasTextInput() bool {
	return false
}

// Render renders the sidebar for the given task.
func (s *SidebarComponent) Render(m *ProjectTreeModel, task *models.Task, width, height int) string {
	if !s.enabled {
		return ""
	}
	return m.renderSidebar(task, width, height)
}

// HandleKeys handles keyboard input for the sidebar.
func (s *SidebarComponent) HandleKeys(m *ProjectTreeModel, msg tea.KeyMsg) (bool, tea.Cmd) {
	if !s.focused {
		return false, nil
	}

	// Get current task
	visible := m.visibleNodes()
	if len(visible) == 0 || m.cursor >= len(visible) {
		return false, nil
	}
	task := visible[m.cursor].Task

	switch msg.String() {
	case "j", "down":
		// Move down in dependencies or scroll
		if m.hasDependencies(task) {
			s.moveDependencyCursor(m, task, 1)
		}
		return true, nil

	case "k", "up":
		// Move up in dependencies or scroll
		if m.hasDependencies(task) {
			s.moveDependencyCursor(m, task, -1)
		}
		return true, nil

	case "h", "left":
		// Switch to "Blocked by" section
		if m.hasDependencies(task) && s.dependencySection != 0 {
			s.dependencySection = 0
			s.dependencyCursor = 0
		}
		return true, nil

	case "l", "right":
		// Switch to "Blocking" section
		if m.hasDependencies(task) && s.dependencySection != 1 {
			s.dependencySection = 1
			s.dependencyCursor = 0
		}
		return true, nil

	case "enter":
		// Jump to selected dependency
		if m.hasDependencies(task) {
			return true, s.jumpToSelectedDependency(m, task)
		}
		return true, nil

	case "esc", "q":
		// Unfocus sidebar
		s.focused = false
		return true, nil
	}

	return false, nil
}

// moveDependencyCursor moves the dependency cursor by delta.
func (s *SidebarComponent) moveDependencyCursor(m *ProjectTreeModel, task *models.Task, delta int) {
	var maxCursor int
	if s.dependencySection == 0 {
		maxCursor = len(m.project.GetBlockingTasks(task)) - 1
	} else {
		maxCursor = len(m.project.GetBlockedByTasks(task)) - 1
	}

	s.dependencyCursor += delta
	if s.dependencyCursor < 0 {
		s.dependencyCursor = 0
	}
	if s.dependencyCursor > maxCursor {
		s.dependencyCursor = maxCursor
	}
}

// jumpToSelectedDependency jumps to the selected dependency task.
func (s *SidebarComponent) jumpToSelectedDependency(m *ProjectTreeModel, task *models.Task) tea.Cmd {
	var targetTask *models.Task

	if s.dependencySection == 0 {
		// Jump to blocking task
		blockingTasks := m.project.GetBlockingTasks(task)
		if s.dependencyCursor < len(blockingTasks) {
			targetTask = blockingTasks[s.dependencyCursor]
		}
	} else {
		// Jump to blocked task
		blockedTasks := m.project.GetBlockedByTasks(task)
		if s.dependencyCursor < len(blockedTasks) {
			targetTask = blockedTasks[s.dependencyCursor]
		}
	}

	if targetTask != nil {
		// Find and jump to the task
		for i, node := range m.visibleNodes() {
			if node.Task.ID == targetTask.ID {
				m.cursor = i
				s.focused = false // Unfocus sidebar after jump
				break
			}
		}
	}

	return nil
}
