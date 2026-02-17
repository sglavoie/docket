package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// handleKeyMsg processes key events in the outliner view.
func (m *OutlinerModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle score filter custom value input
	if m.scoreFilterInput.IsActive() {
		cmd := m.scoreFilterInput.ProcessKey(msg, m.config, m.configManager, &m.filter.ScoreMin, func() { m.rebuildRows() })
		return m, cmd
	}

	// Handle search mode input first
	if m.IsSearchMode() {
		handled, cmd, valueChanged := m.HandleSearchInput(msg)
		if valueChanged {
			m.rebuildRows()
		}
		if handled {
			return m, cmd
		}
		return m, cmd
	}

	// Handle quick edit mode input first
	if m.quickEdit.IsActive() {
		return m.handleQuickEditMode(msg)
	}

	// Handle confirm dialog before anything else
	if m.confirm.IsActive() {
		return m.handleConfirmDialog(key)
	}

	// Sidebar-focused key handling (before prefix/action keys)
	if m.sidebar.IsFocused() {
		handled, cmd := m.sidebar.HandleKeys(m, msg)
		if handled {
			return m, cmd
		}
	}

	// Handle pending prefix keys first (which-key style two-key sequences)
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// Multi-select mode key handling (when active, intercept selection keys)
	if m.multiSelectMode {
		switch key {
		case "v":
			m.toggleMultiSelectMode()
			return m, nil
		case " ":
			if len(m.rows) > 0 && m.cursor < len(m.rows) {
				m.ToggleSelection(m.rows[m.cursor].task.ID)
			}
			return m, nil
		case "a":
			m.selectAllVisible()
			return m, nil
		case "A":
			m.deselectAllVisible()
			return m, nil
		}
	}

	// Sidebar toggle
	if key == "tab" {
		task := m.SidebarSelectedTask()
		m.sidebar.CycleState(task)
		// When entering focused state, start in first non-empty linked section
		if m.sidebar.IsFocused() && task != nil {
			sections := sidebarGetAvailableSections(m.project, task)
			if len(sections) > 0 {
				m.sidebar.SetDependencySection(sections[0])
			}
		}
		return m, nil
	}

	// Prefix key activation
	switch key {
	case ",", "t", "!", "d", "g", "b", "c", "p", "s", "z", "m", "F":
		m.pendingPrefix = key
		return m, nil
	}

	switch key {
	case "q":
		if len(m.zoomStack) == 0 {
			return m, func() tea.Msg {
				return CloseOutlinerMsg{}
			}
		}
		top := m.zoomStack[len(m.zoomStack)-1]
		m.zoomStack = m.zoomStack[:len(m.zoomStack)-1]
		m.cursor = top.CursorIndex
		return m, nil

	case "esc":
		if m.multiSelectMode {
			m.toggleMultiSelectMode()
			return m, nil
		}
		if m.SearchQuery() != "" {
			m.ClearSearch()
			m.rebuildRows()
			return m, nil
		}
		if len(m.zoomStack) > 0 {
			top := m.zoomStack[len(m.zoomStack)-1]
			m.zoomStack = m.zoomStack[:len(m.zoomStack)-1]
			m.cursor = top.CursorIndex
			return m, nil
		}
		return m, func() tea.Msg {
			return CloseOutlinerMsg{}
		}

	// Cursor movement
	case "j", "down":
		if m.cursor < len(m.rows)-1 {
			m.cursor++
		}
		return m, nil
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "G":
		if len(m.rows) > 0 {
			m.cursor = len(m.rows) - 1
		}
		return m, nil
	case "ctrl+d", "pgdown":
		half := max(m.contentHeight()/2, 1)
		m.cursor += half
		m.clampCursor()
		return m, nil
	case "ctrl+u", "pgup":
		half := max(m.contentHeight()/2, 1)
		m.cursor -= half
		m.clampCursor()
		return m, nil

	// Task movement
	case "J":
		// Move task down (swap with sibling below)
		if len(m.rows) > 0 && m.cursor < len(m.rows) {
			task := m.rows[m.cursor].task
			siblings := m.getSiblings(task)
			idx := -1
			for i, s := range siblings {
				if s.ID == task.ID {
					idx = i
					break
				}
			}
			if idx >= 0 && idx < len(siblings)-1 {
				m.project.SwapTasks(task, siblings[idx+1])
				m.setUndoContext("Move Down", task.Title)
				m.rebuildRows()
				// Advance cursor to follow the task
				if m.cursor < len(m.rows)-1 {
					m.cursor++
				}
				return m, m.saveWithWorkflow()
			}
		}
		return m, nil
	case "K":
		// Move task up (swap with sibling above)
		if len(m.rows) > 0 && m.cursor < len(m.rows) {
			task := m.rows[m.cursor].task
			siblings := m.getSiblings(task)
			idx := -1
			for i, s := range siblings {
				if s.ID == task.ID {
					idx = i
					break
				}
			}
			if idx > 0 {
				m.project.SwapTasks(task, siblings[idx-1])
				m.setUndoContext("Move Up", task.Title)
				m.rebuildRows()
				if m.cursor > 0 {
					m.cursor--
				}
				return m, m.saveWithWorkflow()
			}
		}
		return m, nil
	case "right":
		// Indent task (nest under task above in the flat list)
		if len(m.rows) > 1 && m.cursor > 0 && m.cursor < len(m.rows) {
			task := m.rows[m.cursor].task
			candidate := m.rows[m.cursor-1].task
			for candidate != nil {
				if ok, _ := task.CanIndent(candidate); ok {
					task.ParentID = &candidate.ID
					m.project.BuildTaskTree()
					m.setUndoContext("Indent", task.Title)
					m.validateZoomStack()
					m.rebuildRows()
					return m, m.saveWithWorkflow()
				}
				candidate = candidate.Parent
			}
		}
		return m, nil
	case "left":
		// Outdent task (to parent level)
		if len(m.rows) > 0 && m.cursor < len(m.rows) {
			task := m.rows[m.cursor].task
			if ok, _ := task.CanOutdent(); ok {
				_ = m.project.OutdentTask(task)
				m.setUndoContext("Outdent", task.Title)
				m.validateZoomStack()
				m.rebuildRows()
				return m, m.saveWithWorkflow()
			}
		}
		return m, nil

	// Open task detail view
	case "o":
		if len(m.rows) > 0 && m.cursor < len(m.rows) {
			task := m.rows[m.cursor].task
			project := m.project
			return m, func() tea.Msg {
				return ShowTaskDetailMsg{Task: task, Project: project}
			}
		}
		return m, nil

	case "e":
		// Open full editor (with multi-select bulk edit support)
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			return m, func() tea.Msg {
				return ShowBulkEditMsg{
					Tasks:   m.getSelectedTasks(),
					Project: m.project,
				}
			}
		}
		if len(m.rows) > 0 && m.cursor < len(m.rows) {
			task := m.rows[m.cursor].task
			return m, func() tea.Msg {
				return EditTaskMsg{Task: task}
			}
		}
		return m, nil

	case "E":
		// Quick edit with full modifier parsing (same as O)
		if len(m.rows) > 0 && m.cursor < len(m.rows) {
			task := m.rows[m.cursor].task
			return m, m.quickEdit.Enter(task, false)
		}
		return m, nil

	// Delete with confirmation
	case "D":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			m.confirm.Show("bulk-delete", fmt.Sprintf("Delete %d selected tasks? (y/n)", len(m.selectedTasks)))
			m.statusMessage = m.confirm.Prompt()
			return m, nil
		}
		if len(m.rows) > 0 && m.cursor < len(m.rows) {
			task := m.rows[m.cursor].task
			if task.IsRecurring() {
				m.confirm.Show("delete-recurring",
					fmt.Sprintf("Delete recurring task #%d '%s'? (d)elete instance / (s)top recurrence / (n)o",
						task.ShortID, task.Title))
			} else {
				m.confirm.Show("delete", fmt.Sprintf("Delete task #%d? (y/n)", task.ShortID))
			}
			m.statusMessage = m.confirm.Prompt()
			m.deleteTarget = task
		}
		return m, nil

	// View switching
	case "ctrl+p":
		return m, func() tea.Msg {
			return ShowProjectTreeMsg{Project: m.project}
		}

	case "ctrl+k":
		return m, func() tea.Msg {
			return ShowKanbanMsg{Project: m.project}
		}

	case "ctrl+f":
		return m, func() tea.Msg {
			return ShowFilterMsg{CommonFilter: &m.filter, Project: m.project}
		}

	// Enter: zoom into parent tasks or open detail for leaf tasks
	case "enter":
		if len(m.rows) == 0 {
			return m, nil
		}
		row := m.rows[m.cursor]

		// Handle section header toggle
		if row.IsSectionHeader {
			m.collapsedSections[row.SectionName] = !m.collapsedSections[row.SectionName]
			m.rebuildRows()
			return m, nil
		}

		// Handle regular task rows
		task := row.task
		if len(task.Children) > 0 {
			m.zoomStack = append(m.zoomStack, ZoomEntry{
				TaskID:      task.ID,
				CursorIndex: m.cursor,
			})
			m.cursor = 0
			return m, nil
		}
		return m, func() tea.Msg {
			return ShowTaskDetailMsg{Task: task, Project: m.project}
		}

	// Backspace: zoom out one level (no-op at root)
	case "backspace":
		if len(m.zoomStack) > 0 {
			top := m.zoomStack[len(m.zoomStack)-1]
			m.zoomStack = m.zoomStack[:len(m.zoomStack)-1]
			m.cursor = top.CursorIndex
		}
		return m, nil

	// Multi-select mode
	case "v":
		m.toggleMultiSelectMode()
		return m, nil

	// Clipboard operations
	case "y", "Y", "ctrl+y":
		if handled, cmd := handleClipboardKey(key,
			func() *models.Task {
				if len(m.rows) > 0 && m.cursor < len(m.rows) {
					return m.rows[m.cursor].task
				}
				return nil
			},
			func() []*models.Task {
				if m.multiSelectMode && len(m.selectedTasks) > 0 {
					return m.getSelectedTasks()
				}
				return nil
			},
			m.clearMultiSelect,
		); handled {
			return m, cmd
		}
		return m, nil

	// Cut: copy title(s) to clipboard, then initiate delete
	case "x":
		return m.handleCutKey()

	// Score editor
	case "S":
		if len(m.rows) > 0 && m.cursor < len(m.rows) {
			task := m.rows[m.cursor].task
			return m, func() tea.Msg {
				return ShowScoreEditorMsg{Task: task, Project: m.project}
			}
		}
		return m, nil

	// Filter keys
	case "T":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			tasks := m.getSelectedTasks()
			return m, func() tea.Msg {
				return ShowBulkTagSelectorMsg{Tasks: tasks, Project: m.project}
			}
		}
		tags := BuildTagList(m.project.Tasks, false)
		return m, func() tea.Msg {
			return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: ViewOutliner, Title: "Filter by Tags"}
		}

	case "N":
		return m, func() tea.Msg {
			return ShowSectionFilterSelectorMsg{Project: m.project}
		}

	case "C":
		return m, func() tea.Msg {
			return ShowContextPickerMsg{}
		}

	// Additional views
	case "f":
		return m, func() tea.Msg {
			return ShowProjectFocusViewMsg{Project: m.project}
		}

	case "A":
		// Archive completed tasks when in Completed perspective (p8)
		if m.activePerspective == 8 {
			completedCount := m.project.CountCompletedTasks()
			if completedCount > 0 {
				m.confirm.Show("archive-tasks",
					fmt.Sprintf("Archive %d completed task(s) to completed.md? (y/n)", completedCount))
				m.statusMessage = m.confirm.Prompt()
			}
			return m, nil
		}
		// Otherwise open all tasks view
		return m, func() tea.Msg {
			return ShowAllTasksMsg{}
		}

	// Search
	case "/":
		return m, m.ActivateSearch()

	// Help
	case "?":
		return m, func() tea.Msg {
			return ShowHelpMsg{
				SourceView: ViewOutliner,
				ViewName:   "Outliner",
				Items:      m.GetHelpItems(),
			}
		}

	// Type change keys
	case "<", ">", "H", "L":
		op, undoAction, ok := typeChangeOpForKey(key)
		if !ok {
			return m, nil
		}
		if len(m.rows) == 0 || m.cursor >= len(m.rows) {
			return m, nil
		}
		task := m.rows[m.cursor].task
		m.setUndoContext(undoAction, task.Title)
		result := applyTypeChange(m.workflowPersistFn(), m.project, task.ID, op)
		if result.Err != nil {
			return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }
		}
		if result.StatusMessage != "" {
			m.rebuildRows()
		}
		return m, result.Cmd

	// Toggle active flag (with multi-select support)
	case "*":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			count := len(m.selectedTasks)
			for _, task := range m.getSelectedTasks() {
				task.ToggleActive()
			}
			m.setUndoContext("Toggle Active", fmt.Sprintf("%d tasks", count))
			m.clearMultiSelect()
			m.rebuildRows()
			return m, m.saveWithWorkflow()
		}
		if len(m.rows) > 0 && m.cursor < len(m.rows) {
			task := m.rows[m.cursor].task
			m.setUndoContext("Toggle Active", task.Title)
			result := applyToggleActive(m.workflowPersistFn(), m.project, task.ID)
			if result.Err != nil {
				return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }
			}
			m.rebuildRows()
			return m, result.Cmd
		}
		return m, nil

	// Task jumper
	case "\\":
		return m, func() tea.Msg {
			return ShowTaskJumperMsg{Project: m.project}
		}

	// Quick capture
	case "i":
		return m, func() tea.Msg {
			return ShowQuickCaptureMsg{Project: m.project, AllProjects: m.config.Projects}
		}

	// Bulk capture
	case "I":
		return m, func() tea.Msg {
			return ShowBulkCaptureMsg{Project: m.project}
		}

	// Refresh from disk
	case "ctrl+r":
		_ = storage.LoadProject(m.project)
		m.rebuildRows()
		return m, nil

	// Priority view
	case "P":
		return m, func() tea.Msg {
			return ShowPriorityViewMsg{Project: m.project}
		}

	// Project blockers
	case "B":
		return m, func() tea.Msg {
			return ShowBlockersMsg{Project: m.project}
		}

	// Inbox processor
	case "w":
		return m, func() tea.Msg {
			return ShowInboxProcessorMsg{Project: m.project}
		}

	// Custom section tabs
	case "[":
		tabs := m.customSectionTabs()
		if tabs != nil {
			if m.customSectionTab > 0 {
				m.customSectionTab--
			} else {
				m.customSectionTab = len(tabs) - 1
			}
			m.rebuildRows()
		}
		return m, nil
	case "]":
		tabs := m.customSectionTabs()
		if tabs != nil {
			if m.customSectionTab < len(tabs)-1 {
				m.customSectionTab++
			} else {
				m.customSectionTab = 0
			}
			m.rebuildRows()
		}
		return m, nil

	// Notes view
	case "{":
		return m, func() tea.Msg {
			return ShowNotesMsg{Project: m.project}
		}

	// Create note
	case "}":
		return m, func() tea.Msg {
			return ShowNoteCreatorMsg{Project: m.project}
		}

	// Archive/unarchive project
	case "ctrl+a":
		if m.project.Sandbox {
			return m, func() tea.Msg {
				return ErrorMsg{Err: fmt.Errorf("cannot archive the sandbox project")}
			}
		}
		return m, m.archiveProject()

	// Edit last modified task
	case "ctrl+l":
		recentTasks := storage.GetRecentTasks(m.project.Tasks, 1)
		if len(recentTasks) > 0 {
			return m, func() tea.Msg {
				return EditTaskMsg{Task: recentTasks[0]}
			}
		}
		return m, nil

	// Undo last change
	case "u":
		return m, m.performUndo()

	// Undo history
	case "ctrl+z":
		return m, func() tea.Msg {
			return ShowUndoHistoryMsg{Project: m.project}
		}

	// Tag manager
	case "ctrl+t":
		return m, func() tea.Msg {
			return ShowTagManagerMsg{Project: m.project}
		}

	// Section manager
	case "ctrl+s":
		return m, func() tea.Msg {
			return ShowSectionManagerMsg{ProjectTree: nil, Project: m.project}
		}

	// Project cycling
	case "`":
		return m, func() tea.Msg { return CycleProjectMsg{Forward: true} }
	case "~":
		return m, func() tea.Msg { return CycleProjectMsg{Forward: false} }
	case "@":
		return m, func() tea.Msg { return ShowProjectCycleManagerMsg{} }

	// Sort keys: bare 1-5, 0
	case "1", "2", "3", "4", "5", "0":
		if field, isReset, ok := SortFieldForKey(key); ok {
			if isReset {
				m.sortField = SortByDefault
				m.sortAscending = true
				m.rebuildRows()
			} else {
				m.toggleSort(field)
			}
		}
		return m, nil
	}

	return m, nil
}

