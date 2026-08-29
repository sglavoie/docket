package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// bulkMoveSource groups one source project with the canonical subtree roots the
// batch takes out of it, already ordered the way they will land in the
// destination.
type bulkMoveSource struct {
	project *models.Project
	roots   []*models.Task
}

// BulkMoveToProject moves every selected task subtree into one destination
// project, committing every source project, the destination, and every document
// whose soft references had to follow the move as ONE transaction with ONE undo
// entry. It is the batch spelling of the `%project` sigil's cross-project move,
// and it is shaped like deleteTasks rather than bulkAttribute: a destination
// holding none of the selection still needs its own project commit, the moved
// attachment bytes need a side-effect seam, and the planning requalification
// needs a builder — none of which the attribute runner has.
//
// Four things are decided here that a single move never has to answer:
//
//   - Overlap. An ancestor and its own descendant may both be selected. The
//     descendant already travels inside the ancestor's subtree, and the model
//     refuses a destination that already holds a moved ID, so the selection is
//     collapsed to maximal subtrees by canonicalSubtreeRoots before any move runs.
//   - Order. Every moved subtree appends to the destination, so the batch fixes
//     what "the end" means: source projects in first-seen request order, and
//     within each source its roots in that project's own task order. The result
//     is stable for one selection regardless of how the presentation enumerated it.
//   - Depth. Each subtree lands at the destination root, so depth is measured
//     once over the graph all the subtrees produce rather than per task — one
//     ApplyParentAssignments pass naming every moved root as a root, which is the
//     existing projected-graph validator applied to the final destination.
//   - Already home. A selected task that already lives in the destination is a
//     no-op that still commits, matching both the idempotent bulk attribute
//     mutations and the single `%` move, which treats picking the current project
//     as an ordinary unchanged edit rather than an error.
//
// The batch is all-or-nothing: any refusal restores every touched project
// newest-first and commits nothing.
func (s *Service) BulkMoveToProject(locators []TaskLocator, destProjectID string) (BulkResult, error) {
	if len(locators) == 0 {
		return BulkResult{}, invalidInput("tasks", "at least one task id is required")
	}
	destinationID := strings.TrimSpace(destProjectID)
	if destinationID == "" {
		return BulkResult{}, invalidInput("project", "destination project id is required")
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return BulkResult{}, err
	}
	destination := snap.ProjectByID(destinationID)
	if destination == nil {
		return BulkResult{}, fmt.Errorf("%w: %s", ErrProjectNotFound, destinationID)
	}
	refs, err := resolveRequestedTasks(snap, nil, locators)
	if err != nil {
		return BulkResult{}, err
	}
	roots := canonicalSubtreeRoots(refs)
	sources := bulkMoveSources(roots, destination.ID)

	// Cross-project rollback keeps the live task pointers, so it uses the
	// hierarchy backups the single move uses rather than a snapshot clone: a
	// clone-based restore would hand the source project fresh copies of the very
	// task objects the destination still points at.
	backups := make([]hierarchyProjectBackup, 0, len(sources)+1)
	for _, source := range sources {
		backups = append(backups, captureHierarchyProject(source.project))
	}
	backups = append(backups, captureHierarchyProject(destination))
	committed := false
	defer func() {
		if committed {
			return
		}
		for i := len(backups) - 1; i >= 0; i-- {
			backups[i].restore()
		}
	}()

	delta, err := applyBulkSubtreeMoves(sources, destination)
	if err != nil {
		return BulkResult{}, err
	}
	if err = validateBulkMoveDepth(destination, delta.movedRoots); err != nil {
		return BulkResult{}, err
	}

	artifacts, err := bulkMovePlanningArtifacts(delta.planningMoves, s.defaultedPlanningArtifacts(nil))
	if err != nil {
		return BulkResult{}, err
	}

	// Every moved task keeps its identity and changes address, so each incoming
	// reference across the whole catalog — archived projects included — is
	// re-pointed in this same transaction. One pass carries the full map, so a
	// holder that references two moved tasks in different source projects is
	// rewritten once rather than twice.
	pass, err := s.maintainTaskReferences(
		snap,
		TaskReferenceMaintenance{Moved: delta.moved},
		bulkMoveCommittingProjects(sources, destination),
	)
	if err != nil {
		return BulkResult{}, err
	}
	defer func() {
		if !committed {
			pass.restore()
		}
	}()

	commits, destinationAt := bulkMoveCommits(snap, sources, destination, delta.attachmentOps, artifacts)
	commits = append(commits, pass.commits...)
	outcome, err := s.commitProjects(
		commits,
		"Move to Project",
		fmt.Sprintf("%d task(s) → %s", len(roots), destination.Slug),
	)
	if err != nil {
		return BulkResult{}, err
	}
	committed = true

	return bulkMoveResult(roots, destination, snap, outcome, commits, destinationAt), nil
}

