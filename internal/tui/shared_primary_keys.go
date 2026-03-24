package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// primaryProjectView is the interface that primary views (ProjectTree,
// Outliner) must implement to use the shared key dispatcher.
type primaryProjectView interface {
	getProject() *models.Project
	getCursorTask() *models.Task
	getConfig() *config.Config
	reloadFromDisk() tea.Cmd
	performUndo() tea.Cmd
}

// prefixableView is the interface that primary views must implement to use
// the shared prefix handlers for !, d, and s keys.
type prefixableView interface {
	getCursorTask() *models.Task
	getProject() *models.Project
	getMultiSelectedTasks() []*models.Task // nil = not in multi-select
	clearMultiSelect()
	refreshView()                                                  // buildTree / loadTasks / rebuildRows
	workflowPersistFn() persistFn                                  // for single-task operations
	setUndoContext(action, detail string)                          // sets undo label before persist
	saveAndRefreshBulk(prevStates map[string]*models.Task) tea.Cmd // bulk: persist + refresh
	setStatusMessage(msg string)                                   // sets the status bar text
}

// handlePriorityPrefix handles the ! prefix key for setting priority.
// Returns (cmd, handled). When handled is false the key was not
// recognized by PriorityForPrefixKey.
func handlePriorityPrefix(key string, v prefixableView) (tea.Cmd, bool) {
	priority, ok := PriorityForPrefixKey(key)
	if !ok {
		return nil, false
	}

	project := v.getProject()

	if tasks := v.getMultiSelectedTasks(); tasks != nil {
		prevStates := captureTaskStates(project.Tasks)
		count := applyBulkPriority(project, tasks, priority)
		v.clearMultiSelect()
		msg := fmt.Sprintf("Set priority %d on %d tasks", priority, count)
		v.setUndoContext("Set Priority", msg)
		v.setStatusMessage(msg)
		return v.saveAndRefreshBulk(prevStates), true
	}

	task := v.getCursorTask()
	if task == nil {
		return nil, true
	}
	v.setUndoContext("Set Priority", fmt.Sprintf("P%d: %s", priority, task.Title))
	result := applyPriorityChange(v.workflowPersistFn(), project, task.ID, priority)
	if result.Err != nil {
		return func() tea.Msg { return ErrorMsg{Err: result.Err} }, true
	}
	if result.StatusMessage != "" {
		v.setStatusMessage(result.StatusMessage)
	}
	v.refreshView()
	return result.Cmd, true
}

// handleStagePrefix handles the . prefix key for setting workflow stage.
// Returns (cmd, handled). When handled is false the key was not recognized.
func handleStagePrefix(key string, v prefixableView, stages []string) (tea.Cmd, bool) {
	stage := StageForPrefixKey(key, stages)
	if stage == nil {
		return nil, false
	}

	project := v.getProject()

	if tasks := v.getMultiSelectedTasks(); tasks != nil {
		prevStates := captureTaskStates(project.Tasks)
		count := 0
		for _, t := range tasks {
			if t.Stage != *stage {
				project.MutateTask(t.ID, func(mt *models.Task) {
					mt.Stage = *stage
				})
				count++
			}
		}
		v.clearMultiSelect()
		label := *stage
		if label == "" {
			label = "cleared"
		}
		msg := fmt.Sprintf("Set stage %s on %d tasks", label, count)
		v.setUndoContext("Set Stage", msg)
		v.setStatusMessage(msg)
		return v.saveAndRefreshBulk(prevStates), true
	}

	task := v.getCursorTask()
	if task == nil {
		return nil, true
	}
	label := *stage
	if label == "" {
		label = "(cleared)"
	}
	v.setUndoContext("Set Stage", fmt.Sprintf("%s: %s", label, task.Title))
	result := applyStageChange(v.workflowPersistFn(), project, task.ID, *stage)
	if result.Err != nil {
		return func() tea.Msg { return ErrorMsg{Err: result.Err} }, true
	}
	if result.StatusMessage != "" {
		v.setStatusMessage(result.StatusMessage)
	}
	v.refreshView()
	return result.Cmd, true
}

