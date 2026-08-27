package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// DeleteRequest asks to permanently delete a task and its whole subtree.
// Destructive requests must set Confirm to signal explicit intent; a request
// without it is rejected before any mutation with ErrDestructiveConfirmationRequired.
type DeleteRequest struct {
	// Locator is the project-qualified subtree root.
	Locator TaskLocator
	// Confirm records explicit destructive intent. A false value aborts the delete
	// before any task is removed or any commit is attempted.
	Confirm bool
}

// DeleteResult reports every project the single atomic transaction touched.
type DeleteResult struct {
	// TouchedProjectIDs lists every project committed in the transaction.
	TouchedProjectIDs []string
	TransactionID     string
}

// Delete permanently removes a task and its whole subtree, committing the removal
// as ONE transaction. The removed tasks' attachment bytes ride the same
// transaction (AttachmentDelete side effects). The service derives durable
// Today/Next cleanup for the complete subtree and folds its revision-checked
// artifact into that same transaction. Additional caller artifacts ride beside
// it; machine-local recent visits remain adapter-owned. The request must carry
// explicit destructive intent; an unconfirmed request never mutates anything.
func (s *Service) Delete(req DeleteRequest, artifacts ...storage.ArtifactChange) (DeleteResult, error) {
	ref, snap, _, err := s.loadTaskSelection("", req.Locator)
	if err != nil {
		return DeleteResult{}, err
	}
	if !req.Confirm {
		return DeleteResult{}, fmt.Errorf("%w: deleting task #%d and its subtree", ErrDestructiveConfirmationRequired, ref.Task.ShortID)
	}

	removed := collectSubtreeIDs(ref.Task)
	removedIDs := sortedKeys(removed)
	removedLocators := make([]TaskLocator, 0, len(removedIDs))
	for _, taskID := range removedIDs {
		removedLocators = append(removedLocators, TaskLocator{ProjectID: ref.Project.ID, TaskID: taskID})
	}
	planningArtifacts, err := planningArtifactsForRequest(PlanningReconcileRequest{
		Remove: removedLocators,
	}, s.defaultedPlanningArtifacts(nil))
	if err != nil {
		return DeleteResult{}, err
	}
	artifacts = append(append([]storage.ArtifactChange(nil), artifacts...), planningArtifacts...)
	undoContext := fmt.Sprintf("#%d %s", ref.Task.ShortID, ref.Task.Title)

	touched, transactionID, err := s.deleteTasks(snap, []TaskRef{ref}, undoContext, artifacts)
	if err != nil {
		return DeleteResult{}, err
	}
	return DeleteResult{
		TouchedProjectIDs: touched,
		TransactionID:     transactionID,
	}, nil
}

// BulkDeleteRequest asks to delete several task subtrees, possibly spanning
// projects, as one transaction. Confirm carries the explicit destructive intent
// the batch requires.
type BulkDeleteRequest struct {
	// TaskIDs identifies each subtree root to delete. At least one is required;
	// duplicates and roots nested under another selected root are de-duplicated.
	TaskIDs []string
	// TaskLocators is the project-qualified alternative used by cross-project
	// clients. It is mutually exclusive with TaskIDs.
	TaskLocators []TaskLocator
	// Confirm records explicit destructive intent for the whole batch.
	Confirm bool
}

// BulkDeleteResult reports a bulk delete: the removed root references, the total
// removed count, and every project the single transaction touched.
type BulkDeleteResult struct {
	// RemovedTaskIDs are all UUIDs removed across every subtree, sorted.
	RemovedTaskIDs []string
	// RemovedTaskLocators names every removed task within its owning project.
	RemovedTaskLocators []TaskLocator
	// TouchedProjectIDs lists every project committed together.
	TouchedProjectIDs []string
	TransactionID     string
}

// BulkDelete removes several task subtrees — which may live in different projects —
// as ONE atomic transaction. Every affected source project commits together, so
// the whole batch lands or nothing does. The service removes every deleted subtree
// member from Today and Next in the same transaction; additional caller
// artifacts ride beside that canonical planning change. The request must carry
// explicit destructive intent.
func (s *Service) BulkDelete(req BulkDeleteRequest, artifacts ...storage.ArtifactChange) (BulkDeleteResult, error) {
	if len(req.TaskIDs) == 0 && len(req.TaskLocators) == 0 {
		return BulkDeleteResult{}, invalidInput("tasks", "at least one task id is required")
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return BulkDeleteResult{}, err
	}
	roots, err := resolveRequestedTasks(snap, req.TaskIDs, req.TaskLocators)
	if err != nil {
		return BulkDeleteResult{}, err
	}
	roots = canonicalSubtreeRoots(roots)
	if !req.Confirm {
		return BulkDeleteResult{}, fmt.Errorf("%w: deleting %d task subtrees", ErrDestructiveConfirmationRequired, len(roots))
	}

	allRemoved := make(map[string]bool)
	allRemovedLocators := make(map[TaskLocator]bool)
	for _, r := range roots {
		for id := range collectSubtreeIDs(r.Task) {
			allRemoved[id] = true
			allRemovedLocators[TaskLocator{ProjectID: r.Project.ID, TaskID: id}] = true
		}
	}
	removedLocators := sortedTaskLocators(allRemovedLocators)
	planningArtifacts, err := planningArtifactsForRequest(PlanningReconcileRequest{
		Remove: removedLocators,
	}, s.defaultedPlanningArtifacts(nil))
	if err != nil {
		return BulkDeleteResult{}, err
	}
	artifacts = append(append([]storage.ArtifactChange(nil), artifacts...), planningArtifacts...)

	touched, transactionID, err := s.deleteTasks(snap, roots, fmt.Sprintf("%d tasks", len(roots)), artifacts)
	if err != nil {
		return BulkDeleteResult{}, err
	}
	return BulkDeleteResult{
		RemovedTaskIDs:      sortedKeys(allRemoved),
		RemovedTaskLocators: removedLocators,
		TouchedProjectIDs:   touched,
		TransactionID:       transactionID,
	}, nil
}

