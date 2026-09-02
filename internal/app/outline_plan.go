package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// OutlinePlanLine is one nonblank line of an edited outline buffer, already
// parsed out of the capture grammar by the adapter that owns the editor round
// trip. A line without a TaskID creates a task; a line carrying one edits the
// task it names.
type OutlinePlanLine struct {
	// SourceLine is the one-based buffer line the values came from. Zero falls
	// back to the line's position in the request when an error is reported.
	SourceLine int
	// TaskID names an existing task in the scope project. Empty creates a task.
	TaskID string
	// Depth is the zero-based indentation depth. The nearest preceding line one
	// level shallower is the parent; depth zero is a root.
	Depth int
	// Title is the required task title.
	Title string
	// Priority is 0..3, where zero is the durable Unset value.
	Priority int
	// Tags replace the task's tag set outright. The dump is always complete, so
	// an omitted tag is a removal rather than an absent instruction.
	Tags []string
	// Context replaces the single-valued task context; empty clears it.
	Context string
	// Due replaces the calendar due date; nil clears it.
	Due *time.Time
	// Pinned replaces the task's pinned state outright. The dump is always
	// complete, so an omitted *pin is an unpin rather than an absent instruction.
	Pinned bool
	// Review replaces the task's review flag outright, for the same reason: an
	// omitted =review clears it rather than saying nothing.
	Review bool
	// Dependencies replace the task's soft-reference set outright, in buffer order.
	// The dump prints every stored reference, so a locator the buffer no longer
	// carries is a removal; nil is the empty set rather than "no instruction". The
	// adapter has already folded -~task and ~0 into this one list, because a
	// declarative line describes a final state rather than a patch.
	Dependencies []TaskLocator
	// Destination is the typed lifecycle/planning destination. Empty preserves an
	// existing task's state and defaults a new task the way capture does.
	Destination CaptureDestination
}

// OutlinePlanRequest is one edited outline buffer, resolved against a single
// project and committed as one transaction.
type OutlinePlanRequest struct {
	// ProjectID owns every line in the buffer. An outline plan never moves a task
	// between projects, which is why %project has no field here.
	ProjectID string
	// ExpectedRevision is the revision the buffer was dumped at. It is required:
	// a buffer describes one exact tree, so an apply that cannot name the tree it
	// was authored against has nothing to be atomic with respect to.
	ExpectedRevision string
	// ScopeTaskIDs are the tasks the buffer was dumped from, and therefore the
	// only tasks absence can speak about. A task the dump never printed is
	// invisible to the buffer and must not be reachable by removing a line.
	ScopeTaskIDs []string
	// DeleteTaskIDs are the scope tasks the returned buffer no longer holds,
	// exactly as the adapter presented them for confirmation. It must name the
	// same set the buffer leaves absent, so what was confirmed and what is
	// removed are provably the same list.
	DeleteTaskIDs []string
	// ConfirmDeletions records explicit destructive intent for DeleteTaskIDs, the
	// way DeleteRequest.Confirm does for a single delete.
	ConfirmDeletions bool
	// Lines are the buffer's nonblank lines in their final order.
	Lines []OutlinePlanLine
	// PlanningArtifacts overrides the service planning builder.
	PlanningArtifacts PlanningArtifactBuilder
}

// OutlinePlanResult reports what one applied buffer did: the tasks it created,
// the existing tasks it actually changed, and the dumped tasks it removed.
type OutlinePlanResult struct {
	Created []TaskResult
	Updated []TaskResult
	Deleted []TaskResult
}

// Summary is the one-line report an adapter shows after applying a buffer. It
// names the deletions separately from the edits, because a removed line and an
// edited line are the two things the buffer cannot tell apart on its own.
func (r OutlinePlanResult) Summary() string {
	return outlinePlanSummary(len(r.Created)+len(r.Updated), len(r.Deleted))
}

// ProjectAtRevision returns a project together with the revision it was loaded
// at, both from one snapshot. An adapter that dumps a project to an external
// editor renders the returned project and carries the returned revision back
// into OutlinePlanRequest, so the buffer and the revision it claims to describe
// can never come from two different reads.
func (s *Service) ProjectAtRevision(projectID string) (*models.Project, string, error) {
	return s.resolveProject(projectID)
}

