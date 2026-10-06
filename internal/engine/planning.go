package engine

import (
	"fmt"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
)

// PlanningArtifacts returns the planning-artifact builder the application layer
// invokes after a transition (and its rule cascade) settles. It loads the
// current planning domain, reconciles Today/Next exclusivity for the
// transitioned tasks, and renders one revision-checked planning.json artifact
// for the same transaction. A nil manager yields a nil builder (a bare
// transition).
func PlanningArtifacts(mgr *config.Manager) app.PlanningArtifactBuilder {
	if mgr == nil {
		return nil
	}
	return func(req app.PlanningReconcileRequest) ([]storage.ArtifactChange, error) {
		coord := mgr.DomainCoordinator()
		planning, err := coord.LoadPlanning()
		if err != nil {
			return nil, err
		}
		if guardErr := RequireUnplanned(planning, req.RequireAbsent); guardErr != nil {
			return nil, guardErr
		}
		changed := app.ReconcilePlanningState(planning, req)
		if !changed && !req.GuardOnly {
			return nil, nil
		}
		artifact, err := coord.CheckedPlanningArtifact(planning)
		if err != nil {
			return nil, err
		}
		return []storage.ArtifactChange{artifact.StorageChange()}, nil
	}
}

// RequireUnplanned refuses a locator that belongs to Today or Next. A nil
// locator asks nothing.
func RequireUnplanned(planning *config.PlanningState, locator *app.TaskLocator) error {
	if locator == nil {
		return nil
	}
	for _, item := range append(append([]config.ProjectTaskLocator(nil), planning.Today...), planning.GetNext()...) {
		if item.ProjectID == locator.ProjectID && item.TaskID == locator.TaskID {
			return fmt.Errorf("task belongs to Today or Next")
		}
	}
	return nil
}
