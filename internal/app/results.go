package app

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

// TaskResult is the presentation-neutral outcome of a task mutation. It carries
// only stable, adapter-agnostic identifiers and derived state so a CLI or TUI
// caller can render its own output without reaching back into the domain model.
type TaskResult struct {
	// TaskID is the task's stable UUID.
	TaskID string
	// ShortID is the task's per-project short numeric ID.
	ShortID int
	// ProjectID is the owning project's stable ID.
	ProjectID string
	// ProjectSlug is the owning project's slug.
	ProjectSlug string
	// QualifiedRef is the canonical "slug:#short-id" cross-project reference.
	QualifiedRef string
	// Title is the task's current title.
	Title string
	// State is the task's canonical workflow state after the mutation.
	State models.WorkflowState
	// Revision is the project's new canonical revision after the commit.
	Revision string
}

// newTaskResult builds a TaskResult from a resolved task/project pair and the
// revision the commit produced.
func newTaskResult(ref TaskRef, revision string) TaskResult {
	return TaskResult{
		TaskID:       ref.Task.ID,
		ShortID:      ref.Task.ShortID,
		ProjectID:    ref.Project.ID,
		ProjectSlug:  ref.Project.Slug,
		QualifiedRef: fmt.Sprintf("%s:#%d", ref.Project.Slug, ref.Task.ShortID),
		Title:        ref.Task.Title,
		State:        ref.Task.State(),
		Revision:     revision,
	}
}
