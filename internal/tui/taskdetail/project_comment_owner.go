package taskdetail

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
)

// projectCommentOwner is the production CommentOwner for a project's own
// comments. It is the second adapter over the one shared panel rather than a
// second panel: the presentation, the grammar, and the controls are the task
// panel's, and everything that differs about a project comment lives here.
//
// Two things differ. A project comment carries a minted identity, so every
// mutation addresses it by that identity and by the text the panel last read,
// which is what lets the application layer refuse a target that moved or changed
// instead of rewriting a row. And the entries are presented newest first, which
// is the opposite of the canonical storage order and the opposite of the task
// panel's stored order — a project comment is a running log of what happened to
// the project, and the most recent line is the one the user opened the panel for.
type projectCommentOwner struct {
	// projectID is the frozen owner identity. It is what Key spells and what
	// every mutation and every reload resolves from, so a renamed, reloaded, or
	// wholesale-replaced project object never changes who this panel is for.
	projectID string
	// project is the last resolution of that identity, held only for the name and
	// the entries. It is replaced on every Reload and never used to address a
	// mutation.
	project *models.Project
	// resolve re-reads the project from the shell's live catalog. It is a function
	// rather than a pointer because a committed transaction replaces the in-memory
	// project object, so an adapter that kept the original pointer would keep
	// drawing and commenting on a discarded one.
	resolve     func(string) *models.Project
	coordinator coordinator.ProjectCommentCoordinator
}

// NewProjectCommentOwner binds the shared Comments panel to one project. The
// resolver is required: without it the panel could not survive its own commits.
func NewProjectCommentOwner(
	project *models.Project,
	resolve func(string) *models.Project,
	comments coordinator.ProjectCommentCoordinator,
) CommentOwner {
	owner := &projectCommentOwner{resolve: resolve, coordinator: comments}
	if project != nil {
		owner.projectID, owner.project = project.ID, project
	}
	return owner
}

// Key spells the project identity in the same shape the task owner spells a task
// one, so the panel's asynchronous correlation stays a comparison between owner
// keys and can never confuse a project with a task that shares its id.
func (o *projectCommentOwner) Key() string {
	if o.projectID == "" {
		return ""
	}
	return "project:" + o.projectID
}

// Title names the project entirely in the prefix and leaves Task nil, because
// there is no task here to borrow semantic coloring from.
func (o *projectCommentOwner) Title() CommentOwnerTitle {
	if o.project == nil {
		return CommentOwnerTitle{Prefix: "Comments"}
	}
	return CommentOwnerTitle{Prefix: "Comments · " + o.project.Name}
}

// Entries projects the stored comments newest first. Storage keeps them oldest
// first because that order is canonical and byte-stable across machines;
// reversing it here is a presentation decision and touches nothing stored.
func (o *projectCommentOwner) Entries() []CommentEntry {
	if o.project == nil {
		return nil
	}
	entries := make([]CommentEntry, 0, len(o.project.Comments))
	for i := len(o.project.Comments) - 1; i >= 0; i-- {
		comment := o.project.Comments[i]
		if comment == nil {
			continue
		}
		entries = append(entries, CommentEntry{ID: comment.ID, Text: comment.Text, CreatedAt: comment.CreatedAt})
	}
	return entries
}

func (o *projectCommentOwner) ExternalAddHeader() string {
	if o.project == nil {
		return "# New comment\n\n<!-- Enter the comment text below. -->\n\n"
	}
	return fmt.Sprintf("# New comment on %s\n\n<!-- Enter the comment text below. -->\n\n", o.project.Name)
}

// ExternalEditHeader names the project rather than the row number. The row a
// comment is drawn at is not its identity here, and a buffer that named one
// would be wrong the moment the list reordered underneath the editor.
func (o *projectCommentOwner) ExternalEditHeader(CommentRef) string {
	if o.project == nil {
		return "# Comment\n\n<!-- Edit the comment text below. The timestamp will be preserved. -->\n\n"
	}
	return fmt.Sprintf(
		"# Comment on %s\n\n<!-- Edit the comment text below. The timestamp will be preserved. -->\n\n",
		o.project.Name)
}

func (o *projectCommentOwner) Add(text string) tea.Cmd {
	return o.coordinator.Add(o.projectID, o.projectName(), text)
}

func (o *projectCommentOwner) Edit(ref CommentRef, text string) tea.Cmd {
	return o.coordinator.Edit(o.target(ref), o.projectName(), text)
}

func (o *projectCommentOwner) Remove(ref CommentRef) tea.Cmd {
	return o.coordinator.Remove(o.target(ref), o.projectName())
}

// target turns a panel reference into the application layer's precondition. The
// index is deliberately dropped: what crosses this seam is the comment's
// identity and the text the panel read for it, so the mutation is re-resolved
// against a fresh snapshot and refused when either no longer holds.
func (o *projectCommentOwner) target(ref CommentRef) app.ProjectCommentTarget {
	return app.ProjectCommentTarget{ProjectID: o.projectID, CommentID: ref.ID, Original: ref.Original}
}

// projectName degrades to the stored identity when the project is gone, so a
// message about a vanished owner still names something the user can recognize.
func (o *projectCommentOwner) projectName() string {
	if o.project == nil {
		return o.projectID
	}
	return o.project.Name
}

// Reload re-resolves the project by identity and reports false when it is gone.
//
// Presence is all it reports. A project archived or parked while the panel is
// open stays readable: the comments are still there to read, copy, and open in an
// editor, and a mutation against it is refused by the application layer with a
// message naming the reason, which is what keeps unsaved text in front of the
// user instead of dismissing the panel out from under it.
func (o *projectCommentOwner) Reload() bool {
	project := o.resolveProject()
	if project == nil {
		return false
	}
	o.project = project
	return true
}

// Available reports that same presence without adopting the result, so the panel
// can name what a key would do without a render changing which project it is
// drawing.
func (o *projectCommentOwner) Available() bool {
	return o.resolveProject() != nil
}

// resolveProject is the one resolution rule both answers read, so "gone" cannot
// mean one thing to a reload and another to the guidance beside a draft.
func (o *projectCommentOwner) resolveProject() *models.Project {
	if o.projectID == "" || o.resolve == nil {
		return nil
	}
	return o.resolve(o.projectID)
}

// TaskRef contributes nothing: this owner is not a task and must not lend the
// shell's task-ref seam one borrowed from the route the panel is mounted on.
func (o *projectCommentOwner) TaskRef() (*models.Task, *models.Project) {
	return nil, nil
}
