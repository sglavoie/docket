package storage

import (
	"sort"

	"github.com/sglavoie/tt/internal/models"
)

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
