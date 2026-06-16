package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

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

// cyclePinnedTagTab handles shift+left and shift+right keys for cycling
// through pinned tag tabs with wraparound. Returns true if the key was
// handled. The afterCycle callback is called after the tab index changes so
// the caller can refresh its data.
func cyclePinnedTagTab(key string, tab *int, tabCount int, afterCycle func()) bool {
	if tabCount <= 1 {
		return false
	}
	switch key {
	case "shift+left":
		if *tab > 0 {
			*tab = *tab - 1
		} else {
			*tab = tabCount - 1
		}
		if afterCycle != nil {
			afterCycle()
		}
		return true
	case "shift+right":
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

func savedFilterPrefixEntry(model tea.Model) keymap.Entry {
	return keymap.Prefix("v", "Saved filters", "Filters", []keymap.Entry{
		keymap.Bind("s", "Save current filters", "Filters", func() (tea.Model, tea.Cmd) {
			cmd, _ := handleSavedFilterPrefixKey("s")
			return model, cmd
		}),
		keymap.Bind("l", "Load saved filters", "Filters", func() (tea.Model, tea.Cmd) {
			cmd, _ := handleSavedFilterPrefixKey("l")
			return model, cmd
		}),
		keymap.Bind("d", "Delete saved filters", "Filters", func() (tea.Model, tea.Cmd) {
			cmd, _ := handleSavedFilterPrefixKey("d")
			return model, cmd
		}),
		keymap.Bind("c", "Create criteria filter", "Filters", func() (tea.Model, tea.Cmd) {
			cmd, _ := handleSavedFilterPrefixKey("c")
			return model, cmd
		}),
	})
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
		detail := fmt.Sprintf("%d tasks", changed)
		v.setUndoContext(undoAction, detail)
		v.setUndoTaskIDs(taskIDsFromSlice(tasks))
		v.setStatusMessage(fmt.Sprintf("%s: %s", undoAction, detail))
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
	if updated := project.GetTaskByID(task.ID); updated != nil {
		v.setMutationFlash(typeMutationFlashBg(updated.Type))
		cmds = append(cmds, mutationFlashTimeout())
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