// handleQuickEditMode handles input while in quick edit mode.
func (m *OutlinerModel) handleQuickEditMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.quickEdit.Exit()
		return m, nil

	case "enter":
		input := strings.TrimSpace(m.quickEdit.GetInput().Value())
		if input == "" {
			m.quickEdit.Exit()
			return m, nil
		}

		// Re-lookup task to avoid stale pointer after concurrent reload
		task := m.project.GetTaskByID(m.quickEdit.GetTask().ID)
		if task == nil {
			m.quickEdit.Exit()
			return m, nil
		}

		if m.quickEdit.IsRenameOnly() {
			// Rename-only mode: just update title
			m.project.MutateTask(task.ID, func(t *models.Task) {
				t.Title = input
			})
		} else {
			// Full quick edit mode: parse modifiers
			result, err := parseQuickEdit(input, task, m.project)
			if err != nil {
				return m, nil
			}
			applyQuickEdit(task, m.project, result)
		}

		m.quickEdit.Exit()
		return m, m.saveWithWorkflow()

	default:
		var cmd tea.Cmd
		inputModel := m.quickEdit.GetInput()
		inputModel, cmd = inputModel.Update(msg)
		m.quickEdit.UpdateInput(inputModel)

		// Update preview (only in full edit mode, not rename mode)
		if !m.quickEdit.IsRenameOnly() {
			inputValue := m.quickEdit.GetInput().Value()
			preview, _ := parseQuickEditForPreview(inputValue, m.quickEdit.GetTask(), m.project)
			m.quickEdit.SetPreview(preview)
		}

		return m, cmd
	}
}

