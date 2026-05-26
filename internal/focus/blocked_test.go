package focus

import (
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

// TestCollectActionableTasks_BlockedExcluded verifies that tasks with
// StatusBlocked are excluded from actionable results regardless of whether
// they have incomplete dependencies.
func TestCollectActionableTasks_BlockedExcluded(t *testing.T) {
	proj, err := models.NewProject(uuid.New().String(), "P", "")
	if err != nil {
		t.Fatal(err)
	}

	// Blocked task with no deps — should be excluded.
	blockedNoDeps, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Blocked no deps")
	if err != nil {
		t.Fatal(err)
	}
	blockedNoDeps.Status = models.StatusBlocked

	// Blocked task that also has a completed dependency — should still be excluded.
	dep, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Completed dep")
	if err != nil {
		t.Fatal(err)
	}
	dep.Status = models.StatusDone

	blockedWithDoneDep, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Blocked with done dep")
	if err != nil {
		t.Fatal(err)
	}
	blockedWithDoneDep.Status = models.StatusBlocked
	blockedWithDoneDep.DependsOn = []string{dep.ID}

	// Normal todo task — should be included.
	actionable, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Actionable")
	if err != nil {
		t.Fatal(err)
	}
	actionable.Status = models.StatusTodo

	proj.Tasks = []*models.Task{dep, blockedNoDeps, blockedWithDoneDep, actionable}

	out := CollectActionableTasks([]*models.Project{proj})
	if len(out) != 1 {
		t.Fatalf("expected 1 actionable task, got %d: %+v", len(out), out)
	}
	if out[0].Task.ID != actionable.ID {
		t.Errorf("expected the actionable todo task, got %+v", out[0].Task)
	}
}
