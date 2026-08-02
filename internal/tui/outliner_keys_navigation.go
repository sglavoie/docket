package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

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
			if models.ValidateTaskTitle(input) != nil {
				qe.Exit()
				return nil, nil
			}
			oldTitle := task.Title
			project.MutateTask(task.ID, func(task *models.Task) { task.Title = input })
			if onUndo != nil {
				onUndo("Rename Task", oldTitle)
			}
		} else {
			result, err := parseQuickEdit(input, task, project)
			if err != nil {
				return nil, nil
			}
			if applyQuickEdit(task, project, result) != "" {
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
		input := qe.GetInput()
		input, cmd := input.Update(msg)
		qe.UpdateInput(input)
		if !qe.IsRenameOnly() {
			preview, _ := parseQuickEditForPreview(qe.GetInput().Value(), qe.GetTask(), project)
			qe.SetPreview(preview)
		}
		return nil, cmd
	}
}

// handleQuickEditMode handles input while in quick edit mode.
//
// handleQuickEditKey applies the parsed rename/quick-edit to the loaded project's
// task in memory, then calls onSave. In single-project mode onSave runs the
// project persist pipeline; in cross-project mode onSave routes the now-mutated
// task through the app-layer Edit seam (a full-form EditRequest that replaces the
// task's editable fields), committing one ledger transaction so the shell reloads
// and toasts. The onUndo callback is retired in cross mode — the ledger owns undo.
func (m *OutlinerModel) handleQuickEditMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	proj := m.project
	// Name the quick-edited task as the planning-reconcile target so a single-project
	// quick edit that changes the task's section (and therefore its state) folds
	// Today/Next exclusivity into the same whole-project transaction.
	var editTaskID string
	if t := m.quickEdit.GetTask(); t != nil {
		editTaskID = t.ID
	}
	saveFn := func() tea.Cmd {
		if editTaskID != "" {
			m.undo.TaskIDs = []string{editTaskID}
		}
		return m.saveProject()
	}
	undoFn := m.setUndoContext
	if proj == nil && m.crossQuickEditProject != nil {
		proj = m.crossQuickEditProject
		editTask := m.quickEdit.GetTask()
		saveFn = func() tea.Cmd {
			m.crossQuickEditProject = nil
			if editTask == nil {
				return nil
			}
			mutated := proj.GetTaskByID(editTask.ID)
			if mutated == nil {
				return nil
			}
			state := mutated.State()
			return m.taskItem.Edit(taskIdentityFor(mutated, proj), app.EditRequest{
				Form:          formEditFromTask(mutated),
				WorkflowState: &state,
			})
		}
		undoFn = func(string, string) {}
	}

	if msg.String() == "enter" && proj != nil && m.quickEdit.GetTask() != nil {
		if proj.GetTaskByID(m.quickEdit.GetTask().ID) == nil {
			m.quickEdit.Exit()
			m.crossQuickEditProject = nil
			return m, func() tea.Msg { return StatusMsg("task was deleted") }
		}
	}

	_, cmd := handleQuickEditKey(msg, m.quickEdit, proj, saveFn, undoFn)
	return m, cmd
}

// formEditFromTask captures a task's full editable field set as an app.FormEdit,
// so a cross-project quick edit — which has already applied its parsed changes to
// the loaded task in memory — can replay the complete post-edit state through the
// app-layer Edit seam. The form's replace-everything semantics mirror the task
// form's own save path (task_form_save.go).
func formEditFromTask(t *models.Task) *app.FormEdit {
	return &app.FormEdit{
		Title:       t.Title,
		Priority:    t.Priority,
		ParentID:    t.ParentID,
		Description: t.Description,
		Tags:        append([]string{}, t.Tags...),
		DueDate:     t.DueDate,
		Pinned:      t.Pinned,
		Checklist:   append([]models.ChecklistItem(nil), t.Checklist...),
	}
}

// escSteps defines the Esc dismissal cascade for Outliner:
// Peek → multi-select → search query → zoom stack (single-project or cross-project).
func (m *OutlinerModel) escSteps() []EscStep {
	return []EscStep{
		{Active: func() bool { return m.peekMode }, Clear: func() {
			m.peekMode = false
			m.resetViewScrollOffset()
		}},
		{Active: func() bool { return m.multiSelectMode }, Clear: func() {
			m.toggleMultiSelectMode()
		}},
		{Active: func() bool { return m.SearchQuery() != "" }, Clear: func() {
			m.ClearSearch()
			m.resetViewScrollOffset()
			rebuildOutlinerPreservingCursor(m)
		}},
		{Active: func() bool { return len(m.zoomStack) > 0 }, Clear: func() {
			top := m.zoomStack[len(m.zoomStack)-1]
			m.zoomStack = m.zoomStack[:len(m.zoomStack)-1]
			m.cursor = top.CursorIndex
			m.resetViewScrollOffset()
		}},
		{Active: func() bool { return len(m.crossZoomStack) > 0 }, Clear: func() {
			top := m.crossZoomStack[len(m.crossZoomStack)-1]
			m.crossZoomStack = m.crossZoomStack[:len(m.crossZoomStack)-1]
			m.cursor = top.CursorIndex
			m.resetViewScrollOffset()
		}},
		// Task zoom unwinds before project scope: Esc pops one scope frame,
		// restoring the previous scope's mode, filter, cursor, and scroll.
		{Active: func() bool { return m.isProjectScoped() }, Clear: func() {
			m.popProjectScope()
		}},
	}
}

func (m *OutlinerModel) ShouldCaptureEsc() bool {
	return m.DefaultShouldCaptureEsc() ||
		m.SearchShouldCaptureEsc() ||
		m.multiSelectMode ||
		m.quickEdit.IsActive() ||
		AnyEscStepActive(m.escSteps())
}
func (m *OutlinerModel) HasTextInput() bool {
	return m.DefaultHasTextInput() || m.quickEdit.IsActive()
}
