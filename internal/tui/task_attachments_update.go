package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

func (m *TaskAttachmentsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocyclo
	var cmd tea.Cmd

	if m.confirm.IsActive() {
		return m.updateConfirmMode(msg)
	}

	if m.addMode {
		return m.updateAddMode(msg)
	}

	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	case tea.WindowSizeMsg:
		m.BaseView.SetSize(msg.Width, msg.Height)
	case tea.KeyPressMsg:
		if m.pendingPrefix == "g" {
			m.pendingPrefix = ""
			_, cmd = handleGotoTopPrefix(msg.String(), &m.cursor, &m.cursorFlash)
			return m, cmd
		}

		return m, m.updateNormalKey(msg.String())
	}

	return m, nil
}

func (m *TaskAttachmentsModel) updateConfirmMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		action := m.confirm.Action()
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
				m.confirm.Show("discard-add", "Discard attachment?")
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
	if cmd, handled := m.handleAttachmentsNavigationKey(key); handled {
		return cmd
	}
	if cmd, handled := m.handleAttachmentsOpenKey(key); handled {
		return cmd
	}
	if cmd, handled := m.handleAttachmentsMutationKey(key); handled {
		return cmd
	}
	if key == "?" {
		return helpCmd(ViewTaskAttachments, "Attachments", m.GetHelpItems())
	}

	return nil
}

func (m *TaskAttachmentsModel) handleAttachmentsNavigationKey(key string) (tea.Cmd, bool) {
	switch key {
	case "q", "esc":
		return func() tea.Msg { return CloseTaskAttachmentsMsg{} }, true
	case "j", "down":
		m.moveCursorDown()
		return nil, true
	case "k", "up":
		m.moveCursorUp()
		return nil, true
	case "g":
		m.pendingPrefix = "g"
		return nil, true
	case "G":
		m.moveCursorBottom()
		return nil, true
	default:
		return nil, false
	}
}

func (m *TaskAttachmentsModel) handleAttachmentsOpenKey(key string) (tea.Cmd, bool) {
	switch key {
	case "a":
		return m.beginAddMode(), true
	case "o", "enter":
		return m.openSelectedAttachmentDefault(), true
	case "e":
		return m.openSelectedAttachmentInEditor(), true
	case "ctrl+e":
		return m.openSelectedOriginalPath(), true
	case "ctrl+o":
		return m.revealSelectedOriginalFile(), true
	case "O":
		return m.openAttachmentsDir(), true
	default:
		return nil, false
	}
}

func (m *TaskAttachmentsModel) handleAttachmentsMutationKey(key string) (tea.Cmd, bool) {
	switch key {
	case "d", "x":
		m.confirmDeleteSelectedAttachment()
		return nil, true
	case "u":
		m.confirmUnlinkSelectedAttachment()
		return nil, true
	case "y":
		return m.copySelectedPath(), true
	case "Y":
		return m.copySelectedPathWithLine(), true
	case "J":
		return m.moveSelectedAttachmentDown(), true
	case "K":
		return m.moveSelectedAttachmentUp(), true
	default:
		return nil, false
	}
}

func (m *TaskAttachmentsModel) HandledKeys() []string {
	return []string{
		// Navigation
		"j", "down", "k", "up", "g", "G",
		// Open/view
		"a", "o", "enter", "e", "ctrl+e", "ctrl+o", "O",
		// Mutations
		"d", "x", "u", "y", "Y", "J", "K",
		// Help
		"?",
	}
}

func (m *TaskAttachmentsModel) moveCursorDown() {
	if len(m.task.Attachments) > 0 && m.cursor < len(m.task.Attachments)-1 {
		m.cursor++
	}
}

func (m *TaskAttachmentsModel) moveCursorUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

func (m *TaskAttachmentsModel) moveCursorBottom() {
	if len(m.task.Attachments) > 0 {
		m.cursor = len(m.task.Attachments) - 1
	}
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
		m.confirm.Show("delete", fmt.Sprintf("Delete attachment [%d]?", m.cursor))
	}
}

func (m *TaskAttachmentsModel) confirmUnlinkSelectedAttachment() {
	if m.hasSelection() {
		m.confirm.Show("unlink", fmt.Sprintf("Unlink attachment [%d]?", m.cursor))
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
	if len(m.task.Attachments) > 1 && m.cursor < len(m.task.Attachments)-1 {
		return m.reorderAttachment(m.cursor, m.cursor+1)
	}
	return nil
}

func (m *TaskAttachmentsModel) moveSelectedAttachmentUp() tea.Cmd {
	if m.cursor > 0 {
		return m.reorderAttachment(m.cursor, m.cursor-1)
	}
	return nil
}

func (m *TaskAttachmentsModel) hasSelection() bool {
	return len(m.task.Attachments) > 0 && m.cursor < len(m.task.Attachments)
}
