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
	// ID is the owner's stable identity for the comment, or empty when the owner
	// stores its comments positionally. The panel treats it as the addressing key
	// whenever it is present, so selection survives a list that reorders and a
	// mutation names a comment rather than a row.
	ID        string
	Text      string
	CreatedAt time.Time
}

// CommentRef names the comment one mutation addresses. It carries all three of
// the things the panel knows about a row, because owners disagree about which of
// them is authoritative: an owner with identities addresses by ID and treats
// Index as a hint, while a positional owner has only Index.
//
// Original is the text the panel last read there, and it is a precondition
// rather than a convenience. The list can be re-sorted, reloaded, or changed by
// another surface between the moment an editor opens and the moment it saves, so
// an owner that can check it refuses a changed target instead of applying the
// user's replacement to text they were no longer looking at.
type CommentRef struct {
	Index    int
	ID       string
	Original string
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

	// ExternalEditHeader is the same preamble for the referenced entry.
	ExternalEditHeader(ref CommentRef) string

	// Add commits new comment text.
	Add(text string) tea.Cmd

	// Edit commits replacement text for the referenced entry, preserving its
	// creation timestamp.
	Edit(ref CommentRef, text string) tea.Cmd

	// Remove commits the removal of the referenced entry.
	Remove(ref CommentRef) tea.Cmd

	// Reload re-resolves the owner against the workspace after a commit replaced
	// the in-memory objects, and reports false when the owner is gone.
	Reload() bool

	// Available asks that same question and answers it without adopting the
	// resolution, so a render may ask it. The panel needs it because it tells the
	// user which of its acts are still open to them, and the one act that needs a
	// live owner — handing a draft to an external editor — is refused for a gone
	// one: guidance that could not read presence would have to either promise
	// that act unconditionally or drop it, and both are wrong beside a draft the
	// user is trying to rescue. An adapter answers it from the same resolution
	// Reload performs, which is what keeps the two from disagreeing.
	Available() bool

	// TaskRef answers the shell's task-ref provider seam, which reads the
	// mounted view for the task the user is looking at. An owner that is not a
	// task returns nothing rather than borrowing one.
	TaskRef() (*models.Task, *models.Project)
}
