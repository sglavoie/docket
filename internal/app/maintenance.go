package app

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

// MaintenanceRepository is the privileged repository surface reserved for repair
// and doctor operations. It extends the normal transactional Repository with an
// explicit raw-save escape hatch that bypasses the optimistic-concurrency check
// and records no undo history. Ordinary user mutations must never require this
// interface: keeping the privileged path on a distinct type makes it visible at
// every call site and unreachable from the everyday mutation Service.
type MaintenanceRepository interface {
	Repository
	// RawSaveProject rewrites a project's canonical files outside the transactional
	// commit boundary. It is the deliberate repair-only path a doctor command uses
	// to fix a diverged or corrupt file; it performs no revision check and captures
	// no undo entry, so it must only be reached through Maintenance.
	RawSaveProject(project *models.Project) error
}

// Maintenance exposes repair/doctor use cases over a privileged repository. It is
// deliberately separate from Service: only a caller holding a MaintenanceRepository
// can construct it, so the privileged raw-save path cannot be reached through the
// ordinary mutation service.
type Maintenance struct {
	repo MaintenanceRepository
}

// NewMaintenance constructs a Maintenance over a privileged repository.
func NewMaintenance(repo MaintenanceRepository) *Maintenance {
	return &Maintenance{repo: repo}
}

// RepairProject reloads the workspace, rebuilds the target project's derived state
// (task tree, reconciled completion, reordered roots), and rewrites it through the
// privileged raw save. Unlike a normal mutation it neither checks a revision nor
// records undo history, so it is safe to run against a file whose on-disk revision
// has diverged from any snapshot — exactly the situation a doctor command repairs.
func (m *Maintenance) RepairProject(projectID string) error {
	snap, err := m.repo.Snapshot()
	if err != nil {
		return err
	}
	project := snap.ProjectByID(projectID)
	if project == nil {
		return fmt.Errorf("%w: %s", ErrProjectNotFound, projectID)
	}
	applyPostMutationHooks(project)
	return m.repo.RawSaveProject(project)
}
