package cmd

import (
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/storage"
)

// cliPlanningReconcileArtifactBuilder returns the planning-artifact builder the
// application layer invokes after a transition (and its rule cascade) settles. It
// only loads and encodes the planning domain; the application-owned request
// performs every invariant mutation.
func cliPlanningReconcileArtifactBuilder(workspace *CLIWorkspace) app.PlanningArtifactBuilder {
	if workspace == nil || workspace.Manager == nil {
		return nil
	}
	return func(req app.PlanningReconcileRequest) ([]storage.ArtifactChange, error) {
		coord := workspace.Manager.DomainCoordinator()
		planning, err := coord.LoadPlanning()
		if err != nil {
			return nil, err
		}
		if !app.ReconcilePlanningState(planning, req) {
			return nil, nil
		}
		artifact, err := coord.CheckedPlanningArtifact(planning)
		if err != nil {
			return nil, err
		}
		return []storage.ArtifactChange{artifact.StorageChange()}, nil
	}
}
