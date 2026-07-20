package cmd

import (
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// reconcilePlanningExclusivity applies Today/Next exclusivity to a planning
// snapshot for exactly the transitioned tasks: a task now in StateNext leaves the
// Daily Plan (Today), and a task that is no longer StateNext drops its stale Next
// order-index entry. It returns true when any entry changed. It is the shared
// reconciliation both the CLI transition path and the focus commands fold into
// the same durable ledger transaction as the task write.
func reconcilePlanningExclusivity(planning *config.PlanningState, targets []app.PlanningReconcileTarget) bool {
	changed := false
	for _, t := range targets {
		if t.State == models.StateNext {
			if planning.RemoveFromDailyPlan(t.TaskID) {
				changed = true
			}
			continue
		}
		if planning.RemoveFromFocusQueue(t.TaskID) {
			changed = true
		}
	}
	return changed
}

// cliPlanningReconcileArtifactBuilder returns the planning-artifact builder the
// application layer invokes after a transition (and its rule cascade) settles. It
// loads the current planning domain, reconciles Today/Next exclusivity for the
// transitioned tasks, and renders one revision-checked planning.json artifact for
// the same transaction — so an injected failure leaves both task and planning
// state at their pre-mutation values and one ledger undo restores them together.
func cliPlanningReconcileArtifactBuilder(workspace *CLIWorkspace) app.PlanningArtifactBuilder {
	if workspace == nil || workspace.Manager == nil {
		return nil
	}
	return func(targets []app.PlanningReconcileTarget) ([]storage.ArtifactChange, error) {
		coord := workspace.Manager.DomainCoordinator()
		planning, err := coord.LoadPlanning()
		if err != nil {
			return nil, err
		}
		if !reconcilePlanningExclusivity(planning, targets) {
			return nil, nil
		}
		artifact, err := coord.CheckedPlanningArtifact(planning)
		if err != nil {
			return nil, err
		}
		return []storage.ArtifactChange{artifact.StorageChange()}, nil
	}
}
