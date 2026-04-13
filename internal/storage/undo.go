package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

const undoDirName = ".undo"
const undoMetaFileName = "meta.json"

// UndoEntry represents a single undo history entry.
type UndoEntry struct {
	Num       int       `json:"num"`
	Action    string    `json:"action"`
	Context   string    `json:"context,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	TaskIDs   []string  `json:"task_ids,omitempty"` // Tasks explicitly changed by user (nil = full-file undo)
}

// UndoMeta holds the metadata for all undo entries.
type UndoMeta struct {
	Entries     []UndoEntry `json:"entries"`
	RedoEntries []UndoEntry `json:"redo_entries"`
}

// getUndoDir returns the path to the .undo directory for a project.
func getUndoDir(project *models.Project) string {
	return filepath.Join(project.Path, undoDirName)
}

// getUndoMetaPath returns the path to the meta.json file.
func getUndoMetaPath(project *models.Project) string {
	return filepath.Join(getUndoDir(project), undoMetaFileName)
}

// getSnapshotPath returns the path to a specific snapshot file.
func getSnapshotPath(project *models.Project, num int) string {
	return filepath.Join(getUndoDir(project), fmt.Sprintf("%04d.md", num))
}

// getRedoSnapshotPath returns the path to a specific redo snapshot file.
func getRedoSnapshotPath(project *models.Project, num int) string {
	return filepath.Join(getUndoDir(project), fmt.Sprintf("redo_%04d.md", num))
}

// loadUndoMeta loads the undo metadata from disk.
func loadUndoMeta(project *models.Project) (*UndoMeta, error) {
	metaPath := getUndoMetaPath(project)

	data, err := os.ReadFile(metaPath)
	if os.IsNotExist(err) {
		return &UndoMeta{Entries: []UndoEntry{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read undo meta: %w", err)
	}

	var meta UndoMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse undo meta: %w", err)
	}

	return &meta, nil
}

// saveUndoMeta saves the undo metadata to disk.
func saveUndoMeta(project *models.Project, meta *UndoMeta) error {
	metaPath := getUndoMetaPath(project)

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal undo meta: %w", err)
	}

	if err := util.AtomicWriteFile(metaPath, data, FilePermissions); err != nil {
		return fmt.Errorf("failed to write undo meta: %w", err)
	}

	return nil
}

// LoadUndoHistory returns available undo entries sorted by number descending (most recent first).
func LoadUndoHistory(project *models.Project) ([]UndoEntry, error) {
	meta, err := loadUndoMeta(project)
	if err != nil {
		return nil, err
	}

	// Sort by num descending (most recent first)
	entries := make([]UndoEntry, len(meta.Entries))
	copy(entries, meta.Entries)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Num > entries[j].Num
	})

	return entries, nil
}

// LoadRedoHistory returns available redo entries sorted by number descending (most recent first).
func LoadRedoHistory(project *models.Project) ([]UndoEntry, error) {
	meta, err := loadUndoMeta(project)
	if err != nil {
		return nil, err
	}

	entries := make([]UndoEntry, len(meta.RedoEntries))
	copy(entries, meta.RedoEntries)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Num > entries[j].Num
	})

	return entries, nil
}

// clearRedoStack deletes all redo snapshot files and clears the redo entries in meta.
func clearRedoStack(project *models.Project, meta *UndoMeta) {
	for _, entry := range meta.RedoEntries {
		_ = os.Remove(getRedoSnapshotPath(project, entry.Num))
	}
	meta.RedoEntries = []UndoEntry{}
}

// UndoHistoryExists checks if there is any undo history available.
func UndoHistoryExists(project *models.Project) bool {
	meta, err := loadUndoMeta(project)
	if err != nil {
		return false
	}
	return len(meta.Entries) > 0
}

// RedoHistoryExists checks if there is any redo history available.
func RedoHistoryExists(project *models.Project) bool {
	meta, err := loadUndoMeta(project)
	if err != nil {
		return false
	}
	return len(meta.RedoEntries) > 0
}

// GetSnapshotContent reads the content of a specific snapshot.
func GetSnapshotContent(project *models.Project, num int) (string, error) {
	snapshotPath := getSnapshotPath(project, num)

	content, err := os.ReadFile(snapshotPath)
	if err != nil {
		return "", fmt.Errorf("failed to read snapshot %d: %w", num, err)
	}

	return string(content), nil
}

// GetRedoSnapshotContent reads the content of a specific redo snapshot.
func GetRedoSnapshotContent(project *models.Project, num int) (string, error) {
	redoPath := getRedoSnapshotPath(project, num)

	content, err := os.ReadFile(redoPath)
	if err != nil {
		return "", fmt.Errorf("failed to read redo snapshot %d: %w", num, err)
	}

	return string(content), nil
}

// GetCurrentContent reads the current tasks.md content.
func GetCurrentContent(project *models.Project) (string, error) {
	tasksPath := GetTasksFilePath(project)

	content, err := os.ReadFile(tasksPath)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to read current tasks: %w", err)
	}

	return string(content), nil
}

// RecordUndoSnapshot captures the current state before a mutation.
// If maxDepth is 0, undo recording is disabled.
// taskIDs identifies the tasks explicitly changed by the user; when non-nil,
// undo will only restore those tasks instead of replacing the entire file.
func RecordUndoSnapshot(project *models.Project, action, context string, maxDepth int, taskIDs []string) error {
	if maxDepth <= 0 {
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
		Num:       nextNum,
		Action:    action,
		Context:   context,
		Timestamp: time.Now(),
		TaskIDs:   taskIDs,
	})

	// Prune old entries if over limit
	if len(meta.Entries) > maxDepth {
		if err := pruneOldSnapshots(project, meta, maxDepth); err != nil {
			// Log error but don't fail the operation
			logging.Warn("failed to prune old snapshots: %v", err)
		}
	}

	// Clear redo stack — any new mutation invalidates redo history
	clearRedoStack(project, meta)

	// Save metadata
	if err := saveUndoMeta(project, meta); err != nil {
		// Clean up the orphaned snapshot file since metadata save failed
		_ = os.Remove(snapshotPath)
		return err
	}

	return nil
}

// pruneOldSnapshots removes the oldest snapshots to stay within maxDepth.
func pruneOldSnapshots(project *models.Project, meta *UndoMeta, maxDepth int) error {
	if len(meta.Entries) <= maxDepth {
		return nil
	}

	// Sort by num ascending to find oldest
	sort.Slice(meta.Entries, func(i, j int) bool {
		return meta.Entries[i].Num < meta.Entries[j].Num
	})

	// Remove oldest entries
	toRemove := len(meta.Entries) - maxDepth
	for i := range toRemove {
		snapshotPath := getSnapshotPath(project, meta.Entries[i].Num)
		_ = os.Remove(snapshotPath) // Ignore errors for missing files
	}

	// Keep only the newest entries
	meta.Entries = meta.Entries[toRemove:]

	return nil
}

// PerformUndo restores the most recent snapshot and updates the history.
// Returns the action description of what was undone.
func PerformUndo(project *models.Project) (string, error) {
	meta, err := loadUndoMeta(project)
	if err != nil {
		return "", err
	}

	if len(meta.Entries) == 0 {
		return "", fmt.Errorf("no undo history available")
	}

	// Sort by num descending to get most recent
	sort.Slice(meta.Entries, func(i, j int) bool {
		return meta.Entries[i].Num > meta.Entries[j].Num
	})

	// Get the most recent entry
	latest := meta.Entries[0]

	// Read the snapshot content
	snapshotContent, err := GetSnapshotContent(project, latest.Num)
	if err != nil {
		return "", err
	}

	// Validate the snapshot is valid markdown
	snapshotResult, err := ParseTasksMarkdownWithSections(snapshotContent)
	if err != nil {
		return "", fmt.Errorf("snapshot is invalid: %w", err)
	}

	// Capture current state as redo snapshot before restoring
	tasksPath := GetTasksFilePath(project)
	currentContent, readErr := os.ReadFile(tasksPath)
	if readErr != nil {
		return "", fmt.Errorf("failed to read current state for redo snapshot: %w", readErr)
	}
	nextRedoNum := 1
	for _, re := range meta.RedoEntries {
		if re.Num >= nextRedoNum {
			nextRedoNum = re.Num + 1
		}
	}
	redoPath := getRedoSnapshotPath(project, nextRedoNum)
	if writeErr := util.AtomicWriteFile(redoPath, currentContent, FilePermissions); writeErr != nil {
		return "", fmt.Errorf("failed to write redo snapshot: %w", writeErr)
	}
	meta.RedoEntries = append(meta.RedoEntries, UndoEntry{
		Num:       nextRedoNum,
		Action:    latest.Action,
		Context:   latest.Context,
		Timestamp: time.Now(),
		TaskIDs:   latest.TaskIDs,
	})

	// Determine what to write: targeted restore or full-file restore
	var restoreContent string
	if len(latest.TaskIDs) > 0 {
		restoreContent, err = buildTargetedRestore(string(currentContent), snapshotResult, latest.TaskIDs)
		if err != nil {
			// Fall back to full-file restore on error
			logging.Warn("targeted undo failed, falling back to full restore: %v", err)
			restoreContent = snapshotContent
		}
	} else {
		restoreContent = snapshotContent
	}

	// Write the restored content to tasks.md
	if err := util.AtomicWriteFile(tasksPath, []byte(restoreContent), FilePermissions); err != nil {
		return "", fmt.Errorf("failed to restore from snapshot: %w", err)
	}

	// Remove the entry from metadata and save BEFORE deleting the snapshot file.
	// If meta save fails, the snapshot file still exists and state is recoverable.
	meta.Entries = meta.Entries[1:]
	if err := saveUndoMeta(project, meta); err != nil {
		return "", err
	}

	// Delete consumed snapshot (cleanup — safe to fail)
	snapshotPath := getSnapshotPath(project, latest.Num)
	_ = os.Remove(snapshotPath)

	// Build action description
	actionDesc := latest.Action
	if latest.Context != "" {
		actionDesc = fmt.Sprintf("%s: %s", latest.Action, latest.Context)
	}

	return actionDesc, nil
}

// PerformRedo restores the most recent redo snapshot and pushes current state onto the undo stack.
// Returns the action description of what was redone.
func PerformRedo(project *models.Project) (string, error) {
	meta, err := loadUndoMeta(project)
	if err != nil {
		return "", err
	}

	if len(meta.RedoEntries) == 0 {
		return "", fmt.Errorf("no redo history available")
	}

	// Sort by num descending to get most recent redo entry
	sort.Slice(meta.RedoEntries, func(i, j int) bool {
		return meta.RedoEntries[i].Num > meta.RedoEntries[j].Num
	})

	latest := meta.RedoEntries[0]

	// Read the redo snapshot content
	redoPath := getRedoSnapshotPath(project, latest.Num)
	redoContent, err := os.ReadFile(redoPath)
	if err != nil {
		return "", fmt.Errorf("failed to read redo snapshot %d: %w", latest.Num, err)
	}

	// Validate the redo snapshot
	redoResult, err := ParseTasksMarkdownWithSections(string(redoContent))
	if err != nil {
		return "", fmt.Errorf("redo snapshot is invalid: %w", err)
	}

	// Capture current state as undo snapshot before applying redo
	tasksPath := GetTasksFilePath(project)
	currentContent, readErr := os.ReadFile(tasksPath)
	if readErr != nil {
		return "", fmt.Errorf("failed to read current state for undo snapshot: %w", readErr)
	}
	nextUndoNum := 1
	for _, e := range meta.Entries {
		if e.Num >= nextUndoNum {
			nextUndoNum = e.Num + 1
		}
	}
	undoPath := getSnapshotPath(project, nextUndoNum)
	if writeErr := util.AtomicWriteFile(undoPath, currentContent, FilePermissions); writeErr != nil {
		return "", fmt.Errorf("failed to write undo snapshot: %w", writeErr)
	}
	meta.Entries = append(meta.Entries, UndoEntry{
		Num:       nextUndoNum,
		Action:    latest.Action,
		Context:   latest.Context,
		Timestamp: time.Now(),
		TaskIDs:   latest.TaskIDs,
	})

	// Determine what to write: targeted restore or full-file restore
	var restoreContent string
	if len(latest.TaskIDs) > 0 {
		restoreContent, err = buildTargetedRestore(string(currentContent), redoResult, latest.TaskIDs)
		if err != nil {
			logging.Warn("targeted redo failed, falling back to full restore: %v", err)
			restoreContent = string(redoContent)
		}
	} else {
		restoreContent = string(redoContent)
	}

	// Restore from redo snapshot
	if err := util.AtomicWriteFile(tasksPath, []byte(restoreContent), FilePermissions); err != nil {
		return "", fmt.Errorf("failed to restore from redo snapshot: %w", err)
	}

	// Remove the consumed redo entry and save meta BEFORE deleting the snapshot file.
	// If meta save fails, the snapshot file still exists and state is recoverable.
	meta.RedoEntries = meta.RedoEntries[1:]
	if err := saveUndoMeta(project, meta); err != nil {
		return "", err
	}

	// Delete consumed redo snapshot (cleanup — safe to fail)
	_ = os.Remove(redoPath)

	actionDesc := latest.Action
	if latest.Context != "" {
		actionDesc = fmt.Sprintf("%s: %s", latest.Action, latest.Context)
	}

	return actionDesc, nil
}

// buildTargetedRestore parses the current content, replaces only the targeted
// tasks with their versions from the snapshot, and re-serializes.
func buildTargetedRestore(currentContent string, snapshotResult *ParseResult, taskIDs []string) (string, error) {
	currentResult, err := ParseTasksMarkdownWithSections(currentContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse current content: %w", err)
	}

	// Build snapshot task lookup
	snapshotMap := make(map[string]*models.Task, len(snapshotResult.Tasks))
	for _, t := range snapshotResult.Tasks {
		snapshotMap[t.ID] = t
	}

	// Replace targeted tasks' fields
	for _, taskID := range taskIDs {
		snapTask, ok := snapshotMap[taskID]
		if !ok {
			continue // Task didn't exist in snapshot (was newly created); skip
		}
		for _, curTask := range currentResult.Tasks {
			if curTask.ID == taskID {
				restoreTaskFields(curTask, snapTask)
				break
			}
		}
	}

	return WriteTasksMarkdownWithSections(
		currentResult.ProjectName,
		currentResult.Tasks,
		currentResult.Sections,
		currentResult.SectionColors,
	), nil
}

// restoreTaskFields copies all persistent fields from src to dst, preserving
// only the ID (must be identical) and computed tree pointers (Children, Parent,
// Project, Level) which are rebuilt by BuildTaskTree.
func restoreTaskFields(dst, src *models.Task) {
	dst.ShortID = src.ShortID
	dst.Type = src.Type
	dst.Title = src.Title
	dst.Description = src.Description
	dst.Status = src.Status
	dst.Priority = src.Priority
	dst.ParentID = src.ParentID
	dst.Section = src.Section
	dst.CustomSection = src.CustomSection
	dst.CreatedAt = src.CreatedAt
	dst.UpdatedAt = src.UpdatedAt
	dst.CompletedAt = src.CompletedAt
	dst.DueDate = src.DueDate
	dst.Active = src.Active
	dst.Pinned = src.Pinned
	dst.PinnedAt = src.PinnedAt
	dst.Dimmed = src.Dimmed
	dst.SnoozedUntil = src.SnoozedUntil
	dst.Stage = src.Stage
	dst.Source = src.Source
	dst.URL = src.URL

	// Deep copy slices and maps
	dst.Tags = copyStringSlice(src.Tags)
	dst.DependsOn = copyStringSlice(src.DependsOn)
	dst.RelatedTo = copyStringSlice(src.RelatedTo)

	// Copy complex types by reference (parsed fresh from snapshot)
	dst.Comments = src.Comments
	dst.Recurrence = src.Recurrence
	dst.Attachments = src.Attachments
	dst.Activity = src.Activity
	dst.Checklist = src.Checklist
	dst.ScoreDimensions = src.ScoreDimensions
	dst.BugAttrs = src.BugAttrs
	dst.IdeaAttrs = src.IdeaAttrs
	dst.TaskAttrs = src.TaskAttrs
	dst.StoryAttrs = src.StoryAttrs
	dst.EpicAttrs = src.EpicAttrs
}

func copyStringSlice(s []string) []string {
	if s == nil {
		return nil
	}
	c := make([]string, len(s))
	copy(c, s)
	return c
}
