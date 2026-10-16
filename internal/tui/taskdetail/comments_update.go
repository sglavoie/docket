package taskdetail

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/tui/chrome"

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

func (m *CommentsModel) updateEditMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			if m.hasUnsavedEditChanges() {
				m.Confirm.Show("discard-edit", "Discard changes?")
				return m, nil
			}
			m.cancelEditMode()
			return m, nil
		case "ctrl+s":
			return m, m.saveEditedComment()
		case "ctrl+e":
			return m, m.editCommentInExternalEditorFromEditMode()
		case "ctrl+x":
			m.exportDraft()
			return m, nil
		case "shift+enter", "alt+enter":
			if m.refuseComposerEdit(commentMutationEdit) {
				return m, nil
			}
			m.EditTextInput.InsertString("\n")
			return m, nil
		}
	}

	before := m.EditTextInput.Value()
	m.EditTextInput, cmd = m.EditTextInput.Update(msg)
	if m.restoreReadOnlyComposer(commentMutationEdit, before) {
		return m, nil
	}
	return m, cmd
}

func (m *CommentsModel) updateAddMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			if m.hasUnsavedAddText() {
				m.Confirm.Show("discard-add", "Discard comment?")
				return m, nil
			}
			m.cancelAddMode()
			return m, nil
		case "ctrl+s":
			return m, m.saveComment()
		case "ctrl+e":
			return m, m.addCommentInExternalEditor()
		case "ctrl+x":
			m.exportDraft()
			return m, nil
		case "shift+enter", "alt+enter":
			if m.refuseComposerEdit(commentMutationAdd) {
				return m, nil
			}
			m.TextInput.InsertString("\n")
			return m, nil
		}
	}

	before := m.TextInput.Value()
	m.TextInput, cmd = m.TextInput.Update(msg)
	if m.restoreReadOnlyComposer(commentMutationAdd, before) {
		return m, nil
	}
	return m, cmd
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

// draftPreviewReadOnlyStatus opens the answer to a press that would have changed
// a preview. It says what the box is and names the way out of it that is always
// open, because the alternative to editing here is not "do without": the whole
// draft is one key from a file whatever else has happened to its target.
//
// It is the whole of what the refusal stores, and that is deliberate: this
// sentence is true in every state the panel can reach afterwards. The editor is
// the other way out and it is not always open, so the sentence naming it is
// appended at render by statusEditorGuidance — the same draftEditorGuidance the
// recovery block above the composer is closed with.
const draftPreviewReadOnlyStatus = "Read-only preview of the kept draft; Ctrl+X writes the whole of it to a file."

// pendingSubmissionReadOnlyStatus is the answer to a press that would have
// changed a composer whose text has already been handed to a commit.
//
// The submitted string is what the transaction is measured against and what a
// refusal puts back, so text accepted after the submission belongs to neither
// outcome: a success closes the composer and a failure reseats what was sent, and
// both would carry the newer keystrokes off with them. The window is one commit
// long and the box stays readable inside it, so the press is turned away while it
// still can be rather than accepted into a buffer no outcome will keep.
const pendingSubmissionReadOnlyStatus = "Saving this comment; the draft cannot be edited until the save answers."

// composerReadOnly reports whether the open composer is showing text the user
// cannot change through it. Two states say so and the keymap treats them alike,
// because the row it gates offers to type into the box: a kept draft the box can
// only preview, and a submission the panel is still waiting on. refuseComposerEdit
// answers the same two questions in the same order and adds the reason, so the
// footer and the refusal cannot describe different grammars.
func (m *CommentsModel) composerReadOnly() bool {
	return m.pending != nil || m.showsDraftPreview()
}

// readOnlyRefusalRecord is the sticky line refuseComposerEdit last wrote, kept so
// the render can ask two questions about it that only the present can answer.
//
// stamp is the status generation it landed on, and is what says the line is still
// this refusal rather than something written over it. Every other status the panel
// writes takes a generation of its own through SetStatus, so an exported draft's
// path — or a copy, or a double-submission refusal — displaces the refusal simply
// by being written and is then printed exactly as it was written. Zero is no
// refusal, because the first SetStatus a panel ever performs lands on generation
// one.
//
// preview says which of the two read-only states wrote it. The two do not end the
// same way and only one of them earns the editor sentence, so a single stamp
// cannot stand for both.
type readOnlyRefusalRecord struct {
	stamp   uint64
	preview bool
}

