package app

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// TaskLineParentMode describes the structural portion of an inline task edit.
// Preserve is the zero value so omitted parent syntax cannot accidentally move a
// task. Root clears the parent, while Set requires a project-qualified Parent.
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
type TaskLineEditRequest struct {
	Source               TaskLocator
	Title                string
	Priority             *int
	DueSet               bool
	Due                  *time.Time
	Destination          *CaptureDestination
	TagsToAdd            []string
	TagsToRemove         []string
	DestinationProjectID string
	Parent               TaskLineParentPatch
	PlanningArtifacts    PlanningArtifactBuilder
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
}

type preparedTaskLineEdit struct {
	title              string
	priority           *int
	dueSet             bool
	due                *time.Time
	tags               []string
	destination        *CaptureDestination
	parentMode         TaskLineParentMode
	parent             *models.Task
	parentChanged      bool
	destinationProject *models.Project
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

	before, err := cloneSnapshot(snap)
	if err != nil {
		return TaskLineEditResult{}, fmt.Errorf("capture task-line edit rollback state: %w", err)
	}
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
		return result, nil
	}

	changed := applyPreparedTaskLineFields(ref.Task, prepared, s.clock.Now())
	if prepared.parentChanged {
		if reparentErr := ref.Project.ReparentTask(ref.Task, prepared.parent); reparentErr != nil {
			return TaskLineEditResult{}, invalidInput("parent", reparentErr.Error())
		}
		changed = true
	}

	planningTargets, destinationChanged, err := s.applyTaskLineDestination(ref, prepared.destination)
	if err != nil {
		return TaskLineEditResult{}, err
	}
	changed = changed || destinationChanged
	artifacts, err := planningArtifactsForTargets(
		planningTargets,
		nil,
		s.defaultedPlanningArtifacts(req.PlanningArtifacts),
	)
	if err != nil {
		return TaskLineEditResult{}, err
	}

	result := taskLineEditResult(ref, "", false)
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
		"Edit Task Line",
		fmt.Sprintf("#%d %s", ref.Task.ShortID, ref.Task.Title),
	)
	if err != nil {
		return TaskLineEditResult{}, err
	}
	committed = true
	result = taskLineEditResult(ref, committedResult.TransactionID, true)
	if prepared.parentChanged {
		result.MovedDescendantCount = max(0, len(collectSubtreeIDs(ref.Task))-1)
	}
	return result, nil
}

func (s *Service) editTaskLineAcrossProjects(snap Snapshot, sourceRef TaskRef, prepared preparedTaskLineEdit, builder PlanningArtifactBuilder) (TaskLineEditResult, error) {
	destination := prepared.destinationProject
	backups := []hierarchyProjectBackup{
		captureHierarchyProject(sourceRef.Project),
		captureHierarchyProject(destination),
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		for i := len(backups) - 1; i >= 0; i-- {
			backups[i].restore()
		}
	}()

	applyPreparedTaskLineFields(sourceRef.Task, prepared, s.clock.Now())

	var destinationParent *models.Task
	if prepared.parentMode == TaskLineParentSet {
		destinationParent = prepared.parent
	}
	move, err := moveTaskSubtreeBetweenProjects(sourceRef.Project, sourceRef.Task, destination, destinationParent)
	if err != nil {
		return TaskLineEditResult{}, invalidInput("move", err.Error())
	}

	finalRef := TaskRef{Task: sourceRef.Task, Project: destination, Base: snap.Document(destination.ID)}
	planningTargets, _, err := s.applyTaskLineDestination(finalRef, prepared.destination)
	if err != nil {
		return TaskLineEditResult{}, err
	}
	artifacts, err := taskLineCrossProjectPlanningArtifacts(
		planningTargets,
		move.PlanningMoves,
		s.defaultedPlanningArtifacts(builder),
	)
	if err != nil {
		return TaskLineEditResult{}, err
	}

	commits := []projectCommit{
		{
			project:          sourceRef.Project,
			expectedRevision: snap.Revision(sourceRef.Project.ID),
			base:             sourceRef.Base,
			artifacts:        artifacts,
			attachmentOps:    move.AttachmentOps,
		},
		{
			project:          destination,
			expectedRevision: snap.Revision(destination.ID),
			base:             snap.Document(destination.ID),
		},
	}
	outcome, err := s.commitProjects(
		commits,
		"Edit Task Line",
		fmt.Sprintf("#%d %s → %s", finalRef.Task.ShortID, finalRef.Task.Title, destination.Slug),
	)
	if err != nil {
		return TaskLineEditResult{}, err
	}

	result := taskLineEditResult(
		finalRef,
		transactionIDAt(outcome.results, 0),
		true,
	)
	result.MovedDescendantCount = max(0, len(move.MovedTasks)-1)
	result.AffectedProjectIDs = []string{sourceRef.Project.ID, destination.ID}
	committed = true
	return result, nil
}