// canonicalSubtreeRoots removes any requested task whose ancestor is already a
// requested root in the same project. Deleting the ancestor carries that task
// exactly once, so result root references, undo context, and removal planning
// all describe the same canonical batch.
func canonicalSubtreeRoots(refs []TaskRef) []TaskRef {
	selected := make(map[TaskLocator]bool, len(refs))
	for _, ref := range refs {
		selected[TaskLocator{ProjectID: ref.Project.ID, TaskID: ref.Task.ID}] = true
	}
	roots := make([]TaskRef, 0, len(refs))
	for _, ref := range refs {
		ancestorSelected := false
		seen := make(map[string]bool)
		parentID := ref.Task.ParentID
		for parentID != nil && *parentID != "" && !seen[*parentID] {
			seen[*parentID] = true
			if selected[TaskLocator{ProjectID: ref.Project.ID, TaskID: *parentID}] {
				ancestorSelected = true
				break
			}
			parent := ref.Project.GetTaskByID(*parentID)
			if parent == nil {
				break
			}
			parentID = parent.ParentID
		}
		if !ancestorSelected {
			roots = append(roots, ref)
		}
	}
	return roots
}

// BulkResult reports a non-destructive bulk mutation: one presentation-neutral
// result per mutated task and every project the single transaction touched.
type BulkResult struct {
	// Tasks carries one result per mutated task, in request order (duplicates
	// collapsed).
	Tasks []TaskResult
	// TouchedProjectIDs lists every distinct project committed together.
	TouchedProjectIDs []string
	// TransactionID identifies the one atomic commit shared by every task.
	TransactionID string
}

// deleteTasks removes every root's subtree from its source project, carries the
// removed tasks' attachment bytes as AttachmentDelete side effects, folds any caller-supplied
// durable artifacts (e.g. a config store with the removed tasks' planning refs
// cleaned) onto the first source project's changeset, and commits every touched
// project as ONE transaction. It returns the new revision per project, the
// committed project order.
func (s *Service) deleteTasks(snap Snapshot, roots []TaskRef, undoContext string, artifacts []storage.ArtifactChange) ([]string, string, error) {
	before, err := cloneSnapshot(snap)
	if err != nil {
		return nil, "", fmt.Errorf("capture delete rollback state: %w", err)
	}
	useRollbackDocuments(before)
	committed := false
	defer func() {
		if !committed {
			restoreSnapshotProjects(snap, before)
		}
	}()

	removalByProject := make(map[string]map[string]bool)
	srcProjects := make(map[string]*models.Project)
	var srcOrder []string
	for _, root := range roots {
		pid := root.Project.ID
		if removalByProject[pid] == nil {
			removalByProject[pid] = make(map[string]bool)
			srcProjects[pid] = root.Project
			srcOrder = append(srcOrder, pid)
		}
		for id := range collectSubtreeIDs(root.Task) {
			removalByProject[pid][id] = true
		}
	}

	// Capture attachment-delete ops from the live task pointers before filtering,
	// then remove the subtree and repair dangling parent references.
	attachByProject := make(map[string][]storage.AttachmentOp)
	for pid, removed := range removalByProject {
		p := srcProjects[pid]
		for _, t := range tasksMatching(p.Tasks, removed) {
			attachByProject[pid] = append(attachByProject[pid], storage.AttachmentOp{
				Kind:          storage.AttachmentDelete,
				Task:          t,
				SourceProject: p,
			})
		}
		p.Tasks = filterOutTasks(p.Tasks, removed)
	}

	// Build one changeset per source project, carrying its attachment operations.
	// The caller's durable artifacts (e.g. a config store with the removed tasks'
	// planning refs cleaned) ride the first source project's changeset so they
	// commit atomically with the removal.
	var changesets []storage.ChangeSet
	var touchedOrder []string
	appendCS := func(p *models.Project, ops []storage.AttachmentOp, arts []storage.ArtifactChange) {
		applyPostMutationHooks(p)
		changesets = append(changesets, storage.ChangeSet{
			Project:          p,
			ExpectedRevision: snap.Revision(p.ID),
			Base:             snap.Document(p.ID),
			Artifacts:        arts,
			AttachmentOps:    ops,
			UndoAction:       "Delete Task",
			UndoContext:      undoContext,
		})
		touchedOrder = append(touchedOrder, p.ID)
	}
	for i, pid := range srcOrder {
		var arts []storage.ArtifactChange
		if i == 0 {
			arts = artifacts
		}
		appendCS(srcProjects[pid], attachByProject[pid], arts)
	}
	results, err := s.repo.Commit(changesets...)
	if err != nil {
		return nil, "", err
	}
	committed = true
	return touchedOrder, transactionIDAt(results, 0), nil
}

