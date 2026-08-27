package storage

import (
	"path/filepath"

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