func taskLineCrossProjectPlanningArtifacts(targets []PlanningReconcileTarget, moves []PlanningProjectMove, builder PlanningArtifactBuilder) ([]storage.ArtifactChange, error) {
	if builder == nil {
		for _, target := range targets {
			if target.Today == PlanningTodayAdd || target.Next == PlanningNextAdd {
				return nil, invalidInput("destination", "planning persistence is unavailable")
			}
		}
		return nil, nil
	}
	return planningArtifactsForRequest(PlanningReconcileRequest{Targets: targets, Moves: moves}, builder)
}

func prepareTaskLineEdit(snap Snapshot, ref TaskRef, req TaskLineEditRequest) (preparedTaskLineEdit, error) {
	title := strings.TrimSpace(req.Title)
	if err := models.ValidateTaskTitle(title); err != nil {
		return preparedTaskLineEdit{}, invalidInput("title", err.Error())
	}
	prepared := preparedTaskLineEdit{
		title:              title,
		dueSet:             req.DueSet,
		due:                cloneCalendarDatePtr(req.Due),
		parentMode:         req.Parent.Mode,
		destinationProject: ref.Project,
	}
	if req.Priority != nil {
		if err := models.ValidatePriority(*req.Priority, true); err != nil {
			return preparedTaskLineEdit{}, invalidInput("priority", err.Error())
		}
		priority := *req.Priority
		prepared.priority = &priority
	}

	add, err := normalizeTagList(req.TagsToAdd)
	if err != nil {
		return preparedTaskLineEdit{}, err
	}
	remove, err := normalizeTagList(req.TagsToRemove)
	if err != nil {
		return preparedTaskLineEdit{}, err
	}
	for _, tag := range add {
		if slices.Contains(remove, tag) {
			return preparedTaskLineEdit{}, invalidInput("tag", fmt.Sprintf("cannot add and remove %q in one edit", tag))
		}
	}
	prepared.tags = patchedTaskLineTags(ref.Task.Tags, add, remove)

	effectiveProjectID := ref.Project.ID
	if projectID := strings.TrimSpace(req.DestinationProjectID); projectID != "" {
		destinationProject := snap.ProjectByID(projectID)
		if destinationProject == nil {
			return preparedTaskLineEdit{}, fmt.Errorf("%w: %s", ErrProjectNotFound, projectID)
		}
		prepared.destinationProject = destinationProject
		effectiveProjectID = projectID
	}

	switch req.Parent.Mode {
	case TaskLineParentPreserve:
		if req.Parent.Parent.ProjectID != "" || req.Parent.Parent.TaskID != "" {
			return preparedTaskLineEdit{}, invalidInput("parent", "preserve mode cannot include a parent locator")
		}
	case TaskLineParentRoot:
		if req.Parent.Parent.ProjectID != "" || req.Parent.Parent.TaskID != "" {
			return preparedTaskLineEdit{}, invalidInput("parent", "root mode cannot include a parent locator")
		}
		prepared.parentChanged = effectiveProjectID != ref.Project.ID || ref.Task.ParentID != nil
	case TaskLineParentSet:
		parentRef, resolveErr := resolveTaskSelection(snap, "", req.Parent.Parent)
		if resolveErr != nil {
			return preparedTaskLineEdit{}, resolveErr
		}
		if parentRef.Project.ID != effectiveProjectID {
			return preparedTaskLineEdit{}, invalidInput("parent", "parent must belong to the effective project")
		}
		prepared.parent = parentRef.Task
		prepared.parentChanged = effectiveProjectID != ref.Project.ID || ref.Task.ParentID == nil || *ref.Task.ParentID != parentRef.Task.ID
	default:
		return preparedTaskLineEdit{}, invalidInput("parent", fmt.Sprintf("unknown parent mode %d", req.Parent.Mode))
	}

	if req.Destination != nil {
		destination, ok := ParseCaptureDestination(string(*req.Destination))
		if !ok {
			return preparedTaskLineEdit{}, invalidInput("destination", InvalidCaptureDestinationError(string(*req.Destination)).Error())
		}
		prepared.destination = &destination
		if err := validateTaskLineDestination(ref, destination); err != nil {
			return preparedTaskLineEdit{}, err
		}
	}
	return prepared, nil
}

