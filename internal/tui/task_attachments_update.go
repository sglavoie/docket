package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/help"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/status"
)

func (m *TaskAttachmentsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocyclo
	if m.Confirm.IsActive() {
		return m.updateConfirmMode(msg)
	}

	if m.addMode {
		return m.updateAddMode(msg)
	}

	switch msg := msg.(type) {
	case status.CursorFlashClearMsg:
		m.CursorFlash = false
		return m, nil
	case tea.WindowSizeMsg:
		m.BaseView.SetSize(msg.Width, msg.Height)
	case tea.KeyPressMsg:
		return m, m.updateNormalKey(msg.String())
	}

	return m, nil
}

func (m *TaskAttachmentsModel) updateConfirmMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		action := m.Confirm.Action()
		if confirmed, handled := m.HandleConfirmKey(msg.String()); handled {
			if confirmed {
				switch action {
				case "delete":
					return m, m.executeDelete()
				case "unlink":
					return m, m.executeKeep()
				case "discard-add":
					m.addMode = false
					m.textInput.Reset()
					return m, nil
				}
			}
			m.ClearError()
			return m, nil
		}
	}

	return m, nil
}

func (m *TaskAttachmentsModel) updateAddMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			if strings.TrimSpace(m.textInput.Value()) != "" {
				m.Confirm.Show("discard-add", "Discard attachment?")
				return m, nil
			}
			m.addMode = false
			m.textInput.Reset()
			return m, nil
		case "enter":
			return m, m.addAttachment()
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m *TaskAttachmentsModel) updateNormalKey(key string) tea.Cmd {
	if m.completeAttachmentsTopChord(key) {
		return nil
	}
	if cmd, handled := m.handleAttachmentsNavigationKey(key); handled {
		return cmd
	}
	if cmd, handled := m.handleAttachmentsGrammarKey(key); handled {
		return cmd
	}
	if key == "?" {
		return help.HelpCmd("Attachments", m.GetHelpItems())
	}
	// The core undo descriptors answer only to the Outliner, so the press asks
	// the shell for the one workspace ledger rather than dispatching. Update has
	// already ruled out the add and confirmation modes above, so u is never a
	// character being typed into the attachment input here.
	if key == "u" || key == "ctrl+r" {
		return status.WorkspaceUndoCmd(key == "ctrl+r")
	}

	return nil
}

// handleAttachmentsNavigationKey moves the panel's own cursor. The keys it names
// are the panel's whole navigation grammar and HandleListNavigation performs
// them, so listing them here rather than handing the helper every press is what
// keeps the paging keys the panel never bound from arriving through the back
// door.
func (m *TaskAttachmentsModel) handleAttachmentsNavigationKey(key string) (tea.Cmd, bool) {
	switch key {
	case "esc":
		return func() tea.Msg { return CloseTaskAttachmentsMsg{} }, true
	case "g":
		m.pendingTopChord = true
		return nil, true
	case "j", "down", "k", "up", "home", "G", "end":
		_, cmd := listsurface.HandleListNavigation(key, &m.Cursor, len(m.task.Attachments), m.Height(), &m.CursorFlash)
		return cmd, true
	default:
		return nil, false
	}
}

// completeAttachmentsTopChord answers the second g of gg and reports the press
// as consumed. Any other key abandons the prefix silently and is then handled
// normally, so a stray g never swallows the key after it.
func (m *TaskAttachmentsModel) completeAttachmentsTopChord(key string) bool {
	if !m.pendingTopChord {
		return false
	}
	m.pendingTopChord = false
	if key != "g" {
		return false
	}
	m.Cursor = 0
	return true
}

// handleAttachmentsGrammarKey routes the keys that act on an attachment through
// the registry. Navigation, `?` and Esc stay above and below it because the
// panel performs those itself; every key listed here reaches its operation
// descriptor instead, so the Space sheet and the key pressed today can never
// describe the same act differently. Space is spelled out because a bare space
// rune normalizes to nothing.
func (m *TaskAttachmentsModel) handleAttachmentsGrammarKey(key string) (tea.Cmd, bool) {
	switch key {
	case "a", "enter", "E", "y", "Y", "J", "K":
		return m.dispatchAttachmentsKey(key), true
	case " ", "space":
		return m.dispatchAttachmentsKey("space"), true
	default:
		return nil, false
	}
}

// dispatchAttachmentsKey resolves one browse key through the Attachments grammar
// and runs whatever operation it reaches.
func (m *TaskAttachmentsModel) dispatchAttachmentsKey(key string) tea.Cmd {
	if m.dispatch == nil {
		return nil
	}
	_, cmd := m.dispatch(operation.InputStrategyAttachments, []string{key}, m.attachmentsCommandContext())
	return cmd
}

// attachmentsCommandContext names the panel rather than an item: an attachment
// carries no entity identity, so the cursor the handler reads is the only thing
// that says which item an operation means.
func (m *TaskAttachmentsModel) attachmentsCommandContext() operation.CommandContext {
	ctx := route.PresentationCommandContext(presentation.ViewTaskAttachments)
	ctx.Capabilities.Add(operation.CapabilityAttachmentsPanel)
	return ctx
}

func (m *TaskAttachmentsModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

func (m *TaskAttachmentsModel) beginAddMode() tea.Cmd {
	m.addMode = true
	m.textInput.Focus()
	return textinput.Blink
}

func (m *TaskAttachmentsModel) openSelectedAttachmentDefault() tea.Cmd {
	if !m.hasSelection() {
		return nil
	}
	return m.openAttachmentDefault()
}

func (m *TaskAttachmentsModel) openSelectedAttachmentInEditor() tea.Cmd {
	if !m.hasSelection() {
		return nil
	}
	return m.openAttachmentInEditor()
}

func (m *TaskAttachmentsModel) confirmDeleteSelectedAttachment() {
	if m.hasSelection() {
		m.Confirm.Show("delete", fmt.Sprintf("Delete attachment [%d]?", m.Cursor))
	}
}

func (m *TaskAttachmentsModel) confirmUnlinkSelectedAttachment() {
	if m.hasSelection() {
		m.Confirm.Show("unlink", fmt.Sprintf("Unlink attachment [%d]?", m.Cursor))
	}
}

func (m *TaskAttachmentsModel) copySelectedPath() tea.Cmd {
	if !m.hasSelection() {
		return nil
	}
	return m.copyPath()
}

func (m *TaskAttachmentsModel) copySelectedPathWithLine() tea.Cmd {
	if !m.hasSelection() {
		return nil
	}
	return m.copyPathWithLine()
}

func (m *TaskAttachmentsModel) openSelectedOriginalPath() tea.Cmd {
	if !m.hasSelection() {
		return nil
	}
	return m.openOriginalPath()
}

func (m *TaskAttachmentsModel) revealSelectedOriginalFile() tea.Cmd {
	if !m.hasSelection() {
		return nil
	}
	return m.revealOriginalFile()
}

func (m *TaskAttachmentsModel) moveSelectedAttachmentDown() tea.Cmd {
	if len(m.task.Attachments) > 1 && m.Cursor < len(m.task.Attachments)-1 {
		return m.reorderAttachment(m.Cursor, m.Cursor+1)
	}
	return nil
}

func (m *TaskAttachmentsModel) moveSelectedAttachmentUp() tea.Cmd {
	if m.Cursor > 0 {
		return m.reorderAttachment(m.Cursor, m.Cursor-1)
	}
	return nil
}

func (m *TaskAttachmentsModel) hasSelection() bool {
	return len(m.task.Attachments) > 0 && m.Cursor < len(m.task.Attachments)
}
