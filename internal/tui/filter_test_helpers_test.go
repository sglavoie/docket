package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func passesCommonFilters(task *models.Task, filter *CommonFilterState, project *models.Project) bool {
	return passesCommonFiltersAt(task, filter, project, time.Now())
}

func passesCommonFiltersAt(task *models.Task, filter *CommonFilterState, project *models.Project, now time.Time) bool {
	return passesCommonFiltersWithResolverAt(task, filter, project, nil, now)
}
