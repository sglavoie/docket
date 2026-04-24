package tui

import (
	"strconv"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *TaskFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	// ── External overlay results ───────────────────────────────────────────────

	case ParentSelectedMsg:
		if msg.Source == ViewTaskForm {
			m.draft.parentID = msg.ParentID
			m.hasChanges = true
			m.err = nil
		}
		return m, nil

	case SectionPickerDoneMsg:
		// Routed here from handler_section.go when Task == nil && currentView == ViewTaskForm.
		if !msg.Canceled && msg.Section != "" {
			m.draft.section = msg.Section
			m.hasChanges = true
			m.err = nil
		}
		return m, nil

	case TagSelectorDoneMsg:
		// Routed here from handler_tag.go via delegateToCurrentView when task is nil.
		if !msg.Canceled {
			m.draft.tags = msg.SelectedTags
			m.hasChanges = true
			m.err = nil
		}
		return m, nil

	case RecurrenceEditorDoneMsg:
		// Routed here from handler_view_editors.go after the recurrence editor pops.
		if !msg.Canceled {
			m.draft.recurrence = msg.Pattern
			m.hasChanges = true
			m.err = nil
		}
		return m, nil

	case TextEditorDoneMsg:
		// Routed here from handler_view_editors.go when ViewTaskForm is current.
		if !msg.Canceled {
			m.applyTextEditorResult(msg.FieldName, msg.Value)
		}
		return m, nil

	case TextEditorExternalDoneMsg:
		// Direct Ctrl+E from task form (no TextEditor view on the stack).
		if !msg.Canceled {
			m.applyTextEditorResult(msg.FieldName, msg.Value)
		}
		return m, nil

	case tea.KeyPressMsg:
		// ── Discard confirmation ───────────────────────────────────────────────
		if m.confirmDiscard {
			switch msg.String() {
			case "y", "Y":
				m.confirmDiscard = false
				m.confirmOverlay.Cancel()
				return m, func() tea.Msg { return BackMsg{} }
			case "n", "N", "esc":
				m.confirmDiscard = false
				m.confirmOverlay.Cancel()
			}
			return m, nil
		}

		// ── Inline overlay intercepts all keys ─────────────────────────────────
		if m.activeOverlay != formOverlayNone {
			cmd, consumed := m.handleFormOverlayKey(msg)
			if consumed {
				return m, cmd
			}
			return m, nil
		}

		// ── Title textarea (field 0 on Basics tab) ─────────────────────────────
		if m.activeTab == formTabBasics && m.focusIndex == 0 {
			switch msg.String() {
			case "tab":
				m.draft.title = m.titleInput.Value()
				m.titleInput.Blur()
				m.focusNext()
				m.syncInputFocus()
				return m, nil
			case "shift+tab":
				m.draft.title = m.titleInput.Value()
				m.titleInput.Blur()
				m.focusPrev()
				m.syncInputFocus()
				return m, nil
			case "[":
				m.draft.title = m.titleInput.Value()
				m.titleInput.Blur()
				m.prevTab()
				m.syncInputFocus()
				m.err = nil
				return m, nil
			case "]":
				m.draft.title = m.titleInput.Value()
				m.titleInput.Blur()
				m.nextTab()
				m.syncInputFocus()
				m.err = nil
				return m, nil
			case "ctrl+s":
				m.draft.title = m.titleInput.Value()
				return m, m.handleSave()
			case "esc":
				m.draft.title = m.titleInput.Value()
				m.titleInput.Blur()
				// In create mode, a non-empty title counts as unsaved work even
				// if hasChanges hasn't been set yet. In edit mode, only hasChanges
				// matters since the title was pre-filled from the existing task.
				dirty := m.hasChanges || (m.mode == FormModeCreate && m.draft.title != "")
				if dirty {
					m.confirmDiscard = true
				} else {
					return m, func() tea.Msg { return BackMsg{} }
				}
				return m, nil
			default:
				before := m.titleInput.Value()
				var cmd tea.Cmd
				m.titleInput, cmd = m.titleInput.Update(msg)
				after := m.titleInput.Value()
				if before != after {
					m.hasChanges = true
					m.err = nil
				}
				return m, cmd
			}
		}

		// ── Due textinput (field 0 on Schedule tab) ────────────────────────────
		if m.activeTab == formTabSchedule && m.focusIndex == 0 {
			if msg.String() == "ctrl+d" {
				m.duePicker = newFormDueDatePicker(m.draft.dueDate)
				m.activeOverlay = formOverlayDue
				return m, nil
			}
			if cmd, ok := m.handleTextInputKey(msg, m.commitDueInput, &m.dueTextInput); ok {
				return m, cmd
			}
		}

		// ── URL textinput (field 0 on Details tab) ────────────────────────────
		if m.activeTab == formTabDetails && m.focusIndex == 0 {
			if cmd, ok := m.handleTextInputKey(msg, m.commitURLInput, &m.urlTextInput); ok {
				return m, cmd
			}
		}

		// ── Source textinput (field 1 on Details tab) ──────────────────────────
		if m.activeTab == formTabDetails && m.focusIndex == 1 {
			if cmd, ok := m.handleTextInputKey(msg, m.commitSourceInput, &m.sourceTextInput); ok {
				return m, cmd
			}
		}

		// ── Priority inline editing (Basics tab, field 3) ─────────────────────
		if m.activeTab == formTabBasics && m.focusIndex == 3 {
			switch msg.String() {
			case "1", "2", "3", "4", "5":
				p, _ := strconv.Atoi(msg.String())
				m.draft.priority = p
				m.hasChanges = true
				m.err = nil
				return m, nil
			case "up", "k":
				if m.draft.priority > 1 {
					m.draft.priority--
					m.hasChanges = true
					m.err = nil
				}
				return m, nil
			case "down", "j":
				if m.draft.priority < 5 {
					m.draft.priority++
					m.hasChanges = true
					m.err = nil
				}
				return m, nil
			}
		}

		// ── Type inline cycling (Basics tab, field 2) ──────────────────────────
		if m.activeTab == formTabBasics && m.focusIndex == 2 {
			types := models.AllTaskTypes()
			curIdx := 0
			for i, t := range types {
				if t == m.draft.taskType {
					curIdx = i
					break
				}
			}
			switch msg.String() {
			case "h", "left":
				m.applyTypeChange(types[(curIdx+len(types)-1)%len(types)])
				return m, nil
			case "l", "right":
				m.applyTypeChange(types[(curIdx+1)%len(types)])
				return m, nil
			}
		}

		// ── Children-tab keys ─────────────────────────────────────────────────
		if m.activeTab == formTabChildren {
			if ret, cmd, handled := m.handleChildrenTabKey(msg); handled {
				return ret, cmd
			}
		}

		// ── Normal navigation ──────────────────────────────────────────────────
		switch msg.String() {
		case "]":
			m.nextTab()
			m.syncInputFocus()
			m.err = nil
		case "[":
			m.prevTab()
			m.syncInputFocus()
			m.err = nil
		case "tab":
			m.focusNext()
			m.syncInputFocus()
		case "shift+tab":
			m.focusPrev()
			m.syncInputFocus()
		case "enter":
			if m.activeTab == formTabBasics {
				return m, m.handleBasicsEnter()
			}
			if m.activeTab == formTabSchedule {
				return m, m.handleScheduleEnter()
			}
			if m.activeTab == formTabDetails {
				return m, m.handleDetailsEnter()
			}
			if m.activeTab == formTabChildren {
				return m, m.handleChildrenEnter()
			}
		case "ctrl+e":
			if m.activeTab == formTabBasics && m.focusIndex == 1 {
				currentDesc := m.draft.desc
				return m, func() tea.Msg {
					return TextEditorExternalMsg{FieldName: "formDesc", CurrentValue: currentDesc}
				}
			}
		case "ctrl+s":
			return m, m.handleSave()
		case "esc":
			if m.hasChanges {
				m.confirmDiscard = true
			} else {
				return m, func() tea.Msg { return BackMsg{} }
			}
		}
	}

	return m, nil
}

