package storage

import (
	"path/filepath"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// writeFileAtomic writes a file atomically to prevent corruption on interrupted writes.
var writeFileAtomic = util.AtomicWriteFile

const tasksFileName = "tasks.md"
const backupFileName = ".tasks.md.bak"

// LockFileName is the advisory lock file used to serialize concurrent writes
// to a project's tasks.md. All callers (CLI, TUI, storage layer) must use
// this same name so the locks actually provide mutual exclusion.
const LockFileName = ".tasks.md.lock"

// File permission constants (from util for consistency).
const (
	DirPermissions  = util.DirPermissions
	FilePermissions = util.FilePermissions
)

// ProjectLoadResult contains the result of loading a project, including any warnings.
type ProjectLoadResult struct {
	Warnings []ParseWarning
	// RawRevision is the optimistic-concurrency revision of the exact tasks.md
	// bytes parsed by this load. A non-nil pointer to "" means the file was
	// absent; nil means the active test repository cannot provide a filesystem
	// revision. Callers that persist load-time normalization must carry this
	// revision into the checked maintenance commit rather than re-reading later.
	RawRevision *string
	// MigrationAudit records the retained retired schema-v1 lifecycle values the
	// loaded tasks carried for migration reporting.
	// Nil when the project was not loaded through the markdown parser (e.g. an
	// in-memory test repository) or the tasks file did not exist.
	MigrationAudit *MigrationAudit
}

// DefaultMaxActivityEntries is the default maximum number of activity entries per task.
const DefaultMaxActivityEntries = 50

// MaxActivityEntries controls the maximum number of activity entries per task.
// Set by the application during startup from config. Falls back to DefaultMaxActivityEntries.
var MaxActivityEntries = DefaultMaxActivityEntries

func tasksFilePath(projectPath string) string {
	return filepath.Join(projectPath, tasksFileName)
}

// TasksFilePath returns the on-disk path of a project's canonical tasks.md.
// It is the source file whose stat identity (mod time + size) the workspace
// snapshot loader uses to decide whether a cached parse is still fresh.
func TasksFilePath(projectPath string) string {
	return tasksFilePath(projectPath)
}

func backupFilePath(projectPath string) string {
	return filepath.Join(projectPath, backupFileName)
}

// GetTasksFilePath returns the path to a project's tasks.md file.
func GetTasksFilePath(project *models.Project) string {
	return tasksFilePath(project.Path)
}

// GetBackupFilePath returns the path to a project's backup file.
func GetBackupFilePath(project *models.Project) string {
	return backupFilePath(project.Path)
}
