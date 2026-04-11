package storage

import (
	"github.com/sglavoie/tt/internal/models"
)

// ProjectRepository defines the interface for project storage operations.
// This interface enables dependency injection and easier testing through mocks.
type ProjectRepository interface {
	// Load reads project data from storage into the project struct.
	Load(project *models.Project) error

	// Save persists the project data to storage.
	Save(project *models.Project) error

	// Backup creates a backup copy of the project data.
	Backup(project *models.Project) error

	// RestoreFromBackup swaps the current data with the backup.
	RestoreFromBackup(project *models.Project) error

	// Exists checks if a project exists at the given path.
	Exists(projectPath string) bool

	// CreateDir creates the project directory and initializes storage.
	CreateDir(project *models.Project) error

	// DeleteDir removes the project directory.
	DeleteDir(projectPath string) error

	// BackupExists checks if a backup exists for the project.
	BackupExists(project *models.Project) bool
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

// Backup implements ProjectRepository.
func (r *FileSystemRepository) Backup(project *models.Project) error {
	return BackupProject(project)
}

// RestoreFromBackup implements ProjectRepository.
func (r *FileSystemRepository) RestoreFromBackup(project *models.Project) error {
	return RestoreFromBackup(project)
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

// BackupExists implements ProjectRepository.
func (r *FileSystemRepository) BackupExists(project *models.Project) bool {
	return BackupExists(project)
}

// DefaultRepository is the default ProjectRepository implementation.
// Package consumers can replace this for testing.
var DefaultRepository ProjectRepository = NewFileSystemRepository()