// saveProject persists the project to storage and rebuilds the row list.
func (m *OutlinerModel) saveProject() tea.Cmd {
	return func() tea.Msg {
		ps := NewProjectService()
		if err := ps.Save(m.project); err != nil {
			return ErrorMsg{Err: err}
		}
		return nil
	}
}

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *OutlinerModel) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
	prefix := m.pendingPrefix
	m.pendingPrefix = "" // Always clear prefix

	if key == "esc" {
		return m, nil // Cancel prefix mode
	}

	// Handle filter sub-prefixes (Ft, Fs, Fd, Fa, Fas-Fap, FS, Fv)
	switch prefix {
	case "Ft", "Fs", "Fd", "Fa", "Fas", "Far", "Fae", "Fac", "Fav", "Faw", "Fao", "Fap", "FS":
		if prefix == "FS" && key == "C" {
			var custom *float64
			if m.config != nil {
				custom = m.config.ScoreFilterCustom
			}
			m.scoreFilterInput.Enter(custom)
			return m, nil
		}
		if dispatchFilterSubPrefix(prefix, key, &m.filter, m.config, func(p string) { m.pendingPrefix = p }) {
			m.rebuildRows()
		}
		return m, nil
	case "Fv":
		return m.handleSavedViewPrefix(key)
	}

	// Get cursor task (may be nil if cursor is out of bounds)
	var cursorTask *models.Task
	if len(m.rows) > 0 && m.cursor < len(m.rows) {
		cursorTask = m.rows[m.cursor].task
	}

	switch prefix {
	case ",":
		if status, ok := StatusForPrefixKey(key); ok {
			if m.multiSelectMode && len(m.selectedTasks) > 0 {
				return m.applyBulkStatusChange(status)
			}
			if cursorTask == nil {
				return m, nil
			}
			pf := m.workflowPersistFn()
			result := applyStatusChange(pf, m.project, cursorTask.ID, status)
			if result.Err != nil {
				return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }
			}
			m.statusMessage = result.StatusMessage
			m.rebuildRows()
			return m, result.Cmd
		}

	case "t":
		// Try shared message-only handlers first (to, tc, ta, tl, td, tm)
		if cursorTask != nil {
			if handled, cmd := handleTaskPrefixMsgKey(key, cursorTask, m.project); handled {
				return m, cmd
			}
		}

		// Try shared action handlers (tn, tN, ts, tt)
		var focusedTask *models.Task
		if len(m.zoomStack) > 0 {
			topID := m.zoomStack[len(m.zoomStack)-1].TaskID
			focusedTask = m.project.GetTaskByID(topID)
		}
		getMultiTasks := func() []*models.Task {
			if m.multiSelectMode && len(m.selectedTasks) > 0 {
				return m.getSelectedTasks()
			}
			return nil
		}
		if handled, cmd := handleTaskActionPrefixKey(key, cursorTask, m.project, focusedTask, getMultiTasks); handled {
			return m, cmd
		}

		// Rename task (tr)
		if key == "r" && cursorTask != nil {
			return m, m.quickEdit.Enter(cursorTask, true)
		}

		// Toggle pinned (tp)
		if key == "p" && cursorTask != nil {
			m.setUndoContext("Toggle Pin", cursorTask.Title)
			result := applyTogglePinned(m.workflowPersistFn(), m.project, cursorTask.ID)
			if result.Err != nil {
				return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }
			}
			m.rebuildRows()
			return m, result.Cmd
		}

	case "p":
		// Perspective switching (p-prefix: p1-p9)
		if key >= "1" && key <= "9" {
			perspKey := int(key[0] - '0')
			m.activePerspective = perspKey
			m.rebuildRows()
			return m, nil
		}

	case "!":
		if priority, ok := PriorityForPrefixKey(key); ok {
			if m.multiSelectMode && len(m.selectedTasks) > 0 {
				tasks := m.getSelectedTasks()
				count := applyBulkPriority(m.project, tasks, priority)
				m.setUndoContext("Set Priority", fmt.Sprintf("P%d on %d tasks", priority, count))
				m.clearMultiSelect()
				m.rebuildRows()
				return m, m.saveWithWorkflow()
			}
			if cursorTask == nil {
				return m, nil
			}
			m.setUndoContext("Set Priority", fmt.Sprintf("P%d: %s", priority, cursorTask.Title))
			result := applyPriorityChange(m.workflowPersistFn(), m.project, cursorTask.ID, priority)
			if result.Err != nil {
				return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }
			}
			m.rebuildRows()
			return m, result.Cmd
		}

	case "d":
		if days, isClear, ok := DueDateDaysForPrefixKey(key); ok {
			if m.multiSelectMode && len(m.selectedTasks) > 0 {
				tasks := m.getSelectedTasks()
				dueDate, desc := dueDateFromDays(days, isClear)
				count := applyBulkDueDate(m.project, tasks, dueDate)
				m.setUndoContext("Set Due Date", fmt.Sprintf("%s on %d tasks", desc, count))
				m.clearMultiSelect()
				m.rebuildRows()
				return m, m.saveWithWorkflow()
			}
			if cursorTask == nil {
				return m, nil
			}
			var result TaskOpResult
			if isClear {
				m.setUndoContext("Set Due Date", fmt.Sprintf("cleared: %s", cursorTask.Title))
				result = applyClearDueDate(m.workflowPersistFn(), m.project, cursorTask.ID)
			} else {
				m.setUndoContext("Set Due Date", fmt.Sprintf("%s: %s", dueDateLabel(days), cursorTask.Title))
				result = applyQuickDueDate(m.workflowPersistFn(), m.project, cursorTask.ID, days)
			}
			if result.Err != nil {
				return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }
			}
			m.rebuildRows()
			return m, result.Cmd
		}
		if key == "d" {
			if cursorTask != nil {
				task := cursorTask
				project := m.project
				return m, func() tea.Msg {
					return SetDueDateMsg{Task: task, Project: project}
				}
			}
			return m, nil
		}

	case "s":
		// Snooze quick actions
		now := time.Now()
		today := util.StartOfDay(now)
		var snoozeDate *time.Time
		var desc string

		switch key {
		case "t":
			d := today.AddDate(0, 0, 1)
			snoozeDate = &d
			desc = "tomorrow"
		case "w":
			d := today.AddDate(0, 0, 7)
			snoozeDate = &d
			desc = "+1 week"
		case "m":
			d := today.AddDate(0, 1, 0)
			snoozeDate = &d
			desc = "+1 month"
		case "n":
			daysUntilMon := (8 - int(today.Weekday())) % 7
			if daysUntilMon == 0 {
				daysUntilMon = 7
			}
			d := today.AddDate(0, 0, daysUntilMon)
			snoozeDate = &d
			desc = "next Monday"
		case "0":
			snoozeDate = nil
			desc = "cleared"
		case "d":
			if cursorTask != nil {
				task := cursorTask
				project := m.project
				return m, func() tea.Msg {
					return SetSnoozeDateMsg{Task: task, Project: project}
				}
			}
			return m, nil
		default:
			return m, nil
		}

		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			count := 0
			for id := range m.selectedTasks {
				if m.project.MutateTask(id, func(t *models.Task) {
					t.SnoozedUntil = snoozeDate
				}) {
					count++
				}
			}
			m.setUndoContext("Snooze", fmt.Sprintf("%s on %d tasks", desc, count))
			m.clearMultiSelect()
			m.rebuildRows()
			return m, m.saveWithWorkflow()
		}
		if cursorTask != nil {
			m.project.MutateTask(cursorTask.ID, func(t *models.Task) {
				t.SnoozedUntil = snoozeDate
			})
			m.setUndoContext("Snooze", fmt.Sprintf("%s: %s", desc, cursorTask.Title))
			m.rebuildRows()
			return m, m.saveWithWorkflow()
		}
		return m, nil

	case "z":
		// Fold operations
		switch key {
		case "a":
			// Toggle fold
			if cursorTask != nil && len(cursorTask.Children) > 0 {
				m.ensureExpandedNodes()
				if m.isExpanded(cursorTask.ID) {
					m.expandedNodes[cursorTask.ID] = false
				} else {
					m.expandedNodes[cursorTask.ID] = true
				}
				m.rebuildRows()
			}
			return m, nil
		case "o":
			// Expand
			if cursorTask != nil && len(cursorTask.Children) > 0 {
				m.ensureExpandedNodes()
				m.expandedNodes[cursorTask.ID] = true
				m.rebuildRows()
			}
			return m, nil
		case "c":
			// Collapse
			if cursorTask != nil && len(cursorTask.Children) > 0 {
				m.ensureExpandedNodes()
				m.expandedNodes[cursorTask.ID] = false
				m.rebuildRows()
			}
			return m, nil
		case "R":
			// Expand all
			m.expandedNodes = make(map[string]bool)
			m.rebuildRows()
			return m, nil
		case "M":
			// Collapse all - mark all parent tasks as collapsed
			m.ensureExpandedNodes()
			m.collapseAllParents(m.project.GetRootTasks())
			m.rebuildRows()
			return m, nil
		case "v":
			// Zoom into current task (reuse existing zoom)
			if cursorTask != nil && len(cursorTask.Children) > 0 {
				m.zoomStack = append(m.zoomStack, ZoomEntry{
					TaskID:      cursorTask.ID,
					CursorIndex: m.cursor,
				})
				m.cursor = 0
			}
			return m, nil
		}

	case "F":
		// Filter prefix
		return m.handleFilterPrefix(key)

	case "m":
		// Move to section
		if section, ok := sectionForMoveKey(key); ok {
			if m.multiSelectMode && len(m.selectedTasks) > 0 {
				count := 0
				for id := range m.selectedTasks {
					if m.project.MutateTask(id, func(t *models.Task) {
						t.Section = section
					}) {
						m.project.RelocateTaskToEndOfSection(id, section)
						count++
					}
				}
				label := "Move to " + section
				if section == "" {
					label = "Move to Active"
				}
				m.setUndoContext(label, fmt.Sprintf("%d tasks", count))
				m.clearMultiSelect()
				m.rebuildRows()
				return m, m.saveWithWorkflow()
			}
			if cursorTask == nil {
				return m, nil
			}
			m.project.MutateTask(cursorTask.ID, func(t *models.Task) {
				t.Section = section
			})
			m.project.RelocateTaskToEndOfSection(cursorTask.ID, section)
			label := "Move to " + section
			if section == "" {
				label = "Move to Active"
			}
			m.setUndoContext(label, cursorTask.Title)
			m.rebuildRows()
			return m, m.saveWithWorkflow()
		}
		// Move to top/bottom
		if key == "K" || key == "J" {
			if cursorTask == nil {
				return m, nil
			}
			return m.moveTaskToPosition(cursorTask, key == "K")
		}

	case "b":
		if cursorTask == nil {
			return m, nil
		}
		if handled, cmd := handleBlockerPrefixKey(key, cursorTask, m.project); handled {
			return m, cmd
		}

	case "c":
		if cursorTask == nil {
			return m, nil
		}
		if handled, cmd := handleLinkedTaskPrefixKey(key, cursorTask, m.project, true, true); handled {
			return m, cmd
		}

	case "g":
		switch key {
		case "g":
			m.cursor = 0
			return m, nil
		case "h":
			if cursorTask != nil {
				task := cursorTask
				project := m.project
				return m, func() tea.Msg {
					return OpenTaskActivityMsg{Task: task, Project: project}
				}
			}
			return m, nil
		case "X":
			// Deselect all visible in multi-select mode
			if m.multiSelectMode {
				m.deselectAllVisible()
			}
			return m, nil
		case "d":
			// Cycle due date filter
			m.cycleDueDateFilter()
			m.rebuildRows()
			return m, nil
		}
	}

	// Invalid prefix sequence - silently ignore
	return m, nil
}

