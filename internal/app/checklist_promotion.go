package app

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

// checklistPromotionUndoAction is the ledger action one promotion records, and
// therefore the action an installed CommitBarrier names to hold a promotion at
// the instant its snapshot is frozen and nothing is written yet. It is the
// application layer's own vocabulary: the key that reaches it is a presentation
// concern and never spelled here.
const checklistPromotionUndoAction = "Promote Checklist Item"

// ChecklistPromotionRequest converts exactly one of a task's checklist items
// into a child task of that same task.
//
// Checklist items carry no stored identity, so an index alone cannot name one:
// a concurrent edit, reorder, or removal would silently move the index onto a
// different item. Checklist is therefore the complete ordered checklist the
// caller froze when the gesture was made, duplicate texts and completion flags
// included, and the promotion is refused whole unless the owner's current
// checklist still equals it exactly. That comparison is the item's identity.
type ChecklistPromotionRequest struct {
	// Owner is the project-qualified task whose checklist item is promoted.
	Owner TaskLocator
	// Index is the zero-based position of the item inside Checklist.
	Index int
	// Checklist is the complete ordered checklist as the caller read it.
	Checklist []models.ChecklistItem
}

// ChecklistPromotionResult reports both halves of the one transaction a
// promotion commits. Owner is the task whose checklist lost the item and Child
// is the task that replaced it. Both carry the same Revision and the same
// TransactionID — there is no third field naming it, because two results that
// could disagree about which transaction produced them is exactly the state this
// use case exists to make impossible.
type ChecklistPromotionResult struct {
	Owner TaskResult
	Child TaskResult
}

// PromoteChecklistItem converts one checklist item into a child task of its
// owner and commits both halves — the created child and the shortened checklist
// — as ONE transaction with one ledger entry, so undo restores the item at its
// original position and redo restores the same child identity.
//
// It is deliberately not Capture followed by RemoveChecklistItem: those are two
// transactions and two ledger entries, and a failure between them would leave
// either a child with no item consumed or an item consumed with no child. The
// creation invariants are reused rather than reimplemented — short-ID
// allocation, lifecycle initialization, parent depth validation, append
// ordering, and the post-mutation reconciliation that reopens a completed
// ancestor — and the whole read set is guarded by a strict revision
// precondition, so a concurrent checklist edit, reorder, or competing creation
// refuses the promotion instead of merging it onto a document the calculation
// never saw.
//
// The child copies no metadata from its owner: it takes the item's literal
// text, the owner as its parent, Backlog or Done, and the item's original index
// for a possible reverse conversion. Everything else starts empty.
func (s *Service) PromoteChecklistItem(req ChecklistPromotionRequest) (ChecklistPromotionResult, error) {
	if !req.Owner.Valid() {
		return ChecklistPromotionResult{}, invalidInput("task", "project id and task id are required")
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return ChecklistPromotionResult{}, err
	}
	ref, err := resolveTaskSelection(snap, "", req.Owner)
	if err != nil {
		return ChecklistPromotionResult{}, err
	}
	revision := snap.Revision(ref.Project.ID)

	// Every refusal that the workspace alone decides is settled here, before a
	// short ID is consumed or a pointer moves: the frozen checklist still matches,
	// the index names an item inside it, that item's literal text is a title the
	// canonical writer would accept, and the owner can still take a child.
	if frozenErr := requireFrozenChecklist(ref.Task, req); frozenErr != nil {
		return ChecklistPromotionResult{}, frozenErr
	}
	item := ref.Task.Checklist[req.Index]
	if titleErr := validatePromotedTaskTitle(item.Text); titleErr != nil {
		return ChecklistPromotionResult{}, titleErr
	}
	if _, parentErr := resolveChildParent(ref.Project, ref.Task.ID); parentErr != nil {
		return ChecklistPromotionResult{}, parentErr
	}
	destination, err := promotedChecklistDestination(item.Done)
	if err != nil {
		return ChecklistPromotionResult{}, err
	}

	before := cloneSnapshot(snap)
	useRollbackDocuments(before)
	committed := false
	defer func() {
		if !committed {
			restoreSnapshotProjects(snap, before)
		}
	}()

	// The owner is touched because its checklist changed; the child's lifecycle
	// timestamps come from the same service clock through the shared creation
	// initialization, which is the only place a created task's clocks are set.
	now := s.clock.Now()
	child, err := s.buildPromotedChild(ref, item.Text, destination, req.Index, now)
	if err != nil {
		return ChecklistPromotionResult{}, err
	}
	insertCapturedTask(ref.Project, child, false)
	// Link the child into the hierarchy before the commit runs the post-mutation
	// hooks: the reconciliation that reopens a completed ancestor reads the built
	// tree, not the stored parent ids, so a promotion under a Done owner would
	// otherwise commit a document whose parent holds an open child.
	ref.Project.BuildTaskTree()
	// Clone before deleting: the rollback snapshot retains the original backing
	// array, so splicing in place would edit the value a refusal reads back.
	ref.Task.Checklist = slices.Delete(slices.Clone(ref.Task.Checklist), req.Index, req.Index+1)
	ref.Task.UpdatedAt = now

	commits := []projectCommit{{
		project:          ref.Project,
		expectedRevision: revision,
		base:             snap.Document(ref.Project.ID),
	}}
	if artifactErr := attachPlanningArtifacts(
		commits,
		[]PlanningReconcileTarget{destination.planningTarget(child.ID, ref.Project.ID)},
		s.defaultedPlanningArtifacts(nil),
	); artifactErr != nil {
		return ChecklistPromotionResult{}, artifactErr
	}
	if guardErr := attachReadSetRevisionGuards(commits); guardErr != nil {
		return ChecklistPromotionResult{}, guardErr
	}
	awaitTaskCommitBarrier(ref, revision, checklistPromotionUndoAction)
	outcome, err := s.commitProjects(
		commits, checklistPromotionUndoAction, fmt.Sprintf("#%d %s", child.ShortID, child.Title))
	if err != nil {
		return ChecklistPromotionResult{}, err
	}
	committed = true

	newRevision := revisionAt(outcome.results, 0)
	transactionID := transactionIDAt(outcome.results, 0)
	return ChecklistPromotionResult{
		Owner: newTaskResult(ref, newRevision, transactionID),
		Child: newTaskResult(TaskRef{Task: child, Project: ref.Project}, newRevision, transactionID),
	}, nil
}