// handleDueDatePrefix handles the d prefix key for setting due dates.
// Returns (cmd, handled). When handled is false the key was not recognized.
func handleDueDatePrefix(key string, v prefixableView) (tea.Cmd, bool) {
	project := v.getProject()

	if days, isClear, ok := DueDateDaysForPrefixKey(key); ok {
		if tasks := v.getMultiSelectedTasks(); tasks != nil {
			dueDate, desc := dueDateFromDays(days, isClear)
			prevStates := captureTaskStates(project.Tasks)
			count := applyBulkDueDate(project, tasks, dueDate)
			v.clearMultiSelect()
			v.setUndoContext("Set Due Date", fmt.Sprintf("%s on %d tasks", desc, count))
			return v.saveAndRefreshBulk(prevStates), true
		}
		task := v.getCursorTask()
		if task == nil {
			return nil, true
		}
		var result TaskOpResult
		if isClear {
			v.setUndoContext("Set Due Date", fmt.Sprintf("cleared: %s", task.Title))
			result = applyClearDueDate(v.workflowPersistFn(), project, task.ID)
		} else {
			v.setUndoContext("Set Due Date", fmt.Sprintf("%s: %s", dueDateLabel(days), task.Title))
			result = applyQuickDueDate(v.workflowPersistFn(), project, task.ID, days)
		}
		if result.Err != nil {
			return func() tea.Msg { return ErrorMsg{Err: result.Err} }, true
		}
		if result.StatusMessage != "" {
			v.setStatusMessage(result.StatusMessage)
		}
		v.refreshView()
		return result.Cmd, true
	}

	if key == "d" {
		task := v.getCursorTask()
		if task == nil {
			return nil, true
		}
		return func() tea.Msg {
			return SetDueDateMsg{Task: task, Project: project}
		}, true
	}

	return nil, false
}

// handleSnoozePrefix handles the s prefix key for snoozing tasks.
// Returns (cmd, handled). When handled is false the key was not recognized.
func handleSnoozePrefix(key string, v prefixableView) (tea.Cmd, bool) {
	project := v.getProject()

	if key == "d" {
		task := v.getCursorTask()
		if task == nil {
			return nil, true
		}
		return func() tea.Msg {
			return SetSnoozeDateMsg{Task: task, Project: project}
		}, true
	}

	snoozeDate, desc, ok := resolveQuickSnoozeDate(key)
	if !ok {
		return nil, false
	}

	prevStates := captureTaskStates(project.Tasks)

	if tasks := v.getMultiSelectedTasks(); tasks != nil {
		count := 0
		for _, task := range tasks {
			if project.MutateTask(task.ID, func(t *models.Task) {
				t.SnoozedUntil = snoozeDate
			}) {
				count++
			}
		}
		v.clearMultiSelect()
		v.setUndoContext("Snooze", fmt.Sprintf("%s on %d tasks", desc, count))
		return v.saveAndRefreshBulk(prevStates), true
	}

	task := v.getCursorTask()
	if task == nil {
		return nil, true
	}
	project.MutateTask(task.ID, func(t *models.Task) {
		t.SnoozedUntil = snoozeDate
	})
	v.setUndoContext("Snooze", fmt.Sprintf("%s: %s", desc, task.Title))
	return v.saveAndRefreshBulk(prevStates), true
}

// handleSkipRecurrence handles the ,k key for skipping the next occurrence of a
// recurring task. Advances the due date to the next occurrence, increments the
// skip counter, and resets the streak. Returns (cmd, handled).
func handleSkipRecurrence(v prefixableView) (tea.Cmd, bool) {
	project := v.getProject()
	task := v.getCursorTask()
	if task == nil {
		return nil, true
	}
	if !task.IsRecurring() {
		v.setStatusMessage("Task is not recurring")
		return nil, true
	}

	prevStates := captureTaskStates(project.Tasks)
	project.MutateTask(task.ID, func(t *models.Task) {
		t.SkipRecurrence()
	})
	dueStr := "none"
	if task.DueDate != nil {
		dueStr = task.DueDate.Format("Mon Jan 2, 2006")
	}
	msg := fmt.Sprintf("Skipped: next due %s", dueStr)
	v.setUndoContext("Skip Recurrence", task.Title)
	v.setStatusMessage(msg)
	return v.saveAndRefreshBulk(prevStates), true
}