// handleFilterPrefix handles the second key after pressing F (filter prefix).
func (m *OutlinerModel) handleFilterPrefix(key string) (tea.Model, tea.Cmd) {
	// View-specific filter keys
	switch key {
	case "v":
		m.pendingPrefix = "Fv"
		return m, nil
	case "d":
		m.pendingPrefix = "Fd"
		return m, nil
	case "c":
		if m.activePerspective == 8 {
			m.activePerspective = 0
		} else {
			m.activePerspective = 8
		}
		m.rebuildRows()
		return m, nil
	case "@":
		return m, func() tea.Msg {
			return ShowSectionFilterSelectorMsg{Project: m.project}
		}
	case "r":
		m.actionableFilter = !m.actionableFilter
		m.rebuildRows()
		return m, nil
	case "x":
		clearCommonFilters(&m.filter)
		m.actionableFilter = false
		m.ClearSearch()
		m.rebuildRows()
		return m, nil
	}

	// Common filter prefix keys
	result := handleCommonFilterPrefix(key, &m.filter, func() tea.Cmd {
		tags := BuildTagList(m.project.Tasks, false)
		return func() tea.Msg {
			return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: ViewOutliner, Title: "Filter by Tags"}
		}
	})
	if cmd := applyCommonFilterResult(result, func(p string) { m.pendingPrefix = p }, func() { m.rebuildRows() }); cmd != nil {
		return m, cmd
	}
	if result.Changed {
		return m, nil
	}
	return m, nil
}

