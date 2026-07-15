package app

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// ProjectBlockerPort keeps blocker serialization and reads separate from the
// workspace-aware note port. Blockers remain a project-scoped whole-list artifact
// and continue to ride a project ChangeSet.
type ProjectBlockerPort interface {
	EncodeBlockers(project *models.Project, blockers []*models.Blocker) (storage.ArtifactChange, error)
	Blockers(project *models.Project) ([]*models.Blocker, error)
}

type storageProjectBlockerPort struct{}

func (storageProjectBlockerPort) EncodeBlockers(project *models.Project, blockers []*models.Blocker) (storage.ArtifactChange, error) {
	content := storage.RenderBlockersContent(blockers)
	return storage.ArtifactChange{Path: storage.BlockersFilePath(project), Data: []byte(content)}, nil
}

func (storageProjectBlockerPort) Blockers(project *models.Project) ([]*models.Blocker, error) {
	return storage.LoadBlockers(project)
}

// BlockersResult is the presentation-neutral outcome of a blocker-list mutation.
type BlockersResult struct {
	// ProjectID owns the blockers.
	ProjectID string
	// Count is the total number of blockers persisted.
	Count int
	// ActiveCount is the number of unresolved blockers.
	ActiveCount int
	// Revision is the owning project's revision after the commit.
	Revision string
}

// SetBlockers replaces a project's whole blocker list, committing the rewritten
// blockers.md atomically with the project. Each blocker is validated (non-empty
// title) and defaulted (a fresh ID and creation time when unset) before the
// commit, so a rejected request never persists a partial list.
func (s *Service) SetBlockers(projectID string, blockers []*models.Blocker) (BlockersResult, error) {
	snap, project, revision, err := s.resolveProject(projectID)
	if err != nil {
		return BlockersResult{}, err
	}
	active := 0
	for _, b := range blockers {
		if b == nil {
			return BlockersResult{}, invalidInput("blocker", "blocker is required")
		}
		if strings.TrimSpace(b.Title) == "" {
			return BlockersResult{}, invalidInput("blocker", "title is required")
		}
		if strings.TrimSpace(b.ID) == "" {
			b.ID = s.ids.NewID()
		}
		if b.CreatedAt.IsZero() {
			b.CreatedAt = s.clock.Now()
		}
		if !b.IsResolved() {
			active++
		}
	}
	change, err := s.blockers.EncodeBlockers(project, blockers)
	if err != nil {
		return BlockersResult{}, err
	}
	rev, err := s.commitArtifact(ArtifactBlockers, project, revision, snap.Document(project.ID), change, "Update Blockers", project.Name)
	if err != nil {
		return BlockersResult{}, err
	}
	return BlockersResult{ProjectID: project.ID, Count: len(blockers), ActiveCount: active, Revision: rev}, nil
}

// commitArtifact commits one project write plus a single durable artifact change as
// one transaction, returning the project's new revision. It refuses a machine-local
// artifact kind: machine-local UI state must never be committed to the durable
// workspace, so routing it here is a programming error caught before any commit.
func (s *Service) commitArtifact(kind ArtifactKind, project *models.Project, expectedRevision string, base *storage.ProjectDocument, change storage.ArtifactChange, undoAction, undoContext string) (string, error) {
	if !kind.IsDurable() {
		return "", invalidInput("artifact", fmt.Sprintf("%s is not durable synced state and cannot be committed to the workspace", kind))
	}
	applyPostMutationHooks(project)
	results, err := s.repo.Commit(storage.ChangeSet{
		Project:          project,
		ExpectedRevision: expectedRevision,
		Base:             base,
		Artifacts:        []storage.ArtifactChange{change},
		UndoAction:       undoAction,
		UndoContext:      undoContext,
	})
	if err != nil {
		return "", err
	}
	return revisionAt(results, 0), nil
}
