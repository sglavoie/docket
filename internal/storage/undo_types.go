package storage

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
)

const (
	undoDirName      = ".undo"
	undoMetaFileName = "meta.json"
)

// UndoEntry represents a single undo history entry.
type UndoEntry struct {
	Num             int       `json:"num"`
	Action          string    `json:"action"`
	Context         string    `json:"context,omitempty"`
	Timestamp       time.Time `json:"timestamp"`
	TaskIDs         []string  `json:"task_ids,omitempty"` // Tasks explicitly changed by user (nil = full-file undo)
	GroupID         string    `json:"group_id,omitempty"`
	GroupProjectIDs []string  `json:"group_project_ids,omitempty"`
}

// UndoSnapshotOptions configures a snapshot captured before a mutation.
type UndoSnapshotOptions struct {
	Action          string
	Context         string
	MaxDepth        int
	TaskIDs         []string
	GroupID         string
	GroupProjectIDs []string
}

// UndoMeta holds the metadata for all undo entries.
type UndoMeta struct {
	Entries     []UndoEntry `json:"entries"`
	RedoEntries []UndoEntry `json:"redo_entries"`
}

// restoreParams groups the direction-dependent parameters for performRestore.
type restoreParams struct {
	label          string                                        // "undo" or "redo" (for error messages)
	sourceEntries  *[]UndoEntry                                  // entries to consume from
	reverseEntries *[]UndoEntry                                  // entries to push the reverse snapshot onto
	snapshotPath   func(project *models.Project, num int) string // path for source snapshot
	reversePath    func(project *models.Project, num int) string // path for reverse snapshot
}

func cloneUndoStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	return append([]string(nil), values...)
}

func cloneUndoEntry(entry UndoEntry) UndoEntry {
	entry.TaskIDs = cloneUndoStringSlice(entry.TaskIDs)
	entry.GroupProjectIDs = cloneUndoStringSlice(entry.GroupProjectIDs)
	return entry
}

func cloneUndoEntrySlice(entries []UndoEntry) []UndoEntry {
	if len(entries) == 0 {
		return nil
	}
	cloned := make([]UndoEntry, len(entries))
	for i, entry := range entries {
		cloned[i] = cloneUndoEntry(entry)
	}
	return cloned
}

func cloneUndoMeta(meta *UndoMeta) *UndoMeta {
	if meta == nil {
		return &UndoMeta{}
	}
	return &UndoMeta{
		Entries:     cloneUndoEntrySlice(meta.Entries),
		RedoEntries: cloneUndoEntrySlice(meta.RedoEntries),
	}
}
