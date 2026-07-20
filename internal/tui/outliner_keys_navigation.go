package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

func (m *OutlinerModel) handleNavigationKey(key string) (tea.Model, tea.Cmd, bool) {
	switch key {
	case "esc":
		RunEscCascade(m.escSteps())
		return m, nil, true

	case "enter":
		if model, cmd, handled := m.crossHandleEnterKey(); handled {
			return model, cmd, true
		}

		if len(m.rows) == 0 || m.cursor >= len(m.rows) {
			return m, nil, true
		}
		row := m.rows[m.cursor]
		if row.IsSectionHeader {
			m.toggleSectionFoldAtCursor()
			return m, nil, true
		}

		task := row.task
		if len(task.Children) > 0 {
			m.zoomStack = append(m.zoomStack, ZoomEntry{
				TaskID:      task.ID,
				CursorIndex: m.cursor,
			})
			m.cursor = 0
			m.resetViewScrollOffset()
			return m, nil, true
		}
		return m, func() tea.Msg {
			return ShowTaskDetailMsg{Task: task, Project: m.project}
		}, true

	case "backspace":
		if m.project == nil && len(m.crossZoomStack) > 0 {
			top := m.crossZoomStack[len(m.crossZoomStack)-1]
			m.crossZoomStack = m.crossZoomStack[:len(m.crossZoomStack)-1]
			m.cursor = top.CursorIndex
			m.resetViewScrollOffset()
			return m, nil, true
		}
		if len(m.zoomStack) > 0 {
			top := m.zoomStack[len(m.zoomStack)-1]
			m.zoomStack = m.zoomStack[:len(m.zoomStack)-1]
			m.cursor = top.CursorIndex
			m.resetViewScrollOffset()
		}
		return m, nil, true
	}

	return nil, nil, false
}

// handleQuickEditMode handles input while in quick edit mode.
//
// handleQuickEditKey applies the parsed rename/quick-edit to the loaded project's
// task in memory, then calls onSave. In single-project mode onSave runs the
// workflow persist pipeline; in cross-project mode onSave routes the now-mutated
// task through the app-layer Edit seam (a full-form EditRequest that replaces the
// task's editable fields), committing one ledger transaction so the shell reloads
// and toasts. The onUndo callback is retired in cross mode — the ledger owns undo.
func (m *OutlinerModel) handleQuickEditMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	proj := m.project
	// Name the quick-edited task as the planning-reconcile target so a single-project
	// quick edit that changes the task's section (and therefore its workflow state)
	// folds Today/Next exclusivity into the same whole-project workflow transaction.
	var editTaskID string
	if t := m.quickEdit.GetTask(); t != nil {
		editTaskID = t.ID
	}
	saveFn := func() tea.Cmd {
		if editTaskID != "" {
			m.undo.TaskIDs = []string{editTaskID}
		}
		return m.saveWithWorkflow()
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
			return m.taskItem.Edit(app.EditRequest{TaskID: mutated.ID, Form: formEditFromTask(mutated)})
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
		Title:         t.Title,
		Type:          t.Type,
		Priority:      t.Priority,
		ParentID:      t.ParentID,
		Section:       models.SectionForState(t.State()),
		Description:   t.Description,
		Tags:          append([]string{}, t.Tags...),
		DueDate:       t.DueDate,
		HasTime:       t.HasTime,
		StartDate:     t.StartDate,
		EstimatedDays: t.EstimatedDays,
		URL:           t.URL,
		Source:        t.Source,
		Dimmed:        t.Dimmed,
		Pinned:        t.Pinned,
		Recurrence:    t.Recurrence,
		Checklist:     append([]models.ChecklistItem(nil), t.Checklist...),
		BugAttrs:      t.BugAttrs,
		IdeaAttrs:     t.IdeaAttrs,
		TaskAttrs:     t.TaskAttrs,
		StoryAttrs:    t.StoryAttrs,
		EpicAttrs:     t.EpicAttrs,
	}
}

// escSteps defines the Esc dismissal cascade for Outliner:
// multi-select → search query → zoom stack (single-project or cross-project).
func (m *OutlinerModel) escSteps() []EscStep {
	return []EscStep{
		{Active: func() bool { return m.multiSelectMode }, Clear: func() {
			m.toggleMultiSelectMode()
		}},
		{Active: func() bool { return m.SearchQuery() != "" }, Clear: func() {
			m.ClearSearch()
			m.resetViewScrollOffset()
			m.rebuildRows()
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
		{Active: func() bool { return m.project == nil && m.crossPriorityMode }, Clear: func() {
			m.exitCrossPriorityMode()
		}},
	}
}

func (m *OutlinerModel) ShouldCaptureEsc() bool {
	return primaryViewShouldCaptureEsc(&m.InteractiveViewState, &m.BaseSearchableView, m.multiSelectMode, m.quickEdit.IsActive(), m.scoreFilterInput.IsActive()) ||
		AnyEscStepActive(m.escSteps()) || m.sidebar.ShouldCaptureEsc()
}
func (m *OutlinerModel) HasTextInput() bool {
	return m.DefaultHasTextInput() || m.quickEdit.IsActive() || m.scoreFilterInput.IsActive()
}
