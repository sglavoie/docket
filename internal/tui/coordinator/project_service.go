package coordinator

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/workspace"
)

// Persister defines the interface for project persistence operations.
// This interface enables dependency injection and testing by allowing
// mock implementations to replace the real storage layer.
type Persister interface {
	// Load reloads a project from disk.
	Load(project *models.Project) error
}

// ProjectService provides consolidated project persistence operations for TUI handlers.
// ProjectService implements the Persister interface.
type ProjectService struct {
	workspaceLoader *workspace.Loader
}

// NewProjectService creates a new ProjectService instance.
func NewProjectService() *ProjectService {
	return &ProjectService{
		workspaceLoader: workspace.NewLoader(),
	}
}

// WorkspaceLoader returns the retained snapshot loader. The loader is what keeps
// an unchanged project's indexed parse across rebuilds, so the cross-scope load
// and the Hub's tally both read it from the one view-owned service rather than
// building a second loader of their own.
func (s *ProjectService) WorkspaceLoader() *workspace.Loader { return s.workspaceLoader }

// Load reloads a project from disk.
func (s *ProjectService) Load(project *models.Project) error {
	return storage.LoadProject(project)
}
