package engine

import (
	"slices"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/storage"
)

func guardCreationRestore(ledger *storage.WorkspaceLedger, dataDir string) {
	if dataDir == "" {
		return
	}
	path := config.PlanningStatePath(dataDir)
	ledger.WithTaskCreationRestoreGuard(path, func(projectIDs []string, taskID string) error {
		planning, err := config.NewDomainFileStore(dataDir).LoadPlanningReadOnly()
		if err != nil {
			return err
		}
		for _, refs := range [][]config.ProjectTaskLocator{planning.Today, planning.Next} {
			for _, ref := range refs {
				if ref.TaskID == taskID && slices.Contains(projectIDs, ref.ProjectID) {
					return &storage.ArtifactConflictError{Path: path, Expected: "task absent from Today and Next", Actual: "task has a planning membership"}
				}
			}
		}
		return nil
	})
}