// bulkMoveSources groups the canonical roots by source project, dropping the
// ones already living in the destination. Sources keep first-seen request order
// and each source's roots are ordered by their position in that project, which
// together fix the order the batch appends to the destination.
func bulkMoveSources(roots []TaskRef, destinationID string) []bulkMoveSource {
	at := make(map[string]int, len(roots))
	var sources []bulkMoveSource
	for _, ref := range roots {
		if ref.Project.ID == destinationID {
			continue
		}
		index, seen := at[ref.Project.ID]
		if !seen {
			index = len(sources)
			at[ref.Project.ID] = index
			sources = append(sources, bulkMoveSource{project: ref.Project})
		}
		sources[index].roots = append(sources[index].roots, ref.Task)
	}
	for i := range sources {
		orderTasksByProjectPosition(sources[i].project, sources[i].roots)
	}
	return sources
}

// orderTasksByProjectPosition sorts tasks into their owning project's own task
// order, so a batch preserves each source's existing relative order instead of
// inheriting whatever order the presentation happened to enumerate a selection in.
func orderTasksByProjectPosition(project *models.Project, tasks []*models.Task) {
	position := make(map[string]int, len(project.Tasks))
	for i, task := range project.Tasks {
		if task != nil {
			position[task.ID] = i
		}
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		return position[tasks[i].ID] < position[tasks[j].ID]
	})
}

// bulkMoveDelta is the accumulated application-owned effect of every subtree
// move in the batch: the roots that actually traveled, the planning
// requalifications and attachment relocations they imply, and the complete
// old-to-new locator map one reference pass needs.
type bulkMoveDelta struct {
	movedRoots    []*models.Task
	planningMoves []PlanningProjectMove
	attachmentOps map[string][]storage.AttachmentOp
	moved         map[models.TaskReference]models.TaskReference
}

// applyBulkSubtreeMoves performs the in-memory transfer one subtree at a time,
// in the fixed order, accumulating the deltas the single transaction carries.
// The caller owns rollback until its grouped commit succeeds.
func applyBulkSubtreeMoves(sources []bulkMoveSource, destination *models.Project) (bulkMoveDelta, error) {
	delta := bulkMoveDelta{
		attachmentOps: make(map[string][]storage.AttachmentOp, len(sources)),
		moved:         make(map[models.TaskReference]models.TaskReference),
	}
	for _, source := range sources {
		for _, root := range source.roots {
			move, err := moveTaskSubtreeBetweenProjects(source.project, root, destination, nil)
			if err != nil {
				return bulkMoveDelta{}, invalidInput("move", err.Error())
			}
			delta.movedRoots = append(delta.movedRoots, root)
			delta.planningMoves = append(delta.planningMoves, move.PlanningMoves...)
			delta.attachmentOps[source.project.ID] = append(delta.attachmentOps[source.project.ID], move.AttachmentOps...)
			for _, moved := range move.MovedTasks {
				if moved == nil || moved.ID == "" {
					continue
				}
				delta.moved[models.TaskReference{ProjectID: source.project.ID, TaskID: moved.ID}] =
					models.TaskReference{ProjectID: destination.ID, TaskID: moved.ID}
			}
		}
	}
	return delta, nil
}

