package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/outliner"
)

// searchGateProjects builds one two-project scope whose task order is the source
// order a sigil-only query must preserve.
func searchGateProjects() (*models.Project, *models.Project) {
	today := time.Now()
	later := today.AddDate(0, 0, 3)

	pinned := primaryRowsTask("pinned", models.StateBacklog)
	pinned.Title = "Pin candidate"
	pinned.Pinned = true
	pinned.Tags = []string{"backend"}
	priorityOne := primaryRowsTask("priority-one", models.StateBacklog)
	priorityOne.Title = "Ship auth"
	priorityOne.Priority = models.PriorityHigh
	priorityOne.DueDate = &today
	contexted := primaryRowsTask("contexted", models.StateBacklog)
	contexted.Title = "Client work"
	contexted.Context = "client-acme"
	contexted.DueDate = &later
	plannedToday := primaryRowsTask("planned-today", models.StateBacklog)
	plannedToday.Title = "Planned today"
	plannedNext := primaryRowsTask("planned-next", models.StateBacklog)
	plannedNext.Title = "Planned next"
	finished := primaryRowsTask("finished", models.StateDone)
	finished.Title = "Old auth work"
	alpha := primaryRowsProject("alpha", pinned, priorityOne, contexted, plannedToday, plannedNext, finished)

	betaTask := primaryRowsTask("beta-task", models.StateBacklog)
	betaTask.Title = "Beta auth"
	betaTask.Tags = []string{"backend"}
	beta := primaryRowsProject("beta", betaTask)

	return alpha, beta
}

func searchGateOutliner() *outliner.OutlinerModel {
	alpha, beta := searchGateProjects()
	outlinerView := scopeTestOutliner(alpha, beta)
	outlinerView.Runtime.Planning().SetToday([]config.ProjectTaskLocator{
		{ProjectID: alpha.ID, TaskID: "planned-today"},
	})
	outlinerView.Runtime.Planning().SetNext([]config.ProjectTaskLocator{
		{ProjectID: alpha.ID, TaskID: "planned-next"},
	})
	return outlinerView
}
