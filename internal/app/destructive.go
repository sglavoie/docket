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
	// TaskID identifies the subtree root to delete. Required.
	TaskID string
	// Confirm records explicit destructive intent. A false value aborts the delete
	// before any task is removed or any commit is attempted.
	Confirm bool
}

// DeleteResult reports the outcome of a delete: which tasks were removed, the
// deleted root's canonical reference, and every project the single atomic
// transaction touched (the source plus any project whose cross-project references
// were repaired).
type DeleteResult struct {
	// RemovedTaskIDs are the UUIDs removed (root plus descendants), sorted.
	RemovedTaskIDs []string
	// QualifiedRef is the deleted root task's "slug:#short-id" reference, captured
	// before removal.
	QualifiedRef string
	// SourceProjectID owns the deleted subtree.
	SourceProjectID string
	// SourceRevision is the source project's new revision after the commit.
	SourceRevision string
	// CrossProjectRefsCleared counts dependency/related references dropped from
	// OTHER projects that pointed at a deleted task.
	CrossProjectRefsCleared int
	// TouchedProjectIDs lists every project committed in the transaction, source
	// first, then any cross-project-reference repairs in snapshot order.
	TouchedProjectIDs []string
}

// Delete permanently removes a task and its whole subtree, committing the removal
// as ONE transaction. The removed tasks' attachment bytes ride the same
// transaction (AttachmentDelete side effects), and dependency/related references
// to the removed tasks are cleaned both within the source project and across every
// other project in the workspace — each repaired project joins the same commit, so
// a delete plus its reference cleanup is atomic. Any artifacts the caller supplies
// — the durable planning refs (Focus Queue, Daily Plan, dismissals) cleaned of the
// removed tasks — ride the same transaction so a partial delete can never leave
// stale planning refs behind; machine-local recent visits are deliberately excluded
// and repaired separately by the adapter after a successful commit. The request must
// carry explicit destructive intent; an unconfirmed request never mutates anything.
func (s *Service) Delete(req DeleteRequest, artifacts ...storage.ArtifactChange) (DeleteResult, error) {
	ref, snap, _, err := s.loadTask(req.TaskID)
	if err != nil {
		return DeleteResult{}, err
	}
	if !req.Confirm {
		return DeleteResult{}, fmt.Errorf("%w: deleting task #%d and its subtree", ErrDestructiveConfirmationRequired, ref.Task.ShortID)
	}

	removed := collectSubtreeIDs(ref.Task)
	qualifiedRef := fmt.Sprintf("%s:#%d", ref.Project.Slug, ref.Task.ShortID)
	undoContext := fmt.Sprintf("#%d %s", ref.Task.ShortID, ref.Task.Title)

	revs, touched, crossCleared, err := s.deleteTasks(snap, []TaskRef{ref}, undoContext, artifacts)
	if err != nil {
		return DeleteResult{}, err
	}
	return DeleteResult{
		RemovedTaskIDs:          sortedKeys(removed),
		QualifiedRef:            qualifiedRef,
		SourceProjectID:         ref.Project.ID,
		SourceRevision:          revs[ref.Project.ID],
		CrossProjectRefsCleared: crossCleared,
		TouchedProjectIDs:       touched,
	}, nil
}

// BulkDeleteRequest asks to delete several task subtrees, possibly spanning
// projects, as one transaction. Confirm carries the explicit destructive intent
// the batch requires.
type BulkDeleteRequest struct {
	// TaskIDs identifies each subtree root to delete. At least one is required;
	// duplicates and roots nested under another selected root are de-duplicated.
	TaskIDs []string
	// Confirm records explicit destructive intent for the whole batch.
	Confirm bool
}

// BulkDeleteResult reports a bulk delete: the removed root references, the total
// removed count, and every project the single transaction touched.
type BulkDeleteResult struct {
	// RemovedRootRefs are the canonical references of the requested roots.
	RemovedRootRefs []string
	// RemovedTaskIDs are all UUIDs removed across every subtree, sorted.
	RemovedTaskIDs []string
	// CrossProjectRefsCleared counts references dropped from projects outside the
	// deleted subtrees.
	CrossProjectRefsCleared int
	// TouchedProjectIDs lists every project committed together.
	TouchedProjectIDs []string
}

