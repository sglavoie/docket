package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
)

// configDomainArtifacts renders the four synced config-domain artifacts
// (workspace/planning/settings/automation.json) for a transactional TUI mutation
// that folds a config change — durable planning-ref cleanup or cross-project
// planning reassignment — into the same atomic commit as the task write it
// accompanies. It wraps config.Manager.SyncedDomainArtifacts into
// storage.ArtifactChange entries and replaces the pre-cutover single-blob
// .tt.json artifact, which a domain-file reload would no longer read.
func configDomainArtifacts(mgr *config.Manager, cfg *config.Config) ([]storage.ArtifactChange, error) {
	domainArts, err := mgr.SyncedDomainArtifacts(cfg)
	if err != nil {
		return nil, err
	}
	arts := make([]storage.ArtifactChange, 0, len(domainArts))
	for _, da := range domainArts {
		arts = append(arts, storage.ArtifactChange{Path: da.Path, Data: da.Data})
	}
	return arts, nil
}
