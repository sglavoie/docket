package tui

import "github.com/sglavoie/docket/internal/tui/operation"

func taskOperationManifestIDs() []operation.OperationID {
	manifest := taskOperationManifest()
	ids := make([]operation.OperationID, 0, len(manifest))
	for _, entry := range manifest {
		if !entry.DirectOnly {
			ids = append(ids, entry.ID)
		}
	}
	return ids
}
