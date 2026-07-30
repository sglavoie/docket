package tui

import (
	"reflect"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestGroupTaskProjectRefsByProjectStableOrderAndNilFiltering(t *testing.T) {
	projectA := mustBulkRefProject(t, "project-a", "Project A")
	projectB := mustBulkRefProject(t, "project-b", "Project B")
	task1 := mustBulkRefTask(t, "task-1", 1)
	task2 := mustBulkRefTask(t, "task-2", 2)
	task3 := mustBulkRefTask(t, "task-3", 3)
	task4 := mustBulkRefTask(t, "task-4", 4)
	task5 := mustBulkRefTask(t, "task-5", 5)

	groups := groupTaskProjectRefsByProject([]TaskProjectRef{
		{Task: task1, Project: projectA},
		{Task: nil, Project: projectA},
		{Task: task2, Project: projectB},
		{Task: task3, Project: projectA},
		{Task: task4, Project: nil},
		{Task: task5, Project: projectB},
	})

	if len(groups) != 2 {
		t.Fatalf("expected 2 project groups, got %d", len(groups))
	}
	if groups[0].Project != projectA {
		t.Fatalf("first group project = %v, want projectA", groups[0].Project)
	}
	if groups[1].Project != projectB {
		t.Fatalf("second group project = %v, want projectB", groups[1].Project)
	}

	gotA := []string{groups[0].Items[0].Task.ID, groups[0].Items[1].Task.ID}
	if want := []string{"task-1", "task-3"}; !reflect.DeepEqual(gotA, want) {
		t.Fatalf("projectA IDs = %v, want %v", gotA, want)
	}
	gotB := []string{groups[1].Items[0].Task.ID, groups[1].Items[1].Task.ID}
	if want := []string{"task-2", "task-5"}; !reflect.DeepEqual(gotB, want) {
		t.Fatalf("projectB IDs = %v, want %v", gotB, want)
	}
}

func TestTaskIDsByProjectFromTaskRefs(t *testing.T) {
	projectA := mustBulkRefProject(t, "project-a", "Project A")
	projectB := mustBulkRefProject(t, "project-b", "Project B")

	idsByProject := taskIDsByProjectFromTaskRefs([]TaskProjectRef{
		{Task: mustBulkRefTask(t, "task-1", 1), Project: projectA},
		{Task: mustBulkRefTask(t, "task-2", 2), Project: projectB},
		{Task: mustBulkRefTask(t, "task-3", 3), Project: projectA},
		{Task: mustBulkRefTask(t, "", 4), Project: projectA},
		{Task: nil, Project: projectB},
		{Task: mustBulkRefTask(t, "task-4", 5), Project: nil},
	})

	if want := []string{"task-1", "task-3"}; !reflect.DeepEqual(idsByProject[projectA.ID], want) {
		t.Fatalf("projectA IDs = %v, want %v", idsByProject[projectA.ID], want)
	}
	if want := []string{"task-2"}; !reflect.DeepEqual(idsByProject[projectB.ID], want) {
		t.Fatalf("projectB IDs = %v, want %v", idsByProject[projectB.ID], want)
	}
}

func TestAffectedProjectsFromTaskRefsStableOrder(t *testing.T) {
	projectA := mustBulkRefProject(t, "project-a", "Project A")
	projectB := mustBulkRefProject(t, "project-b", "Project B")

	projects := affectedProjectsFromTaskRefs([]TaskProjectRef{
		{Task: mustBulkRefTask(t, "task-1", 1), Project: projectB},
		{Task: mustBulkRefTask(t, "task-2", 2), Project: projectA},
		{Task: mustBulkRefTask(t, "task-3", 3), Project: projectB},
		{Task: nil, Project: projectA},
	})

	if len(projects) != 2 {
		t.Fatalf("expected 2 affected projects, got %d", len(projects))
	}
	if projects[0] != projectB || projects[1] != projectA {
		t.Fatalf("projects = %#v, want [projectB projectA]", projects)
	}
}

func TestFinishTaskBulkSelection(t *testing.T) {
	status := ""
	state := &MultiSelectState{
		multiSelectMode: true,
		selectedTaskIdentities: map[TaskIdentity]bool{
			{ProjectID: "project", TaskID: "task-1"}: true,
			{ProjectID: "project", TaskID: "task-2"}: true,
		},
	}

	finishTaskBulkSelection(state, &status, "Changed 2 tasks")

	if state.multiSelectMode {
		t.Fatal("expected multi-select mode to be disabled")
	}
	if state.SelectedCount() != 0 {
		t.Fatalf("expected selection to be cleared, got %d selections", state.SelectedCount())
	}
	if status != "Changed 2 tasks" {
		t.Fatalf("status = %q, want %q", status, "Changed 2 tasks")
	}
}

func mustBulkRefProject(t *testing.T, id, name string) *models.Project {
	t.Helper()
	project, err := models.NewProject(id, name, "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	return project
}

func mustBulkRefTask(t *testing.T, id string, shortID int) *models.Task {
	t.Helper()
	task, err := models.NewGenericTask(id, shortID, "Task "+id)
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	return task
}