// handleSavedViewPrefix handles the third key after pressing Fv (saved views sub-prefix).
func (m *OutlinerModel) handleSavedViewPrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "s":
		return m, func() tea.Msg {
			return ShowSaveViewDialogMsg{}
		}
	case "l":
		return m, func() tea.Msg {
			return ShowCommandPaletteMsg{PreFilter: ":sv "}
		}
	case "d":
		return m, func() tea.Msg {
			return ShowCommandPaletteMsg{PreFilter: ":sv "}
		}
	}
	return m, nil
}

// cycleDueDateFilter cycles through due date filter modes.
func (m *OutlinerModel) cycleDueDateFilter() {
	switch m.filter.DueDate {
	case DueDateAll:
		m.filter.DueDate = DueDateOverdue
	case DueDateOverdue:
		m.filter.DueDate = DueDateToday
	case DueDateToday:
		m.filter.DueDate = DueDateThisWeek
	case DueDateThisWeek:
		m.filter.DueDate = DueDateNone
	default:
		m.filter.DueDate = DueDateAll
	}
}

// ensureExpandedNodes initializes the expanded nodes map if it's empty,
// marking all parent tasks as expanded.
func (m *OutlinerModel) ensureExpandedNodes() {
	if len(m.expandedNodes) == 0 {
		m.expandedNodes = make(map[string]bool)
		m.markAllExpanded(m.project.GetRootTasks())
	}
}

