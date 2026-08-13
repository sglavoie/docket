package tui

import (
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func makeBoardTask(t *testing.T, id string, shortID int, title string) *models.Task {
	t.Helper()
	task, err := models.NewGenericTask(id, shortID, title)
	if err != nil {
		t.Fatal(err)
	}
	task.WorkflowState = models.StateDoing
	return task
}

func modeRowQualifiedIDs(rows []OutlinerModeRow) []string {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Task != nil && row.Project != nil {
			ids = append(ids, row.Project.ID+"/"+row.Task.ID)
		}
	}
	return ids
}

func TestTodayModeUsesExplicitDurableOrderWithoutImplicitDescendants(t *testing.T) {
	parent := primaryRowsTask("parent", models.StateWaiting)
	child := primaryRowsTask("child", models.StateDoing)
	child.ParentID = &parent.ID
	unplannedDescendant := primaryRowsTask("unplanned", models.StateBacklog)
	unplannedDescendant.ParentID = &child.ID
	terminal := primaryRowsTask("terminal", models.StateDone)
	alpha := primaryRowsProject("alpha", parent, child, unplannedDescendant, terminal)

	sameID := primaryRowsTask("child", models.StatePaused)
	snoozed := primaryRowsTask("snoozed", models.StateBacklog)
	future := time.Now().AddDate(0, 0, 2)
	snoozed.DeferredUntil = &future
	beta := primaryRowsProject("beta", sameID, snoozed)

	rows := BuildTodayModeRows(OutlinerModeInput{
		Projects: []*models.Project{alpha, beta},
		TodayOrder: []PlanningTask{
			{Task: child, Project: alpha},
			{Task: sameID, Project: beta},
			{Task: parent, Project: alpha},
			{Task: snoozed, Project: beta},
			{Task: terminal, Project: alpha},
		},
	})

	want := []string{"alpha/child", "beta/child", "alpha/parent"}
	if got := modeRowQualifiedIDs(rows); !slices.Equal(got, want) {
		t.Fatalf("Today projection = %v, want durable order %v", got, want)
	}
	if rows[0].Breadcrumb != "parent ›" {
		t.Fatalf("planned child breadcrumb = %q, want parent context", rows[0].Breadcrumb)
	}
	for _, row := range rows {
		if row.Task.ID == unplannedDescendant.ID {
			t.Fatalf("Today row %+v implies hierarchy expansion", row)
		}
	}

	rows = BuildTodayModeRows(OutlinerModeInput{
		Projects:       []*models.Project{alpha, beta},
		TodayOrder:     []PlanningTask{{Task: snoozed, Project: beta}},
		IncludeSnoozed: true,
	})
	if got := modeRowQualifiedIDs(rows); !slices.Equal(got, []string{"beta/snoozed"}) {
		t.Fatalf("explicit snoozed Today projection = %v", got)
	}
}

func TestNextModeUsesQualifiedDurableOrderAndDirectFilters(t *testing.T) {
	alphaParent := primaryRowsTask("parent", models.StateBacklog)
	alphaChild := primaryRowsTask("shared", models.StateBacklog)
	alphaChild.ParentID = &alphaParent.ID
	alpha := primaryRowsProject("alpha", alphaParent, alphaChild)

	betaShared := primaryRowsTask("shared", models.StateBacklog)
	betaOther := primaryRowsTask("other", models.StateBacklog)
	beta := primaryRowsProject("beta", betaShared, betaOther)

	order := []config.ProjectTaskLocator{
		{ProjectID: beta.ID, TaskID: betaShared.ID},
		{ProjectID: alpha.ID, TaskID: alphaChild.ID},
		{ProjectID: beta.ID, TaskID: betaOther.ID},
	}
	rows := BuildNextModeRows(OutlinerModeInput{
		Projects:  []*models.Project{alpha, beta},
		NextOrder: order,
		TaskFilter: func(task *models.Task, _ *models.Project) bool {
			return task.ID == "shared"
		},
	})

	want := []string{"beta/shared", "alpha/shared"}
	if got := modeRowQualifiedIDs(rows); !slices.Equal(got, want) {
		t.Fatalf("filtered Next projection = %v, want %v", got, want)
	}
	if rows[1].Breadcrumb != "parent ›" {
		t.Fatalf("promoted Next child breadcrumb = %q, want parent context", rows[1].Breadcrumb)
	}
}

func TestPlanningModesHonorFocusCandidatesWithoutAddingAncestors(t *testing.T) {
	parent := primaryRowsTask("parent", models.StateBacklog)
	child := primaryRowsTask("child", models.StateBacklog)
	child.ParentID = &parent.ID
	outside := primaryRowsTask("outside", models.StateBacklog)
	project := primaryRowsProject("project", parent, child, outside)

	input := OutlinerModeInput{
		Projects:       []*models.Project{project},
		CandidateTasks: map[string][]*models.Task{project.ID: {child}},
		TodayOrder: []PlanningTask{
			{Task: parent, Project: project},
			{Task: child, Project: project},
		},
		NextOrder: []config.ProjectTaskLocator{
			{ProjectID: project.ID, TaskID: outside.ID},
			{ProjectID: project.ID, TaskID: child.ID},
		},
	}
	if got := modeRowQualifiedIDs(BuildTodayModeRows(input)); !slices.Equal(got, []string{"project/child"}) {
		t.Fatalf("focused Today = %v, want only explicit child", got)
	}
	if got := modeRowQualifiedIDs(BuildNextModeRows(input)); !slices.Equal(got, []string{"project/child"}) {
		t.Fatalf("focused Next = %v, want only explicit child", got)
	}
}