// validateBulkMoveDepth measures the nesting cap once, after every subtree has
// landed, by asking the destination to re-affirm each moved root as a root. That
// runs the projected-graph depth validator over the graph the whole batch
// produced — the check a per-task validation could not make — while changing no
// edge, since each moved root already has its parent cleared.
func validateBulkMoveDepth(destination *models.Project, movedRoots []*models.Task) error {
	if len(movedRoots) == 0 {
		return nil
	}
	assignments := make([]models.ParentAssignment, 0, len(movedRoots))
	for _, root := range movedRoots {
		assignments = append(assignments, models.ParentAssignment{Task: root, Parent: nil})
	}
	if _, err := destination.ApplyParentAssignments(assignments); err != nil {
		return invalidInput("move", err.Error())
	}
	return nil
}

// bulkMovePlanningArtifacts renders the Today/Next requalification for the whole
// batch. A batch that moved nothing asks the builder for nothing, so an
// already-home selection does not rewrite planning.json.
func bulkMovePlanningArtifacts(moves []PlanningProjectMove, builder PlanningArtifactBuilder) ([]storage.ArtifactChange, error) {
	if len(moves) == 0 {
		return nil, nil
	}
	return planningArtifactsForRequest(PlanningReconcileRequest{Moves: moves}, builder)
}

// bulkMoveCommittingProjects names the projects this mutation commits itself, so
// the reference pass rewrites them in place instead of adding a second changeset
// for the same document.
func bulkMoveCommittingProjects(sources []bulkMoveSource, destination *models.Project) map[string]bool {
	committing := make(map[string]bool, len(sources)+1)
	for _, source := range sources {
		committing[source.project.ID] = true
	}
	committing[destination.ID] = true
	return committing
}

// bulkMoveCommits builds the source commits — each carrying its own attachment
// relocations — followed by the explicit destination commit that
// distinctProjectCommits could never derive, since the destination may hold none
// of the selection. The planning artifacts ride the first commit so they land in
// the same transaction. It returns the commits and the destination's index in
// them, which is where every moved task's new revision is reported.
func bulkMoveCommits(
	snap Snapshot,
	sources []bulkMoveSource,
	destination *models.Project,
	attachmentOps map[string][]storage.AttachmentOp,
	artifacts []storage.ArtifactChange,
) ([]projectCommit, int) {
	commits := make([]projectCommit, 0, len(sources)+1)
	for _, source := range sources {
		commits = append(commits, projectCommit{
			project:          source.project,
			expectedRevision: snap.Revision(source.project.ID),
			base:             snap.Document(source.project.ID),
			attachmentOps:    attachmentOps[source.project.ID],
		})
	}
	commits = append(commits, projectCommit{
		project:          destination,
		expectedRevision: snap.Revision(destination.ID),
		base:             snap.Document(destination.ID),
	})
	destinationAt := len(commits) - 1
	if len(artifacts) > 0 {
		commits[0].artifacts = append(commits[0].artifacts, artifacts...)
	}
	return commits, destinationAt
}

// bulkMoveResult reports one entry per canonical root at its post-move address:
// every root now lives in the destination, so each carries the destination's new
// revision. Roots collapsed into a selected ancestor are absent for the same
// reason a duplicate locator is — the batch moved them exactly once, inside
// someone else's subtree.
func bulkMoveResult(
	roots []TaskRef,
	destination *models.Project,
	snap Snapshot,
	outcome groupedCommitOutcome,
	commits []projectCommit,
	destinationAt int,
) BulkResult {
	result := BulkResult{
		Tasks:         make([]TaskResult, 0, len(roots)),
		TransactionID: transactionIDAt(outcome.results, 0),
	}
	revision := revisionAt(outcome.results, destinationAt)
	transactionID := transactionIDAt(outcome.results, destinationAt)
	base := snap.Document(destination.ID)
	for _, ref := range roots {
		result.Tasks = append(result.Tasks, newTaskResult(
			TaskRef{Task: ref.Task, Project: destination, Base: base},
			revision,
			transactionID,
		))
	}
	for _, commit := range commits {
		result.TouchedProjectIDs = append(result.TouchedProjectIDs, commit.project.ID)
	}
	return result
}
