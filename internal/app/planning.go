package app

import (
	"fmt"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// PlanningAction names one canonical horizon mutation. Adapters select an
// intent; the application service owns every state and membership side effect.
type PlanningAction string

const (
	PlanningAddToday    PlanningAction = "add_today"
	PlanningRemoveToday PlanningAction = "remove_today"
	PlanningAddNext     PlanningAction = "add_next"
	PlanningRemoveNext  PlanningAction = "remove_next"
	PlanningBacklog     PlanningAction = "backlog"
)

// PlanningHorizon identifies one explicit planning membership for toggle
// policy. Lifecycle state remains independent from both values.
type PlanningHorizon string

const (
	PlanningHorizonToday PlanningHorizon = "today"
	PlanningHorizonNext  PlanningHorizon = "next"
)

// PlanningRequest applies one planning action to a project-qualified task.
type PlanningRequest struct {
	TaskID  string
	Locator TaskLocator
	Action  PlanningAction
}

// PlanningBulkRequest applies one uniform planning intent to a
// project-qualified selection.
type PlanningBulkRequest struct {
	Targets []TaskLocator
	Action  PlanningAction
}

// ResolvePlanningToggleAction implements the shared single/mixed-selection
// toggle rule. If every selected task is already a member the operation removes
// every target; otherwise it adds every target. Callers supply a revision-bound
// membership snapshot, and the resulting artifact commit detects stale input.
func ResolvePlanningToggleAction(horizon PlanningHorizon, targets, members []TaskLocator) (PlanningAction, error) {
	if len(targets) == 0 {
		return "", invalidInput("tasks", "at least one task is required")
	}
	memberSet := make(map[TaskLocator]bool, len(members))
	for _, member := range members {
		if member.Valid() {
			memberSet[member] = true
		}
	}
	allMembers := true
	for _, target := range targets {
		if !target.Valid() {
			return "", invalidInput("task", "project id and task id are required")
		}
		if !memberSet[target] {
			allMembers = false
		}
	}
	switch horizon {
	case PlanningHorizonToday:
		if allMembers {
			return PlanningRemoveToday, nil
		}
		return PlanningAddToday, nil
	case PlanningHorizonNext:
		if allMembers {
			return PlanningRemoveNext, nil
		}
		return PlanningAddNext, nil
	default:
		return "", invalidInput("planning", fmt.Sprintf("unknown horizon %q", horizon))
	}
}

// ApplyPlanning is the single task/planning mutation boundary. State changes and
// planning.json changes share one transaction; planning-only actions use the
// repository's artifact transaction so they are revision checked and participate
// in the same durable undo/redo ledger.
func (s *Service) ApplyPlanning(req PlanningRequest) (TaskResult, error) {
	builder := s.defaultedPlanningArtifacts(nil)
	if builder == nil {
		return TaskResult{}, fmt.Errorf("app: planning mutations require a planning artifact builder")
	}
	ref, _, revision, err := s.loadTaskSelection(req.TaskID, req.Locator)
	if err != nil {
		return TaskResult{}, err
	}
	beforeTask := *ref.Task
	succeeded := false
	defer func() {
		if !succeeded {
			*ref.Task = beforeTask
		}
	}()

	target, stateChanged, err := ApplyPlanningActionToTask(
		ref.Task, ref.Project.ID, req.Action, s.clock.Now(),
	)
	if err != nil {
		return TaskResult{}, err
	}
	action := planningUndoAction(req.Action)
	artifacts, err := planningArtifactsForRequest(PlanningReconcileRequest{
		Targets: []PlanningReconcileTarget{target},
	}, builder)
	if err != nil {
		return TaskResult{}, err
	}

	if stateChanged {
		result, commitErr := s.commitProjectWithArtifacts(ref, revision, artifacts, action, ref.Task.Title)
		if commitErr != nil {
			return TaskResult{}, commitErr
		}
		succeeded = true
		return result, nil
	}
	transactionID, err := s.commitPlanningOnly(artifacts, []string{ref.Project.ID}, action, ref.Task.Title)
	if err != nil {
		return TaskResult{}, err
	}
	succeeded = true
	return newTaskResult(ref, revision, transactionID), nil
}

// ApplyPlanningBulk validates the complete selection before mutation, applies
// every task-state and planning membership effect, and commits all touched
// projects plus the checked planning artifact in one transaction.
func (s *Service) ApplyPlanningBulk(req PlanningBulkRequest) (BulkResult, error) {
	if len(req.Targets) == 0 {
		return BulkResult{}, invalidInput("tasks", "at least one task is required")
	}
	builder := s.defaultedPlanningArtifacts(nil)
	if builder == nil {
		return BulkResult{}, fmt.Errorf("app: planning mutations require a planning artifact builder")
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return BulkResult{}, err
	}
	refs, err := resolveRequestedTasks(snap, nil, req.Targets)
	if err != nil {
		return BulkResult{}, err
	}
	before, err := cloneSnapshot(snap)
	if err != nil {
		return BulkResult{}, fmt.Errorf("capture bulk planning rollback state: %w", err)
	}
	useRollbackDocuments(before)
	committed := false
	defer func() {
		if !committed {
			restoreSnapshotProjects(snap, before)
		}
	}()

	now := s.clock.Now()
	for _, ref := range refs {
		clone := *ref.Task
		if _, _, validationErr := ApplyPlanningActionToTask(
			&clone, ref.Project.ID, req.Action, now,
		); validationErr != nil {
			return BulkResult{}, validationErr
		}
	}
	targets := make([]PlanningReconcileTarget, 0, len(refs))
	for _, ref := range refs {
		target, _, mutationErr := ApplyPlanningActionToTask(
			ref.Task, ref.Project.ID, req.Action, now,
		)
		if mutationErr != nil {
			return BulkResult{}, mutationErr
		}
		targets = append(targets, target)
	}
	artifacts, err := planningArtifactsForRequest(
		PlanningReconcileRequest{Targets: targets},
		builder,
	)
	if err != nil {
		return BulkResult{}, err
	}
	commits, indexes := distinctProjectCommits(refs, snap)
	if len(commits) == 0 {
		return BulkResult{}, invalidInput("tasks", "at least one task is required")
	}
	commits[0].artifacts = append(commits[0].artifacts, artifacts...)
	outcome, err := s.commitProjects(
		commits,
		planningUndoAction(req.Action),
		fmt.Sprintf("%d tasks", len(refs)),
	)
	if err != nil {
		return BulkResult{}, err
	}
	committed = true

	result := BulkResult{
		Tasks:         make([]TaskResult, 0, len(refs)),
		TransactionID: transactionIDAt(outcome.results, 0),
	}
	for _, ref := range refs {
		result.Tasks = append(result.Tasks, newTaskResult(
			ref,
			revisionAt(outcome.results, indexes[ref.Project.ID]),
			transactionIDAt(outcome.results, indexes[ref.Project.ID]),
		))
	}
	for _, commit := range commits {
		result.TouchedProjectIDs = append(result.TouchedProjectIDs, commit.project.ID)
	}
	return result, nil
}

// ApplyPlanningActionToTask applies the task-state half of a planning action and
// returns the matching reconciliation target. It is exported for isolated,
// non-persistent UI models; production callers use Service.ApplyPlanning.
func ApplyPlanningActionToTask(task *models.Task, projectID string, action PlanningAction, now time.Time) (PlanningReconcileTarget, bool, error) {
	if task == nil {
		return PlanningReconcileTarget{}, false, invalidInput("planning", "task is required")
	}
	targetState := task.State()
	today := PlanningTodayPreserve
	next := PlanningNextPreserve
	clearDefer := false
	switch action {
	case PlanningAddToday:
		if targetState.IsTerminal() {
			return PlanningReconcileTarget{}, false, invalidInput("planning", "terminal tasks cannot be added to Today")
		}
		if targetState != models.StateDoing {
			targetState = models.StateBacklog
		}
		today = PlanningTodayAdd
		next = PlanningNextRemove
		clearDefer = true
	case PlanningRemoveToday:
		today = PlanningTodayRemove
	case PlanningAddNext:
		if targetState.IsTerminal() {
			return PlanningReconcileTarget{}, false, invalidInput("planning", "terminal tasks cannot be added to Next")
		}
		targetState = models.StateBacklog
		today = PlanningTodayRemove
		next = PlanningNextAdd
		clearDefer = true
	case PlanningRemoveNext:
		next = PlanningNextRemove
	case PlanningBacklog:
		targetState = models.StateBacklog
		today = PlanningTodayRemove
		next = PlanningNextRemove
	default:
		return PlanningReconcileTarget{}, false, invalidInput("planning", fmt.Sprintf("unknown action %q", action))
	}

	stateChanged := targetState != task.State()
	if stateChanged {
		if err := validateLifecycleTransition(TaskRef{Task: task}, targetState); err != nil {
			return PlanningReconcileTarget{}, false, err
		}
		if err := task.ApplyStateTransitionAt(targetState, now); err != nil {
			return PlanningReconcileTarget{}, false, invalidInput("state", err.Error())
		}
	}
	if clearDefer && task.DeferredUntil != nil {
		task.DeferredUntil = nil
		task.UpdatedAt = now
		stateChanged = true
	}
	return PlanningReconcileTarget{
		TaskID: task.ID, ProjectID: projectID, State: task.State(),
		Today: today, Next: next, StateChanged: stateChanged,
	}, stateChanged, nil
}

func planningUndoAction(action PlanningAction) string {
	switch action {
	case PlanningAddToday:
		return "Plan for Today"
	case PlanningRemoveToday:
		return "Remove from Today"
	case PlanningAddNext:
		return "Move to Next"
	case PlanningRemoveNext:
		return "Leave Next"
	case PlanningBacklog:
		return "Move to Backlog"
	default:
		return "Update Planning"
	}
}

// ReconcilePlanning sanitizes all persisted Today and Next entries against
// one fresh workspace snapshot. It removes dangling/duplicate/illegal entries;
// both collections remain explicit ordered membership.
func (s *Service) ReconcilePlanning() (bool, error) {
	builder := s.defaultedPlanningArtifacts(nil)
	if builder == nil {
		return false, nil
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return false, err
	}
	artifacts, err := planningArtifactsForRequest(PlanningReconcileRequest{
		Targets:  planningTargetsForSnapshot(snap),
		Complete: true,
	}, builder)
	if err != nil {
		return false, err
	}
	if len(artifacts) == 0 {
		return false, nil
	}
	if _, err := s.commitPlanningOnly(artifacts, nil, "Reconcile Planning", "Today and Next"); err != nil {
		return false, err
	}
	return true, nil
}

// ClearToday removes every Daily Plan entry through the same checked,
// ledger-backed planning transaction used by task-scoped planning actions.
func (s *Service) ClearToday() error {
	return s.applyPlanningOrder(PlanningReconcileRequest{ClearToday: true}, "Clear Today")
}

// ReorderNext replaces the explicit Next order while preserving any valid
// member omitted by a stale caller at the end.
func (s *Service) ReorderNext(order []TaskLocator) error {
	return s.applyPlanningOrder(PlanningReconcileRequest{
		Next: append([]TaskLocator(nil), order...), SetNext: true,
	}, "Reorder Next")
}

// ReorderToday replaces the workspace-wide Today order while retaining any
// valid omitted member at the end.
func (s *Service) ReorderToday(order []TaskLocator) error {
	return s.applyPlanningOrder(PlanningReconcileRequest{
		TodayOrder: append([]TaskLocator(nil), order...), SetTodayOrder: true,
	}, "Reorder Today")
}

func (s *Service) applyPlanningOrder(req PlanningReconcileRequest, undoAction string) error {
	builder := s.defaultedPlanningArtifacts(nil)
	if builder == nil {
		return fmt.Errorf("app: planning mutations require a planning artifact builder")
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return err
	}
	req.Targets = planningTargetsForSnapshot(snap)
	req.Complete = true
	artifacts, err := planningArtifactsForRequest(req, builder)
	if err != nil {
		return err
	}
	_, err = s.commitPlanningOnly(artifacts, nil, undoAction, "")
	return err
}

// planningTargetsForSnapshot enumerates every task the snapshot can see, which
// is what makes a Complete reconcile request authoritative: any locator without
// a target here is pruned. The snapshot population deliberately includes parked
// projects, so a parked project's Today and Next entries survive reconciliation
// and unparking restores its plan untouched. Parking is a read-time predicate
// applied by RuntimeState.NextOrder, never a planning edit.
func planningTargetsForSnapshot(snap Snapshot) []PlanningReconcileTarget {
	if snap == nil {
		return nil
	}
	var targets []PlanningReconcileTarget
	for _, project := range snap.Projects() {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil {
				continue
			}
			targets = append(targets, PlanningReconcileTarget{
				TaskID: task.ID, ProjectID: project.ID, State: task.State(),
			})
		}
	}
	return targets
}

func planningArtifactsForRequest(req PlanningReconcileRequest, builder PlanningArtifactBuilder) ([]storage.ArtifactChange, error) {
	if builder == nil {
		return nil, nil
	}
	artifacts, err := builder(req)
	if err != nil {
		return nil, err
	}
	for _, artifact := range artifacts {
		if artifact.ExpectedRevision == nil {
			return nil, fmt.Errorf("app: planning artifact %q has no expected revision", artifact.Path)
		}
	}
	return artifacts, nil
}

func (s *Service) commitPlanningOnly(artifacts []storage.ArtifactChange, projectIDs []string, undoAction, undoContext string) (string, error) {
	if len(artifacts) == 0 {
		return "", nil
	}
	committer, ok := s.repo.(ArtifactCommitter)
	if !ok {
		return "", fmt.Errorf("app: repository does not support artifact-only planning commits")
	}
	return committer.CommitArtifactTransaction(storage.ArtifactCommitRequest{
		Artifacts: artifacts, ProjectIDs: projectIDs, UndoAction: undoAction, UndoContext: undoContext,
	})
}
