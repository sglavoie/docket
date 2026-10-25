package taskdetail

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sglavoie/docket/internal/external"
)

// draftRecovery is one comment draft held verbatim beside the composer showing
// it, which is the only way the panel can promise to give it back.
//
// The composer is not that copy and cannot be made into one: its textarea
// rewrites tabs as spaces and stops accepting runes at CommentCharLimit, both of
// which are right for something being typed and wrong for something that arrived
// whole. Three routes put text there the box cannot vouch for — an external
// editor's buffer, a submission coming back from a refused commit, and the stored
// comment an inline edit is opened over — and the first two have no other copy at
// all, the editor's temporary file being deleted before its result ever reaches
// the panel. The third does still exist in the workspace, which is precisely why
// it has to be held: a save measured against the whole original would otherwise
// be allowed to overwrite it with the part of it the box could show.
type draftRecovery struct {
	// kind names the composer the text belongs to, so an add draft can never be
	// submitted as an edit or recovered under one.
	kind commentMutationKind
	// text is the draft exactly as it arrived — no limit, no sanitization.
	text string
	// seated is what the composer holds once that text has been put into it.
	// While the two still agree, the user has not touched the composer and the
	// verbatim text is what a save writes; once they diverge, the user is editing
	// and the composer is theirs.
	seated string
}

// retainAddDraft and retainEditDraft seat a draft in the composer that owns it
// and record the verbatim text beside it. They are the capture half of that
// recovery: whichever way the submission then goes, the whole text is somewhere
// the View draws, a save can still write, and the discard grammar can reach. A
// confirmed commit closes the composer through completeCommentMutation, so the
// capture costs a successful edit nothing.
//
// Every route that puts text into a composer it did not type goes through one of
// these two, opening an inline edit included. Seating a textarea and recording
// what it took are one act, so there is no order in which a composer holds text
// the panel has not measured itself against.
func (m *CommentsModel) retainAddDraft(text string) {
	m.EditMode = false
	m.AddMode = true
	m.TextInput.SetValue(text)
	m.TextInput.Focus()
	m.recovered = &draftRecovery{kind: commentMutationAdd, text: text, seated: m.TextInput.Value()}
}

func (m *CommentsModel) retainEditDraft(ref CommentRef, text string) {
	m.AddMode = false
	m.EditMode = true
	m.editRef = ref
	m.EditTextInput.SetValue(text)
	m.EditTextInput.Focus()
	m.recovered = &draftRecovery{kind: commentMutationEdit, text: text, seated: m.EditTextInput.Value()}
}

// retainedDraftText is the verbatim draft being held for one composer.
func (m *CommentsModel) retainedDraftText(kind commentMutationKind) (string, bool) {
	if m.recovered == nil || m.recovered.kind != kind {
		return "", false
	}
	return m.recovered.text, true
}

// heldDraftPreview is a retained draft the composer showing it could not take
// whole, which is the state the composer stops being a buffer in.
//
// The seating is what decides it: a draft that went into the textarea unchanged
// is on screen in full and the box is the user's ordinary editor for it, while
// one the box truncated or flattened is being previewed rather than held. The
// difference matters to every reader of a draft, because in the second case what
// the panel writes, exports, and asks about is the copy beside the box and not
// the box — and the keys that would edit the box are refused for exactly that
// reason.
func (m *CommentsModel) heldDraftPreview(kind commentMutationKind) (string, bool) {
	if m.recovered == nil || m.recovered.kind != kind || m.recovered.text == m.recovered.seated {
		return "", false
	}
	return m.recovered.text, true
}

// showsDraftPreview reports whether the open composer is previewing a kept draft
// rather than holding an editable one. The keymap reads it, because a footer
// offering to insert a newline into a box that refuses the press describes a
// grammar the panel does not have.
func (m *CommentsModel) showsDraftPreview() bool {
	_, held := m.heldDraftPreview(m.openComposerKind())
	return held
}

// draftEditorRouteHint and draftEditorRouteUnavailableHint are what the panel
// adds about Ctrl+E wherever it explains that a composer has become a read-only
// preview. There is one string per state and both are shared by the recovery
// block and the refusal status, because those two are the same claim printed in
// two places: if they could disagree, one of them would still be advertising an
// act the other had already withdrawn.
const (
	draftEditorRouteHint            = " Ctrl+E opens it in an external editor."
	draftEditorRouteUnavailableHint = " Ctrl+E needs a target this draft no longer has."
)

// draftEditorGuidance is that sentence for one composer, and it is the whole
// reason the availability below is asked at all.
//
// A retained draft is the state in which the panel has the least to offer and
// says the most: the box under it cannot be edited, the save has already been
// refused, and the two keys named are all that is left. Naming a key that
// answers with a refusal spends the user's one remaining move on nothing, so the
// editor half is written from what that key would actually do and the export
// half — which needs nothing from the workspace — is stated unconditionally
// beside it.
func (m *CommentsModel) draftEditorGuidance(kind commentMutationKind) string {
	if m.externalDraftEditorAvailable(kind) {
		return draftEditorRouteHint
	}
	return draftEditorRouteUnavailableHint
}

