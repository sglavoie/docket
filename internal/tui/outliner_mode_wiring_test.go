package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestOpenChildCountsCensusesEveryDepth(t *testing.T) {
	rootID := "root"
	midID := "mid"
	leafID := "leaf"
	closedID := "closed-parent"
	tasks := []*models.Task{
		{ID: rootID, Title: "Root"},
		{ID: midID, Title: "Mid", ParentID: &rootID},
		{ID: leafID, Title: "Leaf", ParentID: &midID},
		{ID: "grandchild", Title: "Grandchild", ParentID: &leafID},
		{ID: "done", Title: "Done", WorkflowState: models.StateDone, ParentID: &rootID},
		{ID: "canceled", Title: "Canceled", WorkflowState: models.StateCanceled, ParentID: &midID},
		{ID: closedID, Title: "Closed parent"},
		{ID: "closed-child", Title: "Closed child", WorkflowState: models.StateDone, ParentID: &closedID},
	}
	project := renderFixtureProject(t, "project", "project", "Project", tasks)

	counts := openChildCounts([]*models.Project{project})

	want := map[string]int{rootID: 1, midID: 1, leafID: 1}
	for id, expected := range want {
		got := counts[TaskIdentity{ProjectID: project.ID, TaskID: id}]
		if got != expected {
			t.Fatalf("open children of %q = %d, want %d", id, got, expected)
		}
	}
	if len(counts) != len(want) {
		t.Fatalf("census recorded %d tasks, want %d — leaves and all-terminal parents must be absent", len(counts), len(want))
	}
}

func TestOpenChildCountsSkipsNilProjects(t *testing.T) {
	if counts := openChildCounts([]*models.Project{nil}); len(counts) != 0 {
		t.Fatalf("nil project produced %d counts", len(counts))
	}
}
