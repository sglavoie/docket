package coordinator

import (
	"fmt"
	"path/filepath"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/engine"
	"github.com/sglavoie/tt/internal/storage"
)

// The planning-artifact builders are the port every surface commits a planning
// consequence through. They live here rather than in the shell because a
// builder is a pure pairing of a workspace snapshot with the application
// layer's reconcile call, and both the Outliner's inline edit and the shell's
// own mutations need the same pairing. Keeping one copy is what keeps a
// mismatched manager/runtime pair rejected identically on both routes.

// ClonePlanningState returns a defensive copy of a planning snapshot whose
// mutable slices are freshly allocated, so a mutation applied before a checked
// commit can never alias and corrupt the cached RuntimeState snapshot.
func ClonePlanningState(p *config.PlanningState) *config.PlanningState {
	return engine.ClonePlanning(p)
}

// PlanningArtifacts binds application-owned invariant mutations to the shell's
// revision-bound planning snapshot. A mismatched manager/runtime pair is
// rejected before mutation instead of writing planning through an unrelated
// workspace root.
func PlanningArtifacts(runtime *config.RuntimeState, mgr *config.Manager) app.PlanningArtifactBuilder {
	if runtime != nil {
		if planningWorkspaceMismatch(runtime, mgr) {
			return mismatchedPlanningArtifacts()
		}
		return RuntimePlanningArtifacts(runtime)
	}
	return ManagerPlanningArtifacts(mgr)
}

// StructuralPlanningArtifacts is used for delete and project move. Their
// request contains the complete qualified remove/reassign delta, so it can
// reconcile the newest on-disk planning state without adopting stale shell
// intent. The resulting expected revision still detects a writer racing the
// combined project-plus-planning commit.
func StructuralPlanningArtifacts(runtime *config.RuntimeState, mgr *config.Manager) app.PlanningArtifactBuilder {
	if planningWorkspaceMismatch(runtime, mgr) {
		return mismatchedPlanningArtifacts()
	}
	if mgr != nil {
		return ManagerPlanningArtifacts(mgr)
	}
	return RuntimePlanningArtifacts(runtime)
}

// mismatchedPlanningArtifacts is the refusal both selectors return. It is a
// builder rather than an error so the refusal lands where the mutation would
// have written, which is the only place that can report it.
func mismatchedPlanningArtifacts() app.PlanningArtifactBuilder {
	return func(app.PlanningReconcileRequest) ([]storage.ArtifactChange, error) {
		return nil, fmt.Errorf("planning manager and runtime refer to different workspaces")
	}
}

func planningWorkspaceMismatch(runtime *config.RuntimeState, mgr *config.Manager) bool {
	return runtime != nil && mgr != nil && runtime.DataDir() != "" &&
		filepath.Clean(config.WorkspaceRootFromConfigPath(mgr.GetConfigPath())) != filepath.Clean(runtime.DataDir())
}

// ManagerPlanningArtifacts is the shared runtime's planning-artifact builder
// over a manager: it reconciles the newest on-disk planning domain, so every
// frontend folds Today/Next exclusivity into a transition the same way.
func ManagerPlanningArtifacts(mgr *config.Manager) app.PlanningArtifactBuilder {
	return engine.PlanningArtifacts(mgr)
}

// RuntimePlanningArtifacts binds an interactive planning action to the shell's
// current runtime snapshot. This keeps injected/failing runtime tests honest and
// prevents a long-lived service from writing through a stale manager after the
// shell switches workspace state.
func RuntimePlanningArtifacts(runtime *config.RuntimeState) app.PlanningArtifactBuilder {
	if runtime == nil {
		return nil
	}
	return func(req app.PlanningReconcileRequest) ([]storage.ArtifactChange, error) {
		planning := ClonePlanningState(runtime.Planning())
		if err := engine.RequireUnplanned(planning, req.RequireAbsent); err != nil {
			return nil, err
		}
		changed := app.ReconcilePlanningState(planning, req)
		if !changed && !req.GuardOnly {
			return nil, nil
		}
		artifact, err := runtime.CheckedPlanningArtifact(planning)
		if err != nil {
			return nil, err
		}
		return []storage.ArtifactChange{artifact.StorageChange()}, nil
	}
}
