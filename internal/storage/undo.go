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

// restoreParams groups the direction-dependent parameters for performRestore.
type restoreParams struct {
	label          string                                        // "undo" or "redo" (for error messages)
	sourceEntries  *[]UndoEntry                                  // entries to consume from
	reverseEntries *[]UndoEntry                                  // entries to push the reverse snapshot onto
	snapshotPath   func(project *models.Project, num int) string // path for source snapshot
	reversePath    func(project *models.Project, num int) string // path for reverse snapshot
}

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
		return "", fmt.Errorf("failed to restore from %s snapshot: %w", p.label, err)
	}

	// Remove consumed entry and save meta BEFORE deleting the snapshot file.
	*p.sourceEntries = (*p.sourceEntries)[1:]
	if err := saveUndoMeta(project, meta); err != nil {
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

	// Build set of targeted task IDs for O(1) lookup
	targetSet := make(map[string]bool, len(taskIDs))
	for _, id := range taskIDs {
		targetSet[id] = true
	}

	// Remove tasks that were newly created (exist in current but not in
	// snapshot). Undoing a creation means the task should disappear.
	filtered := make([]*models.Task, 0, len(currentResult.Tasks))
	for _, t := range currentResult.Tasks {
		if targetSet[t.ID] && snapshotMap[t.ID] == nil {
			continue // Task was created after snapshot; remove it
		}
		filtered = append(filtered, t)
	}
	currentResult.Tasks = filtered

	// Build a set of current task IDs for quick lookup
	currentIDs := make(map[string]bool, len(currentResult.Tasks))
	for _, t := range currentResult.Tasks {
		currentIDs[t.ID] = true
	}

	// Replace targeted tasks' fields, or re-add tasks that were deleted
	for _, taskID := range taskIDs {
		snapTask, ok := snapshotMap[taskID]
		if !ok {
			continue // Task didn't exist in snapshot and was already removed above
		}
		if !currentIDs[taskID] {
			// Task was deleted after snapshot; re-add it from the snapshot
			currentResult.Tasks = append(currentResult.Tasks, snapTask)
			continue
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
	dst.HasTime = src.HasTime
	dst.Active = src.Active
	dst.Pinned = src.Pinned
	dst.PinnedAt = src.PinnedAt
	dst.Dimmed = src.Dimmed
	dst.SnoozedUntil = src.SnoozedUntil
	dst.Stage = src.Stage
	dst.StartDate = src.StartDate
	dst.EstimatedDays = src.EstimatedDays
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
