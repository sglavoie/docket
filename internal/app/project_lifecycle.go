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

// CommitProjectMutationResult commits one user-visible project lifecycle
// mutation through the shared journal and workspace undo ledger. The returned
// ID is the durable transaction identity recorded in the workspace ledger.
// Legacy repositories that only implement OpsCommitter still commit atomically
// but return an empty identity, so presentation layers fail closed and omit
// recovery controls.
func (s *Service) CommitProjectMutationResult(request storage.OpsCommitRequest) (string, error) {
	oc, err := s.opsCommitter()
	if err != nil {
		return "", err
	}
	if undoable, ok := oc.(UndoableOpsCommitter); ok {
		return undoable.CommitOpsTransaction(request)
	}
	return "", oc.CommitOps(request.Ops, request.Artifacts)
}

// CommitProjectArtifactsResult commits artifact-only project pin, archive, and
// restore metadata changes through the ledger-aware application boundary
// without manufacturing a tasks.md rewrite. It returns the durable transaction
// identity recorded in the workspace ledger.
func (s *Service) CommitProjectArtifactsResult(request storage.ArtifactCommitRequest) (string, error) {
	committer, ok := s.repo.(ArtifactCommitter)
	if !ok {
		return "", fmt.Errorf("app: repository does not support project artifact commits")
	}
	return committer.CommitArtifactTransaction(request)
}

// RenameProjectRequest asks to rename a project as one atomic transaction. Project
// is the in-memory project already mutated to its new Name/Slug; OldSlug/NewSlug
// derive the directory move. TasksMarkdown is the renamed project's rewritten
// tasks.md (heading reflecting the new name), and TasksExpectedRevision is the
// raw revision captured from the exact old-path bytes used to render it. Leave
// TasksMarkdown nil to skip the tasks write. Config is the encoded catalog after
// the rename.
type RenameProjectRequest struct {
	Project                *models.Project
	SystemUnfiledProjectID string
	DataDir                string
	OldName                string
	OldSlug                string
	NewSlug                string
	TasksMarkdown          []byte
	TasksExpectedRevision  *string
	Config                 []storage.ArtifactChange
}

// RenameProjectResult renames a project's data directory, rewrites its tasks.md
// heading, and updates the catalog as ONE atomic journaled transaction. Either
// the whole rename lands (directory at the new slug, heading and catalog
// updated) or none of it does; a crash mid-commit is rolled forward by recovery.
// It returns the transaction ID that correlates the resulting user feedback
// with the newest workspace-ledger entry.
func (s *Service) RenameProjectResult(req RenameProjectRequest) (string, error) {
	if req.Project == nil {
		return "", invalidInput("project", "project is required")
	}
	protectedID := req.SystemUnfiledProjectID
	if protectedID != "" &&
		req.Project.ID == protectedID &&
		(req.OldName == "" || req.OldName != req.Project.Name || req.OldSlug != req.NewSlug) {
		return "", fmt.Errorf(
			"%w: project %q name and slug are immutable",
			ErrProtectedSystemUnfiled,
			req.Project.ID,
		)
	}
	if req.TasksMarkdown != nil && req.TasksExpectedRevision == nil {
		return "", invalidInput("tasks revision", "renamed tasks.md requires an expected revision")
	}
	plan, err := storage.PlanProjectRename(req.DataDir, req.OldSlug, req.NewSlug)
	if err != nil {
		return "", err
	}

	ops := make([]transaction.Op, 0, len(plan.Ops))
	ops = append(ops, plan.Ops...)
	artifacts := make([]storage.ArtifactChange, 0, 1+len(req.Config))
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
	artifacts = append(artifacts, req.Config...)
	transactionID, err := s.CommitProjectMutationResult(storage.OpsCommitRequest{
		Ops:         ops,
		Artifacts:   artifacts,
		ProjectIDs:  []string{req.Project.ID},
		UndoAction:  "Edit Project",
		UndoContext: req.Project.Name,
	})
	if err != nil {
		return "", err
	}
	req.Project.Path = plan.NewPath
	return transactionID, nil
}

// DeleteProjectRequest asks to delete a project as one atomic transaction.
// Project is the project being removed, Config the encoded catalog and planning
// artifacts that no longer reference it. The system Unfiled project is never
// deletable, so SystemUnfiledProjectID is the guard the application layer
// checks rather than trusting the caller's list.
type DeleteProjectRequest struct {
	Project                *models.Project
	SystemUnfiledProjectID string
	DataDir                string
	Config                 []storage.ArtifactChange
}

// DeleteProjectResult removes a project's data directory and the catalog and
// planning entries naming it as ONE atomic journaled transaction. Either the
// whole delete lands or none of it does, and the workspace undo ledger captures
// every removed file's bytes first so `u` restores the project entire. It
// returns the transaction ID that correlates the resulting user feedback with
// the newest ledger entry.
func (s *Service) DeleteProjectResult(req DeleteProjectRequest) (string, error) {
	if req.Project == nil {
		return "", invalidInput("project", "project is required")
	}
	if req.SystemUnfiledProjectID != "" && req.Project.ID == req.SystemUnfiledProjectID {
		return "", fmt.Errorf(
			"%w: project %q cannot be deleted",
			ErrProtectedSystemUnfiled,
			req.Project.ID,
		)
	}
	ops, err := storage.PlanProjectDelete(req.DataDir, req.Project.Slug)
	if err != nil {
		return "", err
	}
	// Every address under the deleted project stops existing, so the catalog
	// documents that referenced one are rewritten inside this same transaction
	// rather than left to dangle until a later doctor run.
	referenceWrites, err := s.referenceArtifactsForRemovedProject(req.Project.ID)
	if err != nil {
		return "", err
	}
	artifacts := make([]storage.ArtifactChange, 0, len(referenceWrites)+len(req.Config))
	artifacts = append(artifacts, referenceWrites...)
	artifacts = append(artifacts, req.Config...)
	return s.CommitProjectMutationResult(storage.OpsCommitRequest{
		Ops:         ops,
		Artifacts:   artifacts,
		ProjectIDs:  []string{req.Project.ID},
		UndoAction:  "Delete Project",
		UndoContext: req.Project.Name,
	})
}
