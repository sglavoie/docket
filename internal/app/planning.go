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

// PlanningRequest applies one planning action to a task. PlannedDate is optional
// for AddToday and defaults to the service clock's local calendar day.
type PlanningRequest struct {
	TaskID      string
	Locator     TaskLocator
	Action      PlanningAction
	PlannedDate string
}

// PlanningTodayUpdate changes the planned date of one existing Today member.
type PlanningTodayUpdate struct {
	Locator     TaskLocator
	PlannedDate string
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

	plannedDate := req.PlannedDate
	if plannedDate == "" {
		plannedDate = s.clock.Now().Format("2006-01-02")
	}
	target, stateChanged, err := ApplyPlanningActionToTask(
		ref.Task, ref.Project.ID, req.Action, plannedDate, s.clock.Now(),
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
	if err := s.commitPlanningOnly(artifacts, []string{ref.Project.ID}, action, ref.Task.Title); err != nil {
		return TaskResult{}, err
	}
	succeeded = true
	return newTaskResult(ref, revision), nil
}

// ApplyPlanningActionToTask applies the task-state half of a planning action and
// returns the matching reconciliation target. It is exported for isolated,
// non-persistent UI models; production callers use Service.ApplyPlanning.
func ApplyPlanningActionToTask(task *models.Task, projectID string, action PlanningAction, plannedDate string, now time.Time) (PlanningReconcileTarget, bool, error) {
	if task == nil {
		return PlanningReconcileTarget{}, false, invalidInput("planning", "task is required")
	}
	targetState := task.State()
	today := PlanningTodayPreserve
	switch action {
	case PlanningAddToday:
		if targetState.IsTerminal() {
			return PlanningReconcileTarget{}, false, invalidInput("planning", "terminal tasks cannot be added to Today")
		}
		if targetState == models.StateInbox || targetState == models.StateNext {
			targetState = models.StateReady
		}
		today = PlanningTodayAdd
	case PlanningRemoveToday:
		today = PlanningTodayRemove
	case PlanningAddNext:
		targetState = models.StateNext
	case PlanningRemoveNext:
		if targetState == models.StateNext {
			targetState = models.StateReady
		}
	case PlanningBacklog:
		targetState = models.StateReady
		today = PlanningTodayRemove
	default:
		return PlanningReconcileTarget{}, false, invalidInput("planning", fmt.Sprintf("unknown action %q", action))
	}

	stateChanged := targetState != task.State()
	if stateChanged {
		if reason := models.CanTransitionState(task.State(), targetState); reason != "" {
			return PlanningReconcileTarget{}, false, &TransitionError{From: string(task.State()), To: string(targetState), Reason: reason}
		}
		if err := task.ApplyStateTransitionAt(targetState, now); err != nil {
			return PlanningReconcileTarget{}, false, invalidInput("state", err.Error())
		}
	}
	return PlanningReconcileTarget{
		TaskID: task.ID, ProjectID: projectID, State: task.State(),
		Today: today, PlannedDate: plannedDate, EnsureNextOrder: action == PlanningAddNext,
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

// ReconcilePlanning sanitizes all persisted Today and Next-order entries against
// one fresh workspace snapshot. It removes dangling/duplicate/illegal entries;
// StateNext itself remains authoritative membership while FocusQueue is only an
// optional explicit order index.
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
	if err := s.commitPlanningOnly(artifacts, nil, "Reconcile Planning", "Today and Next"); err != nil {
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
// StateNext member omitted by a stale caller at the end.
func (s *Service) ReorderNext(order []TaskLocator) error {
	return s.applyPlanningOrder(PlanningReconcileRequest{
		FocusOrder: append([]TaskLocator(nil), order...), SetFocusOrder: true,
	}, "Reorder Next")
}

// ReorderToday replaces Daily Plan order while preserving each entry's planned
// date and retaining any valid omitted member at the end.
func (s *Service) ReorderToday(order []TaskLocator) error {
	return s.applyPlanningOrder(PlanningReconcileRequest{
		DailyOrder: append([]TaskLocator(nil), order...), SetDailyOrder: true,
	}, "Reorder Today")
}

// UpdateTodayDates applies carry-over dates through the canonical reconciler,
// preserving Today eligibility and all Next invariants.
func (s *Service) UpdateTodayDates(updates []PlanningTodayUpdate) error {
	builder := s.defaultedPlanningArtifacts(nil)
	if builder == nil {
		return fmt.Errorf("app: planning mutations require a planning artifact builder")
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return err
	}
	targets := planningTargetsForSnapshot(snap)
	byLocator := make(map[TaskLocator]string, len(updates))
	for _, update := range updates {
		if update.Locator.Valid() && update.PlannedDate != "" {
			byLocator[update.Locator] = update.PlannedDate
		}
	}
	for i := range targets {
		locator := TaskLocator{ProjectID: targets[i].ProjectID, TaskID: targets[i].TaskID}
		if date := byLocator[locator]; date != "" {
			targets[i].Today = PlanningTodayAdd
			targets[i].PlannedDate = date
			targets[i].UpdatePlannedDate = true
		}
	}
	artifacts, err := planningArtifactsForRequest(PlanningReconcileRequest{
		Targets: targets, Complete: true,
	}, builder)
	if err != nil {
		return err
	}
	return s.commitPlanningOnly(artifacts, nil, "Carry Today Forward", "")
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
	return s.commitPlanningOnly(artifacts, nil, undoAction, "")
}

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

func (s *Service) commitPlanningOnly(artifacts []storage.ArtifactChange, projectIDs []string, undoAction, undoContext string) error {
	if len(artifacts) == 0 {
		return nil
	}
	committer, ok := s.repo.(ArtifactCommitter)
	if !ok {
		return fmt.Errorf("app: repository does not support artifact-only planning commits")
	}
	_, err := committer.CommitArtifactTransaction(storage.ArtifactCommitRequest{
		Artifacts: artifacts, ProjectIDs: projectIDs, UndoAction: undoAction, UndoContext: undoContext,
	})
	return err
}
