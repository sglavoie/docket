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
// effects — a directory move, a tombstone, checked domain artifacts,
// cross-reference repairs, a recursive delete — as ONE atomic transaction committed
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

// CommitConfig commits synced domain-file artifacts as one atomic journaled
// transaction while retaining each artifact's revision contract. Project archive
// and unarchive use it for catalog/planning changes; machine-local state is
// committed separately because it lives outside the workspace transaction root.
func (s *Service) CommitConfig(configArtifacts ...storage.ArtifactChange) error {
	oc, err := s.opsCommitter()
	if err != nil {
		return err
	}
	return oc.CommitOps(nil, configArtifacts)
}

// CommitWorkspaceBootstrap atomically applies bootstrap file operations and
// their revision-checked catalog/project artifacts. It is intentionally narrow:
// adapters use it only while installing or upgrading workspace-owned projects.
func (s *Service) CommitWorkspaceBootstrap(ops []transaction.Op, artifacts ...storage.ArtifactChange) error {
	oc, err := s.opsCommitter()
	if err != nil {
		return err
	}
	return oc.CommitOps(ops, artifacts)
}

// RenameProjectRequest asks to rename a project as one atomic transaction. Project
// is the in-memory project already mutated to its new Name/Slug; OldSlug/NewSlug
// derive the directory move. TasksMarkdown is the renamed project's rewritten
// tasks.md (heading reflecting the new name), and TasksExpectedRevision is the
// raw revision captured from the exact old-path bytes used to render it. Leave
// TasksMarkdown nil to skip the tasks write. Config is the encoded catalog after
// the rename, and CrossRefWrites carry every other project's revision-checked
// tasks.md rewrite to point its cross-project references at the new slug.
type RenameProjectRequest struct {
	Project               *models.Project
	DataDir               string
	OldSlug               string
	NewSlug               string
	TasksMarkdown         []byte
	TasksExpectedRevision *string
	Config                []storage.ArtifactChange
	CrossRefWrites        []storage.ArtifactChange
}

// RenameProjectResult reports a rename: the project's new on-disk path and whether
// the slug actually changed (a name change can leave the slug unchanged).
type RenameProjectResult struct {
	NewPath     string
	SlugChanged bool
}

// RenameProject renames a project's data directory, rewrites its tasks.md heading,
// updates the catalog, and repairs cross-project references — all as ONE atomic
// journaled transaction. Either the whole rename lands (directory at the new slug,
// heading and catalog updated, references repointed) or none of it does; a crash
// mid-commit is rolled forward by recovery. This replaces the pre-migration path
// that renamed the directory eagerly during validation and then saved the config,
// tasks.md, and each referencing project as independent effects, any of which
// could fail after the directory had already moved.
func (s *Service) RenameProject(req RenameProjectRequest) (RenameProjectResult, error) {
	if req.Project == nil {
		return RenameProjectResult{}, invalidInput("project", "project is required")
	}
	if req.TasksMarkdown != nil && req.TasksExpectedRevision == nil {
		return RenameProjectResult{}, invalidInput("tasks revision", "renamed tasks.md requires an expected revision")
	}
	if err := validateLifecycleTaskWrites(req.CrossRefWrites); err != nil {
		return RenameProjectResult{}, err
	}
	oc, err := s.opsCommitter()
	if err != nil {
		return RenameProjectResult{}, err
	}

	plan, err := storage.PlanProjectRename(req.DataDir, req.OldSlug, req.NewSlug)
	if err != nil {
		return RenameProjectResult{}, err
	}

	ops := make([]transaction.Op, 0, len(plan.Ops))
	ops = append(ops, plan.Ops...)
	artifacts := make([]storage.ArtifactChange, 0, 1+len(req.CrossRefWrites)+len(req.Config))
	if req.TasksMarkdown != nil {
		tasksWrite := storage.ArtifactChange{
			Path:             storage.TasksFilePath(plan.NewPath),
			Data:             req.TasksMarkdown,
			Mode:             storage.FilePermissions,
			ExpectedRevision: req.TasksExpectedRevision,
		}
		// A slug-changing rename validates and locks the old tasks.md even though
		// the replacement is written below the new path after the directory move.
		// When no move is planned (same slug or an unpersisted source), the target
		// itself is the correct expected-revision path.
		if len(plan.Ops) > 0 {
			tasksWrite.ExpectedRevisionPath = storage.TasksFilePath(plan.Ops[0].Source)
		}
		artifacts = append(artifacts, tasksWrite)
	}
	artifacts = append(artifacts, req.CrossRefWrites...)
	artifacts = append(artifacts, req.Config...)
	if err := oc.CommitOps(ops, artifacts); err != nil {
		return RenameProjectResult{}, err
	}
	req.Project.Path = plan.NewPath
	return RenameProjectResult{NewPath: plan.NewPath, SlugChanged: req.OldSlug != req.NewSlug}, nil
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
	if err := validateLifecycleTaskWrites(req.CrossRefWrites); err != nil {
		return DeleteProjectResult{}, err
	}
	oc, err := s.opsCommitter()
	if err != nil {
		return DeleteProjectResult{}, err
	}

	plan, err := storage.PlanTrash(req.Project, req.DataDir, req.Reason, req.ConfigRefs)
	if err != nil {
		return DeleteProjectResult{}, err
	}

	ops := make([]transaction.Op, 0, len(plan.Ops))
	ops = append(ops, plan.Ops...)
	artifacts := append(append([]storage.ArtifactChange(nil), req.CrossRefWrites...), req.Config...)
	if err := oc.CommitOps(ops, artifacts); err != nil {
		return DeleteProjectResult{}, err
	}
	return DeleteProjectResult{EntryID: plan.EntryID, ProjectSlug: req.Project.Slug}, nil
}

// validateLifecycleTaskWrites fails closed when an adapter tries to submit a
// lifecycle-generated full-file tasks.md rewrite without the revision captured
// during planning. The commit boundary can only prevent stale-byte clobbers when
// every rewrite participates in the same optimistic-concurrency preflight.
func validateLifecycleTaskWrites(writes []storage.ArtifactChange) error {
	for i := range writes {
		if writes[i].ExpectedRevision == nil {
			return invalidInput("tasks revision", fmt.Sprintf("cross-project rewrite %d requires an expected revision", i))
		}
	}
	return nil
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

	ops := make([]transaction.Op, 0, len(plan.Ops))
	ops = append(ops, plan.Ops...)
	if err := oc.CommitOps(ops, configArtifacts); err != nil {
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
	if err := oc.CommitOps(ops, nil); err != nil {
		return PurgeProjectResult{}, err
	}
	return PurgeProjectResult{Purged: true, Slug: req.Slug}, nil
}
