package taskform

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// handleExtrasTabKey handles keys specific to the Extras tab.
// It returns handled=true when the key was consumed.
func (m *TaskFormModel) handleExtrasTabKey(msg tea.KeyPressMsg) (tea.Model, bool) {
	switch msg.String() {
	case "a":
		m.attachmentsExpanded = !m.attachmentsExpanded
		m.clampExtrasFocusIndex()
	case "c":
		m.commentsExpanded = !m.commentsExpanded
		m.clampExtrasFocusIndex()
	case "ctrl+l":
		m.handleExtrasAddAttachment()
		return m, true
	case "ctrl+y":
		m.handleExtrasAddComment()
		return m, true
	case "ctrl+d":
		m.handleExtrasDelete()
	case "ctrl+j":
		m.handleExtrasReorder(1)
	case "ctrl+k":
		m.handleExtrasReorder(-1)
	default:
		return m, false
	}
	return m, true
}

// handleExtrasEnter handles Enter on the Extras tab:
// - on a section header: toggles expand/collapse
// - on an attachment or comment draft: opens that draft's edit overlay
func (m *TaskFormModel) handleExtrasEnter() {
	if m.focusIndex == 0 {
		m.attachmentsExpanded = !m.attachmentsExpanded
		m.clampExtrasFocusIndex()
		return
	}
	if m.focusIndex == m.extrasCommentsHeaderIdx() {
		m.commentsExpanded = !m.commentsExpanded
		m.clampExtrasFocusIndex()
		return
	}

	kind, idx := m.focusedExtrasItem()
	switch kind {
	case extrasFocusNone:
		return
	case extrasFocusAttachmentItem:
		m.extrasEditIdx = idx
		m.extrasAddingNew = false
		m.initAttachmentItemInput(m.draft.attachments[idx].value)
		m.attachmentItemInput.Focus()
		m.activeOverlay = formOverlayAttachmentEdit
	case extrasFocusCommentItem:
		m.extrasEditIdx = idx
		m.extrasAddingNew = false
		m.initCommentItemInput(m.draft.comments[idx].text)
		m.commentItemInput.Focus()
		m.activeOverlay = formOverlayCommentEdit
	}
}

// handleExtrasAddAttachment appends a blank attachment draft and opens the edit
// overlay for it. Confirming with a blank value drops the draft again, so an
// accidental Ctrl+L leaves nothing behind.
func (m *TaskFormModel) handleExtrasAddAttachment() {
	m.draft.attachments = append(m.draft.attachments, formAttachmentDraft{})
	m.attachmentsExpanded = true
	m.extrasEditIdx = len(m.draft.attachments) - 1
	m.extrasAddingNew = true
	m.initAttachmentItemInput("")
	m.attachmentItemInput.Focus()
	m.activeOverlay = formOverlayAttachmentEdit
	m.hasChanges = true
	m.err = nil
}

// handleExtrasAddComment appends a blank comment draft and opens the edit
// overlay for it, mirroring handleExtrasAddAttachment: confirming blank drops
// the draft again, so an accidental Ctrl+Y leaves nothing behind.
func (m *TaskFormModel) handleExtrasAddComment() {
	m.draft.comments = append(m.draft.comments, formCommentDraft{})
	m.commentsExpanded = true
	m.extrasEditIdx = len(m.draft.comments) - 1
	m.extrasAddingNew = true
	m.initCommentItemInput("")
	m.commentItemInput.Focus()
	m.activeOverlay = formOverlayCommentEdit
	m.hasChanges = true
	m.err = nil
}

// handleExtrasDelete deletes the currently focused attachment or comment draft.
func (m *TaskFormModel) handleExtrasDelete() {
	kind, idx := m.focusedExtrasItem()
	switch kind {
	case extrasFocusAttachmentItem:
		m.draft.attachments = append(m.draft.attachments[:idx], m.draft.attachments[idx+1:]...)
	case extrasFocusCommentItem:
		m.draft.comments = append(m.draft.comments[:idx], m.draft.comments[idx+1:]...)
	default:
		return
	}
	m.hasChanges = true
	m.err = nil
	m.clampExtrasFocusIndex()
}

// handleExtrasReorder moves the focused attachment draft by delta positions
// (+1 = down, -1 = up). Comment drafts are deliberately not reorderable: a
// comment carries its own creation timestamp, so its position is chronology
// rather than an ordering the author chooses.
func (m *TaskFormModel) handleExtrasReorder(delta int) {
	kind, idx := m.focusedExtrasItem()
	switch kind {
	case extrasFocusAttachmentItem:
		neighbor := idx + delta
		if neighbor < 0 || neighbor >= len(m.draft.attachments) {
			return
		}
		m.draft.attachments[idx], m.draft.attachments[neighbor] = m.draft.attachments[neighbor], m.draft.attachments[idx]
	default:
		return
	}
	m.focusIndex += delta
	m.hasChanges = true
	m.err = nil
}

// TaskFormAttachmentEditedMsg is emitted when an attachment draft edit is confirmed.
type TaskFormAttachmentEditedMsg struct {
	Idx   int
	Value string
}

// handleAttachmentEditKey handles key events for the attachment edit overlay.
func (m *TaskFormModel) handleAttachmentEditKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch msg.String() {
	case "enter":
		value := strings.TrimSpace(m.attachmentItemInput.Value())
		return nil, true, TaskFormAttachmentEditedMsg{Idx: m.extrasEditIdx, Value: value}
	case "esc":
		return nil, true, nil // cancel
	default:
		var cmd tea.Cmd
		m.attachmentItemInput, cmd = m.attachmentItemInput.Update(msg)
		return cmd, false, nil
	}
}

// TaskFormCommentEditedMsg is emitted when a comment draft edit is confirmed.
type TaskFormCommentEditedMsg struct {
	Idx  int
	Text string
}

// handleCommentEditKey handles key events for the comment edit overlay.
func (m *TaskFormModel) handleCommentEditKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch msg.String() {
	case "enter":
		text := strings.TrimSpace(m.commentItemInput.Value())
		return nil, true, TaskFormCommentEditedMsg{Idx: m.extrasEditIdx, Text: text}
	case "esc":
		return nil, true, nil // cancel
	default:
		var cmd tea.Cmd
		m.commentItemInput, cmd = m.commentItemInput.Update(msg)
		return cmd, false, nil
	}
}
