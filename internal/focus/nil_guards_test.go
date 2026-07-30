package focus

import (
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

// TestCollectActionableTasksSkipsNilProject guards against nil projects or
// projects containing nil tasks. Loosely-assembled slices fed in from CLI
// layers used to panic on the nil-deref.
func TestCollectActionableTasksSkipsNilProject(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic on nil project/task: %v", r)
		}
	}()

	good, err := models.NewProject(uuid.New().String(), "Good", "")
	if err != nil {
		t.Fatal(err)
	}
	task, err := models.NewGenericTask(uuid.New().String(), 1, "T")
	if err != nil {
		t.Fatal(err)
	}
	good.Tasks = []*models.Task{task, nil}

	out := CollectActionableTasks([]*models.Project{nil, good, nil})
	if len(out) != 1 {
		t.Fatalf("expected 1 actionable task, got %d", len(out))
	}
	if out[0].Task != task {
		t.Errorf("expected the one good task, got %+v", out[0])
	}
}

// TestCollectActionableTasksSkipsNilProjectTasksDuringDependencyChecks guards
// against nil entries in Project.Tasks while dependency checks are resolving
// blockers. Focus accepts loosely-assembled project slices and should skip nil
// task entries instead of panicking inside model lookup helpers.
func TestCollectActionableTasksSkipsNilProjectTasksDuringDependencyChecks(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic on nil project task during dependency check: %v", r)
		}
	}()

	project, err := models.NewProject(uuid.New().String(), "P", "")
	if err != nil {
		t.Fatal(err)
	}
	blocker, err := models.NewGenericTask(uuid.New().String(), 1, "Blocker")
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := models.NewGenericTask(uuid.New().String(), 2, "Blocked")
	if err != nil {
		t.Fatal(err)
	}
	blocked.DependsOn = []string{blocker.ID}
	actionable, err := models.NewGenericTask(uuid.New().String(), 3, "Actionable")
	if err != nil {
		t.Fatal(err)
	}

	project.Tasks = []*models.Task{nil, blocker, blocked, actionable}

	out := CollectActionableTasks([]*models.Project{project})
	if len(out) != 2 {
		t.Fatalf("expected blocker and actionable tasks, got %d: %+v", len(out), out)
	}
	for _, ft := range out {
		if ft.Task == blocked {
			t.Fatal("blocked task with incomplete dependency should not be actionable")
		}
	}
}
