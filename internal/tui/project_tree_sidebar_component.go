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
	dependencySection int  // 0 = "Blocked by", 1 = "Blocking", 2 = "Related"
	depsScrollOffset  int  // Scroll offset for deps/related sections
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

// GetDependencySection returns the active dependency section (0=Blocked by, 1=Blocking, 2=Related).
func (s *SidebarComponent) GetDependencySection() int {
	return s.dependencySection
}

// SetDependencySection sets the active dependency section.
func (s *SidebarComponent) SetDependencySection(section int) {
	s.dependencySection = section
}

// GetDepsScrollOffset returns the scroll offset for deps/related sections.
func (s *SidebarComponent) GetDepsScrollOffset() int {
	return s.depsScrollOffset
}

// SetDepsScrollOffset sets the scroll offset for deps/related sections.
func (s *SidebarComponent) SetDepsScrollOffset(offset int) {
	s.depsScrollOffset = offset
}

// ResetScroll resets the scroll position to the top.
func (s *SidebarComponent) ResetScroll() {
	s.scroll = 0
	s.dependencyCursor = 0
	s.dependencySection = 0
	s.depsScrollOffset = 0
}

// Focus focuses the sidebar and resets scroll position.
func (s *SidebarComponent) Focus() {
	s.focused = true
	s.scroll = 0
	s.dependencyCursor = 0
	s.dependencySection = 0
	s.depsScrollOffset = 0
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
		s.depsScrollOffset = 0
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
		// Move down in linked tasks or scroll
		if m.hasLinkedTasks(task) {
			s.moveDependencyCursor(m, task, 1)
		}
		return true, nil

	case "k", "up":
		// Move up in linked tasks or scroll
		if m.hasLinkedTasks(task) {
			s.moveDependencyCursor(m, task, -1)
		}
		return true, nil

	case "h", "left":
		// Cycle sections backward: Related -> Blocking -> Blocked by -> Related
		if m.hasLinkedTasks(task) {
			s.cycleSectionBackward(m, task)
		}
		return true, nil

	case "l", "right":
		// Cycle sections forward: Blocked by -> Blocking -> Related -> Blocked by
		if m.hasLinkedTasks(task) {
			s.cycleSectionForward(m, task)
		}
		return true, nil

	case "enter":
		// Jump to selected linked task
		if m.hasLinkedTasks(task) {
			return true, s.jumpToSelectedLinkedTask(m, task)
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
	switch s.dependencySection {
	case 0:
		maxCursor = len(m.project.GetBlockingTasks(task)) - 1
	case 1:
		maxCursor = len(m.project.GetBlockedByTasks(task)) - 1
	case 2:
		maxCursor = len(m.getRelatedTasks(task)) - 1
	}

	s.dependencyCursor += delta
	if s.dependencyCursor < 0 {
		s.dependencyCursor = 0
	}
	if s.dependencyCursor > maxCursor {
		s.dependencyCursor = maxCursor
	}

	// Update scroll offset to keep cursor visible (max 5 visible items per section)
	maxVisible := 5
	if s.dependencyCursor >= s.depsScrollOffset+maxVisible {
		s.depsScrollOffset = s.dependencyCursor - maxVisible + 1
	}
	if s.dependencyCursor < s.depsScrollOffset {
		s.depsScrollOffset = s.dependencyCursor
	}
}

// cycleSectionForward cycles to the next non-empty section (0 -> 1 -> 2 -> 0).
func (s *SidebarComponent) cycleSectionForward(m *ProjectTreeModel, task *models.Task) {
	sections := m.getAvailableSections(task)
	if len(sections) <= 1 {
		return
	}

	// Find current section index and move to next
	currentIdx := 0
	for i, sec := range sections {
		if sec == s.dependencySection {
			currentIdx = i
			break
		}
	}
	nextIdx := (currentIdx + 1) % len(sections)
	s.dependencySection = sections[nextIdx]
	s.dependencyCursor = 0
	s.depsScrollOffset = 0
}

// cycleSectionBackward cycles to the previous non-empty section (0 -> 2 -> 1 -> 0).
func (s *SidebarComponent) cycleSectionBackward(m *ProjectTreeModel, task *models.Task) {
	sections := m.getAvailableSections(task)
	if len(sections) <= 1 {
		return
	}

	// Find current section index and move to previous
	currentIdx := 0
	for i, sec := range sections {
		if sec == s.dependencySection {
			currentIdx = i
			break
		}
	}
	prevIdx := (currentIdx - 1 + len(sections)) % len(sections)
	s.dependencySection = sections[prevIdx]
	s.dependencyCursor = 0
	s.depsScrollOffset = 0
}

// jumpToSelectedLinkedTask jumps to the selected linked task.
func (s *SidebarComponent) jumpToSelectedLinkedTask(m *ProjectTreeModel, task *models.Task) tea.Cmd {
	var targetTask *models.Task

	switch s.dependencySection {
	case 0:
		// Jump to blocking task
		blockingTasks := m.project.GetBlockingTasks(task)
		if s.dependencyCursor < len(blockingTasks) {
			targetTask = blockingTasks[s.dependencyCursor]
		}
	case 1:
		// Jump to blocked task
		blockedTasks := m.project.GetBlockedByTasks(task)
		if s.dependencyCursor < len(blockedTasks) {
			targetTask = blockedTasks[s.dependencyCursor]
		}
	case 2:
		// Jump to related task
		relatedTasks := m.getRelatedTasks(task)
		if s.dependencyCursor < len(relatedTasks) {
			targetTask = relatedTasks[s.dependencyCursor]
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
