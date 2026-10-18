package taskdetail

import (
	"fmt"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/help"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/status"
)

// Update routes one message. The four commit-lifecycle messages are answered
// before the sub-state switch because they are outcomes rather than input: a
// transaction that confirms while the user is typing in the composer still has
// to land, and one that arrives for another owner or an abandoned submission
// still has to be dropped.
func (m *CommentsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case commentEditorFinishedMsg:
		return m, m.applyCommentEditorResult(msg)
	case CommentAddedMsg:
		m.completeCommentMutation(msg.Owner, msg.Token, msg.CommentID)
		return m, nil
	case CommentEditedMsg:
		m.completeCommentMutation(msg.Owner, msg.Token, "")
		return m, nil
	case CommentDeletedMsg:
		m.completeCommentMutation(msg.Owner, msg.Token, "")
		return m, nil
	case CommentMutationFailedMsg:
		return m, m.failCommentMutation(msg)
	}

	switch {
	case m.ExpandedView:
		return m.updateExpandedView(msg)
	case m.Confirm.IsActive():
		return m.updateConfirmMode(msg)
	case m.EditMode:
		return m.updateEditMode(msg)
	case m.AddMode:
		return m.updateAddMode(msg)
	default:
		return m.updateNormalMode(msg)
	}
}

// updateExpandedView is the reader's whole grammar: the two keys that close it
// and the keys that move its window.
//
// The window exists because the panel is cropped to the terminal and a comment is
// not. Before these keys the reader drew the entire body and the shell cut
// whatever did not fit, which meant a long comment had a tail no key could reach
// — including, at a short enough terminal, the line naming the keys that close
// the reader. So every distance below is measured against the same window the
// render draws, and each one is clamped rather than wrapped: the ends of a
// comment are ends, not a cycle.
func (m *CommentsModel) updateExpandedView(msg tea.Msg) (tea.Model, tea.Cmd) {
	press, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	key := press.String()
	if m.completeExpandedTopChord(key) {
		return m, nil
	}

	switch key {
	case "esc", "enter":
		m.closeExpandedView()
	case "g":
		m.pendingTopChord = true
	case "j", "down":
		m.scrollExpanded(1)
	case "k", "up":
		m.scrollExpanded(-1)
	case "ctrl+d", "pgdown":
		m.scrollExpanded(m.expandedScrollDistance(expandedHalfPage))
	case "ctrl+u", "pgup":
		m.scrollExpanded(-m.expandedScrollDistance(expandedHalfPage))
	case "ctrl+f":
		m.scrollExpanded(m.expandedScrollDistance(expandedFullPage))
	case "ctrl+b":
		m.scrollExpanded(-m.expandedScrollDistance(expandedFullPage))
	case "home":
		m.expandedOffset = 0
	case "G", "end":
		m.scrollExpandedToEnd()
	}

	return m, nil
}

// The two paging distances. A half page is half the window and a full page is
// the window less one row, so paging forward and back leaves one line of what
// was on screen and a reader never has to guess whether they skipped something.
const (
	expandedHalfPage = true
	expandedFullPage = false
)

// closeExpandedView leaves the reader and forgets where it was. Reopening a
// comment opens it at the top, because the press that opens one is Enter on a
// row rather than a return to something suspended.
func (m *CommentsModel) closeExpandedView() {
	m.ExpandedView = false
	m.expandedOffset = 0
	m.pendingTopChord = false
}

// completeExpandedTopChord answers the second g of gg inside the reader. It is
// the panel's browse chord spelled against the body instead of the list, which
// is the same act — go to the beginning of what is in front of you — and an
// abandoned prefix is silent here exactly as it is there.
func (m *CommentsModel) completeExpandedTopChord(key string) bool {
	if !m.pendingTopChord {
		return false
	}
	m.pendingTopChord = false
	if key != "g" {
		return false
	}
	m.expandedOffset = 0
	return true
}

// scrollExpanded is the only writer of the reader's offset, which is what keeps
// a render from moving the window the reader put where it is.
func (m *CommentsModel) scrollExpanded(rows int) {
	if !m.HasSelection() {
		return
	}
	m.expandedOffset = m.expandedReaderWindow().clamp(m.expandedOffset + rows)
}

func (m *CommentsModel) scrollExpandedToEnd() {
	if !m.HasSelection() {
		return
	}
	m.expandedOffset = m.expandedReaderWindow().maxOffset()
}

// expandedScrollDistance reads the distance off the window the frame is drawn
// at, so a resized terminal pages by what it now shows rather than by a constant
// taken against a size it no longer has.
func (m *CommentsModel) expandedScrollDistance(half bool) int {
	if !m.HasSelection() {
		return 1
	}
	visible := m.expandedReaderWindow().visible
	if half {
		return max(visible/2, 1)
	}
	return max(visible-1, 1)
}