// handleChildrenTabKey handles keys specific to the Children tab.
// It returns handled=true when the key was consumed.
func (m *TaskFormModel) handleChildrenTabKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	switch msg.String() {
	case "c":
		m.checklistExpanded = !m.checklistExpanded
		m.clampChildrenFocusIndex()
	case "S":
		if m.mode == FormModeCreate && formTypeCanHaveSubtasks(m.draft.taskType) {
			m.subtasksExpanded = !m.subtasksExpanded
			m.clampChildrenFocusIndex()
		}
	case "ctrl+a":
		return m, m.handleChildrenCtrlA(), true
	case "ctrl+t":
		return m, m.handleChildrenCtrlT(), true
	case "ctrl+d":
		m.handleChildrenDelete()
	case "ctrl+j":
		m.handleChildrenReorder(1)
	case "ctrl+k":
		m.handleChildrenReorder(-1)
	case "ctrl+o":
		m.handleSubtaskToggleOptional()
	case "space":
		m.handleChecklistToggleDone()
	default:
		return m, nil, false
	}
	return m, nil, true
}

// handleScheduleEnter dispatches Enter key events for focused Schedule-tab rows.
func (m *TaskFormModel) handleScheduleEnter() tea.Cmd {
	switch m.focusIndex {
	case 0:
		// Due: textinput is already focused; Enter commits and moves to Recurrence.
		m.commitDueInput()
		m.focusNext()
		m.syncInputFocus()
		return nil

	case 1:
		// Recurrence: open the recurrence editor as a full-screen overlay.
		pattern := m.draft.recurrence
		dueDate := m.draft.dueDate
		return func() tea.Msg {
			return ShowRecurrenceEditorMsg{
				CurrentPattern: pattern,
				DueDate:        dueDate,
			}
		}
	}
	return nil
}

