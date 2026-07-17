package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
)

// workflowProjectStatusArtifactBuilder applies project-rule status changes to
// the catalog domain and renders one optimistic-concurrency-checked artifact.
// The app service groups it with every task, destination-project, and planning
// write produced by the same cascade.
func workflowProjectStatusArtifactBuilder(mgr *config.Manager) app.WorkflowProjectStatusArtifactBuilder {
	if mgr == nil {
		return nil
	}
	return func(changes []app.WorkflowProjectStatusChange) ([]storage.ArtifactChange, error) {
		coord := mgr.DomainCoordinator()
		catalog, err := coord.LoadCatalog()
		if err != nil {
			return nil, err
		}
		changed := false
		for _, change := range changes {
			found := false
			for i := range catalog.Projects {
				if catalog.Projects[i].ID != change.ProjectID {
					continue
				}
				found = true
				if catalog.Projects[i].StatusIcon != change.StatusIcon || catalog.Projects[i].StatusIconSource != change.StatusIconSource {
					catalog.Projects[i].StatusIcon = change.StatusIcon
					catalog.Projects[i].StatusIconSource = change.StatusIconSource
					changed = true
				}
				break
			}
			if !found {
				return nil, fmt.Errorf("workflow project status target %q is absent from the workspace catalog", change.ProjectID)
			}
		}
		if !changed {
			return nil, nil
		}
		artifact, err := coord.CheckedCatalogArtifact(catalog)
		if err != nil {
			return nil, err
		}
		return []storage.ArtifactChange{artifact.StorageChange()}, nil
	}
}

// syncedDomainArtifacts renders the four explicit synced configuration domains
// into revision-checked artifacts for a transaction that intentionally spans
// catalog, planning, settings, and automation state.
func syncedDomainArtifacts(
	mgr *config.Manager,
	catalog *config.WorkspaceCatalog,
	planning *config.PlanningState,
	settings *config.Settings,
	automation *config.AutomationConfig,
) ([]storage.ArtifactChange, error) {
	coord := mgr.DomainCoordinator()
	if _, err := coord.LoadDomains(); err != nil {
		return nil, err
	}
	domainArts, err := coord.CheckedDomainArtifacts(catalog, planning, settings, automation)
	if err != nil {
		return nil, err
	}
	arts := make([]storage.ArtifactChange, 0, len(domainArts))
	for _, da := range domainArts {
		arts = append(arts, da.StorageChange())
	}
	return arts, nil
}

// planningDomainArtifact renders only planning.json for a task mutation that
// also removes durable planning references. Keeping this domain-native avoids
// rebuilding the legacy Config facade and prevents unrelated catalog/settings/
// automation artifacts from joining the task transaction.
func planningDomainArtifact(mgr *config.Manager, planning *config.PlanningState) ([]storage.ArtifactChange, error) {
	coord := mgr.DomainCoordinator()
	if _, err := coord.LoadPlanning(); err != nil {
		return nil, err
	}
	artifact, err := coord.CheckedPlanningArtifact(planning)
	if err != nil {
		return nil, err
	}
	return []storage.ArtifactChange{artifact.StorageChange()}, nil
}

// catalogDomainArtifact renders only workspace.json for a project metadata
// mutation that must commit atomically with project-directory and tasks.md
// changes. It never reconstructs the Config facade or rewrites unrelated
// configuration domains.
func catalogDomainArtifact(mgr *config.Manager, catalog *config.WorkspaceCatalog) ([]storage.ArtifactChange, error) {
	coord := mgr.DomainCoordinator()
	if _, err := coord.LoadCatalog(); err != nil {
		return nil, err
	}
	artifact, err := coord.CheckedCatalogArtifact(catalog)
	if err != nil {
		return nil, err
	}
	return []storage.ArtifactChange{artifact.StorageChange()}, nil
}
