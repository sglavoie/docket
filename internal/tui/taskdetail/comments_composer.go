package taskdetail

import (
	"strings"

	"github.com/sglavoie/tt/internal/tui/chrome"

	tea "charm.land/bubbletea/v2"
)

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
