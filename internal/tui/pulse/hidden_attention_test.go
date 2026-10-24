package pulse

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// TestHiddenWorkIsNeverCountedOrArrived holds both attention readers to the
// hidden rule: a flagged task and the child it hides by inheritance contribute
// to no pulse component and raise no arrival.
func TestHiddenWorkIsNeverCountedOrArrived(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	overdue := now.Add(-2 * time.Hour)
	parent := &models.Task{ID: "parent", Title: "Parent", WorkflowState: models.StateInbox, DueDate: &overdue}
	child := &models.Task{ID: "child", Title: "Child", WorkflowState: models.StateInbox, DueDate: &overdue, ParentID: &parent.ID}
	project := &models.Project{ID: "p", Slug: "p", Tasks: []*models.Task{parent, child}}
	project.BuildTaskTree()
	today := map[taskref.TaskIdentity]bool{taskref.For(parent, project): true}

	if got := ComputeWorkspacePulse([]*models.Project{project}, today, now); got.Inbox != 2 || got.Overdue != 2 {
		t.Fatalf("visible fixture pulse = %+v, want two inbox and two overdue", got)
	}
	if got := len(Arrivals([]*models.Project{project}, now.Add(-3*time.Hour), now)); got != 2 {
		t.Fatalf("visible fixture arrivals = %d, want 2", got)
	}
	parent.Hidden = true
	if got := ComputeWorkspacePulse([]*models.Project{project}, today, now); got != (WorkspacePulse{}) {
		t.Fatalf("hidden work counted in the pulse: %+v", got)
	}
	if got := Arrivals([]*models.Project{project}, now.Add(-3*time.Hour), now); len(got) != 0 {
		t.Fatalf("hidden work arrived: %+v", got)
	}
}
