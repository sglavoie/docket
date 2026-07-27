package tui

import "github.com/sglavoie/tt/internal/models"

// collectDescendantIDs collects all IDs of a task and its descendants recursively.
func collectDescendantIDs(tasks []*models.Task, rootIDs map[string]bool) map[string]bool {
	idsToDelete := make(map[string]bool)
	var collectIDs func(t *models.Task)
	collectIDs = func(t *models.Task) {
		idsToDelete[t.ID] = true
		for _, child := range t.Children {
			collectIDs(child)
		}
	}
	for _, t := range tasks {
		if rootIDs[t.ID] {
			collectIDs(t)
		}
	}
	return idsToDelete
}
