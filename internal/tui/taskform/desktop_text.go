package taskform

import "github.com/sglavoie/tt/internal/tui/chrome"

func (m *TaskComposerModel) DesktopTextBuffer() *chrome.DesktopTextBuffer {
	if m.SubmissionInFlight || m.ConfirmDiscard || m.projectOffer != nil || m.matchingTaskPreview != nil || m.HelpExpanded {
		return nil
	}
	if m.Mode == ComposerExpanded {
		if m.Expanded != nil {
			return m.Expanded.DesktopTextBuffer()
		}
		return nil
	}
	before := m.Input.Value()
	return chrome.DesktopTextarea(&m.Input, func() {
		m.AC.Clear()
		m.refreshCaptureErrorsAfterEdit(before, m.Preset.Project)
	})
}

func (m *TaskFormModel) DesktopTextBuffer() *chrome.DesktopTextBuffer {
	if m.ConfirmDiscard || m.submissionInFlight {
		return nil
	}
	changed := func() { m.hasChanges = true; m.err = nil }
	switch m.activeOverlay { //nolint:exhaustive // Non-text pickers own input without exposing an editable buffer.
	case formOverlayRelation:
		return chrome.DesktopInput(&m.relationInput, func() { m.relationError = nil })
	case formOverlayCheckIn:
		if m.checkInEditor != nil {
			return m.checkInEditor.DesktopTextBuffer()
		}
	case formOverlayAttachmentEdit:
		return chrome.DesktopInput(&m.attachmentItemInput, changed)
	case formOverlayCommentEdit:
		return chrome.DesktopInput(&m.commentItemInput, changed)
	case formOverlayChecklistEdit:
		return chrome.DesktopInput(&m.checklistItemInput, changed)
	case formOverlaySubtaskEdit:
		return chrome.DesktopInput(&m.subtaskItemInput, changed)
	case formOverlayNone:
		if m.activeTab == formTabBasics {
			if m.focusIndex == 0 {
				return chrome.DesktopTextarea(&m.titleInput, changed)
			}
			if m.focusIndex == formBasicsContextField {
				return chrome.DesktopInput(&m.contextInput, changed)
			}
		}
		if m.activeTab == formTabSchedule && m.focusIndex == 0 {
			return chrome.DesktopInput(&m.dueTextInput, changed)
		}
	}
	return nil
}

func (m *CheckInEditorModel) DesktopTextBuffer() *chrome.DesktopTextBuffer {
	if m.submitted || m.picker != checkInPickerNone {
		return nil
	}
	changed := func() { m.notice = "" }
	if m.focus == 0 {
		return chrome.DesktopInput(&m.dateInput, changed)
	}
	return chrome.DesktopInput(&m.repeatInput, changed)
}
