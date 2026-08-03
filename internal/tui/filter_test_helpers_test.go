package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func passesCommonFilters(task *models.Task, filter *CommonFilterState, project *models.Project) bool {
	return passesCommonFiltersAt(task, filter, time.Now())
}
