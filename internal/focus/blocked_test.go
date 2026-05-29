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

func TestCollectActionableTasks_CrossProjectDependencies(t *testing.T) {
	depProject, err := models.NewProject(uuid.New().String(), "Dependencies", "")
	if err != nil {
		t.Fatal(err)
	}
	targetProject, err := models.NewProject(uuid.New().String(), "Target", "")
	if err != nil {
		t.Fatal(err)
	}

	doneDep, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Done dependency")
	if err != nil {
		t.Fatal(err)
	}
	doneDep.Status = models.StatusDone

	openDep, err := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Open dependency")
	if err != nil {
		t.Fatal(err)
	}

	unblocked, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Unblocked")
	if err != nil {
		t.Fatal(err)
	}
	unblocked.DependsOn = []string{depProject.ID + ":" + doneDep.ID}

	blockedByOpen, err := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Blocked by open")
	if err != nil {
		t.Fatal(err)
	}
	blockedByOpen.DependsOn = []string{depProject.ID + ":" + openDep.ID}

	blockedByMissing, err := models.NewTask(uuid.New().String(), 3, models.TaskTypeTask, "Blocked by missing")
	if err != nil {
		t.Fatal(err)
	}
	blockedByMissing.DependsOn = []string{depProject.ID + ":missing"}

	depProject.Tasks = []*models.Task{doneDep, openDep}
	targetProject.Tasks = []*models.Task{unblocked, blockedByOpen, blockedByMissing}

	out := CollectActionableTasks([]*models.Project{depProject, targetProject})
	got := make(map[string]bool)
	for _, ft := range out {
		got[ft.Task.ID] = true
	}

	if !got[unblocked.ID] {
		t.Fatal("task with completed cross-project dependency should be actionable")
	}
	if got[blockedByOpen.ID] {
		t.Fatal("task with incomplete cross-project dependency should not be actionable")
	}
	if got[blockedByMissing.ID] {
		t.Fatal("task with unresolved cross-project dependency should not be actionable")
	}
}

func TestCollectActionableTasks_MissingLocalDependencyBlocksTask(t *testing.T) {
	proj, err := models.NewProject(uuid.New().String(), "P", "")
	if err != nil {
		t.Fatal(err)
	}

	blockedByMissing, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Blocked by missing")
	if err != nil {
		t.Fatal(err)
	}
	blockedByMissing.DependsOn = []string{"missing"}

	actionable, err := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Actionable")
	if err != nil {
		t.Fatal(err)
	}

	proj.Tasks = []*models.Task{blockedByMissing, actionable}

	out := CollectActionableTasks([]*models.Project{proj})
	if len(out) != 1 {
		t.Fatalf("expected 1 actionable task, got %d: %+v", len(out), out)
	}
	if out[0].Task.ID != actionable.ID {
		t.Fatalf("expected task without missing dependency, got %q", out[0].Task.Title)
	}
}