// handleBasicsEnter dispatches Enter key events for focused Basics-tab rows.
func (m *TaskFormModel) handleBasicsEnter() tea.Cmd {
	switch m.focusIndex {
	case 0:
		// Title: focus textarea (already focused by focusTitleInput, no-op here)
		return nil

	case 1:
		// Description: open TextEditorModel.
		currentDesc := m.draft.desc
		return func() tea.Msg {
			return ShowTextEditorMsg{FieldName: "formDesc", CurrentValue: currentDesc}
		}

	case 2:
		// Type: open inline type picker
		m.typePicker = newFormTypePicker(m.draft.taskType)
		m.activeOverlay = formOverlayType
		return nil

	case 3:
		// Priority: open inline priority picker
		m.priorityPicker = newFormPriorityPicker(m.draft.priority)
		m.activeOverlay = formOverlayPriority
		return nil

	case 4:
		// Parent: open full parent selector (disabled for Epics)
		if m.draft.taskType == models.TaskTypeEpic {
			return nil
		}
		parents := m.buildValidParents()
		return func() tea.Msg {
			return OpenParentSelectorMsg{
				ValidParents: parents,
				CurrentID:    m.draft.parentID,
				AllowNone:    true,
				Source:       ViewTaskForm,
			}
		}

	case 5:
		// Section: open section picker (Task=nil signals form context)
		project := m.project
		return func() tea.Msg {
			return ShowSectionPickerMsg{
				Task:    nil,
				Project: project,
			}
		}

	case 6:
		// Flags: open inline flags picker
		m.flagsPicker = newFormFlagsPicker(m.draft.pinned, m.draft.active, m.draft.dimmed)
		m.activeOverlay = formOverlayFlags
		return nil

	case 7:
		// Tags: open tag selector
		tags := append([]string(nil), m.draft.tags...)
		project := m.project
		return func() tea.Msg {
			return ShowTagSelectorMsg{
				Task:        nil,
				Project:     project,
				CurrentTags: tags,
			}
		}

	default:
		// Attr rows (focusIndex >= formBasicsFieldCount).
		if m.focusIndex >= formBasicsFieldCount {
			return m.openAttrOverlayForFocus()
		}
	}
	return nil
}

// handleDetailsEnter dispatches Enter key events for focused Details-tab rows.
func (m *TaskFormModel) handleDetailsEnter() tea.Cmd {
	switch m.focusIndex {
	case 0:
		// URL: textinput is focused; Enter commits and moves to Source.
		m.commitURLInput()
		m.focusNext()
		m.syncInputFocus()
		return nil
	case 1:
		// Source: textinput is focused; Enter commits and wraps.
		m.commitSourceInput()
		m.focusNext()
		m.syncInputFocus()
		return nil
	}
	return nil
}

