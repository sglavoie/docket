package engine

import (
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/storage"
)

// ResolveProjectConflict shares the runtime's transactional committer, history
// and sync enrollment with ordinary edits. Text is used only for a manual merge.
func (r *Runtime) ResolveProjectConflict(id string, choice storage.ConflictResolution, text string) (*storage.ConflictResolveResult, error) {
	store := storage.NewConflictStore(config.ConflictsDir(r.DataDir()))
	if choice == storage.ResolveUseFile {
		return storage.ResolveConflictText(store, r.repo.svc, id, text)
	}
	return storage.ResolveConflict(store, r.repo.svc, id, choice, "")
}
