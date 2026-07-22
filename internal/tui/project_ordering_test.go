package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestSortProjectsByDisplayOrderIgnoresPinnedAndNormalizesName(t *testing.T) {
	projects := []*models.Project{
		projectForDisplayOrder("pinned-zeta", "zeta", "  zeta", true),
		projectForDisplayOrder("plain-alpha", "alpha", "Alpha", false),
		projectForDisplayOrder("pinned-beta", "beta", "BETA  ", true),
	}

	sortProjectsByDisplayOrder(projects)

	assertProjectIDs(t, projects, []string{"plain-alpha", "pinned-beta", "pinned-zeta"})
}

func TestSortProjectsByDisplayOrderFallbackKeys(t *testing.T) {
	projects := []*models.Project{
		projectForDisplayOrder("id-c", "same-b", "Alpha", false),
		projectForDisplayOrder("id-b", "same-a", " alpha ", false),
		projectForDisplayOrder("id-a", "same-a", "ALPHA", false),
	}

	sortProjectsByDisplayOrder(projects)

	assertProjectIDs(t, projects, []string{"id-a", "id-b", "id-c"})
}

func TestSortProjectsByDisplayOrderNilProjectsLast(t *testing.T) {
	alpha := projectForDisplayOrder("alpha", "alpha", "Alpha", false)
	beta := projectForDisplayOrder("beta", "beta", "Beta", false)
	projects := []*models.Project{nil, beta, alpha, nil}

	sortProjectsByDisplayOrder(projects)

	if projects[0] != alpha || projects[1] != beta {
		t.Fatalf("known projects sorted before nils: got %#v", projects)
	}
	if projects[2] != nil || projects[3] != nil {
		t.Fatalf("nil projects should sort last: got %#v", projects)
	}
}

func TestSortCrossProjectTasksByProjectDisplayOrderPreservesSameProjectTaskOrder(t *testing.T) {
	alpha := projectForDisplayOrder("alpha", "alpha", "Alpha", false)
	beta := projectForDisplayOrder("beta", "beta", "Beta", true)
	tasks := []CrossProjectTask{
		{Task: taskForDisplayOrder("beta-1"), Project: beta},
		{Task: taskForDisplayOrder("alpha-1"), Project: alpha},
		{Task: taskForDisplayOrder("alpha-2"), Project: alpha},
		{Task: taskForDisplayOrder("nil-1"), Project: nil},
		{Task: taskForDisplayOrder("beta-2"), Project: beta},
	}

	sortCrossProjectTasksByProjectDisplayOrder(tasks)

	assertCrossProjectTaskIDs(t, tasks, []string{"alpha-1", "alpha-2", "beta-1", "beta-2", "nil-1"})
}

func TestSortProjectTaskGroupsByProjectDisplayOrderUsesLookupAndKeepsUnknownLast(t *testing.T) {
	alpha := projectForDisplayOrder("alpha", "alpha", "Alpha", false)
	beta := projectForDisplayOrder("beta", "beta", "Beta", false)
	groups := []projectTasksGroup{
		{projectID: "beta", tasks: []*models.Task{taskForDisplayOrder("beta-1")}},
		{projectID: "missing", tasks: []*models.Task{taskForDisplayOrder("missing-1")}},
		{projectID: "alpha", tasks: []*models.Task{taskForDisplayOrder("alpha-1")}},
		{projectID: "beta", tasks: []*models.Task{taskForDisplayOrder("beta-2")}},
	}

	sortProjectTaskGroupsByProjectDisplayOrder(groups, []*models.Project{nil, beta, alpha})

	assertProjectGroupIDs(t, groups, []string{"alpha", "beta", "beta", "missing"})
	assertProjectGroupFirstTaskIDs(t, groups, []string{"alpha-1", "beta-1", "beta-2", "missing-1"})
}

func projectForDisplayOrder(id, slug, name string, pinned bool) *models.Project {
	return &models.Project{
		ID:     id,
		Slug:   slug,
		Name:   name,
		Pinned: pinned,
	}
}

func taskForDisplayOrder(id string) *models.Task {
	return &models.Task{ID: id}
}

func assertProjectIDs(t *testing.T, projects []*models.Project, want []string) {
	t.Helper()
	if len(projects) != len(want) {
		t.Fatalf("got %d projects, want %d", len(projects), len(want))
	}
	for i, project := range projects {
		if project == nil {
			t.Fatalf("project[%d] is nil, want ID %q", i, want[i])
		}
		if project.ID != want[i] {
			t.Fatalf("project[%d] ID: got %q, want %q", i, project.ID, want[i])
		}
	}
}

func assertCrossProjectTaskIDs(t *testing.T, tasks []CrossProjectTask, want []string) {
	t.Helper()
	if len(tasks) != len(want) {
		t.Fatalf("got %d tasks, want %d", len(tasks), len(want))
	}
	for i, task := range tasks {
		if task.Task.ID != want[i] {
			t.Fatalf("task[%d] ID: got %q, want %q", i, task.Task.ID, want[i])
		}
	}
}

func assertProjectGroupIDs(t *testing.T, groups []projectTasksGroup, want []string) {
	t.Helper()
	if len(groups) != len(want) {
		t.Fatalf("got %d groups, want %d", len(groups), len(want))
	}
	for i, group := range groups {
		if group.projectID != want[i] {
			t.Fatalf("group[%d] projectID: got %q, want %q", i, group.projectID, want[i])
		}
	}
}

func assertProjectGroupFirstTaskIDs(t *testing.T, groups []projectTasksGroup, want []string) {
	t.Helper()
	if len(groups) != len(want) {
		t.Fatalf("got %d groups, want %d", len(groups), len(want))
	}
	for i, group := range groups {
		if len(group.tasks) == 0 {
			t.Fatalf("group[%d] has no tasks, want first task %q", i, want[i])
		}
		if group.tasks[0].ID != want[i] {
			t.Fatalf("group[%d] first task ID: got %q, want %q", i, group.tasks[0].ID, want[i])
		}
	}
}
