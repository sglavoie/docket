package app

import (
	"fmt"
	"slices"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// PlanningTodayDirective describes the Today-side effect attached to one
// application-owned planning transition. The zero value preserves Today unless
// the task's final state makes membership illegal.
type PlanningTodayDirective uint8

const (
	PlanningTodayPreserve PlanningTodayDirective = iota
	PlanningTodayAdd
	PlanningTodayRemove
)

// PlanningReconcileTarget names a task whose workflow state a transaction set,
// paired with the final state the task holds after the mutation and cascade.
// Today optionally requests an explicit add/remove on top of state-derived
// cleanup.
type PlanningReconcileTarget struct {
	TaskID    string
	ProjectID string
	State     models.WorkflowState
	Today     PlanningTodayDirective
}

// PlanningReconcileRequest is the complete application-owned mutation applied to
// the planning domain. Complete means Targets describe every task in the loaded
// workspace and therefore authorizes pruning unknown persisted entries. The
// ordering fields are optional full-order replacements used by J/K and the CLI
// move command; any valid member omitted from an order is retained at the end.
type PlanningReconcileRequest struct {
	Targets       []PlanningReconcileTarget
	Complete      bool
	NextOrder     []TaskLocator
	SetNextOrder  bool
	TodayOrder    []TaskLocator
	SetTodayOrder bool
	ClearToday    bool
}

// PlanningArtifactBuilder is the persistence edge for application-owned
// planning changes. Production adapters only load and encode planning.json; the
// invariant mutation itself is ReconcilePlanningState below. Returning no
// artifacts means the request was already satisfied.
type PlanningArtifactBuilder func(PlanningReconcileRequest) ([]storage.ArtifactChange, error)

// ReconcilePlanningState applies the canonical Today/Next invariants to one
// planning snapshot. It is deliberately application-owned so CLI, TUI,
// action-plan, undo/redo, and startup paths cannot drift:
//
//   - StateNext owns Next membership and has one NextOrder entry, and excludes Today.
//   - non-Next tasks have no NextOrder entry.
//   - Inbox and terminal tasks cannot be in Today.
//   - explicit Today adds are accepted only for eligible final states.
//   - complete reconciliation removes dangling and duplicate entries.
//
// It returns whether the snapshot changed.
func ReconcilePlanningState(planning *config.PlanningState, req PlanningReconcileRequest) bool {
	if planning == nil {
		return false
	}
	beforeNext := append([]config.ProjectTaskLocator(nil), planning.NextOrder...)
	beforeToday := append([]config.ProjectTaskLocator(nil), planning.Today...)
	planning.NormalizePlanningReferences()

	known := make(map[TaskLocator]PlanningReconcileTarget, len(req.Targets))
	for _, target := range req.Targets {
		locator := TaskLocator{ProjectID: target.ProjectID, TaskID: target.TaskID}
		if !locator.Valid() || !target.State.IsValid() {
			continue
		}
		known[locator] = target
	}

	if req.Complete {
		planning.NextOrder = sanitizeNextOrder(planning.NextOrder, known)
		planning.Today = sanitizeToday(planning.Today, known)
	}

	applied := make(map[TaskLocator]bool, len(known))
	for _, requested := range req.Targets {
		locator := TaskLocator{ProjectID: requested.ProjectID, TaskID: requested.TaskID}
		target, ok := known[locator]
		if !ok || applied[locator] {
			continue
		}
		applied[locator] = true
		if target.State == models.StateNext {
			planning.RemoveFromTodayInProject(locator.TaskID, locator.ProjectID)
			ensureNextOrderEntry(planning, locator)
		} else {
			planning.RemoveFromNextOrderInProject(locator.TaskID, locator.ProjectID)
		}
		if target.State == models.StateInbox || target.State.IsTerminal() {
			planning.RemoveFromTodayInProject(locator.TaskID, locator.ProjectID)
		}
		switch target.Today {
		case PlanningTodayPreserve:
		case PlanningTodayAdd:
			if PlanningTodayEligible(target.State) {
				ensureTodayEntry(planning, locator)
			}
		case PlanningTodayRemove:
			planning.RemoveFromTodayInProject(locator.TaskID, locator.ProjectID)
		}
	}

	if req.ClearToday {
		planning.Today = nil
	}
	if req.SetNextOrder {
		for _, locator := range req.NextOrder {
			if target, ok := known[locator]; ok && target.State == models.StateNext {
				ensureNextOrderEntry(planning, locator)
			}
		}
		planning.NextOrder = reorderPlanningRefs(planning.NextOrder, req.NextOrder)
	}
	if req.SetTodayOrder {
		planning.Today = reorderPlanningRefs(planning.Today, req.TodayOrder)
	}

	return !slices.Equal(beforeNext, planning.NextOrder) || !slices.Equal(beforeToday, planning.Today)
}

// PlanningTodayEligible reports whether a task in state may belong to Today.
func PlanningTodayEligible(state models.WorkflowState) bool {
	return state.IsValid() && state != models.StateInbox && state != models.StateNext && !state.IsTerminal()
}

func sanitizeNextOrder(items []config.ProjectTaskLocator, known map[TaskLocator]PlanningReconcileTarget) []config.ProjectTaskLocator {
	out := make([]config.ProjectTaskLocator, 0, len(items))
	seen := make(map[TaskLocator]bool, len(items))
	for _, item := range items {
		locator := TaskLocator{ProjectID: item.ProjectID, TaskID: item.TaskID}
		target, ok := known[locator]
		if !ok || target.State != models.StateNext || seen[locator] {
			continue
		}
		seen[locator] = true
		out = append(out, item)
	}
	return out
}

func sanitizeToday(items []config.ProjectTaskLocator, known map[TaskLocator]PlanningReconcileTarget) []config.ProjectTaskLocator {
	out := make([]config.ProjectTaskLocator, 0, len(items))
	seen := make(map[TaskLocator]bool, len(items))
	for _, item := range items {
		locator := TaskLocator{ProjectID: item.ProjectID, TaskID: item.TaskID}
		target, ok := known[locator]
		if !ok || !PlanningTodayEligible(target.State) || seen[locator] {
			continue
		}
		seen[locator] = true
		out = append(out, item)
	}
	return out
}

func ensureNextOrderEntry(planning *config.PlanningState, locator TaskLocator) {
	for _, item := range planning.NextOrder {
		if item.TaskID == locator.TaskID && item.ProjectID == locator.ProjectID {
			return
		}
	}
	planning.NextOrder = append(planning.NextOrder, config.ProjectTaskLocator{
		TaskID: locator.TaskID, ProjectID: locator.ProjectID,
	})
}

func ensureTodayEntry(planning *config.PlanningState, locator TaskLocator) {
	for _, item := range planning.Today {
		if item.TaskID == locator.TaskID && item.ProjectID == locator.ProjectID {
			return
		}
	}
	planning.Today = append(planning.Today, config.ProjectTaskLocator{
		TaskID: locator.TaskID, ProjectID: locator.ProjectID,
	})
}

func reorderPlanningRefs(items []config.ProjectTaskLocator, order []TaskLocator) []config.ProjectTaskLocator {
	byLocator := make(map[TaskLocator]config.ProjectTaskLocator, len(items))
	for _, item := range items {
		byLocator[TaskLocator{ProjectID: item.ProjectID, TaskID: item.TaskID}] = item
	}
	out := make([]config.ProjectTaskLocator, 0, len(items))
	for _, locator := range order {
		if item, ok := byLocator[locator]; ok {
			out = append(out, item)
			delete(byLocator, locator)
		}
	}
	for _, item := range items {
		locator := TaskLocator{ProjectID: item.ProjectID, TaskID: item.TaskID}
		if _, ok := byLocator[locator]; ok {
			out = append(out, item)
			delete(byLocator, locator)
		}
	}
	return out
}

// collectPlanningReconcileTargets resolves each task id to its final state and
// owning project among the committed projects, so the builder reconciles against
// post-mutation, post-cascade state. Ids absent from every committed project
// (e.g. a task moved out concurrently) are skipped.
func collectPlanningReconcileTargets(commits []projectCommit, taskIDs []string, locators []TaskLocator) []PlanningReconcileTarget {
	if len(taskIDs) == 0 && len(locators) == 0 {
		return nil
	}
	seen := make(map[TaskLocator]bool, len(taskIDs)+len(locators))
	targets := make([]PlanningReconcileTarget, 0, len(taskIDs)+len(locators))
	appendTarget := func(locator TaskLocator, task *models.Task) {
		if task == nil || !locator.Valid() || seen[locator] {
			return
		}
		seen[locator] = true
		targets = append(targets, PlanningReconcileTarget{
			TaskID: locator.TaskID, ProjectID: locator.ProjectID, State: task.State(),
		})
	}
	for _, locator := range locators {
		for _, c := range commits {
			if c.project != nil && c.project.ID == locator.ProjectID {
				appendTarget(locator, c.project.GetTaskByID(locator.TaskID))
				break
			}
		}
	}
	for _, id := range taskIDs {
		if id == "" {
			continue
		}
		for _, c := range commits {
			if c.project == nil {
				continue
			}
			if task := c.project.GetTaskByID(id); task != nil {
				appendTarget(TaskLocator{ProjectID: c.project.ID, TaskID: id}, task)
			}
		}
	}
	return targets
}

// attachPlanningArtifacts folds the planning reconciliation implied by targets
// into the first project's changeset, mirroring attachWorkflowProjectStatusArtifacts.
// Every planning artifact must carry an expected revision so a concurrent config
// writer is detected rather than silently overwritten.
func attachPlanningArtifacts(commits []projectCommit, targets []PlanningReconcileTarget, builder PlanningArtifactBuilder) error {
	if builder == nil || len(targets) == 0 {
		return nil
	}
	artifacts, err := builder(PlanningReconcileRequest{Targets: targets})
	if err != nil {
		return err
	}
	if len(artifacts) == 0 {
		return nil
	}
	if len(commits) == 0 {
		return fmt.Errorf("app: planning reconciliation requires at least one project commit")
	}
	for _, artifact := range artifacts {
		if artifact.ExpectedRevision == nil {
			return fmt.Errorf("app: planning artifact %q has no expected revision", artifact.Path)
		}
	}
	commits[0].artifacts = append(commits[0].artifacts, artifacts...)
	return nil
}
