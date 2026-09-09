package taskdetail

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// CommentEntry is one comment as the shared Comments panel sees it. The
// presentation reads nothing else: an owner projects its stored comments into
// these entries in display order, so the panel can neither reach past its
// adapter into a task pointer nor learn which kind of entity it is drawing.
type CommentEntry struct {
	Text      string
	CreatedAt time.Time
}

// CommentOwnerTitle is the panel header, split the way chrome's task-scoped
// title renderer needs it. Prefix takes the accent style; Task, when it is
// non-nil, contributes its own semantic title coloring. An owner that is not a
// task names itself entirely in Prefix and leaves Task nil, which is the one
// place the shared presentation still mentions a task at all.
type CommentOwnerTitle struct {
	Prefix string
	Task   *models.Task
}

// CommentOwner adapts one commentable entity to the shared Comments panel. It
// is the whole seam between the presentation and whatever stores the comments:
// the panel resolves its rows, its labels, its external-editor buffers, and its
// three commits through this interface, so a second kind of owner needs an
// adapter rather than a second panel.
//
// Every mutation returns the coordinator command that commits it, never an
// applied change. The panel records the submission, leaves the user's draft and
// selection alone until the transaction confirms, and applies the outcome when
// the commit's own message comes back, so nothing here may edit a
// repository-backed model in place.
type CommentOwner interface {
	// Key names the commented entity for the whole lifetime of one mounted
	// panel. Every asynchronous result carries the key it was submitted under, so
	// a panel that has since been dismissed or rebound to another owner ignores
	// it instead of applying it to whatever is mounted now.
	Key() string

	// Title names the owner above the list.
	Title() CommentOwnerTitle

	// Entries projects the owner's stored comments in display order.
	Entries() []CommentEntry

	// ExternalAddHeader is the explanatory preamble a new-comment editor buffer
	// opens with. The panel appends the seeded text.
	ExternalAddHeader() string

	// ExternalEditHeader is the same preamble for the entry at index.
	ExternalEditHeader(index int) string

	// Add commits new comment text.
	Add(text string) tea.Cmd

	// Edit commits replacement text for the entry at index, preserving its
	// creation timestamp.
	Edit(index int, text string) tea.Cmd

	// Remove commits the removal of the entry at index.
	Remove(index int) tea.Cmd

	// Reload re-resolves the owner against the workspace after a commit replaced
	// the in-memory objects, and reports false when the owner is gone.
	Reload() bool

	// TaskRef answers the shell's task-ref provider seam, which reads the
	// mounted view for the task the user is looking at. An owner that is not a
	// task returns nothing rather than borrowing one.
	TaskRef() (*models.Task, *models.Project)
}
