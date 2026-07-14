package cmd

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// LoadProjectWithMigrationAudit loads a project's tasks and captures the
// loader-level audit of retired legacy lifecycle values (status/section/stage/
// active). It is the config-aware entry point that pairs storage.LoadProject
// with storage.CaptureMigrationAudit so the config-aware layer can consume the
// audit — the v1 parser itself has no config/focus-queue access. Loading fails
// closed: any load error is returned and no audit is produced.
func LoadProjectWithMigrationAudit(project *models.Project) (*storage.MigrationAudit, error) {
	result, err := storage.LoadProjectWithWarnings(project)
	if err != nil {
		return nil, err
	}
	return result.MigrationAudit, nil
}

// MigrateLegacyActiveToFocusQueue consumes a loader-level migration audit at the
// config-aware boundary: every task that carried the legacy active:true flag is
// added to the Focus Queue if not already present, so the marker is preserved
// when the v2 writer stops persisting Active. Returns the number of tasks newly
// added to the queue. The retired Status/Section/Stage/Active values remain only
// in the audit report, never as operational fields. Callers must persist the
// config after a non-zero return.
func MigrateLegacyActiveToFocusQueue(planning *config.PlanningState, audit *storage.MigrationAudit) int {
	if planning == nil || audit == nil {
		return 0
	}
	added := 0
	for _, id := range audit.ActiveTaskIDs() {
		if planning.AddToFocusQueue(id, audit.ProjectID) {
			added++
		}
	}
	return added
}
