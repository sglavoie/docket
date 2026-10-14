package taskform

import (
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/status"
)

// escRaisesDiscard is the condition Esc tests before it leaves, in one place so
// the footer names the act the key reaches rather than repeating its arithmetic.
// A non-empty title counts as unsaved work even before hasChanges is set — an
// expanded compact draft, for one — and only while the title field holds the
// focus, which is the branch that Esc takes.
func (m *TaskFormModel) escRaisesDiscard() bool {
	if m.activeTab == formTabBasics && m.focusIndex == 0 {
		return m.hasChanges || m.titleInput.Value() != ""
	}
	return m.hasChanges
}

//nolint:gocyclo // main Update dispatch
func (m *TaskFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	// ── External overlay results ───────────────────────────────────────────────
	case coordinator.TaskCreatedMsg:
		m.submissionInFlight = false
		return m, nil

	case status.ErrorMsg:
		m.submissionInFlight = false
		m.err = msg.Err
		return m, nil

	case listsurface.ParentSelectedMsg:
		if msg.Source == presentation.ViewTaskComposer {
			m.draft.parentID = msg.ParentID
			m.hasChanges = true
			m.err = nil
		}
		return m, nil

	case listsurface.TagSelectorDoneMsg:
		// Routed here from handler_tag.go via delegateToCurrentView when task is nil.
		if !msg.Canceled {
			m.draft.tags = msg.SelectedTags
			m.hasChanges = true
			m.err = nil
		}
		return m, nil

	case chrome.TextEditorDoneMsg:
		// Routed here from handler_view_editors.go when the composer is current.
		if !msg.Canceled {
			m.applyTextEditorResult(msg.FieldName, msg.Value)
		}
		return m, nil

	case chrome.TextEditorExternalDoneMsg:
		// Direct Ctrl+E from task form (no TextEditor view on the stack).
		if !msg.Canceled {
			m.applyTextEditorResult(msg.FieldName, msg.Value)
		}
		return m, nil

	case tea.KeyPressMsg:
		// ── Discard confirmation ───────────────────────────────────────────────
		if m.ConfirmDiscard {
			switch msg.String() {
			case "y", "Y":
				m.ConfirmDiscard = false
				m.ConfirmOverlay.Cancel()
				return m, func() tea.Msg { return status.BackMsg{} }
			case "n", "N", "esc":
				m.ConfirmDiscard = false
				m.ConfirmOverlay.Cancel()
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
			case "shift+left":
				m.draft.title = m.titleInput.Value()
				m.titleInput.Blur()
				m.prevTab()
				m.syncInputFocus()
				m.err = nil
				return m, nil
			case "shift+right":
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
				if m.escRaisesDiscard() {
					m.ConfirmDiscard = true
				} else {
					return m, func() tea.Msg { return status.BackMsg{} }
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

		// ── Context textinput (field 7 on the expanded Basics tab) ─────────────
		if m.activeTab == formTabBasics && !m.collapsedBasics && m.focusIndex == formBasicsContextField {
			return m, m.handleTextInputKey(msg, m.commitContextInput, &m.contextInput)
		}

		// ── Due textinput (field 0 on Schedule tab) ────────────────────────────
		if m.activeTab == formTabSchedule && m.focusIndex == 0 {
			if msg.String() == "ctrl+d" {
				m.duePicker = newFormDueDatePicker(m.draft.dueDate)
				m.activeOverlay = formOverlayDue
				return m, nil
			}
			return m, m.handleTextInputKey(msg, m.commitDueInput, &m.dueTextInput)
		}

		// ── Priority inline editing (Basics tab, field 2) ─────────────────────
		if m.activeTab == formTabBasics && !m.collapsedBasics && m.focusIndex == 2 {
			switch msg.String() {
			case "0", "1", "2", "3":
				p, _ := strconv.Atoi(msg.String())
				m.draft.priority = p
				m.hasChanges = true
				m.err = nil
				return m, nil
			case "up", "k":
				if m.draft.priority > 0 {
					m.draft.priority--
					m.hasChanges = true
					m.err = nil
				}
				return m, nil
			case "down", "j":
				if m.draft.priority < models.PriorityLow {
					m.draft.priority++
					m.hasChanges = true
					m.err = nil
				}
				return m, nil
			}
		}

		// ── Children-tab keys ─────────────────────────────────────────────────
		if m.activeTab == formTabChildren {
			if ret, handled := m.handleChildrenTabKey(msg); handled {
				return ret, nil
			}
		}

		// ── Extras-tab keys ───────────────────────────────────────────────────
		if m.activeTab == formTabExtras {
			if ret, handled := m.handleExtrasTabKey(msg); handled {
				return ret, nil
			}
		}

		// ── Normal navigation ──────────────────────────────────────────────────
		switch msg.String() {
		case "shift+right":
			m.nextTab()
			m.syncInputFocus()
			m.err = nil
		case "shift+left":
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
			if m.activeTab == formTabChildren {
				m.handleChildrenEnter()
				return m, nil
			}
			if m.activeTab == formTabExtras {
				m.handleExtrasEnter()
				return m, nil
			}
		case "ctrl+e":
			if m.activeTab == formTabBasics && m.focusIndex == 1 {
				currentDesc := m.draft.desc
				return m, func() tea.Msg {
					return chrome.TextEditorExternalMsg{FieldName: chrome.TextEditorFieldFormDescription, CurrentValue: currentDesc}
				}
			}
		case "ctrl+s":
			return m, m.handleSave()
		case "esc":
			if m.escRaisesDiscard() {
				m.ConfirmDiscard = true
			} else {
				return m, func() tea.Msg { return status.BackMsg{} }
			}
		}

	case tea.PasteMsg:
		return m.handlePaste(msg)
	}

	return m, nil
}