// markAllExpanded marks all parent tasks as expanded recursively.
func (m *OutlinerModel) markAllExpanded(tasks []*models.Task) {
	for _, task := range tasks {
		if len(task.Children) > 0 {
			m.expandedNodes[task.ID] = true
			m.markAllExpanded(task.Children)
		}
	}
}

// collapseAllParents marks all parent tasks as collapsed recursively.
func (m *OutlinerModel) collapseAllParents(tasks []*models.Task) {
	for _, task := range tasks {
		if len(task.Children) > 0 {
			m.expandedNodes[task.ID] = false
			m.collapseAllParents(task.Children)
		}
	}
}

// moveTaskToPosition moves the cursor task to the top or bottom of its siblings.
func (m *OutlinerModel) moveTaskToPosition(task *models.Task, toTop bool) (tea.Model, tea.Cmd) {
	siblings := m.getSiblings(task)
	idx := -1
	for i, s := range siblings {
		if s.ID == task.ID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return m, nil
	}

	if toTop && idx == 0 {
		return m, nil
	}
	if !toTop && idx == len(siblings)-1 {
		return m, nil
	}

	if toTop {
		for i := idx - 1; i >= 0; i-- {
			m.project.SwapTasks(task, siblings[i])
		}
	} else {
		for i := idx + 1; i < len(siblings); i++ {
			m.project.SwapTasks(task, siblings[i])
		}
	}

	label := "Move to Top"
	if !toTop {
		label = "Move to Bottom"
	}
	m.setUndoContext(label, task.Title)
	m.rebuildRows()
	return m, m.saveWithWorkflow()
}

