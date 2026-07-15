package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// inboxTriageUndoAction labels the workspace-ledger entry recorded for a single
// inbox triage mutation, so `tt undo --global` names the change meaningfully.
const inboxTriageUndoAction = "Inbox Triage"

// commitInboxProject commits an already-mutated inbox project through the
// app-layer transactional repository — the journaled commit service with the
// workspace undo ledger attached — then reloads it from disk so the caller's
// in-memory copy reflects the committed transaction. It replaces the retired
// raw saveProjectSync path: the inbox operations still mutate their in-memory
// project in place (and keep their own view-local undo history), but the durable
// write now flows through the same transactional boundary every other TUI
// mutation uses. No post-mutation reconciliation hooks run here, matching the
// legacy inbox save behavior (SaveProject never ran them either).
func commitInboxProject(dataDir string, deps *config.RuntimeDependencies, ledger *storage.WorkspaceLedger, project *models.Project) error {
	if project == nil {
		return nil
	}
	repo := newTUIAppRepository(dataDir, deps, ledger)
	if _, err := repo.Commit(storage.ChangeSet{
		Project:    project,
		UndoAction: inboxTriageUndoAction,
	}); err != nil {
		return err
	}
	return storage.LoadProject(project)
}
