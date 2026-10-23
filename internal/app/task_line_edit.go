package app

import (
	"fmt"
	"time"
)

// TaskLineParentMode describes the parent edge of an inline task edit. Preserve
// is the zero value so omitted parent syntax cannot accidentally move a task.
// Root clears the parent, while Set requires a project-qualified Parent. Adopted
// children ride the separate additive Children field, which has no clear mode.
type TaskLineParentMode uint8

const (
	TaskLineParentPreserve TaskLineParentMode = iota
	TaskLineParentRoot
	TaskLineParentSet
)

// TaskLineParentPatch is the resolved parent intent produced by the inline-text
// adapter. Parent is used only for TaskLineParentSet.
type TaskLineParentPatch struct {
	Mode   TaskLineParentMode
	Parent TaskLocator
}

// TaskLineEditRequest is the application-layer command for one parsed inline
// task edit. Source is always qualified. Title is the complete post-edit title;
// other fields are preserve-by-default patches. DueSet distinguishes an omitted
// due token from an explicit clear (DueSet true with Due nil).
//
// DestinationProjectID selects the final owner. A different project moves the
// complete subtree while applying content and planning patches only to its root.
//
// Children is additive: every locator names an existing task in the effective
// project that the edited task adopts, keeping its own subtree. Unmentioned
// children are untouched and repeated locators collapse to the first occurrence.
//
// Pinned is preserve-on-nil like Priority, and re-declaring the state the task
// already holds is a no-op that keeps PinnedAt. Review is preserve-on-nil for
// the same reason and carries no timestamp of its own.
//
// Description is preserve-on-nil too, and carries the body an external-editor
// round trip returns so title and description land in the same transaction.
// ResumeNote is preserve-on-nil as well; a pointer to the empty note clears it,
// through the same models.ValidateResumeNote gate SetResumeNote applies.
type TaskLineEditRequest struct {
	Source               TaskLocator
	Title                string
	Description          *string
	ResumeNote           *string
	Priority             *int
	DueSet               bool
	Due                  *time.Time
	DeferSet             bool
	Defer                *time.Time
	Context              *string
	Pinned               *bool
	Review               *bool
	Destination          *CaptureDestination
	TagsToAdd            []string
	TagsToRemove         []string
	DestinationProjectID string
	Parent               TaskLineParentPatch
	Children             []TaskLocator
	// Dependencies patches the task's soft references. Its zero value expresses no
	// intent, so an edit that never mentioned one leaves the stored set alone.
	// Additions are judged against the effective destination project, which is what
	// makes a line that moves the task and depends on its own new address a
	// self-reference.
	Dependencies      DependencyPatch
	PlanningArtifacts PlanningArtifactBuilder

	// Action names the edit in the undo ledger. A caller that reaches this
	// request through a narrower surface than the inline editor sets it so the
	// undo entry says what the user did; blank keeps the generic edit label.
	Action string
}

// TaskLineEditResult identifies the final task and the single durable
// transaction that produced it. Changed is false for semantic no-ops; those
// results deliberately have no transaction ID and no affected projects.
type TaskLineEditResult struct {
	Task                 TaskLocator
	ShortID              int
	Title                string
	MovedDescendantCount int
	AffectedProjectIDs   []string
	TransactionID        string
	Changed              bool
	StateEffect          *TaskStateEffect
	// DueEdited mirrors the request's DueSet: the edited line carried an explicit
	// deadline instruction. It is intent, not effect, so a clear of a deadline
	// that was already absent still reports true — a caller remembering a cleared
	// wall clock has to hear that the user overrode it, and the resulting
	// deadline cannot tell it apart from a line that never mentioned one.
	DueEdited bool
	// Moves is the committed source-to-destination address map of an edit that
	// crossed a project boundary, every relocated descendant included. An edit
	// that stayed in its project leaves it nil.
	Moves []TaskAddressChange
}