// getSiblings returns the sibling list for a task at the current zoom level.
func (m *OutlinerModel) getSiblings(task *models.Task) []*models.Task {
	if task.Parent != nil {
		return task.Parent.Children
	}
	// Root level or zoomed level
	if len(m.zoomStack) > 0 {
		topID := m.zoomStack[len(m.zoomStack)-1].TaskID
		if t := m.project.GetTaskByID(topID); t != nil {
			return t.Children
		}
	}
	return m.project.GetRootTasks()
}

// handleConfirmDialog processes key input while a confirmation dialog is active.
func (m *OutlinerModel) handleConfirmDialog(key string) (tea.Model, tea.Cmd) {
	// Capture action before HandleKey clears it on confirm/cancel.
	action := m.confirm.Action()

	// Handle recurring delete separately (d/s/n instead of y/n)
	if action == "delete-recurring" && m.deleteTarget != nil {
		switch key {
		case "d", "D":
			m.confirm.Cancel()
			m.statusMessage = ""
			return m.executeDelete()
		case "s", "S":
			m.deleteTarget.ClearRecurrence()
			m.confirm.Cancel()
			m.statusMessage = ""
			return m.executeDelete()
		case "n", "N", "esc":
			m.confirm.Cancel()
			m.statusMessage = ""
			m.deleteTarget = nil
			return m, nil
		}
		return m, nil
	}

	// Standard y/n handling
	confirmed, handled := m.confirm.HandleKey(key)
	if handled {
		if confirmed {
			switch action {
			case "delete":
				return m.executeDelete()
			case "bulk-delete":
				return m.executeBulkDelete()
			case "archive-tasks":
				return m.archiveCompletedTasks()
			}
		}
		m.statusMessage = ""
		m.deleteTarget = nil
		return m, nil
	}
	return m, nil
}

// applyBulkStatusChange applies a status transition to all multi-selected tasks.
func (m *OutlinerModel) applyBulkStatusChange(newStatus models.TaskStatus) (tea.Model, tea.Cmd) {
	tasks := m.getSelectedTasks()
	if len(tasks) == 0 {
		return m, nil
	}

	valid, _ := analyzeBulkStatusTransition(m.project, tasks, newStatus)
	if len(valid) == 0 {
		m.statusMessage = "No selected tasks can transition to " + string(newStatus)
		m.clearMultiSelect()
		return m, nil
	}

	pf := m.workflowPersistFn()
	for _, task := range valid {
		pf(m.project, task.ID, func(t *models.Task) {
			t.ApplyTransition(newStatus)
		})
	}

	m.statusMessage = fmt.Sprintf("Changed %d tasks to %s", len(valid), newStatus)
	m.clearMultiSelect()
	m.rebuildRows()
	return m, nil
}

// executeDelete removes the current deleteTarget task using shared helpers.
func (m *OutlinerModel) executeDelete() (tea.Model, tea.Cmd) {
	if m.deleteTarget == nil {
		return m, nil
	}
	task := m.deleteTarget
	m.deleteTarget = nil
	m.statusMessage = ""

	pf := m.workflowPersistFn()
	result := applyDeleteTask(pf, m.project, task.ID, m.config.DataDir)
	if result.Err != nil {
		m.err = result.Err
		return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }
	}
	m.rebuildRows()
	return m, result.Cmd
}

// executeBulkDelete removes all multi-selected tasks using shared helpers.
func (m *OutlinerModel) executeBulkDelete() (tea.Model, tea.Cmd) {
	if len(m.selectedTasks) == 0 {
		return m, nil
	}

	count := len(m.selectedTasks)
	pf := m.workflowPersistFn()
	var lastCmd tea.Cmd
	for taskID := range m.selectedTasks {
		result := applyDeleteTask(pf, m.project, taskID, m.config.DataDir)
		if result.Err != nil {
			m.err = result.Err
			return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }
		}
		lastCmd = result.Cmd
	}

	m.clearMultiSelect()
	m.rebuildRows()
	m.statusMessage = fmt.Sprintf("Deleted %d tasks", count)
	return m, lastCmd
}

// archiveCompletedTasks moves all completed tasks to the archive file.
func (m *OutlinerModel) archiveCompletedTasks() (tea.Model, tea.Cmd) {
	undoDepth := 0
	if m.config.UndoHistoryDepth != nil {
		undoDepth = *m.config.UndoHistoryDepth
	}
	completedCount := m.project.CountCompletedTasks()

	svc := NewProjectService()
	cmd := func() tea.Msg {
		if completedCount == 0 {
			return nil
		}
		if undoDepth > 0 {
			_ = svc.RecordUndoSnapshot(m.project, "Archive Tasks",
				fmt.Sprintf("%d tasks", completedCount), undoDepth)
		}
		if err := svc.Backup(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}
		count, err := svc.ArchiveTasks(m.project)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to archive tasks: %w", err)}
		}
		_ = svc.Load(m.project)
		m.rebuildRows()
		return TasksArchivedMsg{Count: count}
	}
	m.statusMessage = ""
	return m, cmd
}

