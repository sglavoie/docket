package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
)

// syncedDomainArtifacts renders the two explicit synced configuration domains
// into revision-checked artifacts for a transaction that intentionally spans
// catalog and planning state.
func syncedDomainArtifacts(
	mgr *config.Manager,
	catalog *config.WorkspaceCatalog,
	planning *config.PlanningState,
) ([]storage.ArtifactChange, error) {
	coord := mgr.DomainCoordinator()
	if _, err := coord.LoadDomains(); err != nil {
		return nil, err
	}
	domainArts, err := coord.CheckedDomainArtifacts(catalog, planning)
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
// rebuilding the legacy Config facade and prevents unrelated catalog artifacts
// from joining the task transaction.
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
