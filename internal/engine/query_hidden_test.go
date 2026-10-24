package engine

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// TestRuntimeQueryExcludesHiddenWorkUnlessAClauseOpensIt holds the shared
// engine to the hidden population rule: an effectively hidden task, flagged
// itself or under a flagged ancestor, is outside every mode, Recent included,
// until a positive hidden clause opens it; :only then narrows to it.
func TestRuntimeQueryExcludesHiddenWorkUnlessAClauseOpensIt(t *testing.T) {
	env := newHeadlessEnvironment(t)
	runtime := openHeadless(t, env.config(true))
	service := runtime.Service()
	alpha := createHeadlessProject(t, runtime, "Alpha")

	captureHeadless(t, runtime, app.CaptureRequest{ProjectID: alpha.ID, Title: "Visible"})
	parent := captureHeadless(t, runtime, app.CaptureRequest{ProjectID: alpha.ID, Title: "Hidden parent"})
	captureHeadless(t, runtime, app.CaptureRequest{ProjectID: alpha.ID, Title: "Inherited child", ParentID: &parent.TaskID})
	if _, err := service.SetHidden([]app.TaskLocator{parent}, true); err != nil {
		t.Fatalf("SetHidden: %v", err)
	}

	hidden := func(value string) models.QueryClause {
		return clauseOf(t, models.QueryFieldHidden, models.QueryPolarityPositive, models.QueryOperatorIs, value)
	}
	requireTitles(t, "all", runQuery(t, runtime, queryOf(t, models.QueryModeAll)), "Visible")
	requireTitles(t, "inbox", runQuery(t, runtime, queryOf(t, models.QueryModeInbox)), "Visible")
	requireTitles(t, "recent", runQuery(t, runtime, queryOf(t, models.QueryModeRecent)), "Visible")
	requireTitles(t, "+hidden:only", runQuery(t, runtime, queryOf(t, models.QueryModeAll, hidden(models.QueryPopulationOnly))),
		"Hidden parent", "Inherited child")
	if got := len(runQuery(t, runtime, queryOf(t, models.QueryModeAll, hidden(models.QueryPopulationInclude)))); got != 3 {
		t.Fatalf("+hidden admitted %d tasks, want all three", got)
	}
}

// TestPlanRemindersRaisesNothingForHiddenWork: hidden work is never owed
// attention, so neither a flagged task nor its descendant raises a reminder.
func TestPlanRemindersRaisesNothingForHiddenWork(t *testing.T) {
	now := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	due := now.Add(2 * time.Hour)
	project := &models.Project{ID: "p", Name: "P"}
	parent, err := models.NewGenericTask("parent", 1, "Parent")
	if err != nil {
		t.Fatal(err)
	}
	child, err := models.NewGenericTask("child", 2, "Child")
	if err != nil {
		t.Fatal(err)
	}
	child.ParentID = &parent.ID
	for _, task := range []*models.Task{parent, child} {
		task.DueDate = &due
	}
	project.Tasks = []*models.Task{parent, child}
	project.BuildTaskTree()
	opts := ReminderOptions{Now: now, Location: time.UTC}
	if got := len(PlanReminders([]*models.Project{project}, opts).Reminders); got != 2 {
		t.Fatalf("visible fixture planned %d reminders, want 2", got)
	}
	parent.Hidden = true
	if got := PlanReminders([]*models.Project{project}, opts).Reminders; len(got) != 0 {
		t.Fatalf("hidden work planned reminders: %+v", got)
	}
}
