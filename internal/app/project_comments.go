package app

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// This file owns the project-comment use cases. A project comment is not a task
// comment with a different owner: it lives in the project's own document, it
// carries a minted identity, and it must never move a task's touch time. That is
// why nothing here routes through mutateRichRecordSelection, which resolves a
// TaskRef and would write task.UpdatedAt on the way past.

// ProjectCommentResult is the presentation-neutral outcome of a project-comment
// mutation. It names the project and the comment identity rather than wrapping a
// TaskResult the way CommentResult does, and carries the committed revision and
// transaction ID so an adapter can reload exactly the project it wrote and undo
// exactly the entry it created.
type ProjectCommentResult struct {
	// ProjectID is the project the comment was committed to.
	ProjectID string
	// CommentID is the identity minted for the new comment.
	CommentID string
	// Revision is the owning project's revision after the commit.
	Revision string
	// TransactionID is the workspace transaction the commit was recorded under.
	TransactionID string
}

// AddProjectComment appends one timestamped comment to a project and commits it
// as one ChangeSet under one undo entry.
//
// It resolves a single immutable snapshot and validates the owner against it:
// an absent project fails with ErrProjectNotFound and an archived one is
// refused, while a parked project and the system Unfiled project are ordinary
// writable owners — parking hides a project from the default population rather
// than sealing it, and Unfiled is protected against lifecycle changes, not
// against content. Text is normalized through the same normalizeCommentText a
// task comment goes through, so the two entry points cannot disagree about what
// an empty comment is.
//
// The identity and instant are minted here, from the injected generator and
// clock, and gated by models.ValidateProjectComment before anything is staged,
// so a generator that cannot mint a canonical UUID fails the request instead of
// producing a document the canonical writer would refuse.
func (s *Service) AddProjectComment(projectID, text string) (ProjectCommentResult, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return ProjectCommentResult{}, invalidInput("project", "id is required")
	}
	trimmed, err := normalizeCommentText(text)
	if err != nil {
		return ProjectCommentResult{}, err
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return ProjectCommentResult{}, err
	}
	project := snap.ProjectByID(projectID)
	if project == nil {
		return ProjectCommentResult{}, fmt.Errorf("%w: %s", ErrProjectNotFound, projectID)
	}
	if project.Archived {
		return ProjectCommentResult{}, invalidInput("project", "project is archived; restore it to add comments")
	}
	base := snap.Document(project.ID)
	if base == nil {
		return ProjectCommentResult{}, fmt.Errorf("app: project %s has no immutable base document", project.ID)
	}

	comment := &models.ProjectComment{
		ID:        s.ids.NewID(),
		Text:      trimmed,
		CreatedAt: s.clock.Now(),
	}
	if err = models.ValidateProjectComment(comment.ID, comment.Text, comment.CreatedAt); err != nil {
		return ProjectCommentResult{}, invalidInput("comment", err.Error())
	}
	// The collection is rebuilt detached so the snapshot's own comments are never
	// aliased by the one this commit renders, then sorted into the canonical order
	// the document is written in.
	merged := append(models.CloneProjectComments(project.Comments), comment)
	models.SortProjectComments(merged)
	if err = models.ValidateProjectComments(merged); err != nil {
		return ProjectCommentResult{}, invalidInput("comment", err.Error())
	}
	project.Comments = merged

	outcome, err := s.commitProjects([]projectCommit{{
		project:          project,
		expectedRevision: snap.Revision(project.ID),
		base:             base,
		tasksUntouched:   true,
	}}, "Add Project Comment", project.Name)
	if err != nil {
		return ProjectCommentResult{}, err
	}
	return ProjectCommentResult{
		ProjectID:     project.ID,
		CommentID:     comment.ID,
		Revision:      revisionAt(outcome.results, 0),
		TransactionID: transactionIDAt(outcome.results, 0),
	}, nil
}