// BulkDelete removes several task subtrees — which may live in different projects —
// as ONE atomic transaction. Every affected project (a source of removals or a
// carrier of cross-project references to a removed task) commits together, so the
// whole batch lands or nothing does. Any artifacts the caller supplies — the durable
// planning refs cleaned of every removed task — ride the same transaction. The
// request must carry explicit destructive intent.
func (s *Service) BulkDelete(req BulkDeleteRequest, artifacts ...storage.ArtifactChange) (BulkDeleteResult, error) {
	if len(req.TaskIDs) == 0 {
		return BulkDeleteResult{}, invalidInput("tasks", "at least one task id is required")
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return BulkDeleteResult{}, err
	}
	roots, err := resolveDistinct(snap, req.TaskIDs)
	if err != nil {
		return BulkDeleteResult{}, err
	}
	if !req.Confirm {
		return BulkDeleteResult{}, fmt.Errorf("%w: deleting %d task subtrees", ErrDestructiveConfirmationRequired, len(roots))
	}

	rootRefs := make([]string, 0, len(roots))
	allRemoved := make(map[string]bool)
	for _, r := range roots {
		rootRefs = append(rootRefs, fmt.Sprintf("%s:#%d", r.Project.Slug, r.Task.ShortID))
		for id := range collectSubtreeIDs(r.Task) {
			allRemoved[id] = true
		}
	}

	_, touched, crossCleared, err := s.deleteTasks(snap, roots, fmt.Sprintf("%d tasks", len(roots)), artifacts)
	if err != nil {
		return BulkDeleteResult{}, err
	}
	return BulkDeleteResult{
		RemovedRootRefs:         rootRefs,
		RemovedTaskIDs:          sortedKeys(allRemoved),
		CrossProjectRefsCleared: crossCleared,
		TouchedProjectIDs:       touched,
	}, nil
}

// BulkResult reports a non-destructive bulk mutation: one presentation-neutral
// result per mutated task and every project the single transaction touched.
type BulkResult struct {
	// Tasks carries one result per mutated task, in request order (duplicates
	// collapsed).
	Tasks []TaskResult
	// TouchedProjectIDs lists every distinct project committed together.
	TouchedProjectIDs []string
	// RuleTriggerCounts accumulates fired workflow-rule IDs when the request
	// carried rules; the adapter persists them to its config store best-effort.
	RuleTriggerCounts map[string]int
}

// deleteTasks removes every root's subtree from its source project, carries the
// removed tasks' attachment bytes as AttachmentDelete side effects, repairs
// cross-project references pointing at any removed task, folds any caller-supplied
// durable artifacts (e.g. a config store with the removed tasks' planning refs
// cleaned) onto the first source project's changeset, and commits every touched
// project as ONE transaction. It returns the new revision per project, the
// committed project order (source projects first, then cross-reference repairs in
// snapshot order), and the count of cross-project references cleared.
func (s *Service) deleteTasks(snap Snapshot, roots []TaskRef, undoContext string, artifacts []storage.ArtifactChange) (map[string]string, []string, int, error) {
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
	// then remove the subtree and clean same-project dangling references.
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

	// Repair cross-project references from every other project to any removed task.
	crossCleared := 0
	touchedByCross := make(map[string]bool)
	for _, p := range snap.Projects() {
		for pid, removed := range removalByProject {
			deleted := srcProjects[pid]
			if p.ID == deleted.ID {
				continue
			}
			if n := clearCrossProjectRefs(p, deleted, removed); n > 0 {
				crossCleared += n
				touchedByCross[p.ID] = true
			}
		}
	}

	// Build one changeset per touched project: source projects first (carrying
	// their attachment ops), then cross-reference-only repairs in snapshot order.
	// The caller's durable artifacts (e.g. a config store with the removed tasks'
	// planning refs cleaned) ride the first source project's changeset so they
	// commit atomically with the removal.
	var changesets []storage.ChangeSet
	var touchedOrder []string
	added := make(map[string]bool)
	appendCS := func(p *models.Project, ops []storage.AttachmentOp, arts []storage.ArtifactChange) {
		applyPostMutationHooks(p)
		changesets = append(changesets, storage.ChangeSet{
			Project:          p,
			ExpectedRevision: snap.Revision(p.ID),
			Artifacts:        arts,
			AttachmentOps:    ops,
			UndoAction:       "Delete Task",
			UndoContext:      undoContext,
		})
		touchedOrder = append(touchedOrder, p.ID)
		added[p.ID] = true
	}
	for i, pid := range srcOrder {
		var arts []storage.ArtifactChange
		if i == 0 {
			arts = artifacts
		}
		appendCS(srcProjects[pid], attachByProject[pid], arts)
	}
	for _, p := range snap.Projects() {
		if added[p.ID] || !touchedByCross[p.ID] {
			continue
		}
		appendCS(p, nil, nil)
	}

	results, err := s.repo.Commit(changesets...)
	if err != nil {
		return nil, nil, 0, err
	}
	revs := make(map[string]string, len(touchedOrder))
	for i, pid := range touchedOrder {
		revs[pid] = revisionAt(results, i)
	}
	return revs, touchedOrder, crossCleared, nil
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
		ref, ok := snap.Lookup(id)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrTaskNotFound, id)
		}
		refs = append(refs, ref)
	}
	return refs, nil
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
		commits = append(commits, projectCommit{project: r.Project, expectedRevision: snap.Revision(r.Project.ID)})
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