func (m *CommentsModel) updateConfirmMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		action := m.Confirm.Action()
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
			// A key that dismissed the prompt answered the question, so a frozen
			// delete target it did not confirm is dropped here rather than waiting
			// for some later prompt to overwrite it. Any other key was blocked by the
			// dialog, which is still up, and the target it named still stands.
			if !m.Confirm.IsActive() {
				m.forgetDeleteConfirmation()
			}
			m.ClearError()
			return m, nil
		}
	}

	return m, nil
}

func (m *CommentsModel) updateNormalMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case status.CursorFlashClearMsg:
		m.CursorFlash = false
		return m, nil
	case tea.WindowSizeMsg:
		m.BaseView.SetSize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyPressMsg:
		return m, m.HandleNormalKey(msg.String())
	}

	return m, nil
}

func (m *CommentsModel) HandleNormalKey(key string) tea.Cmd {
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
		return help.HelpCmd("Comments", m.GetHelpItems())
	}
	// The core undo descriptors answer only to the Outliner, so the press asks the
	// shell for the one workspace ledger rather than dispatching. Update has
	// already routed the add, edit, expanded, and confirm states elsewhere, so u is
	// never a character being typed into a comment here.
	if key == "u" || key == "ctrl+r" {
		return status.WorkspaceUndoCmd(key == "ctrl+r")
	}

	return nil
}

// handleCommentsNavigationKey moves the panel's own cursor. The keys it names
// are the panel's whole navigation grammar and HandleListNavigation performs
// them, so listing them here rather than handing the helper every press is what
// keeps the paging keys the panel never bound from arriving through the back
// door.
func (m *CommentsModel) handleCommentsNavigationKey(key string) (tea.Cmd, bool) {
	switch key {
	case "esc":
		return func() tea.Msg { return CloseTaskCommentsMsg{} }, true
	case "g":
		m.pendingTopChord = true
		return nil, true
	case "j", "down", "k", "up", "home", "G", "end":
		_, cmd := listsurface.HandleListNavigation(key, &m.Cursor, len(m.entries), m.Height(), &m.CursorFlash)
		return cmd, true
	default:
		return nil, false
	}
}

// completeCommentsTopChord answers the second g of gg and reports the press as
// consumed. Any other key abandons the prefix silently and is then handled
// normally, so a stray g never swallows the key after it.
func (m *CommentsModel) completeCommentsTopChord(key string) bool {
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

// handleCommentsGrammarKey routes the keys that act on a comment through the
// registry. Navigation and Esc stay above it because the panel performs those
// itself; every key listed here reaches its operation descriptor instead, so the
// key pressed and the descriptor that names it can never drift apart. Space is
// spelled out because a bare space key and the name the grammar binds describe
// the same act differently.
func (m *CommentsModel) handleCommentsGrammarKey(key string) (tea.Cmd, bool) {
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
func (m *CommentsModel) dispatchCommentsKey(key string) tea.Cmd {
	if m.Dispatch == nil {
		return nil
	}
	_, cmd := m.Dispatch(operation.InputStrategyComments, []string{key}, m.commentsCommandContext())
	return cmd
}

// commentsCommandContext names the panel rather than a comment: a comment
// carries no entity identity, so the cursor the handler reads is the only thing
// that says which one an operation means.
func (m *CommentsModel) commentsCommandContext() operation.CommandContext {
	ctx := route.PresentationCommandContext(presentation.ViewTaskComments)
	ctx.Capabilities.Add(operation.CapabilityCommentsPanel)
	return ctx
}

func (m *CommentsModel) HandledKeys() []string {
	return m.BuildKeyMap().HandledKeys()
}

func (m *CommentsModel) HasSelection() bool {
	return len(m.entries) > 0 && m.Cursor >= 0 && m.Cursor < len(m.entries)
}

func (m *CommentsModel) BeginAddMode() tea.Cmd {
	if m.refuseWhilePending() {
		return nil
	}
	m.AddMode = true
	m.TextInput.Focus()
	return textarea.Blink
}

func (m *CommentsModel) StartEditSelectedComment() tea.Cmd {
	if !m.HasSelection() || m.refuseWhilePending() {
		return nil
	}
	return m.startEditComment()
}

func (m *CommentsModel) EditSelectedCommentInExternalEditor() tea.Cmd {
	if !m.HasSelection() {
		return nil
	}
	return m.editInExternalEditor()
}

func (m *CommentsModel) CopySelectedCommentText() {
	if m.HasSelection() {
		m.copyCommentText()
	}
}

// ConfirmDeleteSelectedComment asks about the comment under the cursor and
// freezes it in the same act. The row number in the prompt is what the user is
// looking at; the frozen reference is what the answer will address.
func (m *CommentsModel) ConfirmDeleteSelectedComment() {
	if !m.HasSelection() || m.refuseWhilePending() {
		return
	}
	ref, ok := m.commentRefAt(m.Cursor)
	if !ok {
		return
	}
	m.armDeleteConfirmation(ref)
	m.Confirm.Show("delete", fmt.Sprintf("Delete comment [%d]?", m.Cursor))
}

func (m *CommentsModel) ExpandSelectedComment() {
	if m.HasSelection() {
		m.ExpandedView = true
		m.expandedOffset = 0
	}
}
