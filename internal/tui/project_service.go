package tui

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

// Load reloads a project from disk.
func (s *ProjectService) Load(project *models.Project) error {
	return storage.LoadProject(project)
}
