package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

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
}

// UndoMeta holds the metadata for all undo entries.
type UndoMeta struct {
	Entries []UndoEntry `json:"entries"`
}

// GetUndoDir returns the path to the .undo directory for a project.
func GetUndoDir(project *models.Project) string {
	return filepath.Join(project.Path, undoDirName)
}

// getUndoMetaPath returns the path to the meta.json file.
func getUndoMetaPath(project *models.Project) string {
	return filepath.Join(GetUndoDir(project), undoMetaFileName)
}

// getSnapshotPath returns the path to a specific snapshot file.
func getSnapshotPath(project *models.Project, num int) string {
	return filepath.Join(GetUndoDir(project), fmt.Sprintf("%04d.md", num))
}

// LoadUndoMeta loads the undo metadata from disk.
func LoadUndoMeta(project *models.Project) (*UndoMeta, error) {
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

	if err := util.AtomicWriteFile(metaPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write undo meta: %w", err)
	}

	return nil
}

// LoadUndoHistory returns available undo entries sorted by number descending (most recent first).
func LoadUndoHistory(project *models.Project) ([]UndoEntry, error) {
	meta, err := LoadUndoMeta(project)
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

// UndoHistoryExists checks if there is any undo history available.
func UndoHistoryExists(project *models.Project) bool {
	meta, err := LoadUndoMeta(project)
	if err != nil {
		return false
	}
	return len(meta.Entries) > 0
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
func RecordUndoSnapshot(project *models.Project, action, context string, maxDepth int) error {
	if maxDepth <= 0 {
		return nil // Undo disabled
	}

	undoDir := GetUndoDir(project)

	// Create undo directory if it doesn't exist
	if err := os.MkdirAll(undoDir, 0755); err != nil {
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
	meta, err := LoadUndoMeta(project)
	if err != nil {
		// Start fresh if meta is corrupted
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
	if err := util.AtomicWriteFile(snapshotPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write snapshot: %w", err)
	}

	// Add new entry
	meta.Entries = append(meta.Entries, UndoEntry{
		Num:       nextNum,
		Action:    action,
		Context:   context,
		Timestamp: time.Now(),
	})

	// Prune old entries if over limit
	if len(meta.Entries) > maxDepth {
		if err := pruneOldSnapshots(project, meta, maxDepth); err != nil {
			// Log error but don't fail the operation
			fmt.Fprintf(os.Stderr, "warning: failed to prune old snapshots: %v\n", err)
		}
	}

	// Save metadata
	if err := saveUndoMeta(project, meta); err != nil {
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
	for i := 0; i < toRemove; i++ {
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
	meta, err := LoadUndoMeta(project)
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
	content, err := GetSnapshotContent(project, latest.Num)
	if err != nil {
		return "", err
	}

	// Validate the snapshot is valid markdown
	_, err = ParseTasksMarkdownWithSections(content)
	if err != nil {
		return "", fmt.Errorf("snapshot is invalid: %w", err)
	}

	// Write the snapshot content to tasks.md
	tasksPath := GetTasksFilePath(project)
	if err := util.AtomicWriteFile(tasksPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to restore from snapshot: %w", err)
	}

	// Remove the snapshot file
	snapshotPath := getSnapshotPath(project, latest.Num)
	_ = os.Remove(snapshotPath)

	// Remove the entry from metadata
	meta.Entries = meta.Entries[1:]

	// Save updated metadata
	if err := saveUndoMeta(project, meta); err != nil {
		return "", err
	}

	// Build action description
	actionDesc := latest.Action
	if latest.Context != "" {
		actionDesc = fmt.Sprintf("%s: %s", latest.Action, latest.Context)
	}

	return actionDesc, nil
}
