package taskdetail

import (
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/help"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// CommentCharLimit bounds a comment however it is typed. The Comments panel
// owns the number because it owns comments; the Outliner's quick Add Comment
// popup reads it from here rather than restating it, so the two routes cannot
// promise different lengths.
const CommentCharLimit = 2000

// addComposerRows and editComposerRows are the heights the two composers are
// drawn at wherever the panel has room for them, and composerMinRows the floor
// they shrink to where it does not. The floor exists because a composer is the
// one part of the panel that cannot yield entirely: a box with no rows is
// indistinguishable from a lost draft.
const (
	addComposerRows  = 4
	editComposerRows = 6
	composerMinRows  = 2
)

// CommentsModel displays and manages one owner's comments. It reaches the
// commented entity only through its CommentOwner, so the same panel serves any
// owner an adapter exists for and none of its rendering, navigation, or
// grammar knows what kind of entity it is drawing.
type CommentsModel struct {
	listsurface.BaseView
	listsurface.InteractiveViewState

	// owner is the adapter this panel was mounted for and never changes; entries
	// is its comments as of the last projection. The View reads entries alone,
	// so a command goroutine can never observe a half-applied list.
	owner   CommentOwner
	entries []CommentEntry

	// pending is the one submitted-but-unconfirmed mutation. Its presence is
	// what refuses a second submission, and its token and owner key are what let
	// an outcome arriving after the user moved on be recognized as stale.
	pending *commentMutation

	// Add mode
	AddMode   bool
	TextInput textarea.Model

	// Edit mode. editRef is the comment the open editor was opened on, captured
	// when it opened: the list can reorder or reload while the editor is up, so
	// the row the user started from is a stored reference and never the cursor.
	EditMode      bool
	editRef       CommentRef
	EditTextInput textarea.Model

	// recovered is the draft the open composer is showing, held exactly as it
	// arrived. It is nil for text the user typed here, because a textarea can
	// always give typed text back; it is set whenever a draft reached the panel
	// from somewhere the textarea cannot faithfully hold — an external editor's
	// buffer, a submission coming back from a refused commit, or the stored
	// comment an inline edit was opened over.
	recovered *draftRecovery

	// readOnlyRefusal is the last read-only refusal the composer wrote, and is
	// the whole of what says the line on screen is still that refusal and still
	// true of the panel drawing it.
	readOnlyRefusal readOnlyRefusalRecord

	// deleteRef is the comment an open delete confirmation was opened on, frozen
	// the moment the question appeared for the same reason editRef is frozen: the
	// list can reload or reorder while the prompt is on screen, and a removal
	// resolved from the cursor at the moment the user answers would delete
	// whichever comment had slid under it. It is nil when no confirmation is
	// outstanding, so a delete can never fall back to a position.
	deleteRef *CommentRef

	// Expanded view mode (show full comment). expandedOffset is the first body
	// row that reader is showing, which is a row index into the rendered comment
	// and not a page ordinal, so the one-row and paging distances compose instead
	// of each snapping to a grid of its own. Only the scrolling keys write it; a
	// render clamps a copy and leaves the field alone, so no redraw moves the
	// window under the reader.
	ExpandedView   bool
	expandedOffset int

	// Editor config
	editor string

	// pendingTopChord holds the first g of gg. The shell's chord state serves the
	// Outliner only, so the panel keeps its own one-key prefix.
	pendingTopChord bool

	Dispatch operation.KeyDispatch
}

// SetOperationDispatch hands the panel the shell's key dispatch seam. Without it
// the browse grammar resolves nothing, so a host that mounts this model must
// supply it. The registry stays out because help still derives from buildKeyMap.
func (m *CommentsModel) SetOperationDispatch(dispatch operation.KeyDispatch) {
	m.Dispatch = dispatch
}

// NewCommentsPanel mounts the shared Comments panel over one owner. It is the
// single production constructor: the owner is what the panel is for, so there is
// no half-built panel that acquires one later.
func NewCommentsPanel(owner CommentOwner, editor string) *CommentsModel {
	ti := textarea.New()
	ti.Placeholder = "Enter comment text..."
	ti.CharLimit = CommentCharLimit
	ti.SetHeight(addComposerRows)
	ti.ShowLineNumbers = false
	theme.ApplyThemedTextarea(&ti)

	editTi := textarea.New()
	editTi.Placeholder = "Edit comment text..."
	editTi.CharLimit = CommentCharLimit
	editTi.SetHeight(editComposerRows)
	editTi.ShowLineNumbers = false
	theme.ApplyThemedTextarea(&editTi)

	m := &CommentsModel{
		owner:         owner,
		TextInput:     ti,
		EditTextInput: editTi,
		editor:        editor,
	}
	m.reloadEntries()
	return m
}

// Entries is the list the panel is drawing, which is the only comment state it
// holds. Callers read it rather than the owner so what they see is what the last
// render saw.
func (m *CommentsModel) Entries() []CommentEntry {
	return m.entries
}

// ownerKey names the mounted owner for asynchronous correlation. A panel with no
// owner still answers, so an outcome for a real owner can never match it.
func (m *CommentsModel) ownerKey() string {
	if m.owner == nil {
		return ""
	}
	return m.owner.Key()
}

// reloadEntries re-projects the owner's comments into the panel's own list and
// keeps the cursor on the comment it was on.
//
// Selection follows identity wherever the owner supplies one, because a project's
// comments are presented newest first and a new comment therefore arrives at the
// top: an index-preserving cursor would silently slide onto a neighbor every
// time the list grew. An owner whose entries carry no identity keeps the index,
// which is the only thing it has.
func (m *CommentsModel) reloadEntries() {
	if m.owner == nil {
		m.entries = nil
		return
	}
	selected := m.selectedCommentID()
	m.entries = m.owner.Entries()
	m.restoreSelection(selected)
}

// selectedCommentID names the comment under the cursor, or "" when there is no
// row or the owner stores no identities.
func (m *CommentsModel) selectedCommentID() string {
	if m.Cursor < 0 || m.Cursor >= len(m.entries) {
		return ""
	}
	return m.entries[m.Cursor].ID
}

// restoreSelection puts the cursor back on one identity. A comment that is gone
// leaves the cursor where it was, so a deletion settles by position and the
// caller's clamp decides the rest.
func (m *CommentsModel) restoreSelection(id string) {
	if id == "" {
		return
	}
	for i, entry := range m.entries {
		if entry.ID == id {
			m.Cursor = i
			return
		}
	}
}

// commentRefAt names the entry at one position for a mutation: its identity when
// the owner has one, and the text the panel is currently showing there.
func (m *CommentsModel) commentRefAt(index int) (CommentRef, bool) {
	if index < 0 || index >= len(m.entries) {
		return CommentRef{}, false
	}
	entry := m.entries[index]
	return CommentRef{Index: index, ID: entry.ID, Original: entry.Text}, true
}

// indexOfRef resolves a stored reference against the list as it stands now,
// returning -1 when the comment it names is gone. An identified comment is found
// by identity wherever it has moved to; a positional one can only be
// bounds-checked, which is exactly what the task panel has always done.
func (m *CommentsModel) indexOfRef(ref CommentRef) int {
	if ref.ID != "" {
		for i, entry := range m.entries {
			if entry.ID == ref.ID {
				return i
			}
		}
		return -1
	}
	if ref.Index < 0 || ref.Index >= len(m.entries) {
		return -1
	}
	return ref.Index
}

// refreshRef re-points a reference at the row its comment now occupies. It
// reports false when the comment is gone, which is the one refusal every edit
// path shares.
func (m *CommentsModel) refreshRef(ref CommentRef) (CommentRef, bool) {
	index := m.indexOfRef(ref)
	if index < 0 {
		return CommentRef{}, false
	}
	ref.Index = index
	return ref, true
}

// armDeleteConfirmation freezes the comment a delete confirmation is being
// asked about, so the answer applies to what the question named.
func (m *CommentsModel) armDeleteConfirmation(ref CommentRef) {
	m.deleteRef = &ref
}

// spendDeleteConfirmation takes that frozen reference and forgets it in the same
// act, because one confirmation authorizes exactly one removal. It reports false
// when no confirmation is outstanding, which is what keeps a delete from ever
// resolving a target of its own.
func (m *CommentsModel) spendDeleteConfirmation() (CommentRef, bool) {
	if m.deleteRef == nil {
		return CommentRef{}, false
	}
	ref := *m.deleteRef
	m.deleteRef = nil
	return ref, true
}

// forgetDeleteConfirmation drops a frozen target the user did not confirm.
func (m *CommentsModel) forgetDeleteConfirmation() {
	m.deleteRef = nil
}

// clampCursor keeps the cursor inside the list it points into.
func (m *CommentsModel) clampCursor() {
	if m.Cursor >= len(m.entries) {
		m.Cursor = max(len(m.entries)-1, 0)
	}
	if m.Cursor < 0 {
		m.Cursor = 0
	}
}

// RefreshAfterReload re-resolves the owner and re-projects its comments after a
// commit replaced the workspace objects. It reports whether the panel is still
// worth showing, because the shell dismisses it when the answer is no. It touches
// no draft: a reload must never cost the user text they are still typing.
func (m *CommentsModel) RefreshAfterReload() bool {
	if m.owner == nil || !m.owner.Reload() {
		return m.keepDraftOverMissingOwner()
	}
	m.reloadEntries()
	m.clampCursor()
	return true
}

// keepDraftOverMissingOwner answers that question for an owner the workspace no
// longer holds, and it is the one case where "gone" does not mean "leave".
//
// A panel with no composer open has nothing left to offer, so it reports the
// dismissal the shell acts on. A panel holding an add or edit draft reports the
// missing owner instead and stays, because that buffer is the only copy of the
// user's text there is: the shell's dismissal pops the view outright, without the
// discard question every other exit from a composer asks, so leaving here would
// silently destroy work the user never agreed to lose. Installing the refusal is
// what makes the reason visible beside the draft rather than only inferable from
// a list that stopped changing; the write itself is still refused when they save,
// by the same precondition that refuses any mutation against a gone owner.
func (m *CommentsModel) keepDraftOverMissingOwner() bool {
	if !m.HasTextInput() {
		return false
	}
	m.Err = errCommentOwnerGone
	return true
}

// GetTaskRef answers the shell's task-ref seam through the adapter, so an owner
// that is not a task contributes no task rather than one borrowed from the route
// the panel happens to be mounted on.
func (m *CommentsModel) GetTaskRef() (*models.Task, *models.Project) {
	if m.owner == nil {
		return nil, nil
	}
	return m.owner.TaskRef()
}

func (m *CommentsModel) Init() tea.Cmd {
	return nil
}

// SetSize updates the view dimensions.
func (m *CommentsModel) SetSize(width, height int) {
	m.BaseView.SetSize(width, height)
	cardWidth := min(width-4, 80)
	contentWidth := cardWidth - 6 // card border + padding
	m.TextInput.SetWidth(contentWidth)
	m.EditTextInput.SetWidth(contentWidth)
}

// HasTextInput returns true when add/edit mode is active.
func (m *CommentsModel) HasTextInput() bool {
	return m.AddMode || m.EditMode
}

// OnExit resets all interactive input state so stale textarea content or edit
// modes don't persist in the registry after the view is popped. It also forgets
// any pending mutation: the commit is already in flight and will still land, but
// the panel the user walked away from must not wake up to apply it.
func (m *CommentsModel) OnExit() {
	m.AddMode = false
	m.EditMode = false
	m.ExpandedView = false
	m.expandedOffset = 0
	m.editRef = CommentRef{}
	m.recovered = nil
	m.forgetDeleteConfirmation()
	m.pending = nil
	m.TextInput.Reset()
	m.EditTextInput.Reset()
}

// ShouldCaptureEsc returns true when in expanded, add/edit, or delete confirmation modes.
func (m *CommentsModel) ShouldCaptureEsc() bool {
	return m.ExpandedView || m.DefaultShouldCaptureEsc() || m.AddMode || m.EditMode
}

// GetHelpItems returns help items for the comments panel. The keymap is the one
// source: the core help descriptor the strategy binds is inapplicable off the
// Outliner, so the registry half of a merge would contribute nothing and only the
// keymap can name this panel's own grammar.
func (m *CommentsModel) GetHelpItems() []operation.HelpItem {
	return help.HelpItemsFromKeyMap(m.BuildKeyMap())
}

func (m *CommentsModel) StatusBarLabel() string { return "Comments" }
