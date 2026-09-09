package coordinator

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
)

// ProjectCommentMutator is the narrow port the project-comment surfaces depend
// on. It is separate from SubviewMutator, which is task-qualified in every
// method: a project comment is committed against a project id and never against
// a TaskLocator, so a surface that comments on a project must not be able to
// reach a task mutation by accident.
type ProjectCommentMutator interface {
	// AddProjectComment appends one timestamped comment to a project and commits
	// it as one ChangeSet with one undo entry.
	AddProjectComment(projectID, text string) (app.ProjectCommentResult, error)

	// EditProjectComment replaces the text of the comment the target names,
	// preserving its identity and creation instant.
	EditProjectComment(target app.ProjectCommentTarget, text string) (app.ProjectCommentResult, error)

	// RemoveProjectComment deletes the comment the target names.
	RemoveProjectComment(target app.ProjectCommentTarget) (app.ProjectCommentResult, error)
}

// ProjectCommentCoordinator backs every surface that writes a project comment.
type ProjectCommentCoordinator struct {
	comments ProjectCommentMutator
}

// NewProjectCommentCoordinator binds a project-comment mutator. It is the only
// way to populate the coordinator from outside this package, which is what lets
// the quick-comment tests drive the prompt against a recording double rather
// than a whole application service.
func NewProjectCommentCoordinator(mutator ProjectCommentMutator) ProjectCommentCoordinator {
	return ProjectCommentCoordinator{comments: mutator}
}

// Add appends one comment to a project and reports the commit as the ordinary
// batch result, so the shell reloads exactly the project it wrote and settles
// every open view through the one post-transaction refresh every other
// project-scoped mutation goes through.
//
// The committed identity and revision are checked rather than merely returned. A
// commit that reports neither did not round-trip through the ledger, and
// reporting success for it would hand the user an undo affordance and a
// confirmation that name a transaction the workspace does not hold.
func (c ProjectCommentCoordinator) Add(projectID, projectName, text string) tea.Cmd {
	return c.commit("Add Project Comment", projectName, "Commented on "+projectName, func() (app.ProjectCommentResult, error) {
		return c.comments.AddProjectComment(projectID, text)
	})
}

// Edit replaces one comment's text. The target carries the identity and the
// text the panel last read, so the application layer refuses a comment that
// moved or changed rather than rewriting whatever now sits where the list drew
// it.
func (c ProjectCommentCoordinator) Edit(target app.ProjectCommentTarget, projectName, text string) tea.Cmd {
	return c.commit("Edit Project Comment", projectName, "Comment updated on "+projectName, func() (app.ProjectCommentResult, error) {
		return c.comments.EditProjectComment(target, text)
	})
}

// Remove deletes one comment under the same preconditions Edit commits under.
func (c ProjectCommentCoordinator) Remove(target app.ProjectCommentTarget, projectName string) tea.Cmd {
	return c.commit("Remove Project Comment", projectName, "Comment removed from "+projectName, func() (app.ProjectCommentResult, error) {
		return c.comments.RemoveProjectComment(target)
	})
}

// commit is the one shape the three project-comment mutations report in, so an
// identity-less or revision-less commit is caught the same way whichever act
// produced it.
func (c ProjectCommentCoordinator) commit(
	action, projectName, summary string,
	mutate func() (app.ProjectCommentResult, error),
) tea.Cmd {
	return runBatch(action, func() ([]string, string, string, error) {
		res, err := mutate()
		if err != nil {
			return nil, "", "", err
		}
		if res.CommentID == "" || res.Revision == "" {
			return nil, "", "", fmt.Errorf(
				"project comment on %s committed without an identity or revision", projectName,
			)
		}
		return []string{res.ProjectID}, summary, res.TransactionID, nil
	})
}
