package app

import (
	"fmt"
	"slices"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

// SetDueDate sets or clears a task's calendar due date.
func (s *Service) SetDueDate(locator TaskLocator, due *time.Time) (TaskResult, error) {
	return s.mutateTask(locator, "Set Due Date", "", func(ref TaskRef) error {
		ref.Task.DueDate = cloneDueDatePtr(due)
		ref.Task.UpdatedAt = s.clock.Now()
		return nil
	})
}

// SetDueDateAndPlanToday is the single-task form of the atomic
// due-today planning mutation.
func (s *Service) SetDueDateAndPlanToday(locator TaskLocator, due time.Time) (TaskResult, error) {
	result, err := s.BulkSetDueDateAndPlanToday([]TaskLocator{locator}, due)
	if err != nil {
		return TaskResult{}, err
	}
	if len(result.Tasks) != 1 {
		return TaskResult{}, fmt.Errorf("app: due-today mutation returned %d tasks, want 1", len(result.Tasks))
	}
	return result.Tasks[0], nil
}

// SetPriority sets a task's priority. The value must be within the
// valid 1..PriorityMax range, or zero to clear it.
func (s *Service) SetPriority(locator TaskLocator, priority int) (TaskResult, error) {
	return s.mutateTask(locator, "Set Priority", "", func(ref TaskRef) error {
		if err := models.ValidatePriority(priority, true); err != nil {
			return invalidInput("priority", err.Error())
		}
		ref.Task.Priority = priority
		ref.Task.UpdatedAt = s.clock.Now()
		return nil
	})
}

// SetPinned pins or unpins a task.
func (s *Service) SetPinned(locator TaskLocator, pinned bool) (TaskResult, error) {
	action := "Unpin Task"
	if pinned {
		action = "Pin Task"
	}
	return s.mutateTask(locator, action, "", func(ref TaskRef) error {
		if pinned {
			ref.Task.PinAt(s.clock.Now())
		} else {
			ref.Task.UnpinAt(s.clock.Now())
		}
		return nil
	})
}

// PinnedReorderResult reports the projects a pinned-task reorder committed, so
// an adapter can reload exactly those projects and rebuild its pinned view.
type PinnedReorderResult struct {
	ProjectIDs    []string
	ShortID       int
	TransactionID string
}

// ReorderPinnedTasks swaps the PinnedAt timestamps of two pinned tasks
// so their relative order in a PinnedAt-sorted list flips, committing every
// touched project as ONE transaction. Both tasks must resolve and both must be
// pinned; when their timestamps are identical the second is nudged one second
// forward so the swap produces a strict ordering instead of a no-op. A
// same-project pair commits once; a cross-project pair commits both projects
// atomically.
func (s *Service) ReorderPinnedTasks(taskA, taskB TaskLocator) (PinnedReorderResult, error) {
	refA, snap, revA, err := s.loadTaskSelection("", taskA)
	if err != nil {
		return PinnedReorderResult{}, err
	}
	refB, err := resolveTaskSelection(snap, "", taskB)
	if err != nil {
		return PinnedReorderResult{}, err
	}
	if !refA.Task.Pinned || !refB.Task.Pinned {
		return PinnedReorderResult{}, invalidInput("pin", "both tasks must be pinned to reorder")
	}

	var aTime, bTime time.Time
	if refA.Task.PinnedAt != nil {
		aTime = *refA.Task.PinnedAt
	}
	if refB.Task.PinnedAt != nil {
		bTime = *refB.Task.PinnedAt
	}
	if aTime.Equal(bTime) {
		bTime = bTime.Add(time.Second)
	}
	newA, newB := bTime, aTime
	refA.Task.PinnedAt = &newA
	refB.Task.PinnedAt = &newB

	commits := []projectCommit{{project: refA.Project, expectedRevision: revA, base: refA.Base}}
	if refB.Project.ID != refA.Project.ID {
		commits = append(commits, projectCommit{project: refB.Project, expectedRevision: snap.Revision(refB.Project.ID), base: snap.Document(refB.Project.ID)})
	}
	undoContext := fmt.Sprintf("#%d <-> #%d", refA.Task.ShortID, refB.Task.ShortID)
	outcome, err := s.commitProjects(commits, "Reorder Pinned", undoContext)
	if err != nil {
		return PinnedReorderResult{}, err
	}
	projectIDs := make([]string, 0, len(commits))
	for _, commit := range commits {
		projectIDs = append(projectIDs, commit.project.ID)
	}
	return PinnedReorderResult{
		ProjectIDs:    projectIDs,
		ShortID:       refA.Task.ShortID,
		TransactionID: transactionIDAt(outcome.results, 0),
	}, nil
}

// ReorderPinnedTasksOrder rewrites the pin order of an explicitly named
// chain of pinned tasks so they display in the given top-to-bottom order, as ONE
// transaction over every project the chain touches. Only the chain's own PinnedAt
// values are redistributed, which is what stops a rotated pin from crossing a pin
// outside the chain. Ties are broken by nudging the earlier-displayed task one
// second upward, the same rule and direction the two-task seam applies. moved
// names the task the request is about, which the order alone cannot say, and the
// transaction is recorded against it.
func (s *Service) ReorderPinnedTasksOrder(moved TaskLocator, order []TaskLocator) (PinnedReorderResult, error) {
	if len(order) < 2 {
		return PinnedReorderResult{}, invalidInput("pin", "reordering needs at least two pinned tasks")
	}
	subject, snap, subjectRevision, err := s.loadTaskSelection("", moved)
	if err != nil {
		return PinnedReorderResult{}, err
	}
	refs := make([]TaskRef, 0, len(order))
	seen := make(map[string]bool, len(order))
	inOrder := false
	for _, locator := range order {
		ref, resolveErr := resolveTaskSelection(snap, "", locator)
		if resolveErr != nil {
			return PinnedReorderResult{}, resolveErr
		}
		if seen[ref.Task.ID] {
			return PinnedReorderResult{}, invalidInput("pin", "a task may appear only once in the reorder order")
		}
		seen[ref.Task.ID] = true
		inOrder = inOrder || ref.Task.ID == subject.Task.ID
		refs = append(refs, ref)
	}
	if !inOrder {
		return PinnedReorderResult{}, invalidInput("pin", "the moved task must appear in the reorder order")
	}

	times := make([]time.Time, 0, len(refs))
	for _, ref := range refs {
		if !ref.Task.Pinned {
			return PinnedReorderResult{}, invalidInput("pin", "every task must be pinned to reorder")
		}
		var at time.Time
		if ref.Task.PinnedAt != nil {
			at = *ref.Task.PinnedAt
		}
		times = append(times, at)
	}
	slices.SortFunc(times, func(a, b time.Time) int { return b.Compare(a) })
	for i := len(times) - 2; i >= 0; i-- {
		if !times[i].After(times[i+1]) {
			times[i] = times[i+1].Add(time.Second)
		}
	}

	commits := []projectCommit{{project: subject.Project, expectedRevision: subjectRevision, base: subject.Base}}
	committed := map[string]bool{subject.Project.ID: true}
	for i, ref := range refs {
		assigned := times[i]
		ref.Task.PinnedAt = &assigned
		if committed[ref.Project.ID] {
			continue
		}
		committed[ref.Project.ID] = true
		commits = append(commits, projectCommit{
			project:          ref.Project,
			expectedRevision: snap.Revision(ref.Project.ID),
			base:             snap.Document(ref.Project.ID),
		})
	}
	outcome, err := s.commitProjects(commits, "Reorder Pinned", fmt.Sprintf("#%d", subject.Task.ShortID))
	if err != nil {
		return PinnedReorderResult{}, err
	}
	projectIDs := make([]string, 0, len(commits))
	for _, commit := range commits {
		projectIDs = append(projectIDs, commit.project.ID)
	}
	return PinnedReorderResult{
		ProjectIDs:    projectIDs,
		ShortID:       subject.Task.ShortID,
		TransactionID: transactionIDAt(outcome.results, 0),
	}, nil
}

// Defer hides a task until the given date, or clears the deferral when
// until is nil. Deferring a nonterminal task moves it to Backlog and removes it
// from both planning horizons in the same transaction. Terminal tasks cannot be
// deferred.
func (s *Service) Defer(locator TaskLocator, until *time.Time) (TaskResult, error) {
	return s.deferTask(locator, until)
}

// DeferralRefusesState is the one deferral-eligibility predicate, and it returns
// "" when the state can be deferred. It is policy rather than an invariant —
// ApplyStateTransitionAt's doc comment says the model does not enforce policy —
// so it lives here, and every surface that must refuse a deferral reads it
// instead of restating the rule. It yields a reason rather than an error because
// InvalidInputError renders as "app: invalid defer: …", which a picker cannot
// show; deferralRefusesState wraps it for the callers that do want that error.
func DeferralRefusesState(state models.WorkflowState) string {
	if !state.IsTerminal() {
		return ""
	}
	return fmt.Sprintf("cannot defer a %s task", state)
}

// deferralRefusesState is the error-returning form the capture and inline-edit
// grammars route their @@ token through, so a refused deferral is worded the
// same wherever it is expressed.
func deferralRefusesState(state models.WorkflowState) error {
	if reason := DeferralRefusesState(state); reason != "" {
		return invalidInput("defer", reason)
	}
	return nil
}

func (s *Service) deferTask(locator TaskLocator, until *time.Time) (TaskResult, error) {
	ref, snap, revision, err := s.loadTaskSelection("", locator)
	if err != nil {
		return TaskResult{}, err
	}
	before := rollbackSnapshot(snap)
	committed := false
	defer func() {
		if !committed {
			restoreSnapshotProjects(snap, before)
		}
	}()

	now := s.clock.Now()
	if until == nil {
		ref.Task.DeferredUntil = nil
		ref.Task.UpdatedAt = now
		result, commitErr := s.commitProject(ref, revision, "Defer Task", "clear")
		if commitErr != nil {
			return TaskResult{}, commitErr
		}
		committed = true
		return result, nil
	}
	if refusal := deferralRefusesState(ref.Task.State()); refusal != nil {
		return TaskResult{}, refusal
	}
	u := *until
	if stateErr := ref.Task.ApplyDeferAt(u, now); stateErr != nil {
		return TaskResult{}, invalidInput("defer", stateErr.Error())
	}
	artifacts, err := s.transitionPlanningArtifacts(
		ref, s.defaultedPlanningArtifacts(nil), true,
	)
	if err != nil {
		return TaskResult{}, err
	}
	result, err := s.commitProjectWithArtifacts(ref, revision, artifacts, "Defer Task", "defer")
	if err != nil {
		return TaskResult{}, err
	}
	committed = true
	return result, nil
}
