package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
)

// syncedDomainArtifacts renders the two explicit synced configuration domains
// into revision-checked artifacts for a transaction that intentionally spans
// catalog and planning state.
//
// The candidates were derived from a runtime snapshot, so they are stamped with
// the revisions that snapshot observed. Reloading the domains here to obtain a
// fresher baseline would silently re-parent stale in-memory intent onto a
// concurrent writer's state and overwrite it; the snapshot's own revision is
// what makes an intervening write fail closed.
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