// EditTaskLine applies every inline-text patch as one application transaction.
// It resolves exactly one fresh snapshot, validates the complete request before
// mutation, and restores all repository-owned in-memory projects on any
// structural, planning, revision, attachment, or persistence failure.
func (s *Service) EditTaskLine(req TaskLineEditRequest) (TaskLineEditResult, error) {
	snap, err := s.repo.Snapshot()
	if err != nil {
		return TaskLineEditResult{}, err
	}
	ref, err := resolveTaskSelection(snap, "", req.Source)
	if err != nil {
		return TaskLineEditResult{}, err
	}
	prepared, err := prepareTaskLineEdit(snap, ref, req)
	if err != nil {
		return TaskLineEditResult{}, err
	}

	before := cloneSnapshot(snap)
	useRollbackDocuments(before)
	committed := false
	defer func() {
		if !committed {
			restoreSnapshotProjects(snap, before)
		}
	}()

	if prepared.destinationProject.ID != ref.Project.ID {
		// Crossing a project boundary always changes task ownership (and may
		// requalify descendants, planning references, and attachments), so this
		// branch cannot be a semantic no-op. The no-op guard below intentionally
		// applies only to edits that remain in the source project.
		result, editErr := s.editTaskLineAcrossProjects(snap, ref, prepared, req.PlanningArtifacts)
		if editErr != nil {
			return TaskLineEditResult{}, editErr
		}
		committed = true
		result.DueEdited = req.DueSet
		// Ownership changes do not imply creation. Compare the final task to
		// its source document, where this identity lived before the edit.
		result.StateEffect = taskStateEffect(TaskRef{
			Task: snap.ProjectByID(result.Task.ProjectID).GetTaskByID(result.Task.TaskID),
			Base: before.Document(req.Source.ProjectID),
		})
		return result, nil
	}

	// The edited task's own subtree is measured before any adoption, so an
	// adopted child never inflates the count of descendants this edit relocated.
	movedSubtreeSize := 0
	if prepared.parentChanged {
		movedSubtreeSize = len(collectSubtreeIDs(ref.Task))
	}

	changed := applyPreparedTaskLineFields(ref.Task, prepared, s.clock.Now())
	structuralChanges, err := applyTaskLineEditEdges(ref.Project, ref.Task, prepared.parent, prepared.parentChanged, prepared.children, s.clock.Now())
	if err != nil {
		return TaskLineEditResult{}, err
	}
	changed = changed || structuralChanges > 0

	// Hierarchy lands before the lifecycle move so a terminal destination is
	// refused against the children this edit produced, not the ones it replaced.
	if guardErr := taskLineDestinationChildrenGuard(ref, prepared.destination); guardErr != nil {
		return TaskLineEditResult{}, guardErr
	}
	planningTargets, destinationChanged, err := s.applyTaskLineDestination(ref, prepared.destination)
	if err != nil {
		return TaskLineEditResult{}, err
	}
	changed = changed || destinationChanged
	deferChanged, err := applyPreparedTaskLineDeferral(ref.Task, prepared, s.clock.Now())
	if err != nil {
		return TaskLineEditResult{}, err
	}
	changed = deferChanged || changed
	artifacts, err := planningArtifactsForTargets(
		planningTargets,
		nil,
		s.defaultedPlanningArtifacts(req.PlanningArtifacts),
	)
	if err != nil {
		return TaskLineEditResult{}, err
	}

	result := taskLineEditResult(ref, "", false)
	result.DueEdited = req.DueSet
	if !changed && len(artifacts) == 0 {
		committed = true
		return result, nil
	}

	if changed {
		ref.Task.UpdatedAt = s.clock.Now()
	}
	committedResult, err := s.commitProjectWithArtifacts(
		ref,
		snap.Revision(ref.Project.ID),
		artifacts,
		prepared.action,
		fmt.Sprintf("#%d %s", ref.Task.ShortID, ref.Task.Title),
	)
	if err != nil {
		return TaskLineEditResult{}, err
	}
	committed = true
	result = taskLineEditResult(ref, committedResult.TransactionID, true)
	result.DueEdited = req.DueSet
	result.MovedDescendantCount = max(0, movedSubtreeSize-1)
	return result, nil
}

func taskLineEditResult(ref TaskRef, transactionID string, changed bool) TaskLineEditResult {
	result := TaskLineEditResult{
		Task:          TaskLocator{ProjectID: ref.Project.ID, TaskID: ref.Task.ID},
		ShortID:       ref.Task.ShortID,
		Title:         ref.Task.Title,
		TransactionID: transactionID,
		Changed:       changed,
	}
	if changed {
		result.StateEffect = taskStateEffect(ref)
		result.AffectedProjectIDs = []string{ref.Project.ID}
	}
	return result
}
