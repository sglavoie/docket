package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
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
		if !m.sidebar.IsFocused() {
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
			result := m.getSelectedSidebarLinkedTaskWithProject(selectedTask)
			if result.Task != nil && !result.IsMissing {
				m.sidebar.SetFocused(false)
				// For cross-project links, emit message so app can switch projects
				if result.Project != nil {
					project := result.Project
					task := result.Task
					taskRef := result.TaskRef
					return true, func() tea.Msg {
						return LinkedTaskSelectedMsg{Task: task, Project: project, TaskRef: taskRef}
					}
				}
				// Same-project link, navigate directly
				m.NavigateToTask(result.Task)
			}
		}
		return true, nil

	case "x":
		if selectedTask != nil && m.hasLinkedTasks(selectedTask) {
			return true, m.unlinkSelectedSidebarTask(selectedTask)
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

// getRelatedTasks returns the tasks that are related to the given task (same-project only).
// For cross-project display, use resolveSidebarTaskRef instead.
func (m *ProjectTreeModel) getRelatedTasks(task *models.Task) []*models.Task {
	var related []*models.Task
	for _, relRef := range task.RelatedTo {
		taskRef := models.ParseTaskRef(relRef)
		if taskRef.IsCrossProject() {
			// Skip cross-project refs for backward compatibility
			continue
		}
		if t := m.project.GetTaskByID(taskRef.TaskID); t != nil {
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
	count := m.getCurrentSidebarLinkedItemCount(task)
	if m.sidebar.GetDependencyCursor() < count-1 {
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

// getCurrentSidebarLinkedItemCount returns the count of items in the current linked section.
// This correctly counts cross-project refs for navigation purposes.
func (m *ProjectTreeModel) getCurrentSidebarLinkedItemCount(task *models.Task) int {
	switch m.sidebar.GetDependencySection() {
	case 0:
		return len(task.DependsOn)
	case 1:
		return len(m.project.GetBlockedByTasks(task))
	case 2:
		return len(task.RelatedTo)
	}
	return 0
}

// sidebarLinkedTaskResult holds the result of resolving a sidebar linked task.
type sidebarLinkedTaskResult struct {
	Task      *models.Task
	Project   *models.Project // nil for same-project links
	TaskRef   string          // qualified ref for cross-project links
	IsMissing bool
}

// getSelectedSidebarLinkedTaskWithProject returns the currently selected linked task with its project.
// For cross-project links, loads and returns the task and project.
func (m *ProjectTreeModel) getSelectedSidebarLinkedTaskWithProject(task *models.Task) sidebarLinkedTaskResult {
	cursor := m.sidebar.GetDependencyCursor()

	switch m.sidebar.GetDependencySection() {
	case 0: // Blocked by (DependsOn refs)
		if cursor >= 0 && cursor < len(task.DependsOn) {
			ref := task.DependsOn[cursor]
			return m.resolveTaskRefWithProject(ref)
		}
	case 1: // Blocking (tasks that depend on this - same project only)
		blockedBy := m.project.GetBlockedByTasks(task)
		if cursor >= 0 && cursor < len(blockedBy) {
			return sidebarLinkedTaskResult{Task: blockedBy[cursor], Project: nil}
		}
	case 2: // Related
		if cursor >= 0 && cursor < len(task.RelatedTo) {
			ref := task.RelatedTo[cursor]
			return m.resolveTaskRefWithProject(ref)
		}
	}
	return sidebarLinkedTaskResult{IsMissing: true}
}

// resolveTaskRefWithProject resolves a task reference and returns the task with its project.
func (m *ProjectTreeModel) resolveTaskRefWithProject(ref string) sidebarLinkedTaskResult {
	taskRef := models.ParseTaskRef(ref)

	if !taskRef.IsCrossProject() {
		task := m.project.GetTaskByID(taskRef.TaskID)
		if task != nil {
			return sidebarLinkedTaskResult{Task: task, Project: nil}
		}
		return sidebarLinkedTaskResult{IsMissing: true}
	}

	// Cross-project reference
	if m.config == nil {
		return sidebarLinkedTaskResult{IsMissing: true}
	}

	targetProject := m.config.GetProject(taskRef.ProjectID)
	if targetProject == nil {
		return sidebarLinkedTaskResult{IsMissing: true}
	}

	// Load the project if not already loaded
	if len(targetProject.Tasks) == 0 {
		_ = storage.LoadProject(targetProject)
	}

	task := targetProject.GetTaskByID(taskRef.TaskID)
	if task == nil {
		return sidebarLinkedTaskResult{IsMissing: true, Project: targetProject}
	}

	return sidebarLinkedTaskResult{
		Task:    task,
		Project: targetProject,
		TaskRef: ref,
	}
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

// unlinkSelectedSidebarTask removes the currently selected link from the sidebar.
// Section 0: Blocked by (source task's DependsOn) - remove from current task
// Section 1: Blocking (other task's DependsOn) - remove from the OTHER task
// Section 2: Related (bidirectional) - remove from both tasks
func (m *ProjectTreeModel) unlinkSelectedSidebarTask(sourceTask *models.Task) tea.Cmd {
	section := m.sidebar.GetDependencySection()
	cursor := m.sidebar.GetDependencyCursor()

	return func() tea.Msg {
		switch section {
		case 0: // Blocked by - remove from sourceTask.DependsOn
			if cursor < 0 || cursor >= len(sourceTask.DependsOn) {
				return nil
			}
			ref := sourceTask.DependsOn[cursor]
			sourceTask.RemoveDependency(ref)

			// Backup and save
			if err := m.projectService.Backup(m.project); err != nil {
				return ErrorMsg{Err: err}
			}
			if err := m.projectService.Save(m.project); err != nil {
				return ErrorMsg{Err: err}
			}

		case 1: // Blocking - remove from the other task's DependsOn
			blockedTasks := m.project.GetBlockedByTasks(sourceTask)
			if cursor < 0 || cursor >= len(blockedTasks) {
				return nil
			}
			blockedTask := blockedTasks[cursor]
			blockedTask.RemoveDependency(sourceTask.ID)

			// Backup and save
			if err := m.projectService.Backup(m.project); err != nil {
				return ErrorMsg{Err: err}
			}
			if err := m.projectService.Save(m.project); err != nil {
				return ErrorMsg{Err: err}
			}

		case 2: // Related - bidirectional, remove from both
			if cursor < 0 || cursor >= len(sourceTask.RelatedTo) {
				return nil
			}
			ref := sourceTask.RelatedTo[cursor]
			taskRef := models.ParseTaskRef(ref)

			// Remove from source task
			sourceTask.RemoveRelatedTo(ref)

			// Remove the reverse link from target task
			if taskRef.IsCrossProject() {
				// Cross-project: load target project, modify, save both
				if m.config != nil {
					targetProject := m.config.GetProject(taskRef.ProjectID)
					if targetProject != nil {
						if len(targetProject.Tasks) == 0 {
							if err := storage.LoadProject(targetProject); err != nil {
								return ErrorMsg{Err: err}
							}
						}
						if targetTask := targetProject.GetTaskByID(taskRef.TaskID); targetTask != nil {
							// Build reverse ref: sourceProject:sourceTaskID
							reverseRef := m.project.ID + ":" + sourceTask.ID
							targetTask.RemoveRelatedTo(reverseRef)

							// Backup and save target project
							if err := storage.BackupProject(targetProject); err != nil {
								return ErrorMsg{Err: err}
							}
							if err := storage.SaveProject(targetProject); err != nil {
								return ErrorMsg{Err: err}
							}
						}
					}
				}
			} else {
				// Same project: find and update target task
				if targetTask := m.project.GetTaskByID(taskRef.TaskID); targetTask != nil {
					targetTask.RemoveRelatedTo(sourceTask.ID)
				}
			}

			// Backup and save source project
			if err := m.projectService.Backup(m.project); err != nil {
				return ErrorMsg{Err: err}
			}
			if err := m.projectService.Save(m.project); err != nil {
				return ErrorMsg{Err: err}
			}
		}

		// Reload project and return message
		if err := m.projectService.Load(m.project); err != nil {
			return ErrorMsg{Err: err}
		}
		return LinkRemovedMsg{SourceTask: sourceTask, Section: section}
	}
}