// resolveDistinct resolves each task id against the snapshot, preserving request
// order and dropping duplicates. An empty or unresolved id fails the whole batch
// before any mutation.
func resolveDistinct(snap Snapshot, taskIDs []string) ([]TaskRef, error) {
	refs := make([]TaskRef, 0, len(taskIDs))
	seen := make(map[string]bool, len(taskIDs))
	for _, raw := range taskIDs {
		id := strings.TrimSpace(raw)
		if id == "" {
			return nil, invalidInput("tasks", "task id is required")
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		ref, err := resolveTaskSelection(snap, id, TaskLocator{})
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

func resolveRequestedTasks(snap Snapshot, taskIDs []string, locators []TaskLocator) ([]TaskRef, error) {
	if len(taskIDs) > 0 && len(locators) > 0 {
		return nil, invalidInput("tasks", "provide task ids or task locators, not both")
	}
	if len(locators) == 0 {
		return resolveDistinct(snap, taskIDs)
	}
	refs := make([]TaskRef, 0, len(locators))
	seen := make(map[TaskLocator]bool, len(locators))
	for _, locator := range locators {
		locator.ProjectID = strings.TrimSpace(locator.ProjectID)
		locator.TaskID = strings.TrimSpace(locator.TaskID)
		if !locator.Valid() {
			return nil, invalidInput("tasks", "project id and task id are required")
		}
		if seen[locator] {
			continue
		}
		seen[locator] = true
		ref, err := resolveTaskSelection(snap, "", locator)
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

func sortedTaskLocators(values map[TaskLocator]bool) []TaskLocator {
	locators := make([]TaskLocator, 0, len(values))
	for locator := range values {
		locators = append(locators, locator)
	}
	sort.Slice(locators, func(i, j int) bool {
		if locators[i].ProjectID != locators[j].ProjectID {
			return locators[i].ProjectID < locators[j].ProjectID
		}
		return locators[i].TaskID < locators[j].TaskID
	})
	return locators
}

// distinctProjectCommits collapses a set of task references into one projectCommit
// per distinct project (first-seen order) and returns an index from project ID to
// its position, so a caller can map per-project commit results back to each task.
func distinctProjectCommits(refs []TaskRef, snap Snapshot) ([]projectCommit, map[string]int) {
	idx := make(map[string]int)
	var commits []projectCommit
	for _, r := range refs {
		if _, ok := idx[r.Project.ID]; ok {
			continue
		}
		idx[r.Project.ID] = len(commits)
		commits = append(commits, projectCommit{project: r.Project, expectedRevision: snap.Revision(r.Project.ID), base: snap.Document(r.Project.ID)})
	}
	return commits, idx
}

// collectSubtreeIDs returns the set of UUIDs in a task's subtree (the task plus
// every descendant), walking the built Children links.
func collectSubtreeIDs(root *models.Task) map[string]bool {
	ids := make(map[string]bool)
	var walk func(t *models.Task)
	walk = func(t *models.Task) {
		if t == nil || ids[t.ID] {
			return
		}
		ids[t.ID] = true
		for _, c := range t.Children {
			walk(c)
		}
	}
	walk(root)
	return ids
}

// tasksMatching returns the task pointers in tasks whose UUID is in ids, preserving
// slice order.
func tasksMatching(tasks []*models.Task, ids map[string]bool) []*models.Task {
	var out []*models.Task
	for _, t := range tasks {
		if ids[t.ID] {
			out = append(out, t)
		}
	}
	return out
}

// filterOutTasks removes tasks whose UUID is in removed and clears dangling
// ParentID references in the survivors. The
// post-mutation hooks reconcile parent completion and rebuild the tree after.
func filterOutTasks(tasks []*models.Task, removed map[string]bool) []*models.Task {
	remaining := tasks[:0]
	for _, t := range tasks {
		if removed[t.ID] {
			continue
		}
		if t.ParentID != nil && removed[*t.ParentID] {
			t.ParentID = nil
		}
		remaining = append(remaining, t)
	}
	return remaining
}

// sortedKeys returns the keys of a set in sorted order for stable results.
func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
