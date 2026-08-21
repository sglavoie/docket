package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

func (m *TaskAttachmentsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocyclo
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
	if cmd, handled := m.handleAttachmentsGrammarKey(key); handled {
		return cmd
	}
	if key == "?" {
		return helpCmd(ViewTaskAttachments, "Attachments", m.GetHelpItems())
	}

	return nil
}

func (m *TaskAttachmentsModel) handleAttachmentsNavigationKey(key string) (tea.Cmd, bool) {
	switch key {
	case "esc":
		return func() tea.Msg { return CloseTaskAttachmentsMsg{} }, true
	case "j", "down":
		m.moveCursorDown()
		return nil, true
	case "k", "up":
		m.moveCursorUp()
		return nil, true
	case "home":
		m.cursor = 0
		return nil, true
	case "G":
		m.moveCursorBottom()
		return nil, true
	default:
		return nil, false
	}
}

// handleAttachmentsGrammarKey routes the keys that act on an attachment through
// the registry. Navigation, `?` and Esc stay above and below it because the
// panel performs those itself; every key listed here reaches its operation
// descriptor instead, so the Space sheet and the key pressed today can never
// describe the same act differently. Space is spelled out because a bare space
// rune normalizes to nothing.
func (m *TaskAttachmentsModel) handleAttachmentsGrammarKey(key string) (tea.Cmd, bool) {
	switch key {
	case "a", "enter", "e", "y", "Y", "J", "K":
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
	_, cmd := m.dispatch(InputStrategyAttachments, []string{key}, m.attachmentsCommandContext())
	return cmd
}

// attachmentsCommandContext names the panel rather than an item: an attachment
// carries no entity identity, so the cursor the handler reads is the only thing
// that says which item an operation means.
func (m *TaskAttachmentsModel) attachmentsCommandContext() CommandContext {
	ctx := PresentationCommandContext(ViewTaskAttachments)
	ctx.Capabilities.Add(CapabilityAttachmentsPanel)
	return ctx
}

func (m *TaskAttachmentsModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
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
