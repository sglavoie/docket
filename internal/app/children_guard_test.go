package app_test

import (
	"errors"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// parentWithChild builds a parent task and a child pointing at it (both leaf
// task types, matching the harness's built-tree semantics), returning the
// project seeded with both plus the live pointers.
func parentWithChild(childStatus models.TaskStatus) (*models.Project, *models.Task, *models.Task) {
	parent := newTask("p", 1, "parent")
	child := newTask("c", 2, "child")
	pid := "p"
	child.ParentID = &pid
	child.WorkflowState = models.DeriveWorkflowState(childStatus, "", "")
	if childStatus.IsComplete() {
		child.Complete()
	}
	project := newProject("proj-1", "Alpha", parent, child)
	return project, parent, child
}

func TestTransitionCompleteBlockedByIncompleteChild(t *testing.T) {
	project, _, _ := parentWithChild(models.StatusTodo)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.Complete("p")
	if !errors.Is(err, app.ErrParentHasIncompleteChildren) {
		t.Fatalf("err = %v, want ErrParentHasIncompleteChildren", err)
	}
	var blocked *app.ChildrenBlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("err = %v, want *ChildrenBlockedError", err)
	}
	if blocked.ParentShortID != 1 || len(blocked.Children) != 1 || blocked.Children[0].ShortID != 2 {
		t.Errorf("blocked detail = %+v, want parent #1 with child #2", blocked)
	}
	if len(repo.commits) != 0 {
		t.Errorf("blocked completion committed %d times, want 0", len(repo.commits))
	}
}

func TestTransitionCancelBlockedByIncompleteChild(t *testing.T) {
	project, _, _ := parentWithChild(models.StatusTodo)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.Transition(app.StateRequest{TaskID: "p", Target: models.StateCanceled})
	if !errors.Is(err, app.ErrParentHasIncompleteChildren) {
		t.Fatalf("err = %v, want ErrParentHasIncompleteChildren", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("blocked cancellation committed %d times, want 0", len(repo.commits))
	}
}

func TestTransitionCompleteAllowedWhenChildrenComplete(t *testing.T) {
	project, _, _ := parentWithChild(models.StatusDone)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	res, err := svc.Complete("p")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if res.State != models.StateDone {
		t.Errorf("State = %q, want done", res.State)
	}
}

func TestTransitionNonterminalNotGatedByChildren(t *testing.T) {
	project, _, _ := parentWithChild(models.StatusTodo)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	// Moving a parent with incomplete children to a nonterminal state is allowed;
	// only completion and cancellation are gated.
	if _, err := svc.Transition(app.StateRequest{TaskID: "p", Target: models.StateInProgress}); err != nil {
		t.Fatalf("nonterminal transition should not be gated: %v", err)
	}
}

func TestBulkTransitionCompleteBlockedByIncompleteChild(t *testing.T) {
	project, _, _ := parentWithChild(models.StatusTodo)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.BulkTransition([]string{"p"}, models.StateDone)
	if !errors.Is(err, app.ErrParentHasIncompleteChildren) {
		t.Fatalf("err = %v, want ErrParentHasIncompleteChildren", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("blocked bulk completion committed %d times, want 0", len(repo.commits))
	}
}