// refuseComposerEdit is the read-only rule for the open composer and the whole of
// it: the one question every route that could change text asks, answered in the
// order the two states nest.
//
// A pending submission is the temporary one and names itself. It is recorded as
// the refusal on screen like the other, and deliberately not as the preview one:
// the keys that sentence would advertise are the ones a pending mutation already
// bounces, so appending guidance about them would name a route this very state has
// closed.
func (m *CommentsModel) refuseComposerEdit(kind commentMutationKind) bool {
	if m.pending != nil {
		m.SetStatus(pendingSubmissionReadOnlyStatus)
		m.readOnlyRefusal = readOnlyRefusalRecord{stamp: m.StatusStamp()}
		return true
	}
	return m.refuseDraftPreviewEdit(kind)
}

// readOnlyRefusalOnScreen reports whether the sticky status line is still the
// refusal the composer wrote rather than a message written after it.
func (m *CommentsModel) readOnlyRefusalOnScreen() bool {
	return m.readOnlyRefusal.stamp == m.StatusStamp()
}

// readOnlyRefusalStillHolds reports whether the state that refusal named is the
// state the panel is in now.
//
// A refusal is a sentence about the present — this box cannot be typed into, and
// here is what to do instead — so it stops being a stale line and starts being a
// false one the moment its state ends. A kept draft ends by being discarded or
// committed, and either way the sentence promising to write it to a file names
// text the panel no longer has; a submission ends by answering, and the composer
// the user opens next is an ordinary editable one that a line calling it read-only
// simply misdescribes. Neither is displaced by a later status, because there is no
// later status: nothing about ending a draft writes one.
func (m *CommentsModel) readOnlyRefusalStillHolds() bool {
	if m.readOnlyRefusal.preview {
		return m.showsDraftPreview()
	}
	return m.pending != nil
}

// restoreReadOnlyComposer undoes a press that changed a read-only composer's box
// and says why. It is the enforcement of that rule itself.
//
// A preview is a rendering of text the panel is holding, not the text: the box
// stopped at CommentCharLimit and rewrote every tab, so the runes past the limit
// and the interior tabs are on nobody's screen. Letting an ordinary edit stand
// would hand the box authority over a save — and with it the power to drop, in
// silence, every character the user never saw. Deleting one visible character is
// not an instruction to do that, so the press is refused rather than obeyed, and
// the two acts that do replace the whole draft say so before they run.
//
// A composer whose text is in flight is refused here for the mirror-image reason:
// the box would keep the change and no outcome of the commit would, so obeying the
// press is what loses it.
//
// The check is after the fact on purpose: the textarea's own keymap decides
// which presses change text, and asking it by running the press keeps cursor
// movement, scrolling, and selection working inside a box the user can still
// read.
func (m *CommentsModel) restoreReadOnlyComposer(kind commentMutationKind, before string) bool {
	if m.composerValue(kind) == before {
		return false
	}
	if !m.refuseComposerEdit(kind) {
		return false
	}
	m.setComposerValue(kind, before)
	return true
}

// refuseDraftPreviewEdit reports the read-only refusal for a kept draft the
// composer showing it cannot hold — and is what refuseComposerEdit asks whether
// this composer is previewing at all.
//
// A status message is sticky, so what it stores has to stay true for as long as
// it is on screen. The target a refused draft still had when the box turned a
// keystroke away can be deleted or taken away a moment later, and nothing about
// that loss rewrites a line already written. So only the unconditional half is
// stored; the record beside it is what lets the render recognize this line later
// and measure the rest — the editor sentence, and whether to print the line at
// all — against the state it is being drawn in.
func (m *CommentsModel) refuseDraftPreviewEdit(kind commentMutationKind) bool {
	if _, previewed := m.heldDraftPreview(kind); !previewed {
		return false
	}
	m.SetStatus(draftPreviewReadOnlyStatus)
	m.readOnlyRefusal = readOnlyRefusalRecord{stamp: m.StatusStamp(), preview: true}
	return true
}