// resolveDeferDays maps a defer sub-key to a number of days and description.
// Returns (0, "", false) for unrecognized keys.
func resolveDeferDays(key string) (int, string, bool) {
	switch key {
	case "t":
		return 1, "1 day", true
	case "3":
		return 3, "3 days", true
	case "w":
		return 7, "1 week", true
	case "W":
		return 14, "2 weeks", true
	case "m":
		return 30, "1 month", true
	default:
		return 0, "", false
	}
}

// handleDeferRecurrence handles the ,D prefix for deferring a recurring task.
// Sub-keys: t=1d, 3=3d, w=1w, W=2w, m=1m. Returns (cmd, handled).
func handleDeferRecurrence(key string, v prefixableView) (tea.Cmd, bool) {
	days, desc, ok := resolveDeferDays(key)
	if !ok {
		return nil, false
	}

	project := v.getProject()
	task := v.getCursorTask()
	if task == nil {
		return nil, true
	}
	if !task.IsRecurring() {
		v.setStatusMessage("Task is not recurring")
		return nil, true
	}

	prevStates := captureTaskStates(project.Tasks)
	project.MutateTask(task.ID, func(t *models.Task) {
		t.DeferRecurrence(days)
	})
	dueStr := "none"
	if task.DueDate != nil {
		dueStr = task.DueDate.Format("Mon Jan 2, 2006")
	}
	msg := fmt.Sprintf("Deferred %s: now due %s", desc, dueStr)
	v.setUndoContext("Defer Recurrence", task.Title)
	v.setStatusMessage(msg)
	return v.saveAndRefreshBulk(prevStates), true
}

// handleToggleActiveKey handles the * key for toggling the Active flag.
// Returns (cmd, handled).
func handleToggleActiveKey(v prefixableView) (tea.Cmd, bool) {
	project := v.getProject()

	if tasks := v.getMultiSelectedTasks(); tasks != nil {
		prevStates := captureTaskStates(project.Tasks)
		count := 0
		for _, task := range tasks {
			task.ToggleActive()
			count++
		}
		v.clearMultiSelect()
		msg := fmt.Sprintf("Toggled active on %d tasks", count)
		v.setUndoContext("Toggle Active", msg)
		v.setStatusMessage(msg)
		return v.saveAndRefreshBulk(prevStates), true
	}

	task := v.getCursorTask()
	if task == nil {
		return nil, true
	}
	v.setUndoContext("Toggle Active", task.Title)
	result := applyToggleActive(v.workflowPersistFn(), project, task.ID)
	if result.Err != nil {
		return func() tea.Msg { return ErrorMsg{Err: result.Err} }, true
	}
	cmds := []tea.Cmd{result.Cmd}
	if result.StatusMessage != "" {
		v.setStatusMessage(result.StatusMessage)
	}
	v.refreshView()
	return tea.Batch(cmds...), true
}

// handleTogglePinnedKey handles the tp prefix key for toggling the Pinned flag.
// Returns (cmd, handled).
func handleTogglePinnedKey(v prefixableView) (tea.Cmd, bool) {
	project := v.getProject()

	if tasks := v.getMultiSelectedTasks(); tasks != nil {
		prevStates := captureTaskStates(project.Tasks)
		count := 0
		for _, task := range tasks {
			task.TogglePinned()
			count++
		}
		v.clearMultiSelect()
		msg := fmt.Sprintf("Toggled pin on %d tasks", count)
		v.setUndoContext("Toggle Pin", msg)
		v.setStatusMessage(msg)
		return v.saveAndRefreshBulk(prevStates), true
	}

	task := v.getCursorTask()
	if task == nil {
		return nil, true
	}
	v.setUndoContext("Toggle Pin", task.Title)
	result := applyTogglePinned(v.workflowPersistFn(), project, task.ID)
	if result.Err != nil {
		return func() tea.Msg { return ErrorMsg{Err: result.Err} }, true
	}
	cmds := []tea.Cmd{result.Cmd}
	if result.StatusMessage != "" {
		v.setStatusMessage(result.StatusMessage)
	}
	v.refreshView()
	return tea.Batch(cmds...), true
}

