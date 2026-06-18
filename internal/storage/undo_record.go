package storage

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// RecordUndoSnapshot captures the current state before a mutation.
// If maxDepth is 0, undo recording is disabled.
// taskIDs identifies the tasks explicitly changed by the user; when non-nil,
// undo will only restore those tasks instead of replacing the entire file.
func RecordUndoSnapshot(project *models.Project, action, context string, maxDepth int, taskIDs []string) error {
	return RecordUndoSnapshotWithOptions(project, UndoSnapshotOptions{
		Action:   action,
		Context:  context,
		MaxDepth: maxDepth,
		TaskIDs:  taskIDs,
	})
}

// RecordUndoSnapshotWithOptions captures the current state before a mutation.
func RecordUndoSnapshotWithOptions(project *models.Project, opts UndoSnapshotOptions) error {
	if opts.MaxDepth <= 0 {
		return nil // Undo disabled
	}

	undoDir := getUndoDir(project)

	// Create undo directory if it doesn't exist
	if err := os.MkdirAll(undoDir, DirPermissions); err != nil {
		return fmt.Errorf("failed to create undo directory: %w", err)
	}

	// Read current tasks.md content
	tasksPath := GetTasksFilePath(project)
	content, err := os.ReadFile(tasksPath)
	if os.IsNotExist(err) {
		// No tasks file yet, nothing to snapshot
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read tasks for snapshot: %w", err)
	}

	// Load existing metadata
	meta, err := loadUndoMeta(project)
	if err != nil {
		// Start fresh if meta is corrupted
		logging.Warn("undo metadata corrupted for %s, starting fresh: %v", project.Name, err)
		meta = &UndoMeta{Entries: []UndoEntry{}}
	}

	// Determine next snapshot number
	nextNum := 1
	for _, entry := range meta.Entries {
		if entry.Num >= nextNum {
			nextNum = entry.Num + 1
		}
	}

	// Write snapshot file
	snapshotPath := getSnapshotPath(project, nextNum)
	if err := util.AtomicWriteFile(snapshotPath, content, FilePermissions); err != nil {
		return fmt.Errorf("failed to write snapshot: %w", err)
	}

	// Add new entry
	meta.Entries = append(meta.Entries, UndoEntry{
		Num:             nextNum,
		Action:          opts.Action,
		Context:         opts.Context,
		Timestamp:       time.Now(),
		TaskIDs:         cloneUndoStringSlice(opts.TaskIDs),
		GroupID:         opts.GroupID,
		GroupProjectIDs: cloneUndoStringSlice(opts.GroupProjectIDs),
	})

	redoPathsToDelete := make([]string, 0, len(meta.RedoEntries))
	for _, entry := range meta.RedoEntries {
		redoPathsToDelete = append(redoPathsToDelete, getRedoSnapshotPath(project, entry.Num))
	}
	meta.RedoEntries = []UndoEntry{}

	var prunedPathsToDelete []string
	if len(meta.Entries) > opts.MaxDepth {
		sort.Slice(meta.Entries, func(i, j int) bool {
			return meta.Entries[i].Num < meta.Entries[j].Num
		})
		toRemove := len(meta.Entries) - opts.MaxDepth
		prunedPathsToDelete = make([]string, 0, toRemove)
		for i := range toRemove {
			prunedPathsToDelete = append(prunedPathsToDelete, getSnapshotPath(project, meta.Entries[i].Num))
		}
		meta.Entries = meta.Entries[toRemove:]
	}

	if err := saveUndoMeta(project, meta); err != nil {
		_ = os.Remove(snapshotPath)
		return err
	}

	for _, p := range redoPathsToDelete {
		_ = os.Remove(p)
	}
	for _, p := range prunedPathsToDelete {
		_ = os.Remove(p)
	}

	return nil
}
