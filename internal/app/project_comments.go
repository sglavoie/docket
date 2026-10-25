package app

import (
	"fmt"
	"strings"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
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

// ProjectCommentTarget names the single comment an edit or a removal addresses,
// and the text the caller believed it held.
//
// It is an identity plus a precondition, never a position: a project's comments
// are re-sorted on every write and presented newest-first, so a list index names
// a different comment after any concurrent change. Original is what makes the
// request idempotent-safe rather than merely well-addressed — a comment edited
// under the user by another surface, an undo, or another machine's merge is
// refused instead of being overwritten by a change the user aimed at text they
// were no longer looking at.
type ProjectCommentTarget struct {
	// ProjectID is the project that owns the comment.
	ProjectID string
	// CommentID is the comment's stored identity.
	CommentID string
	// Original is the text the caller last read for that comment, byte for byte.
	// It is a precondition rather than input to normalize: the stored text is
	// compared to it verbatim, so a caller hands back exactly what it read.
	Original string
}

// resolvedProjectComment is one re-resolved mutation target: the project as the
// fresh snapshot holds it, the immutable base document and revision that commit
// must be checked against, and the addressed comment itself.
//
// The comment is carried as a value rather than a position on purpose. The
// collection is rebuilt detached before every commit, and a position measured
// against the snapshot would have to survive that rebuild to stay correct, so the
// mutations re-find their comment by identity in the collection they are about to
// write instead.
type resolvedProjectComment struct {
	project  *models.Project
	base     *storage.ProjectDocument
	revision string
	comment  *models.ProjectComment
}

// resolveProjectCommentTarget re-resolves one target against a fresh snapshot.
//
// Every precondition is measured here, against that snapshot alone: a caller
// hands in what it read, never a pointer it captured, so a project renamed,
// parked, archived, undone, or replaced wholesale in memory since the panel
// mounted is judged by what the workspace holds now.
func (s *Service) resolveProjectCommentTarget(target ProjectCommentTarget) (resolvedProjectComment, error) {
	projectID := strings.TrimSpace(target.ProjectID)
	if projectID == "" {
		return resolvedProjectComment{}, invalidInput("project", "id is required")
	}
	commentID := strings.TrimSpace(target.CommentID)
	if commentID == "" {
		return resolvedProjectComment{}, invalidInput("comment", "id is required")
	}
	// The precondition is checked for presence and then left alone. Trimming it
	// would compare a normalized reading of what the caller read against text the
	// schema-17 writer and reader preserve byte for byte: a stored "  note  " and
	// an exact Original would be refused as changed with no way to reopen out of
	// it, while a stale "  note  " read against a stored "note" would be accepted
	// as unchanged. Newly supplied replacement text is a separate contract and is
	// still normalized where it is accepted.
	if strings.TrimSpace(target.Original) == "" {
		return resolvedProjectComment{}, invalidInput("comment", "original text is required")
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return resolvedProjectComment{}, err
	}
	project := snap.ProjectByID(projectID)
	if project == nil {
		return resolvedProjectComment{}, fmt.Errorf("%w: %s", ErrProjectNotFound, projectID)
	}
	if project.Archived {
		return resolvedProjectComment{}, invalidInput("project", "project is archived; restore it to change comments")
	}
	base := snap.Document(project.ID)
	if base == nil {
		return resolvedProjectComment{}, fmt.Errorf("app: project %s has no immutable base document", project.ID)
	}
	var found *models.ProjectComment
	for _, comment := range project.Comments {
		if comment != nil && comment.ID == commentID {
			found = comment
			break
		}
	}
	if found == nil {
		return resolvedProjectComment{}, invalidInput("comment", "comment is no longer on this project")
	}
	if found.Text != target.Original {
		return resolvedProjectComment{}, invalidInput("comment", "comment changed since it was read; reopen it and try again")
	}
	return resolvedProjectComment{
		project:  project,
		base:     base,
		revision: snap.Revision(project.ID),
		comment:  found,
	}, nil
}

// commitProjectComments stages one rebuilt collection on the resolved project
// and commits it as the single ChangeSet the caller's undo entry covers. The
// collection is cloned by the caller before it is handed in, so the snapshot's
// own comments are never aliased by the one this commit renders.
func (s *Service) commitProjectComments(
	resolved resolvedProjectComment,
	merged []*models.ProjectComment,
	commentID, undoAction string,
) (ProjectCommentResult, error) {
	models.SortProjectComments(merged)
	if err := models.ValidateProjectComments(merged); err != nil {
		return ProjectCommentResult{}, invalidInput("comment", err.Error())
	}
	project := resolved.project
	project.Comments = merged

	// The barrier is spent here rather than inside commitProjects, which serves
	// multi-project transactions too: this is the last instant at which the
	// snapshot, the resolved comment, and the revision the commit is guarded by
	// are all fixed and nothing has been written, which is exactly the state a
	// two-machine race has to hold both processes in.
	awaitProjectCommitBarrier(project.ID, resolved.revision, undoAction)
	outcome, err := s.commitProjects([]projectCommit{{
		project:          project,
		expectedRevision: resolved.revision,
		base:             resolved.base,
		tasksUntouched:   true,
	}}, undoAction, project.Name)
	if err != nil {
		return ProjectCommentResult{}, err
	}
	return ProjectCommentResult{
		ProjectID:     project.ID,
		CommentID:     commentID,
		Revision:      revisionAt(outcome.results, 0),
		TransactionID: transactionIDAt(outcome.results, 0),
	}, nil
}

// EditProjectComment replaces one project comment's text in place, preserving
// its minted identity and its creation instant.
//
// The instant is deliberately not refreshed: CreatedAt is immutable in the merge
// audit, which refuses a moved instant even when both sides moved it alike, so
// rewriting it here would turn every edit into a conflict the next time two
// machines meet. An edit that resolves to the text already stored is still
// committed, exactly as a task comment edit is, because the two entry points must
// not disagree about what a no-op save means.
func (s *Service) EditProjectComment(target ProjectCommentTarget, text string) (ProjectCommentResult, error) {
	trimmed, err := normalizeCommentText(text)
	if err != nil {
		return ProjectCommentResult{}, err
	}
	resolved, err := s.resolveProjectCommentTarget(target)
	if err != nil {
		return ProjectCommentResult{}, err
	}
	existing := resolved.comment
	if err = models.ValidateProjectComment(existing.ID, trimmed, existing.CreatedAt); err != nil {
		return ProjectCommentResult{}, invalidInput("comment", err.Error())
	}
	merged := models.CloneProjectComments(resolved.project.Comments)
	for _, comment := range merged {
		if comment.ID == existing.ID {
			comment.Text = trimmed
		}
	}
	return s.commitProjectComments(resolved, merged, existing.ID, "Edit Project Comment")
}

// RemoveProjectComment deletes one project comment. It measures the same
// preconditions an edit does, so a comment that moved or changed under the user
// is refused rather than deleted by the position a stale list drew it at.
func (s *Service) RemoveProjectComment(target ProjectCommentTarget) (ProjectCommentResult, error) {
	resolved, err := s.resolveProjectCommentTarget(target)
	if err != nil {
		return ProjectCommentResult{}, err
	}
	commentID := resolved.comment.ID
	var merged []*models.ProjectComment
	for _, comment := range models.CloneProjectComments(resolved.project.Comments) {
		if comment.ID != commentID {
			merged = append(merged, comment)
		}
	}
	return s.commitProjectComments(resolved, merged, commentID, "Remove Project Comment")
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
	// aliased by the one this commit renders; commitProjectComments sorts it into
	// the canonical order the document is written in.
	merged := append(models.CloneProjectComments(project.Comments), comment)
	resolved := resolvedProjectComment{project: project, base: base, revision: snap.Revision(project.ID)}
	return s.commitProjectComments(resolved, merged, comment.ID, "Add Project Comment")
}
