package config

func createPreSimplificationV5Snapshot(dataDir string, hooks *preSimplificationV5SnapshotHooks) (*PreSimplificationV5Snapshot, error) {
	return createPreSimplificationV5SnapshotWithAudit(dataDir, hooks, nil)
}
