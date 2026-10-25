package app

import (
	"fmt"
	"slices"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
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

// PlanningNextDirective describes the explicit Next-membership side effect.
type PlanningNextDirective uint8

const (
	PlanningNextPreserve PlanningNextDirective = iota
	PlanningNextAdd
	PlanningNextRemove
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
	Next      PlanningNextDirective
	// StateChanged distinguishes an explicit lifecycle transition from a task
	// merely observed during complete reconciliation.
	StateChanged bool
}

// PlanningProjectMove requalifies one durable planning membership after a task
// moves between projects. Task IDs remain stable across the move; only their
// owning project changes.
type PlanningProjectMove struct {
	TaskID        string
	FromProjectID string
	ToProjectID   string
}

func (m PlanningProjectMove) valid() bool {
	return m.TaskID != "" && m.FromProjectID != "" && m.ToProjectID != "" && m.FromProjectID != m.ToProjectID
}

// PlanningReconcileRequest is the complete application-owned mutation applied to
// the planning domain. Complete means Targets describe every task in the loaded
// workspace and therefore authorizes pruning unknown persisted entries, except
// those in an Unavailable project: its tasks were not read, so an entry naming
// one is kept as found rather than judged. The
// ordering fields are optional full-order replacements used by J/K and the CLI
// move command; any valid member omitted from an order is retained at the end.
type PlanningReconcileRequest struct {
	// RequireAbsent refuses structural conversion when this task is planned.
	// GuardOnly asks adapters to include a revision-checked planning artifact
	// even when reconciliation makes no change.
	RequireAbsent *TaskLocator
	GuardOnly     bool
	Targets       []PlanningReconcileTarget
	Remove        []TaskLocator
	Moves         []PlanningProjectMove
	Complete      bool
	Unavailable   []string
	Next          []TaskLocator
	SetNext       bool
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
//   - Today and Next are explicit mutually-exclusive ordered memberships.
//   - terminal tasks cannot belong to either collection.
//   - lifecycle transitions apply their contract-owned planning cleanup.
//   - explicit Today adds are accepted only for eligible final states.
//   - complete reconciliation removes dangling and duplicate entries.
//
// It returns whether the snapshot changed.
func ReconcilePlanningState(planning *config.PlanningState, req PlanningReconcileRequest) bool {
	if planning == nil {
		return false
	}
	beforeNext := append([]config.ProjectTaskLocator(nil), planning.GetNext()...)
	beforeToday := append([]config.ProjectTaskLocator(nil), planning.Today...)
	planning.NormalizePlanningReferences()

	applyPlanningProjectMoves(planning, req.Moves)
	removePlanningLocators(planning, req.Remove)
	// A move may converge two malformed references on the same final identity.
	// Re-canonicalize after all identity changes while keeping first-seen order.
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
		unavailable := make(map[string]bool, len(req.Unavailable))
		for _, projectID := range req.Unavailable {
			unavailable[projectID] = true
		}
		planning.Next = sanitizeNext(planning.GetNext(), known, unavailable)
		planning.Today = sanitizeToday(planning.Today, known, unavailable)
		for _, item := range planning.Today {
			planning.RemoveFromNextInProject(item.TaskID, item.ProjectID)
		}
	}

	applied := make(map[TaskLocator]bool, len(known))
	for _, requested := range req.Targets {
		locator := TaskLocator{ProjectID: requested.ProjectID, TaskID: requested.TaskID}
		target, ok := known[locator]
		if !ok || applied[locator] {
			continue
		}
		applied[locator] = true
		if target.StateChanged {
			if target.State == models.StateDoing {
				planning.RemoveFromNextInProject(locator.TaskID, locator.ProjectID)
			} else {
				planning.RemoveFromTodayInProject(locator.TaskID, locator.ProjectID)
				planning.RemoveFromNextInProject(locator.TaskID, locator.ProjectID)
			}
		}
		if target.State.IsTerminal() {
			planning.RemoveFromTodayInProject(locator.TaskID, locator.ProjectID)
			planning.RemoveFromNextInProject(locator.TaskID, locator.ProjectID)
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
		switch target.Next {
		case PlanningNextPreserve:
		case PlanningNextAdd:
			if target.State.IsNonterminal() {
				planning.RemoveFromTodayInProject(locator.TaskID, locator.ProjectID)
				ensureNextEntry(planning, locator)
			}
		case PlanningNextRemove:
			planning.RemoveFromNextInProject(locator.TaskID, locator.ProjectID)
		}
	}

	if req.ClearToday {
		planning.Today = nil
	}
	if req.SetNext {
		// Reordering never confers membership. Unknown or stale caller entries
		// are ignored while existing members omitted by the caller stay at the
		// end in their prior relative order.
		planning.Next = reorderPlanningRefs(planning.GetNext(), req.Next)
	}
	if req.SetTodayOrder {
		planning.Today = reorderPlanningRefs(planning.Today, req.TodayOrder)
	}

	planning.SetNext(planning.Next)
	return !slices.Equal(beforeNext, planning.GetNext()) || !slices.Equal(beforeToday, planning.Today)
}

func applyPlanningProjectMoves(planning *config.PlanningState, moves []PlanningProjectMove) {
	if planning == nil || len(moves) == 0 {
		return
	}
	replacements := make(map[TaskLocator]config.ProjectTaskLocator, len(moves))
	for _, move := range moves {
		if !move.valid() {
			continue
		}
		from := TaskLocator{ProjectID: move.FromProjectID, TaskID: move.TaskID}
		replacements[from] = config.ProjectTaskLocator{ProjectID: move.ToProjectID, TaskID: move.TaskID}
	}
	apply := func(items []config.ProjectTaskLocator) {
		for i := range items {
			locator := TaskLocator{ProjectID: items[i].ProjectID, TaskID: items[i].TaskID}
			if replacement, ok := replacements[locator]; ok {
				items[i] = replacement
			}
		}
	}
	apply(planning.Today)
	next := planning.GetNext()
	apply(next)
	planning.SetNext(next)
}

func removePlanningLocators(planning *config.PlanningState, locators []TaskLocator) {
	if planning == nil || len(locators) == 0 {
		return
	}
	remove := make(map[TaskLocator]bool, len(locators))
	for _, locator := range locators {
		if locator.Valid() {
			remove[locator] = true
		}
	}
	filter := func(items []config.ProjectTaskLocator) []config.ProjectTaskLocator {
		out := items[:0]
		for _, item := range items {
			locator := TaskLocator{ProjectID: item.ProjectID, TaskID: item.TaskID}
			if !remove[locator] {
				out = append(out, item)
			}
		}
		return out
	}
	planning.Today = filter(planning.Today)
	planning.SetNext(filter(planning.GetNext()))
}

// PlanningTodayEligible reports whether a task in state may belong to Today.
func PlanningTodayEligible(state models.WorkflowState) bool {
	return state.IsNonterminal()
}

func sanitizeNext(items []config.ProjectTaskLocator, known map[TaskLocator]PlanningReconcileTarget, unavailable map[string]bool) []config.ProjectTaskLocator {
	out := make([]config.ProjectTaskLocator, 0, len(items))
	seen := make(map[TaskLocator]bool, len(items))
	for _, item := range items {
		locator := TaskLocator{ProjectID: item.ProjectID, TaskID: item.TaskID}
		target, ok := known[locator]
		if seen[locator] || !planningEntrySurvives(locator, ok, unavailable, target.State.IsNonterminal()) {
			continue
		}
		seen[locator] = true
		out = append(out, item)
	}
	return out
}

func sanitizeToday(items []config.ProjectTaskLocator, known map[TaskLocator]PlanningReconcileTarget, unavailable map[string]bool) []config.ProjectTaskLocator {
	out := make([]config.ProjectTaskLocator, 0, len(items))
	seen := make(map[TaskLocator]bool, len(items))
	for _, item := range items {
		locator := TaskLocator{ProjectID: item.ProjectID, TaskID: item.TaskID}
		target, ok := known[locator]
		if seen[locator] || !planningEntrySurvives(locator, ok, unavailable, PlanningTodayEligible(target.State)) {
			continue
		}
		seen[locator] = true
		out = append(out, item)
	}
	return out
}

// planningEntrySurvives is the one pruning verdict a complete reconcile gives an
// entry. A task that was read answers by its state; one that was not read is
// dropped only when its project was, since an unread project says nothing about
// what it holds.
func planningEntrySurvives(locator TaskLocator, known bool, unavailable map[string]bool, eligible bool) bool {
	if known {
		return eligible
	}
	return unavailable[locator.ProjectID]
}

func ensureNextEntry(planning *config.PlanningState, locator TaskLocator) {
	for _, item := range planning.GetNext() {
		if item.TaskID == locator.TaskID && item.ProjectID == locator.ProjectID {
			return
		}
	}
	planning.AddToNext(locator.TaskID, locator.ProjectID)
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
			TaskID: locator.TaskID, ProjectID: locator.ProjectID, State: task.State(), StateChanged: true,
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
