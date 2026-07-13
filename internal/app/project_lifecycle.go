package app

import (
	"encoding/json"
	"fmt"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/transaction"
)

// The project-lifecycle use cases route project archive-location, trash, restore,
// and purge through typed application requests, modeling each command's durable
// effects — a directory move, a tombstone, a config rewrite, cross-reference
// repairs, a recursive delete — as ONE atomic journaled transaction committed
// through the OpsCommitter boundary. They mirror the task-mutation use cases:
// validate intent, plan every effect before any write, and commit the whole set
// atomically so a partial project lifecycle can never leave data and catalog out of
// sync.

// opsCommitter resolves the repository's raw-op commit capability, failing closed
// when the injected repository does not support project-lifecycle transactions.
func (s *Service) opsCommitter() (OpsCommitter, error) {
	if oc, ok := s.repo.(OpsCommitter); ok {
		return oc, nil
	}
	return nil, fmt.Errorf("app: repository does not support project-lifecycle commits")
}

// artifactToOp converts a durable config/cross-reference artifact into the journal
// operation that applies it: a delete removes the target, a write replaces it with
// the artifact's bytes (defaulting the mode to the standard private file
// permission).
func artifactToOp(art storage.ArtifactChange) transaction.Op {
	if art.Delete {
		return transaction.Op{Kind: transaction.OpDelete, Target: art.Path}
	}
	return transaction.Op{Kind: transaction.OpWrite, Target: art.Path, Data: art.Data, Mode: art.Mode}
}

// CommitConfig commits a durable configuration rewrite — now the set of synced
// domain-file artifacts — as one atomic journaled transaction. Project archive and
// unarchive change only catalog/config state (the archived flag, the current-
// project pointer, and the archived project's planning references), so their whole
// durable effect is these config artifacts, committed through the same journal that
// guards every other mutation.
func (s *Service) CommitConfig(configArtifacts ...storage.ArtifactChange) error {
	oc, err := s.opsCommitter()
	if err != nil {
		return err
	}
	ops := make([]transaction.Op, 0, len(configArtifacts))
	for _, art := range configArtifacts {
		ops = append(ops, artifactToOp(art))
	}
	return oc.CommitOps(ops)
}

// DeleteProjectRequest asks to move a project into the versioned trash as one atomic
// transaction. Confirm carries the explicit destructive intent; Config is the
// encoded config after the project has been removed from the catalog, and
// CrossRefWrites carry any other project's tasks.md rewritten to drop cross-project
// references to the deleted project. ConfigRefs is the opaque snapshot of the
// project's configuration references stored in the tombstone for a faithful restore.
type DeleteProjectRequest struct {
	Project        *models.Project
	DataDir        string
	Reason         string
	ConfigRefs     json.RawMessage
	Config         []storage.ArtifactChange
	CrossRefWrites []storage.ArtifactChange
	Confirm        bool
}

// DeleteProjectResult reports a delete: the allocated trash entry ID and the
// deleted project's slug (for the restore hint the adapter prints).
type DeleteProjectResult struct {
	EntryID     string
	ProjectSlug string
}

// DeleteProject moves a project's data directory into a new versioned trash entry,
// writes its tombstone, removes it from the catalog, and repairs cross-project
// references — all as ONE atomic journaled transaction. Either the whole delete
// lands (data in trash, catalog updated, references repaired) or none of it does; a
// crash mid-commit is rolled forward by recovery. The request must carry explicit
// destructive intent; an unconfirmed request never moves anything.
func (s *Service) DeleteProject(req DeleteProjectRequest) (DeleteProjectResult, error) {
	if req.Project == nil {
		return DeleteProjectResult{}, invalidInput("project", "project is required")
	}
	if !req.Confirm {
		return DeleteProjectResult{}, fmt.Errorf("%w: deleting project %q", ErrDestructiveConfirmationRequired, req.Project.Slug)
	}
	oc, err := s.opsCommitter()
	if err != nil {
		return DeleteProjectResult{}, err
	}

	plan, err := storage.PlanTrash(req.Project, req.DataDir, req.Reason, req.ConfigRefs)
	if err != nil {
		return DeleteProjectResult{}, err
	}

	ops := make([]transaction.Op, 0, len(plan.Ops)+len(req.Config)+len(req.CrossRefWrites))
	ops = append(ops, plan.Ops...)
	for _, w := range req.CrossRefWrites {
		ops = append(ops, artifactToOp(w))
	}
	for _, c := range req.Config {
		ops = append(ops, artifactToOp(c))
	}

	if err := oc.CommitOps(ops); err != nil {
		return DeleteProjectResult{}, err
	}
	return DeleteProjectResult{EntryID: plan.EntryID, ProjectSlug: req.Project.Slug}, nil
}

