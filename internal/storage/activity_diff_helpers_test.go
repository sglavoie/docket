package storage

import "github.com/sglavoie/tt/internal/models"

func findActivity(task *models.Task, field string) *models.ActivityEntry {
	for _, entry := range task.Activity {
		if entry.Field == field {
			return entry
		}
	}

	return nil
}