// requireFrozenChecklist refuses a promotion whose owner no longer carries the
// exact checklist the caller read. Comparing the whole ordered list rather than
// the item at the index is the point: two items may share a text, an item may
// have been ticked, and either would make an index-only check promote something
// nobody asked for. The index is validated against that same frozen list, so a
// request naming a position outside it is refused by the ordinary range error.
func requireFrozenChecklist(task *models.Task, req ChecklistPromotionRequest) error {
	if !slices.Equal(task.Checklist, req.Checklist) {
		return invalidInput("checklist", fmt.Sprintf(
			"the checklist of #%d changed since it was read; nothing was promoted", task.ShortID))
	}
	return validChecklistIndex(req.Index, len(task.Checklist))
}

// validatePromotedTaskTitle holds a checklist item's literal text to every rule
// a stored task title obeys, before the item is consumed. The text is never
// rewritten: a promotion preserves what the item says — a `#tag`, a `$state`, or
// a `~ref` inside it stays prose, because nothing reparses it — so text the
// canonical writer would refuse is refused by name here rather than trimmed,
// normalized, or discovered at render time.
func validatePromotedTaskTitle(text string) error {
	if err := models.ValidateTaskTitle(text); err != nil {
		return invalidInput("title", err.Error())
	}
	// The task heading is written and parsed trimmed, so an untrimmed title is
	// exactly what format.ValidateV6TaskRecord refuses. Rewriting the item's text
	// to fit would promote something the user did not write.
	if text != strings.TrimSpace(text) {
		return invalidInput("title", "checklist item text must not begin or end with whitespace")
	}
	return nil
}

// promotedChecklistDestination is the whole metadata decision a promotion makes:
// an unchecked item becomes Backlog work and a checked one becomes a completed
// task, with the completion instant recorded by the ordinary lifecycle
// transition. Neither destination carries a planning membership, so Today and
// Next are untouched by construction rather than by a guard.
func promotedChecklistDestination(done bool) (captureDestinationEffect, error) {
	if done {
		return normalizeCaptureDestination(CaptureDestination(models.StateDone), true)
	}
	return normalizeCaptureDestination(CaptureDestinationBacklog, true)
}

// buildPromotedChild mints the child through the same construction seam every
// creation uses: a generated UUID, the project's next short ID, the canonical
// title validation, and the one lifecycle initialization path, which is what
// stamps the state clock and — for a checked item — the completion instant. It
// sets the owner as parent and remembers the consumed item's index. No priority, tags, context,
// description, due date, deferral, pin, review flag, resume note, attachment,
// comment, or reference is inherited, because a checklist item carries none of
// them.
func (s *Service) buildPromotedChild(
	ref TaskRef, title string, destination captureDestinationEffect, index int, now time.Time,
) (*models.Task, error) {
	child, err := newTaskForCreation(s.ids.NewID(), ref.Project.GetNextShortID(), title)
	if err != nil {
		return nil, invalidInput("title", err.Error())
	}
	parentID := ref.Task.ID
	child.ParentID = &parentID
	child.ChecklistOrigin = &models.ChecklistOrigin{ProjectID: ref.Project.ID, TaskID: ref.Task.ID, Index: index}
	if err := s.initializeCreatedTask(child, destination.state, now); err != nil {
		return nil, err
	}
	return child, nil
}