// openAttrOverlayForFocus opens the appropriate overlay for the focused attr row.
// localIdx = m.focusIndex - formBasicsFieldCount.
func (m *TaskFormModel) openAttrOverlayForFocus() tea.Cmd {
	localIdx := m.focusIndex - formBasicsFieldCount

	switch m.draft.taskType {
	case models.TaskTypeBug:
		switch localIdx {
		case 0:
			m.attrPicker = newFormAttrPicker("Reproducible", "bugReproducible",
				attrOptsBugReproducible, string(m.draft.bugAttrs.Reproducible))
			m.activeOverlay = formOverlayAttr
		case 1:
			m.attrPicker = newFormAttrPicker("Severity", "bugSeverity",
				attrOptsBugSeverity, string(m.draft.bugAttrs.Severity))
			m.activeOverlay = formOverlayAttr
		case 2:
			current := m.draft.bugAttrs.Workaround
			return func() tea.Msg {
				return ShowTextEditorMsg{FieldName: "bugWorkaround", CurrentValue: current}
			}
		}

	case models.TaskTypeIdea:
		switch localIdx {
		case 0:
			m.attrPicker = newFormAttrPicker("Category", "ideaCategory",
				attrOptsIdeaCategory, string(m.draft.ideaAttrs.Category))
			m.activeOverlay = formOverlayAttr
		case 1:
			m.attrPicker = newFormAttrPicker("Effort", "ideaEffort",
				attrOptsEffort, string(m.draft.ideaAttrs.Effort))
			m.activeOverlay = formOverlayAttr
		case 2:
			m.attrPicker = newFormAttrPicker("Value", "ideaValue",
				attrOptsIdeaValue, string(m.draft.ideaAttrs.Value))
			m.activeOverlay = formOverlayAttr
		case 3:
			validatedCurrent := ""
			if m.draft.ideaAttrs.Validated {
				validatedCurrent = "true"
			}
			m.attrPicker = newFormAttrPicker("Validated", "ideaValidated",
				attrOptsIdeaValidated, validatedCurrent)
			m.activeOverlay = formOverlayAttr
		}

	case models.TaskTypeTask:
		switch localIdx {
		case 0:
			m.attrPicker = newFormAttrPicker("Effort", "taskEffort",
				attrOptsEffort, string(m.draft.taskAttrs.Effort))
			m.activeOverlay = formOverlayAttr
		case 1:
			m.attrPicker = newFormAttrPicker("Work Type", "taskWorkType",
				attrOptsWorkType, string(m.draft.taskAttrs.WorkType))
			m.activeOverlay = formOverlayAttr
		}

	case models.TaskTypeStory:
		switch localIdx {
		case 0:
			pointsCurrent := ""
			if m.draft.storyAttrs.StoryPoints > 0 {
				pointsCurrent = strconv.Itoa(m.draft.storyAttrs.StoryPoints)
			}
			m.attrPicker = newFormAttrPicker("Story Points", "storyPoints",
				attrOptsStoryPoints, pointsCurrent)
			m.activeOverlay = formOverlayAttr
		case 1:
			current := m.draft.storyAttrs.Persona
			return func() tea.Msg {
				return ShowTextEditorMsg{FieldName: "storyPersona", CurrentValue: current}
			}
		}

	case models.TaskTypeEpic:
		switch localIdx {
		case 0:
			m.attrPicker = newFormAttrPicker("Scope", "epicScope",
				attrOptsEpicScope, string(m.draft.epicAttrs.Scope))
			m.activeOverlay = formOverlayAttr
		case 1:
			current := m.draft.epicAttrs.Milestone
			return func() tea.Msg {
				return ShowTextEditorMsg{FieldName: "epicMilestone", CurrentValue: current}
			}
		case 2:
			current := m.draft.epicAttrs.SuccessMetrics
			return func() tea.Msg {
				return ShowTextEditorMsg{FieldName: "epicSuccessMetrics", CurrentValue: current}
			}
		}
	}

	return nil
}

