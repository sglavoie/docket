package cmd

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// WithBlockerUpdate encapsulates the common Load→Find→Update→Save pattern for blocker commands.
// It loads the current project's blockers, finds one by ID prefix, applies the update function,
// and saves the (potentially modified) blocker list back.
// The update function receives the full blocker list and the matched blocker, and returns
// the blocker list to save (allowing deletion by returning a filtered list).
func WithBlockerUpdate(idPrefix string, updateFn func([]*models.Blocker, *models.Blocker) ([]*models.Blocker, error)) error {
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	blockers, err := storage.LoadBlockers(project)
	if err != nil {
		return fmt.Errorf("failed to load blockers: %w", err)
	}

	blocker := storage.FindBlockerByPrefix(blockers, idPrefix)
	if blocker == nil {
		return fmt.Errorf("blocker not found: %s", idPrefix)
	}

	blockers, err = updateFn(blockers, blocker)
	if err != nil {
		return err
	}

	if err := storage.SaveBlockers(project, blockers); err != nil {
		return fmt.Errorf("failed to save blockers: %w", err)
	}

	return nil
}

// WithBlockerCreate encapsulates the common Load→Create→Save pattern for adding a blocker.
// It loads the current project's blockers, applies the create function to produce a new blocker,
// appends it to the list, and saves.
func WithBlockerCreate(createFn func() (*models.Blocker, error)) (*models.Blocker, error) {
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return nil, err
	}

	blockers, err := storage.LoadBlockers(project)
	if err != nil {
		return nil, fmt.Errorf("failed to load blockers: %w", err)
	}

	blocker, err := createFn()
	if err != nil {
		return nil, err
	}

	blockers = append(blockers, blocker)
	if err := storage.SaveBlockers(project, blockers); err != nil {
		return nil, fmt.Errorf("failed to save blockers: %w", err)
	}

	return blocker, nil
}