// handleTagKey handles the T key for tag filtering or bulk tag assignment.
// selectedTags is the current tag filter selection from the view's filter state.
// Returns (cmd, handled).
func handleTagKey(v prefixableView, source ViewType, includeCompleted bool, selectedTags map[string]bool) (tea.Cmd, bool) {
	project := v.getProject()

	if tasks := v.getMultiSelectedTasks(); tasks != nil {
		return func() tea.Msg {
			return ShowBulkTagSelectorMsg{Tasks: tasks, Project: project}
		}, true
	}

	tags := BuildTagList(project.Tasks, includeCompleted)
	return func() tea.Msg {
		return ShowTagFilterMsg{Tags: tags, Selected: selectedTags, Source: source, Title: "Filter by Tags"}
	}, true
}

// deleteConfirmView is the interface that views must implement to use the
// shared delete confirm handler.
type deleteConfirmView interface {
	getCursorTask() *models.Task
	getMultiSelectedTasks() []*models.Task
	showDeleteConfirm(task *models.Task)
	showBulkDeleteConfirm()
}

// handleDeleteConfirmKey handles the tD prefix key for showing delete
// confirmation dialogs. Returns true if the key was handled.
func handleDeleteConfirmKey(v deleteConfirmView) bool {
	if tasks := v.getMultiSelectedTasks(); tasks != nil {
		v.showBulkDeleteConfirm()
		return true
	}
	task := v.getCursorTask()
	if task != nil {
		v.showDeleteConfirm(task)
	}
	return true
}

// buildReparentValidParents returns tasks that are valid parents for reparenting task.
// Excludes the task itself, its descendants, and tasks that can't parent the task's type.
func buildReparentValidParents(project *models.Project, task *models.Task) []*models.Task {
	if project == nil || task == nil {
		return nil
	}
	var parents []*models.Task
	excluded := taskDescendantIDs(task)
	for _, t := range project.Tasks {
		if t.ID == task.ID || excluded[t.ID] {
			continue
		}
		if t.CanAddChild(task.Type) {
			parents = append(parents, t)
		}
	}
	return parents
}

// taskDescendantIDs returns the set of IDs of all descendants of task.
func taskDescendantIDs(task *models.Task) map[string]bool {
	ids := make(map[string]bool)
	for _, child := range task.Children {
		ids[child.ID] = true
		for id := range taskDescendantIDs(child) {
			ids[id] = true
		}
	}
	return ids
}

