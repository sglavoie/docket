package engine

import "github.com/sglavoie/tt/internal/syncexchange"

// BackgroundSyncExchanger captures an independent exchange worker for an
// enrolled workspace. Construct it on the frontend's owning goroutine; invoke
// it off that goroutine. Each pass loads its own runtime, so importing a catalog
// or planning change never mutates the frontend's snapshot underneath a draft.
// The frontend must refresh its snapshot when it is safe to adopt the result.
// Unenrolled and in-memory workspaces have no worker.
func (r *Runtime) BackgroundSyncExchanger(opts SyncExchangeOptions) func() (*syncexchange.FolderReport, error) {
	if r.manager == nil || !r.state.Catalog().Enrolled() {
		return nil
	}
	manager := *r.manager
	return func() (*syncexchange.FolderReport, error) {
		state, err := manager.LoadRuntimeState()
		if err != nil {
			return nil, err
		}
		worker := Attach(state, &manager, nil, nil)
		return worker.SyncExchange(opts)
	}
}
