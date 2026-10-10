package engine

import (
	"os"

	"github.com/sglavoie/tt/internal/syncexchange"
)

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

// SyncLocalSequenceReader observes committed local history, including undo and
// changes made through secondary views. Call only on the frontend's owning
// goroutine. Cache by file identity to avoid decoding history on every UI event;
// publication and imports may replace the file without advancing NextSeq.
func (r *Runtime) SyncLocalSequenceReader() func() (uint64, error) {
	if r.manager == nil || !r.state.Catalog().Enrolled() || r.repo.svc.SyncHistory() == nil {
		return nil
	}
	history := r.repo.svc.SyncHistory()
	var previous os.FileInfo
	var sequence uint64
	return func() (uint64, error) {
		info, err := os.Stat(history.StatePath())
		if err != nil {
			return 0, err
		}
		if previous != nil && os.SameFile(previous, info) && previous.Size() == info.Size() && previous.ModTime().Equal(info.ModTime()) {
			return sequence, nil
		}
		state, err := history.LoadState()
		if err != nil {
			return 0, err
		}
		previous, sequence = info, state.NextSeq
		return sequence, nil
	}
}