// applyTextEditorResult merges a TextEditorModel result into the draft.
func (m *TaskFormModel) applyTextEditorResult(field, value string) {
	switch field {
	case "formDesc":
		m.draft.desc = value
	case "bugWorkaround":
		m.draft.bugAttrs.Workaround = value
	case "storyPersona":
		m.draft.storyAttrs.Persona = value
	case "epicMilestone":
		m.draft.epicAttrs.Milestone = value
	case "epicSuccessMetrics":
		m.draft.epicAttrs.SuccessMetrics = value
	}
	m.hasChanges = true
	m.err = nil
}

// ── Children tab handlers ─────────────────────────────────────────────────────

// handleChildrenEnter handles Enter on the Children tab:
// - on a section header: toggles expand/collapse
// - on a checklist item: opens checklist edit overlay
// - on a subtask item: opens subtask edit overlay
func (m *TaskFormModel) handleChildrenEnter() tea.Cmd {
	// Section headers: toggle expand/collapse.
	if m.focusIndex == 0 {
		m.checklistExpanded = !m.checklistExpanded
		m.clampChildrenFocusIndex()
		return nil
	}
	if stHeaderIdx := m.childrenSubtasksHeaderIdx(); m.focusIndex == stHeaderIdx {
		m.subtasksExpanded = !m.subtasksExpanded
		m.clampChildrenFocusIndex()
		return nil
	}

	kind, idx := m.focusedChildItem()
	switch kind {
	case childFocusChecklistItem:
		m.childrenEditIdx = idx
		m.childrenAddingNew = false
		m.initChecklistItemInput(m.draft.checklist[idx].Text)
		m.checklistItemInput.Focus()
		m.activeOverlay = formOverlayChecklistEdit
	case childFocusSubtaskItem:
		m.childrenEditIdx = idx
		m.childrenAddingNew = false
		m.initSubtaskItemInput(m.draft.subtasks[idx].title)
		m.subtaskItemInput.Focus()
		m.subtaskTypeCursor = formSubtaskTypeCursorFor(m.draft.subtasks[idx].taskType)
		m.activeOverlay = formOverlaySubtaskEdit
	}
	return nil
}

// handleChildrenCtrlA adds a new checklist item and opens the edit overlay for it.
func (m *TaskFormModel) handleChildrenCtrlA() tea.Cmd {
	m.draft.checklist = append(m.draft.checklist, models.ChecklistItem{})
	m.checklistExpanded = true
	m.childrenEditIdx = len(m.draft.checklist) - 1
	m.childrenAddingNew = true
	m.initChecklistItemInput("")
	m.checklistItemInput.Focus()
	m.activeOverlay = formOverlayChecklistEdit
	m.hasChanges = true
	m.err = nil
	return nil
}

// handleChildrenCtrlT adds a new subtask draft and opens the edit overlay for it.
func (m *TaskFormModel) handleChildrenCtrlT() tea.Cmd {
	if m.mode != FormModeCreate || !formTypeCanHaveSubtasks(m.draft.taskType) {
		return nil
	}
	m.draft.subtasks = append(m.draft.subtasks, formSubtaskDraft{taskType: formSubtaskTypes[0]})
	m.subtasksExpanded = true
	m.childrenEditIdx = len(m.draft.subtasks) - 1
	m.childrenAddingNew = true
	m.initSubtaskItemInput("")
	m.subtaskItemInput.Focus()
	m.subtaskTypeCursor = 0
	m.activeOverlay = formOverlaySubtaskEdit
	m.hasChanges = true
	m.err = nil
	return nil
}

// handleChildrenDelete deletes the currently focused checklist item or subtask.
func (m *TaskFormModel) handleChildrenDelete() {
	kind, idx := m.focusedChildItem()
	switch kind {
	case childFocusChecklistItem:
		m.draft.checklist = append(m.draft.checklist[:idx], m.draft.checklist[idx+1:]...)
	case childFocusSubtaskItem:
		m.draft.subtasks = append(m.draft.subtasks[:idx], m.draft.subtasks[idx+1:]...)
	default:
		return
	}
	m.hasChanges = true
	m.err = nil
	m.clampChildrenFocusIndex()
}