// recoverableDraftTexts names every copy of one composer's draft that closing it
// would destroy: the textarea's own value, and the verbatim copy the panel is
// holding beside it while a recovered draft is outstanding.
//
// The two are not interchangeable, which is the whole reason the second one
// exists. The box is a quick-entry textarea — it stops at CommentCharLimit and
// rewrites tabs as four spaces — so seating a recovered draft into it can land on
// exactly the text the user was replacing, or on nothing but whitespace, while
// the copy beside it still holds work that exists nowhere else in the process.
// A discard guard reading the box alone would then answer a question about the
// wrong string and drop the right one without asking.
//
// A previewed draft is the one case where the box is not a second copy at all
// but a rendering of the first, since every press that would change it is undone
// by restoreReadOnlyComposer. Counting it there would ask about a truncation the user
// never made — which is exactly what opening an inline edit over a comment too
// long or too tabbed for the box produces, before they have touched anything.
func (m *CommentsModel) recoverableDraftTexts(kind commentMutationKind) []string {
	if text, previewed := m.heldDraftPreview(kind); previewed {
		return []string{text}
	}
	composer := m.composerValue(kind)
	if text, ok := m.retainedDraftText(kind); ok && text != composer {
		return []string{composer, text}
	}
	return []string{composer}
}

// hasUnsavedAddText reports whether closing the add composer would cost the user
// text. A blank box is not an empty draft: what the panel retained behind it may
// be a truncation away from text a save would still write.
func (m *CommentsModel) hasUnsavedAddText() bool {
	for _, text := range m.recoverableDraftTexts(commentMutationAdd) {
		if strings.TrimSpace(text) != "" {
			return true
		}
	}
	return false
}

// hasUnsavedEditChanges compares everything the panel can still recover with the
// text the editor was opened on rather than with whatever the list now holds at
// that position. What the user stands to lose is the difference from what they
// started editing, and that is true whether or not the underlying list moved
// while they typed, and whether or not the composer is showing all of it.
func (m *CommentsModel) hasUnsavedEditChanges() bool {
	original := strings.TrimSpace(m.editRef.Original)
	for _, text := range m.recoverableDraftTexts(commentMutationEdit) {
		if strings.TrimSpace(text) != original {
			return true
		}
	}
	return false
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

// cancelAddMode and cancelEditMode close a composer and drop the copy of its
// draft the panel was holding for it. These are the only two routes that reach
// that copy, and both are deliberate: the discard confirmation answers yes, or a
// commit confirms and there is nothing left to recover. A refusal, a reload, and
// a refresh all leave it exactly where it is.
func (m *CommentsModel) cancelAddMode() {
	m.AddMode = false
	m.TextInput.Reset()
	m.forgetRetainedDraft(commentMutationAdd)
}

func (m *CommentsModel) cancelEditMode() {
	m.EditMode = false
	m.EditTextInput.Reset()
	m.forgetRetainedDraft(commentMutationEdit)
}

// QuitBlockReason names content a desktop quit would abandon.
func (m *CommentsModel) QuitBlockReason() string {
	if m.pending != nil {
		return "A comment change is still being saved."
	}
	if (m.AddMode && m.hasUnsavedAddText()) || (m.EditMode && m.hasUnsavedEditChanges()) {
		return "Your comment draft has not been saved."
	}
	return ""
}

func (m *CommentsModel) DesktopTextBuffer() *chrome.DesktopTextBuffer {
	if m.pending != nil || m.Confirm.IsActive() || m.ExpandedView || m.composerReadOnly() {
		return nil
	}
	if m.AddMode {
		return chrome.DesktopTextarea(&m.TextInput, nil)
	}
	if m.EditMode {
		return chrome.DesktopTextarea(&m.EditTextInput, nil)
	}
	return nil
}