func validateTaskLineDestination(ref TaskRef, destination CaptureDestination) error {
	target := destination.WorkflowState()
	switch destination {
	case CaptureDestinationToday:
		if ref.Task.IsTerminalState() {
			return invalidInput("planning", "terminal tasks cannot be added to Today")
		}
		if ref.Task.State() == models.StateDoing {
			return nil
		}
	case CaptureDestinationNext:
		if ref.Task.IsTerminalState() {
			return invalidInput("planning", "terminal tasks cannot be added to Next")
		}
	case CaptureDestinationBacklog:
		// Backlog uses the canonical planning action below so it also clears Today
		// and Next membership.
	default:
	}
	if target == ref.Task.State() {
		return nil
	}
	return validateLifecycleTransition(ref, target)
}

func applyPreparedTaskLineFields(task *models.Task, prepared preparedTaskLineEdit, now time.Time) bool {
	changed := false
	if task.Title != prepared.title {
		task.Title = prepared.title
		changed = true
	}
	if prepared.priority != nil && task.Priority != *prepared.priority {
		task.Priority = *prepared.priority
		changed = true
	}
	if prepared.dueSet && !calendarDatePtrsEqual(task.DueDate, prepared.due) {
		task.DueDate = cloneCalendarDatePtr(prepared.due)
		changed = true
	}
	if !slices.Equal(task.Tags, prepared.tags) {
		task.Tags = slices.Clone(prepared.tags)
		changed = true
	}
	if changed {
		task.UpdatedAt = now
	}
	return changed
}

func patchedTaskLineTags(existing, add, remove []string) []string {
	result := slices.Clone(existing)
	for _, tag := range remove {
		result = slices.DeleteFunc(result, func(candidate string) bool {
			normalized, err := models.NormalizeTaskTag(candidate)
			return err == nil && normalized == tag
		})
	}
	for _, tag := range add {
		if !slices.ContainsFunc(result, func(candidate string) bool {
			normalized, err := models.NormalizeTaskTag(candidate)
			return err == nil && normalized == tag
		}) {
			result = append(result, tag)
		}
	}
	return result
}

func calendarDatePtrsEqual(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func (s *Service) applyTaskLineDestination(ref TaskRef, destination *CaptureDestination) ([]PlanningReconcileTarget, bool, error) {
	if destination == nil {
		return nil, false, nil
	}
	var action PlanningAction
	switch *destination {
	case CaptureDestinationToday:
		action = PlanningAddToday
	case CaptureDestinationNext:
		action = PlanningAddNext
	case CaptureDestinationBacklog:
		action = PlanningBacklog
	default:
		state := destination.WorkflowState()
		if state == ref.Task.State() {
			return nil, false, nil
		}
		if err := applyState(ref.Task, state, s.clock.Now()); err != nil {
			return nil, false, invalidInput("destination", err.Error())
		}
		return []PlanningReconcileTarget{{
			TaskID: ref.Task.ID, ProjectID: ref.Project.ID, State: ref.Task.State(), StateChanged: true,
		}}, true, nil
	}
	target, changed, err := ApplyPlanningActionToTask(ref.Task, ref.Project.ID, action, s.clock.Now())
	if err != nil {
		return nil, false, err
	}
	return []PlanningReconcileTarget{target}, changed, nil
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
		result.AffectedProjectIDs = []string{ref.Project.ID}
	}
	return result
}