// handleCommonPrimaryViewKey handles keys shared across the three primary
// project views (ProjectTree, Outliner). It returns (cmd, true) when
// the key was consumed, or (nil, false) when the caller should continue with
// view-specific handling.
func handleCommonPrimaryViewKey(key string, v primaryProjectView) (tea.Cmd, bool) {
	project := v.getProject()

	switch key {
	// View switching
	case "ctrl+p":
		return func() tea.Msg { return ShowProjectTreeMsg{Project: project} }, true
	case "ctrl+o":
		return func() tea.Msg { return ShowOutlinerMsg{Project: project} }, true

	// Project cycling
	case "`":
		return func() tea.Msg { return CycleProjectMsg{Forward: true} }, true
	case "~":
		return func() tea.Msg { return CycleProjectMsg{Forward: false} }, true
	case "@":
		return func() tea.Msg { return ShowProjectCycleManagerMsg{} }, true

	// Secondary views
	case "B":
		if project == nil {
			return nil, true
		}
		return func() tea.Msg { return ShowBlockersMsg{Project: project} }, true
	case "p":
		if project == nil {
			return nil, true
		}
		return func() tea.Msg { return ShowPriorityViewMsg{Project: project} }, true
	case "w":
		if project == nil {
			return nil, true
		}
		return func() tea.Msg { return ShowInboxProcessorMsg{Project: project} }, true

	// Capture
	case "i":
		cfg := v.getConfig()
		return func() tea.Msg {
			return ShowQuickCaptureMsg{Project: project, AllProjects: cfg.Projects}
		}, true
	case "I":
		return func() tea.Msg { return ShowBulkCaptureMsg{Project: project} }, true

	// Navigation / tools
	case "\\":
		return func() tea.Msg { return ShowTaskJumperMsg{Project: project} }, true
	case "{":
		return func() tea.Msg { return ShowNotesMsg{Project: project} }, true
	case "}":
		return func() tea.Msg { return ShowNoteCreatorMsg{Project: project} }, true

	// Reload from disk
	case "ctrl+r":
		return v.reloadFromDisk(), true

	// Edit last modified task
	case "ctrl+l":
		if project == nil {
			return nil, true
		}
		recentTasks := storage.GetRecentTasks(project.Tasks, 1)
		if len(recentTasks) > 0 {
			return func() tea.Msg { return EditTaskMsg{Task: recentTasks[0]} }, true
		}
		return nil, true

	// Undo
	case "u":
		return v.performUndo(), true
	case "ctrl+z":
		return func() tea.Msg { return ShowUndoHistoryMsg{Project: project} }, true

	// Score editor
	case "S":
		if project == nil {
			return nil, true
		}
		task := v.getCursorTask()
		if task != nil {
			return func() tea.Msg {
				return ShowScoreEditorMsg{Task: task, Project: project}
			}, true
		}
		return nil, true

	// Quick note on selected task
	case "ctrl+.":
		if project == nil {
			return nil, true
		}
		task := v.getCursorTask()
		if task == nil {
			return nil, true
		}
		return func() tea.Msg {
			return OpenQuickNoteMsg{Task: task, Project: project}
		}, true

	// Move/reparent task
	case "ctrl+m":
		if project == nil {
			return nil, true
		}
		task := v.getCursorTask()
		if task == nil {
			return nil, true
		}
		validParents := buildReparentValidParents(project, task)
		taskID := task.ID
		var currentParentID *string
		if task.ParentID != nil {
			id := *task.ParentID
			currentParentID = &id
		}
		return func() tea.Msg {
			return OpenParentSelectorMsg{
				ValidParents:   validParents,
				CurrentID:      currentParentID,
				AllowNone:      true,
				Source:         ViewProjectTree,
				ReparentTaskID: taskID,
			}
		}, true
	}

	return nil, false
}

// taskSpecificPrefixKeys lists prefix keys that require the cursor to be on a task.
var taskSpecificPrefixKeys = map[string]bool{
	",": true, "!": true, "d": true, "s": true, "t": true, "b": true, "m": true, ".": true, "c": true,
}

// activatePrefixKey checks whether the given key activates a prefix key
// sequence shared across primary views. Returns the prefix string and true
// if the key is a recognized prefix activator, or ("", false) otherwise.
// cursorOnTask should be false when the cursor is on a non-task element
// (e.g. section header) and task-specific prefixes should be skipped.
func activatePrefixKey(key string, cursorOnTask bool) (string, bool) {
	switch key {
	case ",", "z", "g", "m", "!", "d", "s", "t", "b", "F", ".":
		// Task-specific prefixes require a cursor task
		if !cursorOnTask {
			switch key {
			case ",", "!", "d", "s", "t", "b", "m", ".":
				return "", false
			}
		}
		return key, true
	case "c":
		if cursorOnTask {
			return key, true
		}
		return "", false
	}
	return "", false
}

// isPrefixKeySkippedOnHeader returns true if the key is a recognized prefix
// key that was skipped because the cursor is on a section header.
func isPrefixKeySkippedOnHeader(key string, cursorOnTask bool) bool {
	if cursorOnTask {
		return false
	}
	return taskSpecificPrefixKeys[key]
}

