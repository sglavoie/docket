package app_test

import (
	"reflect"
	"testing"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
)

// A parked project leaves the task workspace but keeps its plan. Reconciliation
// is Complete, so it would prune any locator the snapshot cannot see; the
// snapshot population is the available-project set precisely so parked owners
// stay visible to it and unparking restores the plan untouched.
func TestReconcilePlanningKeepsParkedProjectEntries(t *testing.T) {
	plainTask := newTask("plain-1", 1, "Plain task")
	plainTask.ApplyWorkflowState(models.StateBacklog)
	parkedTask := newTask("parked-1", 1, "Parked task")
	parkedTask.ApplyWorkflowState(models.StateBacklog)

	plain := newProject("plain", "Plain", plainTask)
	parked := newProject("parked", "Parked", parkedTask)
	parked.Tags = []string{models.ParkedProjectTag}

	planning := config.NewPlanningState()
	planning.Next = []config.ProjectTaskLocator{
		{ProjectID: plain.ID, TaskID: plainTask.ID},
		{ProjectID: parked.ID, TaskID: parkedTask.ID},
		{ProjectID: parked.ID, TaskID: "missing"},
	}
	service := newTestService(newFakeRepo(plain, parked)).
		WithPlanningArtifacts(memoryPlanningBuilder(planning))

	changed, err := service.ReconcilePlanning()
	if err != nil {
		t.Fatalf("ReconcilePlanning: %v", err)
	}
	if !changed {
		t.Fatal("ReconcilePlanning reported no change for a dangling entry")
	}
	want := []string{plainTask.ID, parkedTask.ID}
	if got := planningTaskIDs(planning.GetNext()); !reflect.DeepEqual(got, want) {
		t.Fatalf("reconciled Next = %v, want %v", got, want)
	}
}
