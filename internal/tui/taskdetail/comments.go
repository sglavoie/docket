package taskdetail

import (
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/help"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// CommentCharLimit bounds a comment however it is typed. The Comments panel
// owns the number because it owns comments; the Outliner's quick Add Comment
// popup reads it from here rather than restating it, so the two routes cannot
// promise different lengths.
const CommentCharLimit = 2000

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

	// Edit mode
	EditMode      bool
	editIndex     int
	EditTextInput textarea.Model

	// Expanded view mode (show full comment)
	ExpandedView bool

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
	ti.SetHeight(4)
	ti.ShowLineNumbers = false
	theme.ApplyThemedTextarea(&ti)

	editTi := textarea.New()
	editTi.Placeholder = "Edit comment text..."
	editTi.CharLimit = CommentCharLimit
	editTi.SetHeight(6)
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

// reloadEntries re-projects the owner's comments into the panel's own list.
func (m *CommentsModel) reloadEntries() {
	if m.owner == nil {
		m.entries = nil
		return
	}
	m.entries = m.owner.Entries()
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
// commit replaced the workspace objects, reporting false when the owner is gone
// so the shell can dismiss the panel. It touches no input state: a reload must
// never cost the user a draft they are still typing.
func (m *CommentsModel) RefreshAfterReload() bool {
	if m.owner == nil || !m.owner.Reload() {
		return false
	}
	m.reloadEntries()
	m.clampCursor()
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
	m.editIndex = 0
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
