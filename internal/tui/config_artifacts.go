package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
)

// Both helpers render candidates that were derived from a runtime snapshot, so
// both stamp them with the revisions that snapshot observed. Reloading the
// domains here to obtain a fresher baseline would silently re-parent stale
// in-memory intent onto a concurrent writer's state and overwrite it; the
// snapshot's own revision is what makes an intervening write fail closed.

// syncedDomainArtifacts renders the two explicit synced configuration domains
// into revision-checked artifacts for a transaction that intentionally spans
// catalog and planning state.
func syncedDomainArtifacts(
	runtime *config.RuntimeState,
	catalog *config.WorkspaceCatalog,
	planning *config.PlanningState,
) ([]storage.ArtifactChange, error) {
	domainArts, err := runtime.CheckedSyncedArtifacts(&config.Domains{Catalog: catalog, Planning: planning})
	if err != nil {
		return nil, err
	}
	arts := make([]storage.ArtifactChange, 0, len(domainArts))
	for _, da := range domainArts {
		arts = append(arts, da.StorageChange())
	}
	return arts, nil
}

// catalogDomainArtifact renders only workspace.json for a project metadata
// mutation that must commit atomically with project-directory and tasks.md
// changes. It never reconstructs the Config facade or rewrites unrelated
// configuration domains.
func catalogDomainArtifact(runtime *config.RuntimeState, catalog *config.WorkspaceCatalog) ([]storage.ArtifactChange, error) {
	artifact, err := runtime.CheckedCatalogArtifact(catalog)
	if err != nil {
		return nil, err
	}
	return []storage.ArtifactChange{artifact.StorageChange()}, nil
}