// handleChildrenReorder moves the focused checklist item or subtask by delta
// positions (+1 = down, -1 = up).
func (m *TaskFormModel) handleChildrenReorder(delta int) {
	kind, idx := m.focusedChildItem()
	switch kind {
	case childFocusChecklistItem:
		neighbor := idx + delta
		if neighbor < 0 || neighbor >= len(m.draft.checklist) {
			return
		}
		m.draft.checklist[idx], m.draft.checklist[neighbor] = m.draft.checklist[neighbor], m.draft.checklist[idx]
	case childFocusSubtaskItem:
		neighbor := idx + delta
		if neighbor < 0 || neighbor >= len(m.draft.subtasks) {
			return
		}
		m.draft.subtasks[idx], m.draft.subtasks[neighbor] = m.draft.subtasks[neighbor], m.draft.subtasks[idx]
	default:
		return
	}
	m.focusIndex += delta
	m.hasChanges = true
	m.err = nil
}

// handleSubtaskToggleOptional toggles the optional flag on the focused subtask.
func (m *TaskFormModel) handleSubtaskToggleOptional() {
	if kind, idx := m.focusedChildItem(); kind == childFocusSubtaskItem {
		m.draft.subtasks[idx].optional = !m.draft.subtasks[idx].optional
		m.hasChanges = true
		m.err = nil
	}
}

// handleChecklistToggleDone toggles the Done state of the focused checklist item.
func (m *TaskFormModel) handleChecklistToggleDone() {
	if kind, idx := m.focusedChildItem(); kind == childFocusChecklistItem {
		m.draft.checklist[idx].Done = !m.draft.checklist[idx].Done
		m.hasChanges = true
		m.err = nil
	}
}

// buildValidParents returns tasks in the project that can parent the current draft type.
// In edit mode, the task being edited and all its descendants are excluded to
// prevent circular parent-child relationships.
func (m *TaskFormModel) buildValidParents() []*models.Task {
	// In edit mode, build a set of the task's own ID + all descendant IDs.
	excluded := make(map[string]bool)
	if m.mode == FormModeEdit && m.task != nil {
		var collectDescendants func(t *models.Task)
		collectDescendants = func(t *models.Task) {
			excluded[t.ID] = true
			for _, child := range t.Children {
				collectDescendants(child)
			}
		}
		collectDescendants(m.task)
	}

	var parents []*models.Task
	for _, t := range m.project.Tasks {
		if excluded[t.ID] {
			continue
		}
		if t.CanAddChild(m.draft.taskType) {
			parents = append(parents, t)
		}
	}
	return parents
}

// handleTextInputKey consolidates the common key handling for textinput fields
// (due, URL, source). It calls commit before navigation/save/esc, and forwards
// unrecognized keys to the textinput. Returns (cmd, true) if the key was consumed.
func (m *TaskFormModel) handleTextInputKey(msg tea.KeyPressMsg, commit func(), input *textinput.Model) (tea.Cmd, bool) {
	switch msg.String() {
	case "tab":
		commit()
		m.focusNext()
		m.syncInputFocus()
		return nil, true
	case "shift+tab":
		commit()
		m.focusPrev()
		m.syncInputFocus()
		return nil, true
	case "[":
		commit()
		m.prevTab()
		m.syncInputFocus()
		m.err = nil
		return nil, true
	case "]":
		commit()
		m.nextTab()
		m.syncInputFocus()
		m.err = nil
		return nil, true
	case "enter":
		commit()
		m.focusNext()
		m.syncInputFocus()
		return nil, true
	case "ctrl+s":
		commit()
		return m.handleSave(), true
	case "esc":
		commit()
		if m.hasChanges {
			m.confirmDiscard = true
		} else {
			return func() tea.Msg { return BackMsg{} }, true
		}
		return nil, true
	default:
		before := input.Value()
		var cmd tea.Cmd
		*input, cmd = input.Update(msg)
		after := input.Value()
		if before != after {
			m.hasChanges = true
			m.err = nil
		}
		return cmd, true
	}
}
