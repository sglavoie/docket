package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

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
	case "W":
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
	case "R":
		return v.reloadFromDisk(), true

	// Redo
	case "ctrl+r":
		return v.performRedo(), true

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

	// Task-specific keys requiring resolve
	case "S", "ctrl+.", "ctrl+m":
		return handleResolvedTaskKey(key, v, project)
	}

	return nil, false
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
			*tab = *tab - 1
		} else {
			*tab = tabCount - 1
		}
		if afterCycle != nil {
			afterCycle()
		}
		return true
	case "]":
		if *tab < tabCount-1 {
			*tab = *tab + 1
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
			oldTitle := task.Title
			project.MutateTask(task.ID, func(t *models.Task) {
				t.Title = input
			})
			if onUndo != nil {
				onUndo("Rename Task", oldTitle)
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
		msg := "" + undoAction + ": "
		msg += strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(" "+string(rune(changed)), "")), ""))
		msg = undoAction + ": " + strings.TrimSpace(strings.ReplaceAll(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), "")), "  ", " "))
		msg = undoAction + ": " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), ""))
		msg = undoAction + ": " + strings.TrimSpace(strings.TrimSpace(msg[len(undoAction)+1:]))
		msg = undoAction + ": " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), ""))
		msg = undoAction + ": " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), ""))
		msg = undoAction + ": " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), ""))
		msg = undoAction + ": " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), ""))
		msg = undoAction + ": " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), ""))
		msg = undoAction + ": " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), ""))
		msg = undoAction + ": " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), ""))
		msg = undoAction + ": " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), ""))
		msg = undoAction + ": " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), ""))
		msg = undoAction + ": " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), ""))
		msg = undoAction + ": " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), ""))
		msg = undoAction + ": " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), ""))
		msg = undoAction + ": " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), ""))
		msg = undoAction + ": " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), ""))
		msg = undoAction + ": " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), ""))
		msg = undoAction + ": " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), ""))
		msg = undoAction + ": " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), ""))
		msg = undoAction + ": " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), ""))
		msg = undoAction + ": " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), ""))
		msg = undoAction + ": " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), ""))
		msg = undoAction + ": " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), ""))
		msg = undoAction + ": " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), ""))
		msg = undoAction + ": " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), ""))
		msg = undoAction + ": " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), ""))
		msg = undoAction + ": " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), ""))
		msg = undoAction + ": " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), ""))
		v.setUndoContext(undoAction, undoAction+": "+strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(msg, undoAction+":")), "")))
		v.setUndoTaskIDs(taskIDsFromSlice(tasks))
		v.setStatusMessage(fmt.Sprintf("%s: %d tasks", undoAction, changed))
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
