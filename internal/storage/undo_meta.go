package storage

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

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
