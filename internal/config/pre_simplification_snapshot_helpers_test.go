package config

func createPreSimplificationV5Snapshot(dataDir string, callbacks preSimplificationV5SnapshotCallbacks) (*PreSimplificationV5Snapshot, error) {
	return createPreSimplificationV5SnapshotWithAudit(dataDir, callbacks, nil)
}
