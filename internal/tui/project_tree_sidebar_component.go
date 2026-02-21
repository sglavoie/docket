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

// HandleKeys handles keyboard input for the sidebar using the SidebarHost interface.
func (s *SidebarComponent) HandleKeys(host SidebarHost, msg tea.KeyMsg) (bool, tea.Cmd) {
	if !s.focused {
		return false, nil
	}

	key := msg.String()

	// Tab and Esc work regardless of selected task
	switch key {
	case "tab":
		// Cycle from focused -> hidden (completes the cycle)
		s.enabled = false
		s.focused = false
		return true, nil
	case "esc":
		s.focused = false
		return true, nil
	}

	selectedTask := host.SidebarSelectedTask()
	if selectedTask == nil {
		return false, nil
	}

	project := host.SidebarProject()

	switch key {
	case "j", "down":
		if sidebarHasLinkedTasks(project, selectedTask) {
			s.moveCursorDown(project, selectedTask)
		} else {
			s.scroll++
		}
		return true, nil

	case "k", "up":
		if sidebarHasLinkedTasks(project, selectedTask) {
			s.moveCursorUp()
		} else {
			if s.scroll > 0 {
				s.scroll--
			}
		}
		return true, nil

	case "ctrl+d", "pgdown":
		halfPage := host.SidebarHeight() / 2
		if halfPage < 1 {
			halfPage = 1
		}
		s.scroll += halfPage
		return true, nil

	case "ctrl+u", "pgup":
		halfPage := host.SidebarHeight() / 2
		if halfPage < 1 {
			halfPage = 1
		}
		s.scroll -= halfPage
		if s.scroll < 0 {
			s.scroll = 0
		}
		return true, nil

	case "h", "left":
		if sidebarHasLinkedTasks(project, selectedTask) {
			s.switchSectionBackward(project, selectedTask)
		}
		return true, nil

	case "l", "right":
		if sidebarHasLinkedTasks(project, selectedTask) {
			s.switchSection(project, selectedTask)
		}
		return true, nil

	case "enter":
		{
			result := sidebarGetSelectedLinkedTaskWithProject(project, host.SidebarConfig(), s, selectedTask)
			if result.Task != nil && !result.IsMissing {
				s.focused = false
				// For cross-project links, emit message so app can switch projects
				if result.Project != nil {
					rProject := result.Project
					rTask := result.Task
					rRef := result.TaskRef
					return true, func() tea.Msg {
						return LinkedTaskSelectedMsg{Task: rTask, Project: rProject, TaskRef: rRef}
					}
				}
				// Same-project link, navigate directly
				host.NavigateToSidebarTask(result.Task)
			}
		}
		return true, nil

	case "x":
		if sidebarHasLinkedTasks(project, selectedTask) {
			return true, host.UnlinkSelectedSidebarTask(selectedTask)
		}
		return true, nil
	}

	return false, nil
}

// moveCursorDown moves the dependency cursor down within the current section.
func (s *SidebarComponent) moveCursorDown(project *models.Project, task *models.Task) {
	count := sidebarGetLinkedItemCount(project, s, task)
	if s.dependencyCursor < count-1 {
		s.dependencyCursor++
	}
}

// moveCursorUp moves the dependency cursor up.
func (s *SidebarComponent) moveCursorUp() {
	if s.dependencyCursor > 0 {
		s.dependencyCursor--
	}
}

// switchSectionBackward cycles backward through non-empty sections.
func (s *SidebarComponent) switchSectionBackward(project *models.Project, task *models.Task) {
	sections := sidebarGetAvailableSections(project, task)
	if len(sections) <= 1 {
		return
	}

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
}

// switchSection cycles forward through non-empty sections (0=Blocked by, 1=Blocking, 2=Related).
func (s *SidebarComponent) switchSection(project *models.Project, task *models.Task) {
	sections := sidebarGetAvailableSections(project, task)
	if len(sections) <= 1 {
		return
	}

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
}
