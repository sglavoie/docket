package focus

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestFocusQueueRankKeepsDuplicateTaskIDsIndependent(t *testing.T) {
	queue := []config.FocusQueueItem{
		{ProjectID: "beta", TaskID: "shared"},
		{ProjectID: "alpha", TaskID: "shared"},
	}
	rank := FocusQueueRank(queue)
	beta := config.ProjectTaskLocator{ProjectID: "beta", TaskID: "shared"}
	alpha := config.ProjectTaskLocator{ProjectID: "alpha", TaskID: "shared"}
	if got := RankOrUnindexed(rank, beta); got != 0 {
		t.Fatalf("beta rank = %d, want 0", got)
	}
	if got := RankOrUnindexed(rank, alpha); got != 1 {
		t.Fatalf("alpha rank = %d, want 1", got)
	}
}

func TestLegacyNextQueueMigrationUsesPersistedProjectOwnership(t *testing.T) {
	alphaTask := &models.Task{ID: "shared", WorkflowState: models.StateReady}
	betaTask := &models.Task{ID: "shared", WorkflowState: models.StateReady}
	alpha := &models.Project{ID: "alpha", Tasks: []*models.Task{alphaTask}}
	beta := &models.Project{ID: "beta", Tasks: []*models.Task{betaTask}}
	queue := []config.FocusQueueItem{{ProjectID: beta.ID, TaskID: betaTask.ID}}

	locators := LegacyNextQueueMemberLocators(queue, []*models.Project{alpha, beta})
	want := config.ProjectTaskLocator{ProjectID: beta.ID, TaskID: betaTask.ID}
	if len(locators) != 1 || locators[0] != want {
		t.Fatalf("migration locators = %+v, want [%+v]", locators, want)
	}
}