// handleSortKey handles shared sort key presses (1-5, 0). It calls
// SortFieldForKey and updates the provided SortState. Returns true if the
// key was consumed. The caller should call its own rebuild/refresh method
// after this returns true, using the onReset callback for sort-reset (key 0)
// and onToggle for field changes.
func handleSortKey(key string, s *SortState, onReset func(), onToggle func(SortField)) bool {
	field, isReset, ok := SortFieldForKey(key)
	if !ok {
		return false
	}
	if isReset {
		s.Reset()
		if onReset != nil {
			onReset()
		}
	} else {
		s.Toggle(field)
		if onToggle != nil {
			onToggle(field)
		}
	}
	return true
}

// cycleCustomSectionTab handles [ and ] keys for cycling through custom
// section tabs with wraparound. Returns true if the key was handled.
// The afterCycle callback is called after the tab index changes so the
// caller can refresh its data.
func cycleCustomSectionTab(key string, tab *int, tabCount int, afterCycle func()) bool {
	if tabCount == 0 {
		return false
	}
	switch key {
	case "[":
		if *tab > 0 {
			*tab--
		} else {
			*tab = tabCount - 1
		}
		if afterCycle != nil {
			afterCycle()
		}
		return true
	case "]":
		if *tab < tabCount-1 {
			*tab++
		} else {
			*tab = 0
		}
		if afterCycle != nil {
			afterCycle()
		}
		return true
	}
	return false
}

// handleSavedFilterPrefixKey handles the third key after pressing Fv (saved filters
// sub-prefix). Returns (cmd, handled). This logic is shared between ProjectTree
// and Outliner.
func handleSavedFilterPrefixKey(key string) (tea.Cmd, bool) {
	switch key {
	case "s":
		return func() tea.Msg { return ShowSaveFilterDialogMsg{} }, true
	case "l", "d":
		return func() tea.Msg { return ShowCommandPaletteMsg{PreFilter: ":sv "} }, true
	case "c":
		return func() tea.Msg { return ShowFilterBuilderMsg{} }, true
	}
	return nil, false
}

// handleQuickEditKey handles key input while in quick edit mode. This is shared
// between ProjectTree and Outliner. The onSave callback is called after a
// successful edit to persist and refresh the view. The optional onUndo callback
// is called with (action, detail) before saving to set undo context.
func handleQuickEditKey(
	msg tea.KeyPressMsg,
	qe *QuickEditComponent,
	project *models.Project,
	onSave func() tea.Cmd,
	onUndo func(action, detail string),
) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		qe.Exit()
		return nil, nil

	case "enter":
		input := strings.TrimSpace(qe.GetInput().Value())
		if input == "" {
			qe.Exit()
			return nil, nil
		}

		task := project.GetTaskByID(qe.GetTask().ID)
		if task == nil {
			qe.Exit()
			return nil, nil
		}

		if qe.IsRenameOnly() {
			if err := models.ValidateTaskTitle(input); err != nil {
				qe.Exit()
				return nil, nil
			}
			project.MutateTask(task.ID, func(t *models.Task) {
				t.Title = input
			})
			if onUndo != nil {
				onUndo("Rename Task", task.Title)
			}
		} else {
			result, err := parseQuickEdit(input, task, project)
			if err != nil {
				return nil, nil
			}
			if statusErr := applyQuickEdit(task, project, result); statusErr != "" {
				qe.Exit()
				return nil, nil
			}
			if onUndo != nil {
				onUndo("Quick Edit", task.Title)
			}
		}

		qe.Exit()
		return nil, onSave()

	default:
		inputModel := qe.GetInput()
		inputModel, cmd := inputModel.Update(msg)
		qe.UpdateInput(inputModel)

		if !qe.IsRenameOnly() {
			inputValue := qe.GetInput().Value()
			preview, _ := parseQuickEditForPreview(inputValue, qe.GetTask(), project)
			qe.SetPreview(preview)
		}

		return nil, cmd
	}
}