// ApplyOutlinePlan commits one edited outline buffer as a single transaction
// with one undo entry: heterogeneous edits, creations and reparenting resolve
// from one snapshot, apply in memory against the single scope project, reconcile
// planning once, and reach storage through one project commit.
//
// The buffer is declarative and complete for the scope it was dumped from, so a
// scope task whose line the buffer no longer holds is deleted. The Delete guard
// still owns the decision: the adapter counts the removals when the editor
// returns, presents them, and sends the confirmed list back as DeleteTaskIDs.
// Those counts cannot go stale between the prompt and the write, because the
// revision check below refuses any buffer whose project moved in the meantime.
func (s *Service) ApplyOutlinePlan(req OutlinePlanRequest) (OutlinePlanResult, error) {
	if strings.TrimSpace(req.ProjectID) == "" {
		return OutlinePlanResult{}, invalidInput("project", "project is required")
	}
	// An empty buffer is a legitimate request to empty the project, so it is
	// refused only when it also removes nothing: that buffer asks for no change at
	// all and is far more likely to be a truncated file than an intent.
	if len(req.Lines) == 0 && len(req.DeleteTaskIDs) == 0 {
		return OutlinePlanResult{}, invalidInput("outline", "the buffer holds no tasks")
	}
	if strings.TrimSpace(req.ExpectedRevision) == "" {
		return OutlinePlanResult{}, invalidInput("revision", "the revision the buffer was dumped at is required")
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return OutlinePlanResult{}, err
	}
	project := snap.ProjectByID(req.ProjectID)
	if project == nil {
		return OutlinePlanResult{}, fmt.Errorf("%w: %s", ErrProjectNotFound, req.ProjectID)
	}
	// The dump revision is enforced here rather than only at the commit
	// precondition, because everything between this line and the commit mutates
	// the snapshot's live tasks. A writer that landed while the editor was open
	// must be refused before a single title is overwritten.
	if current := snap.Revision(project.ID); current != req.ExpectedRevision {
		return OutlinePlanResult{}, &storage.RevisionConflictError{
			ProjectID:   project.ID,
			ProjectName: project.Name,
			Expected:    req.ExpectedRevision,
			Actual:      current,
		}
	}
	base := snap.Document(project.ID)
	if base == nil {
		return OutlinePlanResult{}, fmt.Errorf("app: project %s has no immutable base document", project.ID)
	}
	prepared, err := prepareOutlinePlan(snap, project, req.Lines)
	if err != nil {
		return OutlinePlanResult{}, err
	}
	deleted, err := outlineDeletions(project, req, prepared)
	if err != nil {
		return OutlinePlanResult{}, err
	}

	before, err := cloneSnapshot(snap)
	if err != nil {
		return OutlinePlanResult{}, fmt.Errorf("capture outline rollback state: %w", err)
	}
	useRollbackDocuments(before)
	committed := false
	defer func() {
		if !committed {
			restoreSnapshotProjects(snap, before)
		}
	}()

	attachmentOps, removedLocators := removeOutlineTasks(project, deleted)
	targets, err := s.applyOutlinePlan(project, base, prepared)
	if err != nil {
		return OutlinePlanResult{}, err
	}
	created, updated := partitionOutlineChanges(before.ProjectByID(project.ID), prepared)
	// Only a task the buffer really changed is touched. A line that restated a
	// task verbatim must not move it to the top of Recent.
	for _, task := range updated {
		task.UpdatedAt = s.clock.Now()
	}

	// A task the buffer removed has incoming references with nowhere left to point,
	// so every catalog document naming one is repaired inside this same
	// transaction. The pass runs after the touch-time loop above: re-addressing
	// someone else's stored locator is maintenance, not an edit the buffer made,
	// and it must not move a task to the top of Recent or into the change count.
	pass, err := s.outlineReferenceMaintenance(snap, project, removedLocators)
	if err != nil {
		return OutlinePlanResult{}, err
	}
	defer func() {
		if !committed {
			pass.restore()
		}
	}()

	artifacts, err := planningArtifactsForTargets(targets, removedLocators, s.defaultedPlanningArtifacts(req.PlanningArtifacts))
	if err != nil {
		return OutlinePlanResult{}, err
	}
	summary := outlinePlanSummary(len(created)+len(updated), len(deleted))
	commits := append([]projectCommit{{
		project:          project,
		expectedRevision: snap.Revision(project.ID),
		base:             base,
		artifacts:        artifacts,
		attachmentOps:    attachmentOps,
	}}, pass.commits...)
	outcome, err := s.commitProjects(commits, "Apply Outline", summary)
	if err != nil {
		return OutlinePlanResult{}, err
	}
	committed = true

	revision := revisionAt(outcome.results, 0)
	transactionID := transactionIDAt(outcome.results, 0)
	return OutlinePlanResult{
		Created: outlineTaskResults(project, created, revision, transactionID),
		Updated: outlineTaskResults(project, updated, revision, transactionID),
		Deleted: outlineTaskResults(project, deleted, revision, transactionID),
	}, nil
}

func outlineTaskResults(project *models.Project, tasks []*models.Task, revision, transactionID string) []TaskResult {
	if len(tasks) == 0 {
		return nil
	}
	results := make([]TaskResult, 0, len(tasks))
	for _, task := range tasks {
		results = append(results, newTaskResult(TaskRef{Task: task, Project: project}, revision, transactionID))
	}
	return results
}

func outlinePlanSummary(changes, deleted int) string {
	unit := "changes"
	if changes == 1 {
		unit = "change"
	}
	summary := fmt.Sprintf("Applied %d %s", changes, unit)
	if deleted > 0 {
		noun := "tasks"
		if deleted == 1 {
			noun = "task"
		}
		summary += fmt.Sprintf(" · %d %s deleted", deleted, noun)
	}
	return summary
}

func outlineLineError(line OutlinePlanLine, index int, err error) error {
	number := line.SourceLine
	if number <= 0 {
		number = index + 1
	}
	return fmt.Errorf("line %d: %w", number, err)
}
