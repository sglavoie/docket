package app_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// parentWithChild builds a parent task and a child pointing at it (both leaf
// task types, matching the harness's built-tree semantics), returning the
// project seeded with both plus the live pointers.
func parentWithChild(childStatus models.WorkflowState) *models.Project {
	parent := newTask("p", 1, "parent")
	child := newTask("c", 2, "child")
	pid := "p"
	child.ParentID = &pid
	child.WorkflowState = childStatus
	if childStatus.IsTerminal() {
		child.Complete()
	}
	project := newProject("proj-1", "Alpha", parent, child)
	return project
}

func TestTransitionCompleteBlockedByIncompleteChild(t *testing.T) {
	project := parentWithChild(models.StateBacklog)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.Transition(app.StateRequest{Locator: repo.loc("p"), Target: models.StateDone})
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

// TestTransitionBlockedByAHiddenChildSaysItIsHidden names the one blocker the
// parent's own rows cannot show, so the refusal explains itself.
func TestTransitionBlockedByAHiddenChildSaysItIsHidden(t *testing.T) {
	project := parentWithChild(models.StateBacklog)
	project.GetTaskByID("c").Hidden = true
	repo := newFakeRepo(project)
	_, err := newTestService(repo).Transition(app.StateRequest{Locator: repo.loc("p"), Target: models.StateDone})
	var blocked *app.ChildrenBlockedError
	if !errors.As(err, &blocked) || len(blocked.Children) != 1 || !blocked.Children[0].Hidden {
		t.Fatalf("err = %v, want the blocking child marked hidden", err)
	}
	if !strings.Contains(err.Error(), "#2 child (backlog, hidden)") {
		t.Fatalf("refusal = %q, want it to name the child as hidden", err.Error())
	}
}

func TestTransitionCancelBlockedByIncompleteChild(t *testing.T) {
	project := parentWithChild(models.StateBacklog)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.Transition(app.StateRequest{Locator: repo.loc("p"), Target: models.StateCanceled})
	if !errors.Is(err, app.ErrParentHasIncompleteChildren) {
		t.Fatalf("err = %v, want ErrParentHasIncompleteChildren", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("blocked cancellation committed %d times, want 0", len(repo.commits))
	}
}

func TestTransitionCompleteAllowedWhenChildrenComplete(t *testing.T) {
	project := parentWithChild(models.StateDone)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.Transition(app.StateRequest{Locator: repo.loc("p"), Target: models.StateDone})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if task := project.GetTaskByID("p"); task == nil || task.State() != models.StateDone {
		t.Errorf("parent state = %v, want done", task)
	}
}

func TestTransitionNonterminalNotGatedByChildren(t *testing.T) {
	project := parentWithChild(models.StateBacklog)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	// Moving a parent with incomplete children to a nonterminal state is allowed;
	// only completion and cancellation are gated.
	if _, err := svc.Transition(app.StateRequest{Locator: repo.loc("p"), Target: models.StateDoing}); err != nil {
		t.Fatalf("nonterminal transition should not be gated: %v", err)
	}
}

func TestBulkTransitionCompleteBlockedByIncompleteChild(t *testing.T) {
	project := parentWithChild(models.StateBacklog)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.BulkTransition([]app.TaskLocator{repo.loc("p")}, models.StateDone)
	if !errors.Is(err, app.ErrParentHasIncompleteChildren) {
		t.Fatalf("err = %v, want ErrParentHasIncompleteChildren", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("blocked bulk completion committed %d times, want 0", len(repo.commits))
	}
}
