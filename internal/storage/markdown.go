package storage

import (
	"os"
	"path/filepath"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// writeFileAtomic writes a file atomically to prevent corruption on interrupted writes.
var writeFileAtomic = util.AtomicWriteFile

const tasksFileName = "tasks.md"

// LockFileName is the advisory lock file used to serialize concurrent writes
// to a project's tasks.md. All callers (CLI, TUI, storage layer) must use
// this same name so the locks actually provide mutual exclusion.
const LockFileName = ".tasks.md.lock"

// File permission constants (from util for consistency).
const (
	DirPermissions  = util.DirPermissions
	FilePermissions = util.FilePermissions
)

func tasksFilePath(projectPath string) string {
	return filepath.Join(projectPath, tasksFileName)
}

// TasksFilePath returns the on-disk path of a project's canonical tasks.md.
// It is the source file whose stat identity (mod time + size) the workspace
// snapshot loader uses to decide whether a cached parse is still fresh.
func TasksFilePath(projectPath string) string {
	return tasksFilePath(projectPath)
}

// ProjectTasksFileMissing reports whether a filesystem-backed project has no
// tasks.md on disk. A load answers that with an empty project, which is right
// for one never written to and wrong as evidence about tasks other state still
// names: a file a sync provider evicted or has not delivered yet looks the
// same. A test double keeps no file and is never missing.
func ProjectTasksFileMissing(project *models.Project) bool {
	if project == nil {
		return false
	}
	if _, ok := DefaultRepository.(*FileSystemRepository); !ok {
		return false
	}
	_, err := os.Stat(tasksFilePath(project.Path))
	return os.IsNotExist(err)
}