// handleTypeChangeKey handles <, >, H, L keys for type promotion/demotion/cycling.
// Returns (cmd, handled). When handled is false the key was not a type-change key.
func handleTypeChangeKey(key string, v prefixableView) (tea.Cmd, bool) {
	op, undoAction, ok := typeChangeOpForKey(key)
	if !ok {
		return nil, false
	}

	project := v.getProject()

	if tasks := v.getMultiSelectedTasks(); tasks != nil {
		prevStates := captureTaskStates(project.Tasks)
		changed := 0

		// For cycle operations (H/L), synchronize all tasks to the same
		// type first so they cycle in lockstep. If types are mixed, reset
		// to Task before cycling; if uniform, cycle from the common type.
		if op == TypeCycleLeft || op == TypeCycleRight {
			changed = applyBulkTypeCycle(project, tasks, op)
		} else {
			for _, task := range tasks {
				if applyTypeChangeOp(project, task.ID, op) {
					changed++
				}
			}
		}

		if changed == 0 {
			v.setStatusMessage("No tasks could be changed")
			return nil, true
		}
		msg := fmt.Sprintf("%s: %d tasks", undoAction, changed)
		v.setUndoContext(undoAction, msg)
		v.setStatusMessage(msg)
		return v.saveAndRefreshBulk(prevStates), true
	}

	task := v.getCursorTask()
	if task == nil {
		return nil, true
	}
	v.setUndoContext(undoAction, task.Title)
	result := applyTypeChange(v.workflowPersistFn(), project, task.ID, op)
	if result.Err != nil {
		return func() tea.Msg { return ErrorMsg{Err: result.Err} }, true
	}
	cmds := []tea.Cmd{result.Cmd}
	if result.StatusMessage != "" {
		v.setStatusMessage(result.StatusMessage)
		v.refreshView()
	}
	return tea.Batch(cmds...), true
}

// handleSharedTaskPrefix handles the t-prefix key sequence shared across
// primary views: message-only handlers (to, tc, ta, tl, td, tm), action
// handlers (tn, tN, ts, tt), rename (tr), toggle pinned (tp), and delete (tD).
// The onRename callback handles view-specific rename entry. focusedTask is the
// zoom/focus target (may be nil).
func handleSharedTaskPrefix(
	key string,
	v prefixableView,
	focusedTask *models.Task,
	onRename func(task *models.Task) tea.Cmd,
) (tea.Cmd, bool) {
	task := v.getCursorTask()
	project := v.getProject()

	// Multi-select getter for shared handlers
	getMultiTasks := func() []*models.Task {
		return v.getMultiSelectedTasks()
	}

	// Try shared message-only handlers first (to, tc, ta, tl, td, tm)
	if task != nil {
		if handled, cmd := handleTaskPrefixMsgKey(key, task, project); handled {
			return cmd, true
		}
	}

	// Try shared action handlers (tn, tN, ts, tt)
	if handled, cmd := handleTaskActionPrefixKey(key, task, project, focusedTask, getMultiTasks); handled {
		return cmd, true
	}

	switch key {
	case "r":
		if task != nil && onRename != nil {
			return onRename(task), true
		}
		return nil, true
	case "p":
		cmd, _ := handleTogglePinnedKey(v)
		return cmd, true
	case "D":
		if dv, ok := v.(deleteConfirmView); ok {
			handleDeleteConfirmKey(dv)
		}
		return nil, true
	}
	return nil, false
}

// primaryViewShouldCaptureEsc checks conditions common to all primary views:
// pending prefix, confirm dialog, search active, multi-select, quick edit, score filter.
func primaryViewShouldCaptureEsc(
	ivs *InteractiveViewState,
	bsv *BaseSearchableView,
	multiSelect bool,
	quickEditActive bool,
	scoreFilterActive bool,
) bool {
	return ivs.DefaultShouldCaptureEsc() ||
		bsv.SearchShouldCaptureEsc() ||
		multiSelect ||
		quickEditActive ||
		scoreFilterActive
}
