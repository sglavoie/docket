package app

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/transaction"
)

// Project lifecycle use cases route checked directory and artifact changes through
// one atomic transaction so a rename cannot leave data and catalog out of sync.

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

// CommitProjectMutation commits one user-visible project lifecycle mutation
// through the shared journal and workspace undo ledger. Repositories predating
// the ledger-aware capability still preserve atomicity through OpsCommitter.
func (s *Service) CommitProjectMutation(request storage.OpsCommitRequest) error {
	oc, err := s.opsCommitter()
	if err != nil {
		return err
	}
	if undoable, ok := oc.(UndoableOpsCommitter); ok {
		_, err := undoable.CommitOpsTransaction(request)
		return err
	}
	return oc.CommitOps(request.Ops, request.Artifacts)
}

// CommitProjectArtifacts is the artifact-only form used by project pin,
// archive, and restore metadata changes. It shares the same ledger-aware
// application boundary without manufacturing a tasks.md rewrite.
func (s *Service) CommitProjectArtifacts(request storage.ArtifactCommitRequest) error {
	committer, ok := s.repo.(ArtifactCommitter)
	if !ok {
		return fmt.Errorf("app: repository does not support project artifact commits")
	}
	_, err := committer.CommitArtifactTransaction(request)
	return err
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
	SystemInboxProjectID  string
	DataDir               string
	OldName               string
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
	if req.SystemInboxProjectID != "" &&
		req.Project.ID == req.SystemInboxProjectID &&
		(req.OldName == "" || req.OldName != req.Project.Name || req.OldSlug != req.NewSlug) {
		return RenameProjectResult{}, fmt.Errorf(
			"%w: project %q name and slug are immutable",
			ErrProtectedSystemInbox,
			req.Project.ID,
		)
	}
	if req.TasksMarkdown != nil && req.TasksExpectedRevision == nil {
		return RenameProjectResult{}, invalidInput("tasks revision", "renamed tasks.md requires an expected revision")
	}
	if err := validateLifecycleTaskWrites(req.CrossRefWrites); err != nil {
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
	if err := s.CommitProjectMutation(storage.OpsCommitRequest{
		Ops:         ops,
		Artifacts:   artifacts,
		ProjectIDs:  []string{req.Project.ID},
		UndoAction:  "Edit Project",
		UndoContext: req.Project.Name,
	}); err != nil {
		return RenameProjectResult{}, err
	}
	req.Project.Path = plan.NewPath
	return RenameProjectResult{NewPath: plan.NewPath, SlugChanged: req.OldSlug != req.NewSlug}, nil
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