// externalDraftEditorAvailable reports whether Ctrl+E over the composer showing
// a kept draft could still reach an editor.
//
// It asks exactly what that key's own route asks and nothing more. The add
// editor needs an owner to head its buffer with; the edit editor needs both an
// owner the workspace still holds and the comment the composer was opened on,
// and refuses before creating a buffer when either is gone. A retained draft
// outlives both — that is the case the retention exists for — so this is the one
// question the guidance beside it cannot assume the answer to.
//
// The list it measures the comment against is the panel's own, which is the last
// one it could read: the guidance speaks for what is on screen, and a row the
// user can still see is a row Ctrl+E will still resolve.
func (m *CommentsModel) externalDraftEditorAvailable(kind commentMutationKind) bool {
	if m.owner == nil {
		return false
	}
	if kind == commentMutationAdd {
		return true
	}
	return m.owner.Available() && m.indexOfRef(m.editRef) >= 0
}

// openComposerKind names the composer a press or an act is addressed to. Only
// one is ever open, and Update routes edit before add, so the two answers cannot
// disagree.
func (m *CommentsModel) openComposerKind() commentMutationKind {
	if m.EditMode {
		return commentMutationEdit
	}
	return commentMutationAdd
}

// forgetRetainedDraft drops the held copy when its composer closes. Only an
// explicit discard, a confirmed commit, and leaving the panel reach it, which is
// what makes the retention outlast every refusal and every reload in between.
func (m *CommentsModel) forgetRetainedDraft(kind commentMutationKind) {
	if m.recovered != nil && m.recovered.kind == kind {
		m.recovered = nil
	}
}

// submissionText is the text a save from one composer commits.
//
// Typed input is trimmed the way it always has been. A retained draft is written
// exactly as it arrived instead, because the composer value beside it may be a
// truncated, tab-flattened rendering of it — submitting that on a retry would
// quietly persist the damage the retention exists to prevent.
//
// A preview keeps that authority unconditionally, and the box is read-only for
// as long as it does: the runes past the limit and the interior tabs are on
// nobody's screen, so an ordinary press over the visible part is not consent to
// drop them, and the only routes that end the retention say so first. A draft
// the composer did take whole is a different case — every rune of it is on
// screen and editable, so typing hands authority back to the box, which is then
// showing exactly what the user means.
func (m *CommentsModel) submissionText(kind commentMutationKind, composer string) string {
	text, ok := m.retainedDraftText(kind)
	if !ok {
		return strings.TrimSpace(composer)
	}
	if _, previewed := m.heldDraftPreview(kind); previewed || m.recovered.seated == composer {
		return text
	}
	return strings.TrimSpace(composer)
}

// draftExportStatusPrefix heads the line naming the file a draft was written to.
// It is a constant because the path is the whole point of the act: the panel
// tells the user where their text is, and that sentence is the only thing
// standing between a recovered draft and a user who can open it.
const draftExportStatusPrefix = "Draft written to "

// exportDraft writes the draft the open composer is holding to a file of its own
// and names that file on screen.
//
// It is the one act in this panel that asks the owner for nothing. A draft whose
// comment was deleted or whose project disappeared can be saved nowhere, and the
// composer showing it is a quick-entry box that stops at CommentCharLimit and
// rewrites tabs — so the interior of a long recovered draft is held by the
// process and reachable by nobody, the recovery block above it being an excerpt
// rather than the text. Writing a file is what makes the whole of it the user's
// again while the reason it could not be saved stays exactly where it was: no
// owner is resolved, no mutation is submitted, no comment is stored, and the
// composer, the retained copy, and the refusal on screen are all left alone.
//
// The bytes are the ones a save would commit, through the same submissionText
// authority rule, so the file and the panel can never disagree about what the
// draft is.
func (m *CommentsModel) exportDraft() {
	kind := m.openComposerKind()
	text := m.submissionText(kind, m.composerValue(kind))
	if text == "" {
		m.Err = errors.New("there is no draft text to write")
		return
	}
	path, err := external.CreateTempFileWithContent(text, "tt-comment-draft-", ".md")
	if err != nil {
		m.Err = fmt.Errorf("failed to write the draft: %w", err)
		return
	}
	m.SetStatus(draftExportStatusPrefix + path)
}

// composerValue is what the textarea for one kind is currently showing.
func (m *CommentsModel) composerValue(kind commentMutationKind) string {
	if kind == commentMutationAdd {
		return m.TextInput.Value()
	}
	return m.EditTextInput.Value()
}

// setComposerValue writes one textarea back. It exists for the read-only
// composer, which is enforced by undoing whatever a press did rather than by
// naming the presses that do something: the textarea's own grammar decides which
// keys change text, and a list of them kept here would be a second copy of it.
func (m *CommentsModel) setComposerValue(kind commentMutationKind, text string) {
	if kind == commentMutationAdd {
		m.TextInput.SetValue(text)
		return
	}
	m.EditTextInput.SetValue(text)
}

// lossyRetainedDraft is the retained draft whose composer could not take all of
// it, and is what the recovery block in the View exists to draw. A draft the
// composer holds in full needs no second rendering: it is already on screen,
// editable, and identical.
//
// It reads the seating rather than the box the way every other preview question
// does, so the block and the read-only rule can never disagree about which state
// the composer is in.
func (m *CommentsModel) lossyRetainedDraft() (string, bool) {
	if m.recovered == nil {
		return "", false
	}
	return m.heldDraftPreview(m.recovered.kind)
}