// handleCutKey copies the selected task(s) title to clipboard and initiates
// delete confirmation (cut = copy + delete).
func (m *OutlinerModel) handleCutKey() (tea.Model, tea.Cmd) {
	if m.multiSelectMode && len(m.selectedTasks) > 0 {
		tasks := m.getSelectedTasks()
		copyCmd := func() tea.Msg { return CopyTasksTitleMsg{Tasks: tasks} }
		m.confirm.Show("bulk-delete", fmt.Sprintf("Cut %d selected tasks? (y/n)", len(tasks)))
		m.statusMessage = m.confirm.Prompt()
		return m, copyCmd
	}
	if len(m.rows) > 0 && m.cursor < len(m.rows) {
		task := m.rows[m.cursor].task
		copyCmd := func() tea.Msg { return CopyTaskTitleMsg{Task: task} }
		m.confirm.Show("delete", fmt.Sprintf("Cut task #%d? (y/n)", task.ShortID))
		m.statusMessage = m.confirm.Prompt()
		m.deleteTarget = task
		return m, copyCmd
	}
	return m, nil
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *OutlinerModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints:  m.buildPrefixHints(),
	}
}

// OutlinerNavigationPrefixHints contains "g" prefix hints for the outliner view.
var OutlinerNavigationPrefixHints = []PrefixHint{
	{Key: "g", Description: "first", Detail: "Jump to first item"},
	{Key: "h", Description: "history", Detail: "Show task activity log"},
}

// OutlinerPerspectivePrefixHints provides the hint list for the p-prefix popup overlay.
var OutlinerPerspectivePrefixHints = []PrefixHint{
	{Key: "1", Description: "All"},
	{Key: "2", Description: "Active"},
	{Key: "3", Description: "Next"},
	{Key: "4", Description: "Inbox"},
	{Key: "5", Description: "Urgent"},
	{Key: "6", Description: "Waiting"},
	{Key: "7", Description: "Someday"},
	{Key: "8", Description: "Completed"},
	{Key: "9", Description: "Review"},
}

// buildPrefixHints returns the hints map for all supported prefix keys.
func (m *OutlinerModel) buildPrefixHints() PrefixHints {
	hints := PrefixHints{
		",": StatusPrefixHints,
		"t": TaskPrefixHints,
		"s": SnoozePrefixHints,
		"!": PriorityPrefixHints,
		"d": DueDatePrefixHints,
		"b": BlockerPrefixHints,
		"c": CreateLinkedPrefixHints,
		"g": OutlinerNavigationPrefixHints,
		"m": ProjectTreeMovePrefixHints,
		"z": KanbanFoldPrefixHints,
	}

	// Add perspective hints with active annotation
	if m.pendingPrefix == "p" {
		annotated := make([]PrefixHint, len(OutlinerPerspectivePrefixHints))
		copy(annotated, OutlinerPerspectivePrefixHints)
		idx := m.activePerspective
		if idx < 1 {
			idx = 1
		}
		for i := range annotated {
			if annotated[i].Key == fmt.Sprintf("%d", idx) {
				annotated[i].Active = true
			}
		}
		hints["p"] = annotated
	} else {
		hints["p"] = OutlinerPerspectivePrefixHints
	}

	// Add filter hints
	filterHints := buildCommonFilterPrefixPopupHints(&m.filter, m.config, true, true)
	for k, v := range filterHints {
		hints[k] = v
	}

	// Add view-specific filter hints to F prefix
	if fHints, ok := hints["F"]; ok {
		completedDetail := "off"
		if m.activePerspective == 8 {
			completedDetail = "on"
		}
		actionableDetail := "off"
		if m.actionableFilter {
			actionableDetail = "on"
		}
		fHints = append(fHints,
			PrefixHint{Key: "c", Description: "completed", Detail: completedDetail, Active: m.activePerspective == 8},
			PrefixHint{Key: "r", Description: "actionable", Detail: actionableDetail, Active: m.actionableFilter},
			PrefixHint{Key: "@", Description: "section", Detail: "selector"},
		)
		hints["F"] = fHints
	}

	// Annotate current task's status as active in status hints
	if m.pendingPrefix == "," && len(m.rows) > 0 && m.cursor < len(m.rows) {
		task := m.rows[m.cursor].task
		currentStatus := string(task.Status)
		annotated := make([]PrefixHint, len(StatusPrefixHints))
		copy(annotated, StatusPrefixHints)
		for i := range annotated {
			if statusForHint, ok := StatusForPrefixKey(annotated[i].Key); ok {
				if string(statusForHint) == currentStatus {
					annotated[i].Active = true
				}
			}
		}
		hints[","] = annotated
	}

	return hints
}

// GetHelpItems returns help items for the outliner view.
func (m *OutlinerModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewOutliner)
}

// ShouldCaptureEsc returns true only when a transient mode is active or the
// view is zoomed in, so that the global handleEscKey() can pop the view when
// at the root level with no active mode.
func (m *OutlinerModel) ShouldCaptureEsc() bool {
	return m.multiSelectMode || m.quickEdit.IsActive() || m.confirm.IsActive() || m.pendingPrefix != "" || len(m.zoomStack) > 0 || m.IsSearchMode() || m.SearchQuery() != "" || m.scoreFilterInput.IsActive() || m.sidebar.ShouldCaptureEsc()
}

// HasConfirmDialog returns true when a confirmation dialog is active.
func (m *OutlinerModel) HasConfirmDialog() bool {
	return m.confirm.IsActive()
}

// HasTextInput returns true when the outliner has an active text input field.
func (m *OutlinerModel) HasTextInput() bool {
	return m.quickEdit.IsActive() || m.IsSearchMode() || m.scoreFilterInput.IsActive() || m.sidebar.HasTextInput()
}
