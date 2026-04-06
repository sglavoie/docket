package tui

import tea "charm.land/bubbletea/v2"

func (m *OutlinerModel) handleNavigationKey(key string) (tea.Model, tea.Cmd, bool) {
	switch key {
	case "q":
		if m.project == nil && len(m.crossZoomStack) > 0 {
			top := m.crossZoomStack[len(m.crossZoomStack)-1]
			m.crossZoomStack = m.crossZoomStack[:len(m.crossZoomStack)-1]
			m.cursor = top.CursorIndex
			return m, nil, true
		}
		if m.project == nil {
			return m, func() tea.Msg {
				return CloseCrossOutlinerMsg{}
			}, true
		}
		if len(m.zoomStack) == 0 {
			return m, func() tea.Msg {
				return CloseOutlinerMsg{}
			}, true
		}
		top := m.zoomStack[len(m.zoomStack)-1]
		m.zoomStack = m.zoomStack[:len(m.zoomStack)-1]
		m.cursor = top.CursorIndex
		return m, nil, true

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
			keyCollapse := row.effectiveCollapseKey()
			m.collapsedSections[keyCollapse] = !m.collapsedSections[keyCollapse]
			m.rebuildRows()
			return m, nil, true
		}

		task := row.task
		if len(task.Children) > 0 {
			m.zoomStack = append(m.zoomStack, ZoomEntry{
				TaskID:      task.ID,
				CursorIndex: m.cursor,
			})
			m.cursor = 0
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
			return m, nil, true
		}
		if len(m.zoomStack) > 0 {
			top := m.zoomStack[len(m.zoomStack)-1]
			m.zoomStack = m.zoomStack[:len(m.zoomStack)-1]
			m.cursor = top.CursorIndex
		}
		return m, nil, true
	}

	return nil, nil, false
}

// handleQuickEditMode handles input while in quick edit mode.
func (m *OutlinerModel) handleQuickEditMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	proj := m.project
	saveFn := m.saveWithWorkflow
	undoFn := m.setUndoContext
	if proj == nil && m.crossQuickEditProject != nil {
		proj = m.crossQuickEditProject
		saveFn = func() tea.Cmd {
			m.crossQuickEditProject = nil
			return m.crossSaveProject(proj)
		}
		undoFn = func(action, detail string) {
			m.crossUndoCtx.Set(action, detail)
			m.crossUndoCtx.ProjectIDs = []string{proj.ID}
		}
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

// escSteps defines the Esc dismissal cascade for Outliner:
// multi-select → search query → zoom stack (single-project or cross-project).
func (m *OutlinerModel) escSteps() []EscStep {
	return []EscStep{
		{Active: func() bool { return m.multiSelectMode }, Clear: func() {
			m.toggleMultiSelectMode()
		}},
		{Active: func() bool { return m.SearchQuery() != "" }, Clear: func() {
			m.ClearSearch()
			m.rebuildRows()
		}},
		{Active: func() bool { return len(m.zoomStack) > 0 }, Clear: func() {
			top := m.zoomStack[len(m.zoomStack)-1]
			m.zoomStack = m.zoomStack[:len(m.zoomStack)-1]
			m.cursor = top.CursorIndex
		}},
		{Active: func() bool { return len(m.crossZoomStack) > 0 }, Clear: func() {
			top := m.crossZoomStack[len(m.crossZoomStack)-1]
			m.crossZoomStack = m.crossZoomStack[:len(m.crossZoomStack)-1]
			m.cursor = top.CursorIndex
		}},
	}
}

func (m *OutlinerModel) ShouldCaptureEsc() bool {
	return primaryViewShouldCaptureEsc(&m.InteractiveViewState, &m.BaseSearchableView, m.multiSelectMode, m.quickEdit.IsActive(), m.scoreFilterInput.IsActive()) ||
		AnyEscStepActive(m.escSteps()) || m.sidebar.ShouldCaptureEsc()
}

func (m *OutlinerModel) HasConfirmDialog() bool {
	return m.confirm.IsActive()
}

func (m *OutlinerModel) HasTextInput() bool {
	return m.DefaultHasTextInput() || m.quickEdit.IsActive() || m.scoreFilterInput.IsActive()
}
