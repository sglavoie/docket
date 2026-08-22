package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
)

func (m *TaskCommentsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch {
	case m.expandedView:
		return m.updateExpandedView(msg)
	case m.confirm.IsActive():
		return m.updateConfirmMode(msg)
	case m.editMode:
		return m.updateEditMode(msg)
	case m.addMode:
		return m.updateAddMode(msg)
	default:
		return m.updateNormalMode(msg)
	}
}

func (m *TaskCommentsModel) updateExpandedView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "enter":
			m.expandedView = false
		}
	}

	return m, nil
}

func (m *TaskCommentsModel) updateConfirmMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		action := m.confirm.Action()
		if confirmed, handled := m.HandleConfirmKey(msg.String()); handled {
			if confirmed {
				switch action {
				case "delete":
					return m, m.executeDelete()
				case "discard-add":
					m.cancelAddMode()
					return m, nil
				case "discard-edit":
					m.cancelEditMode()
					return m, nil
				}
			}
			m.ClearError()
			return m, nil
		}
	}

	return m, nil
}

func (m *TaskCommentsModel) updateEditMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			if m.hasUnsavedEditChanges() {
				m.confirm.Show("discard-edit", "Discard changes?")
				return m, nil
			}
			m.cancelEditMode()
			return m, nil
		case "ctrl+s":
			return m, m.saveEditedComment()
		case "ctrl+e":
			return m, m.editCommentInExternalEditorFromEditMode()
		case "shift+enter":
			m.editTextInput.InsertString("\n")
			return m, nil
		}
	}

	m.editTextInput, cmd = m.editTextInput.Update(msg)
	return m, cmd
}

func (m *TaskCommentsModel) updateAddMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			if strings.TrimSpace(m.textInput.Value()) != "" {
				m.confirm.Show("discard-add", "Discard comment?")
				return m, nil
			}
			m.cancelAddMode()
			return m, nil
		case "ctrl+s":
			return m, m.saveComment()
		case "ctrl+e":
			return m, m.addCommentInExternalEditor()
		case "shift+enter":
			m.textInput.InsertString("\n")
			return m, nil
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m *TaskCommentsModel) updateNormalMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	case tea.WindowSizeMsg:
		m.BaseView.SetSize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyPressMsg:
		return m, m.handleNormalKey(msg.String())
	}

	return m, nil
}

func (m *TaskCommentsModel) handleNormalKey(key string) tea.Cmd {
	if m.completeCommentsTopChord(key) {
		return nil
	}
	if cmd, handled := m.handleCommentsNavigationKey(key); handled {
		return cmd
	}
	if cmd, handled := m.handleCommentsGrammarKey(key); handled {
		return cmd
	}
	// Last on purpose: Update routes the add, edit, expanded, and confirm states
	// elsewhere, so a trailing `?` is a browse-only press and stays a literal
	// character inside a comment being typed. The core help descriptor answers only
	// to the Outliner, so raising it through the executor would render the Outliner
	// grammar; the panel names its own instead.
	if key == "?" {
		return helpCmd(ViewTaskComments, "Comments", m.GetHelpItems())
	}

	return nil
}

// handleCommentsNavigationKey moves the panel's own cursor. The keys it names
// are the panel's whole navigation grammar and handleListNavigation performs
// them, so listing them here rather than handing the helper every press is what
// keeps the paging keys the panel never bound from arriving through the back
// door.
func (m *TaskCommentsModel) handleCommentsNavigationKey(key string) (tea.Cmd, bool) {
	switch key {
	case "esc":
		return func() tea.Msg { return CloseTaskCommentsMsg{} }, true
	case "g":
		m.pendingTopChord = true
		return nil, true
	case "j", "down", "k", "up", "home", "G", "end":
		_, cmd := handleListNavigation(key, &m.cursor, len(m.task.Comments), m.height, &m.cursorFlash)
		return cmd, true
	default:
		return nil, false
	}
}

// completeCommentsTopChord answers the second g of gg and reports the press as
// consumed. Any other key abandons the prefix silently and is then handled
// normally, so a stray g never swallows the key after it.
func (m *TaskCommentsModel) completeCommentsTopChord(key string) bool {
	if !m.pendingTopChord {
		return false
	}
	m.pendingTopChord = false
	if key != "g" {
		return false
	}
	m.cursor = 0
	return true
}

// handleCommentsGrammarKey routes the keys that act on a comment through the
// registry. Navigation and Esc stay above it because the panel performs those
// itself; every key listed here reaches its operation descriptor instead, so the
// key pressed and the descriptor that names it can never drift apart. Space is
// spelled out because a bare space key and the name the grammar binds describe
// the same act differently.
func (m *TaskCommentsModel) handleCommentsGrammarKey(key string) (tea.Cmd, bool) {
	switch key {
	case "enter", "a", "e", "E", "y", "Y":
		return m.dispatchCommentsKey(key), true
	case " ", "space":
		return m.dispatchCommentsKey("space"), true
	default:
		return nil, false
	}
}

// dispatchCommentsKey resolves one browse key through the Comments grammar and
// runs whatever operation it reaches.
func (m *TaskCommentsModel) dispatchCommentsKey(key string) tea.Cmd {
	if m.dispatch == nil {
		return nil
	}
	_, cmd := m.dispatch(InputStrategyComments, []string{key}, m.commentsCommandContext())
	return cmd
}

// commentsCommandContext names the panel rather than a comment: a comment
// carries no entity identity, so the cursor the handler reads is the only thing
// that says which one an operation means.
func (m *TaskCommentsModel) commentsCommandContext() CommandContext {
	ctx := PresentationCommandContext(ViewTaskComments)
	ctx.Capabilities.Add(CapabilityCommentsPanel)
	return ctx
}

func (m *TaskCommentsModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

func (m *TaskCommentsModel) hasSelection() bool {
	return len(m.task.Comments) > 0 && m.cursor < len(m.task.Comments)
}

func (m *TaskCommentsModel) hasUnsavedEditChanges() bool {
	return m.editIndex < len(m.task.Comments) && strings.TrimSpace(m.editTextInput.Value()) != strings.TrimSpace(m.task.Comments[m.editIndex].Text)
}

func (m *TaskCommentsModel) beginAddMode() tea.Cmd {
	m.addMode = true
	m.textInput.Focus()
	return textarea.Blink
}

func (m *TaskCommentsModel) startEditSelectedComment() tea.Cmd {
	if !m.hasSelection() {
		return nil
	}
	return m.startEditComment()
}

func (m *TaskCommentsModel) editSelectedCommentInExternalEditor() tea.Cmd {
	if !m.hasSelection() {
		return nil
	}
	return m.editInExternalEditor()
}

func (m *TaskCommentsModel) copySelectedCommentText() {
	if m.hasSelection() {
		m.copyCommentText()
	}
}

func (m *TaskCommentsModel) confirmDeleteSelectedComment() {
	if m.hasSelection() {
		m.confirm.Show("delete", fmt.Sprintf("Delete comment [%d]?", m.cursor))
	}
}

func (m *TaskCommentsModel) expandSelectedComment() {
	if m.hasSelection() {
		m.expandedView = true
	}
}

func (m *TaskCommentsModel) cancelAddMode() {
	m.addMode = false
	m.textInput.Reset()
}

func (m *TaskCommentsModel) cancelEditMode() {
	m.editMode = false
	m.editTextInput.Reset()
}
