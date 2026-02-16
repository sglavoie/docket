package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// handleKeyMsg processes key events in the outliner view.
func (m *OutlinerModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle quick edit mode input first
	if m.quickEdit.IsActive() {
		return m.handleQuickEditMode(msg)
	}

	// Handle confirm dialog before anything else
	if m.confirm.IsActive() {
		return m.handleConfirmDialog(key)
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

	// Prefix key activation
	switch key {
	case ",", "t", "s":
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
		return m, nil

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
	case "g":
		m.cursor = 0
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

	// Quick edit keys
	case "o":
		// Rename-only quick edit (no modifier parsing)
		if len(m.rows) > 0 && m.cursor < len(m.rows) {
			task := m.rows[m.cursor].task
			return m, m.quickEdit.Enter(task, true)
		}
		return m, nil

	case "O":
		// Full quick edit with modifier parsing
		if len(m.rows) > 0 && m.cursor < len(m.rows) {
			task := m.rows[m.cursor].task
			return m, m.quickEdit.Enter(task, false)
		}
		return m, nil

	case "e":
		// Open full editor
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
			m.confirm.Show("delete", fmt.Sprintf("Delete task #%d? (y/n)", task.ShortID))
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
			return ShowFocusBoardMsg{}
		}

	// Enter: zoom into parent tasks or open detail for leaf tasks
	case "enter":
		if len(m.rows) == 0 {
			return m, nil
		}
		task := m.rows[m.cursor].task
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

	// Clipboard operations (system clipboard: copy/cut only)
	// Note: p (paste) is not implemented — no internal clipboard buffer exists in any view.
	// The "clipboard" in this codebase copies text to the system clipboard only.
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

	// Help
	case "?":
		return m, func() tea.Msg {
			return ShowHelpMsg{
				SourceView: ViewOutliner,
				ViewName:   "Outliner",
				Items:      m.GetHelpItems(),
			}
		}

	// Perspective switching: keys 1-9
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		perspKey, _ := strconv.Atoi(key)
		m.activePerspective = perspKey
		m.rebuildRows()
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
		return m, m.saveProject()

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
			pf := simplePersist(NewProjectService())
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
		// Use zoomed task as parent when zoomed in, otherwise cursor task
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

	case "s":
		if field, ok := OutlinerSortFieldForKey(key); ok {
			m.toggleSort(field)
			return m, nil
		}
		if key == "0" {
			m.sortField = SortByDefault
			m.sortAscending = true
			m.rebuildRows()
			return m, nil
		}
	}

	// Invalid prefix sequence - silently ignore
	return m, nil
}

// handleConfirmDialog processes key input while a confirmation dialog is active.
func (m *OutlinerModel) handleConfirmDialog(key string) (tea.Model, tea.Cmd) {
	// Capture action before HandleKey clears it on confirm/cancel.
	action := m.confirm.Action()
	confirmed, handled := m.confirm.HandleKey(key)
	if handled {
		if confirmed {
			switch action {
			case "delete":
				return m.executeDelete()
			case "bulk-delete":
				return m.executeBulkDelete()
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

	pf := simplePersist(NewProjectService())
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

	pf := simplePersist(NewProjectService())
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
	pf := simplePersist(NewProjectService())
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

// buildPrefixHints returns the hints map for all supported prefix keys.
func (m *OutlinerModel) buildPrefixHints() PrefixHints {
	hints := PrefixHints{
		",": StatusPrefixHints,
		"t": TaskPrefixHints,
		"s": OutlinerSortPrefixHints,
	}

	// Annotate current sort field as active in sort hints
	if m.pendingPrefix == "s" && m.sortField != SortByDefault {
		annotated := make([]PrefixHint, len(OutlinerSortPrefixHints))
		copy(annotated, OutlinerSortPrefixHints)
		for i := range annotated {
			if field, ok := OutlinerSortFieldForKey(annotated[i].Key); ok && field == m.sortField {
				annotated[i].Active = true
			}
		}
		hints["s"] = annotated
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

// OutlinerSortFieldForKey maps s-prefix second keys to SortField values.
// Returns the field and true if the key matched, or SortByDefault and false otherwise.
// The mapping differs from SortFieldForKey (used by project tree bare numbers):
// s1=Priority, s2=Status, s3=DueDate, s4=Created, s5=Score.
func OutlinerSortFieldForKey(key string) (SortField, bool) {
	switch key {
	case "1":
		return SortByPriority, true
	case "2":
		return SortByStatus, true
	case "3":
		return SortByDueDate, true
	case "4":
		return SortByCreatedAt, true
	case "5":
		return SortByScore, true
	}
	return SortByDefault, false
}

// OutlinerSortPrefixHints provides the hint list for the s-prefix popup overlay.
var OutlinerSortPrefixHints = []PrefixHint{
	{Key: "1", Description: "Priority"},
	{Key: "2", Description: "Status"},
	{Key: "3", Description: "Due Date"},
	{Key: "4", Description: "Created"},
	{Key: "5", Description: "Score"},
	{Key: "0", Description: "Reset"},
}

// GetHelpItems returns help items for the outliner view.
func (m *OutlinerModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewOutliner)
}

// ShouldCaptureEsc returns true so the outliner handles q/Esc internally
// for zoom-level navigation, transient mode dismissal, and confirm dialogs.
func (m *OutlinerModel) ShouldCaptureEsc() bool {
	return true
}

// HasConfirmDialog returns true when a confirmation dialog is active.
func (m *OutlinerModel) HasConfirmDialog() bool {
	return m.confirm.IsActive()
}

// HasTextInput returns true when the outliner has an active text input field.
func (m *OutlinerModel) HasTextInput() bool {
	return m.quickEdit.IsActive()
}
