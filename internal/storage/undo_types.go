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

// restoreParams groups the direction-dependent parameters for performRestore.
type restoreParams struct {
	label          string                                        // "undo" or "redo" (for error messages)
	sourceEntries  *[]UndoEntry                                  // entries to consume from
	reverseEntries *[]UndoEntry                                  // entries to push the reverse snapshot onto
	snapshotPath   func(project *models.Project, num int) string // path for source snapshot
	reversePath    func(project *models.Project, num int) string // path for reverse snapshot
}