// RestoreProjectRequest asks to restore a trashed project as one atomic transaction.
// BuildConfig receives the project identity reconstructed from the tombstone and the
// tombstone itself, and returns the encoded config with the project re-added and its
// references re-applied, so the config rewrite that depends on the restored identity
// commits atomically with the directory move.
type RestoreProjectRequest struct {
	DataDir     string
	EntryID     string
	BuildConfig func(project *models.Project, tomb *storage.TrashTombstone) ([]storage.ArtifactChange, error)
}

// RestoreProjectResult reports a restore: the reconstructed project (with its task
// data reloaded from disk) and the trash entry that was consumed.
type RestoreProjectResult struct {
	Project *models.Project
	EntryID string
}

// RestoreProject moves a trash entry's data back to its project directory, re-adds
// the project to the catalog, re-establishes its configuration references, and
// removes the drained trash entry — all as ONE atomic journaled transaction. It
// fails closed when the entry is missing, the slug is unsafe, or an active project
// already occupies the destination. After the commit places the files, the restored
// project's task data is reloaded from disk into its computed fields.
func (s *Service) RestoreProject(req RestoreProjectRequest) (RestoreProjectResult, error) {
	if req.BuildConfig == nil {
		return RestoreProjectResult{}, invalidInput("config", "a config builder is required")
	}
	oc, err := s.opsCommitter()
	if err != nil {
		return RestoreProjectResult{}, err
	}

	plan, err := storage.PlanRestore(req.DataDir, req.EntryID)
	if err != nil {
		return RestoreProjectResult{}, err
	}

	configArtifacts, err := req.BuildConfig(plan.Project, plan.Tombstone)
	if err != nil {
		return RestoreProjectResult{}, err
	}

	ops := make([]transaction.Op, 0, len(plan.Ops)+len(configArtifacts))
	ops = append(ops, plan.Ops...)
	for _, c := range configArtifacts {
		ops = append(ops, artifactToOp(c))
	}

	if err := oc.CommitOps(ops); err != nil {
		return RestoreProjectResult{}, err
	}

	if err := storage.ReloadRestoredProject(plan.Project); err != nil {
		return RestoreProjectResult{}, err
	}
	return RestoreProjectResult{Project: plan.Project, EntryID: req.EntryID}, nil
}

// PurgeProjectRequest asks to permanently and irreversibly delete a trash entry.
// Confirm carries the explicit destructive intent.
type PurgeProjectRequest struct {
	DataDir string
	EntryID string
	Slug    string
	Confirm bool
}

// PurgeProjectResult reports a purge: whether an entry was actually removed (false
// when the entry was already gone) and its slug.
type PurgeProjectResult struct {
	Purged bool
	Slug   string
}

// PurgeProject permanently and recursively deletes a trash entry as ONE atomic
// journaled transaction (files first, then directories deepest-first, all contained
// within the approved roots). This is irreversible and therefore records no undo.
// The request must carry explicit destructive intent; a missing entry is a no-op
// that reports Purged=false.
func (s *Service) PurgeProject(req PurgeProjectRequest) (PurgeProjectResult, error) {
	if !req.Confirm {
		return PurgeProjectResult{}, fmt.Errorf("%w: purging trash entry %q", ErrDestructiveConfirmationRequired, req.Slug)
	}
	oc, err := s.opsCommitter()
	if err != nil {
		return PurgeProjectResult{}, err
	}

	ops, err := storage.PlanPurge(req.DataDir, req.EntryID)
	if err != nil {
		return PurgeProjectResult{}, err
	}
	if len(ops) == 0 {
		return PurgeProjectResult{Purged: false, Slug: req.Slug}, nil
	}
	if err := oc.CommitOps(ops); err != nil {
		return PurgeProjectResult{}, err
	}
	return PurgeProjectResult{Purged: true, Slug: req.Slug}, nil
}
