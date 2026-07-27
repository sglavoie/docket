package app_test

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// rootOrder reads the root-task IDs in slice order.
func rootOrder(p *models.Project) []string {
	var ids []string
	for _, t := range p.Tasks {
		if t.IsRoot() {
			ids = append(ids, t.ID)
		}
	}
	return ids
}

func assertOrder(t *testing.T, p *models.Project, want ...string) {
	t.Helper()
	got := rootOrder(p)
	if len(got) != len(want) {
		t.Fatalf("root order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("root order = %v, want %v", got, want)
		}
	}
}

// TestMoveTaskToTopBubblesToFront proves MoveTaskToTop moves a task to the top of
// its siblings across multiple positions.
func TestMoveTaskToTopBubblesToFront(t *testing.T) {
	a := newTask("a", 1, "A")
	b := newTask("b", 2, "B")
	c := newTask("c", 3, "C")
	proj := newProject("proj-a", "Alpha", a, b, c)
	repo := newFakeRepo(proj)
	svc := newTestService(repo)

	if _, err := svc.MoveTaskToTop("c"); err != nil {
		t.Fatalf("MoveTaskToTop: %v", err)
	}
	assertOrder(t, proj, "c", "a", "b")
}

// TestMoveTaskToBottomBubblesToEnd proves MoveTaskToBottom moves a task to the
// bottom of its siblings across multiple positions.
func TestMoveTaskToBottomBubblesToEnd(t *testing.T) {
	a := newTask("a", 1, "A")
	b := newTask("b", 2, "B")
	c := newTask("c", 3, "C")
	proj := newProject("proj-a", "Alpha", a, b, c)
	repo := newFakeRepo(proj)
	svc := newTestService(repo)

	if _, err := svc.MoveTaskToBottom("a"); err != nil {
		t.Fatalf("MoveTaskToBottom: %v", err)
	}
	assertOrder(t, proj, "b", "c", "a")
}

// TestMoveTaskToTopAlreadyAtTopIsIdempotent proves a task already at the top
// commits unchanged rather than erroring (unlike MoveTaskUp, which rejects it).
func TestMoveTaskToTopAlreadyAtTopIsIdempotent(t *testing.T) {
	a := newTask("a", 1, "A")
	b := newTask("b", 2, "B")
	proj := newProject("proj-a", "Alpha", a, b)
	repo := newFakeRepo(proj)
	svc := newTestService(repo)

	if _, err := svc.MoveTaskToTop("a"); err != nil {
		t.Fatalf("MoveTaskToTop (already at top): %v", err)
	}
	assertOrder(t, proj, "a", "b")
	if len(repo.commits) != 1 {
		t.Errorf("commits = %d, want 1 (idempotent still commits once)", len(repo.commits))
	}
}

// TestMoveTaskToTopStopsAtCompletionBoundary proves a completed root task cannot
// bubble above an uncompleted root sibling.
func TestMoveTaskToTopStopsAtCompletionBoundary(t *testing.T) {
	a := newTask("a", 1, "A")
	b := newTask("b", 2, "B")
	b.Complete()
	proj := newProject("proj-a", "Alpha", a, b)
	repo := newFakeRepo(proj)
	svc := newTestService(repo)

	if _, err := svc.MoveTaskToTop("b"); err != nil {
		t.Fatalf("MoveTaskToTop: %v", err)
	}
	// b (completed) cannot cross above a (uncompleted): order stays a, b.
	assertOrder(t, proj, "a", "b")
}
