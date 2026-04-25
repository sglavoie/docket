package tui

import (
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

//nolint:gocyclo // main Update dispatch
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
