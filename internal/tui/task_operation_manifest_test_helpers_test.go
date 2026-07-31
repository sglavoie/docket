package tui

func taskOperationManifestIDs() []OperationID {
	manifest := taskOperationManifest()
	ids := make([]OperationID, 0, len(manifest))
	for _, entry := range manifest {
		if !entry.DirectOnly {
			ids = append(ids, entry.ID)
		}
	}
	return ids
}
