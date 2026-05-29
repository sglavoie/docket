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

// performRestore implements the shared undo/redo algorithm:
// read the most recent source snapshot, capture the current file state as a
// reverse snapshot, apply the restore (targeted or full-file), then clean up.
func performRestore(project *models.Project, meta *UndoMeta, p restoreParams) (string, error) {
	if len(*p.sourceEntries) == 0 {
		return "", fmt.Errorf("no %s history available", p.label)
	}

	// Sort by num descending to get most recent
	sort.Slice(*p.sourceEntries, func(i, j int) bool {
		return (*p.sourceEntries)[i].Num > (*p.sourceEntries)[j].Num
	})

	latest := (*p.sourceEntries)[0]
	originalSourceEntries := append([]UndoEntry(nil), (*p.sourceEntries)...)
	originalReverseEntries := append([]UndoEntry(nil), (*p.reverseEntries)...)

	// Read and validate the snapshot
	snapPath := p.snapshotPath(project, latest.Num)
	snapBytes, err := os.ReadFile(snapPath)
	if err != nil {
		return "", fmt.Errorf("failed to read %s snapshot %d: %w", p.label, latest.Num, err)
	}
	snapContent := string(snapBytes)

	snapResult, err := ParseTasksMarkdownWithSections(snapContent)
	if err != nil {
		return "", fmt.Errorf("%s snapshot is invalid: %w", p.label, err)
	}

	// Capture current state as a reverse snapshot
	tasksPath := GetTasksFilePath(project)
	currentContent, err := os.ReadFile(tasksPath)
	if err != nil {
		return "", fmt.Errorf("failed to read current state for %s reverse snapshot: %w", p.label, err)
	}

	nextNum := 1
	for _, e := range *p.reverseEntries {
		if e.Num >= nextNum {
			nextNum = e.Num + 1
		}
	}
	revPath := p.reversePath(project, nextNum)
	if err := util.AtomicWriteFile(revPath, currentContent, FilePermissions); err != nil {
		return "", fmt.Errorf("failed to write %s reverse snapshot: %w", p.label, err)
	}
	*p.reverseEntries = append(*p.reverseEntries, UndoEntry{
		Num:       nextNum,
		Action:    latest.Action,
		Context:   latest.Context,
		Timestamp: time.Now(),
		TaskIDs:   latest.TaskIDs,
	})

	// Determine what to write: targeted restore or full-file restore
	restoreContent := snapContent
	if len(latest.TaskIDs) > 0 {
		targeted, tErr := buildTargetedRestore(string(currentContent), snapResult, latest.TaskIDs)
		if tErr != nil {
			logging.Warn("targeted %s failed, falling back to full restore: %v", p.label, tErr)
		} else {
			restoreContent = targeted
		}
	}

	if err := util.AtomicWriteFile(tasksPath, []byte(restoreContent), FilePermissions); err != nil {
		*p.reverseEntries = originalReverseEntries
		_ = os.Remove(revPath)
		return "", fmt.Errorf("failed to restore from %s snapshot: %w", p.label, err)
	}

	// Remove consumed entry and save meta BEFORE deleting the snapshot file.
	*p.sourceEntries = (*p.sourceEntries)[1:]
	if err := saveUndoMeta(project, meta); err != nil {
		*p.sourceEntries = originalSourceEntries
		*p.reverseEntries = originalReverseEntries
		if rbErr := util.AtomicWriteFile(tasksPath, currentContent, FilePermissions); rbErr != nil {
			logging.Warn("failed to rollback %s restore after metadata save failure: %v", p.label, rbErr)
		}
		_ = os.Remove(revPath)
		return "", err
	}

	// Delete consumed snapshot (cleanup — safe to fail)
	_ = os.Remove(snapPath)

	actionDesc := latest.Action
	if latest.Context != "" {
		actionDesc = fmt.Sprintf("%s: %s", latest.Action, latest.Context)
	}
	return actionDesc, nil
}

// PerformUndo restores the most recent snapshot and updates the history.
// Returns the action description of what was undone.
func PerformUndo(project *models.Project) (string, error) {
	meta, err := loadUndoMeta(project)
	if err != nil {
		return "", err
	}
	return performRestore(project, meta, restoreParams{
		label:          "undo",
		sourceEntries:  &meta.Entries,
		reverseEntries: &meta.RedoEntries,
		snapshotPath:   getSnapshotPath,
		reversePath:    getRedoSnapshotPath,
	})
}

// PerformRedo restores the most recent redo snapshot and pushes current state onto the undo stack.
// Returns the action description of what was redone.
func PerformRedo(project *models.Project) (string, error) {
	meta, err := loadUndoMeta(project)
	if err != nil {
		return "", err
	}
	return performRestore(project, meta, restoreParams{
		label:          "redo",
		sourceEntries:  &meta.RedoEntries,
		reverseEntries: &meta.Entries,
		snapshotPath:   getRedoSnapshotPath,
		reversePath:    getSnapshotPath,
	})
}
