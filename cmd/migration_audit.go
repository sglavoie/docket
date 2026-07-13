package cmd

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/workflow"
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
func MigrateLegacyActiveToFocusQueue(cfg *config.Config, audit *storage.MigrationAudit) int {
	if cfg == nil || audit == nil {
		return 0
	}
	added := 0
	for _, id := range audit.ActiveTaskIDs() {
		if cfg.AddToFocusQueue(id, audit.ProjectID) {
			added++
		}
	}
	return added
}

// applyWorkflowFocusChanges applies the Focus Queue membership changes a
// set_active workflow action produced (surfaced as result.FocusChanges) against
// the config-backed Focus Queue. The task's project ID is used for new entries.
// Returns true if the queue changed so the caller knows to persist config.
func applyWorkflowFocusChanges(cfg *config.Config, projectID string, changes []workflow.FocusChange) bool {
	if cfg == nil || len(changes) == 0 {
		return false
	}
	changed := false
	for _, fc := range changes {
		if fc.Add {
			changed = cfg.AddToFocusQueue(fc.TaskID, projectID) || changed
		} else {
			changed = cfg.RemoveFromFocusQueue(fc.TaskID) || changed
		}
	}
	return changed
}
