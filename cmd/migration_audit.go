package cmd

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// LoadProjectWithMigrationAudit loads a project's tasks and captures the
// loader-level audit of retained legacy lifecycle values (status/section/stage).
// Loading fails closed: any load error is returned and no audit is produced.
func LoadProjectWithMigrationAudit(project *models.Project) (*storage.MigrationAudit, error) {
	result, err := storage.LoadProjectWithWarnings(project)
	if err != nil {
		return nil, err
	}
	return result.MigrationAudit, nil
}
