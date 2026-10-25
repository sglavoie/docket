package storage

import (
	"github.com/sglavoie/docket/internal/models"
)

// ProjectRepository defines the interface for project storage operations.
// This interface enables dependency injection and easier testing through mocks.
type ProjectRepository interface {
	// Load reads project data from storage into the project struct.
	Load(project *models.Project) error

	// Save persists the project data to storage.
	Save(project *models.Project) error

	// Exists checks if a project exists at the given path.
	Exists(projectPath string) bool

	// CreateDir creates the project directory and initializes storage.
	CreateDir(project *models.Project) error

	// DeleteDir removes the project directory.
	DeleteDir(projectPath string) error
}

// FileSystemRepository implements ProjectRepository using the filesystem.
type FileSystemRepository struct{}

// NewFileSystemRepository creates a new filesystem-based repository.
func NewFileSystemRepository() *FileSystemRepository {
	return &FileSystemRepository{}
}

// Load implements ProjectRepository.
func (r *FileSystemRepository) Load(project *models.Project) error {
	return LoadProject(project)
}

// Save implements ProjectRepository.
func (r *FileSystemRepository) Save(project *models.Project) error {
	return SaveProject(project)
}

// Exists implements ProjectRepository.
func (r *FileSystemRepository) Exists(projectPath string) bool {
	return projectExists(projectPath)
}

// CreateDir implements ProjectRepository.
func (r *FileSystemRepository) CreateDir(project *models.Project) error {
	return CreateProjectDir(project)
}

// DeleteDir implements ProjectRepository.
func (r *FileSystemRepository) DeleteDir(projectPath string) error {
	return deleteProjectDir(projectPath)
}

// DefaultRepository is the default ProjectRepository implementation.
// Package consumers can replace this for testing.
var DefaultRepository ProjectRepository = NewFileSystemRepository()