// filterOutTasks removes tasks whose UUID is in removed and clears same-project
// dangling ParentID, DependsOn, and RelatedTo references in the survivors. The
// post-mutation hooks reconcile derived state (auto-unblock, tree rebuild) after.
func filterOutTasks(tasks []*models.Task, removed map[string]bool) []*models.Task {
	remaining := tasks[:0]
	for _, t := range tasks {
		if removed[t.ID] {
			continue
		}
		if t.ParentID != nil && removed[*t.ParentID] {
			t.ParentID = nil
		}
		if len(t.DependsOn) > 0 {
			t.DependsOn = filterOutRefs(t.DependsOn, removed)
		}
		if len(t.RelatedTo) > 0 {
			t.RelatedTo = filterOutRefs(t.RelatedTo, removed)
		}
		remaining = append(remaining, t)
	}
	return remaining
}

// filterOutRefs drops bare same-project references whose UUID is in removed,
// preserving cross-project references (those are repaired separately).
func filterOutRefs(refs []string, removed map[string]bool) []string {
	out := refs[:0]
	for _, ref := range refs {
		if parsed := models.ParseTaskRef(ref); !parsed.IsCrossProject() && removed[parsed.TaskID] {
			continue
		}
		out = append(out, ref)
	}
	return out
}

// clearCrossProjectRefs drops dependency/related references in project p that point
// at a removed task in the deleted project, matching the cross-project reference by
// the deleted project's ID or slug. It returns how many references were cleared.
func clearCrossProjectRefs(p *models.Project, deleted *models.Project, removed map[string]bool) int {
	cleared := 0
	for _, t := range p.Tasks {
		before := len(t.DependsOn)
		t.DependsOn = filterCrossRefs(t.DependsOn, deleted, removed)
		cleared += before - len(t.DependsOn)

		before = len(t.RelatedTo)
		t.RelatedTo = filterCrossRefs(t.RelatedTo, deleted, removed)
		cleared += before - len(t.RelatedTo)
	}
	return cleared
}

// filterCrossRefs removes cross-project references pointing at a removed task in the
// deleted project.
func filterCrossRefs(refs []string, deleted *models.Project, removed map[string]bool) []string {
	if len(refs) == 0 {
		return refs
	}
	out := refs[:0]
	for _, ref := range refs {
		parsed := models.ParseTaskRef(ref)
		if parsed.IsCrossProject() && (parsed.ProjectID == deleted.ID || parsed.ProjectID == deleted.Slug) && removed[parsed.TaskID] {
			continue
		}
		out = append(out, ref)
	}
	return out
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
