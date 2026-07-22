package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestTUIArchiveCleanupPreservesDuplicateIDReferencesInOtherProject(t *testing.T) {
	for _, tc := range []struct {
		name  string
		apply func(*AppModel, *models.Project)
	}{
		{
			name: "task archive",
			apply: func(m *AppModel, beta *models.Project) {
				m.handleTaskArchiveMessages(TasksArchivedMsg{
					Count:                1,
					ArchivedTaskLocators: []app.TaskLocator{{ProjectID: beta.ID, TaskID: "shared-id"}},
				})
			},
		},
		{
			name: "project archive",
			apply: func(m *AppModel, beta *models.Project) {
				beta.Archived = true
				m.handleProjectMessages(ProjectArchivedMsg{Project: beta})
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			domains := config.NewDomains()
			domains.Planning.FocusQueue = []config.FocusQueueItem{
				{ProjectID: "alpha", TaskID: "shared-id"},
				{ProjectID: "beta", TaskID: "shared-id"},
			}
			domains.Planning.DailyPlan = []config.DailyPlanItem{
				{ProjectID: "alpha", TaskID: "shared-id"},
				{ProjectID: "beta", TaskID: "shared-id"},
			}
			domains.Planning.FocusDismissedIDs = []string{"shared-id"}
			domains.Machine.RecentTaskVisits = []config.RecentTaskVisit{
				{ProjectID: "alpha", TaskID: "shared-id"},
				{ProjectID: "beta", TaskID: "shared-id"},
			}
			alpha := &models.Project{ID: "alpha", Name: "Alpha", Tasks: []*models.Task{{ID: "shared-id"}}}
			beta := &models.Project{ID: "beta", Name: "Beta", Tasks: []*models.Task{{ID: "shared-id"}}}

			m := newTestAppModel()
			m.runtime = config.NewRuntimeState("", domains)
			setTestAppProjects(m, alpha, beta)
			if len(beta.Tasks) != 1 {
				t.Fatalf("beta task setup lost before archive: %d", len(beta.Tasks))
			}
			tc.apply(m, beta)

			planning := m.runtime.Planning()
			if len(planning.FocusQueue) != 1 || planning.FocusQueue[0].ProjectID != alpha.ID ||
				len(planning.DailyPlan) != 1 || planning.DailyPlan[0].ProjectID != alpha.ID {
				t.Fatalf("archive cleanup removed wrong planning refs: focus=%+v daily=%+v", planning.FocusQueue, planning.DailyPlan)
			}
			if len(planning.FocusDismissedIDs) != 1 || planning.FocusDismissedIDs[0] != "shared-id" {
				t.Fatalf("qualified archive changed legacy dismissed IDs: %v", planning.FocusDismissedIDs)
			}
			if visits := m.runtime.Machine().RecentTaskVisits; len(visits) != 1 || visits[0].ProjectID != alpha.ID {
				t.Fatalf("archive cleanup removed wrong recent visits: %+v", visits)
			}
		})
	}
}
