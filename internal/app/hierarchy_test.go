package app_test

import (
	"errors"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// typedTask builds a task of a specific type for hierarchy tests.
func typedTask(id string, shortID int, title string) *models.Task {
	tk, err := models.NewGenericTask(id, shortID, title)
	if err != nil {
		panic(err)
	}
	return tk
}

func assertTaskIDs(t *testing.T, tasks []*models.Task, want ...string) {
	t.Helper()
	if len(tasks) != len(want) {
		t.Fatalf("task count = %d, want %d", len(tasks), len(want))
	}
	for i, id := range want {
		if tasks[i] == nil || tasks[i].ID != id {
			t.Fatalf("task[%d] = %v, want %q", i, tasks[i], id)
		}
	}
}

func TestReparentUnderValidParent(t *testing.T) {
	epic := typedTask("epic1", 1, "Epic")
	task := typedTask("t1", 2, "child task")
	projA := newProject("proj-a", "Alpha", epic, task)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	parentID := "epic1"
	if _, err := svc.Reparent("t1", &parentID); err != nil {
		t.Fatalf("Reparent: %v", err)
	}
	if task.ParentID == nil || *task.ParentID != "epic1" {
		t.Fatalf("ParentID = %v, want epic1", task.ParentID)
	}
	if task.Parent == nil || task.Parent.ID != "epic1" {
		t.Errorf("tree not rebuilt: Parent = %v", task.Parent)
	}
}

func TestReparentToRootClearsParent(t *testing.T) {
	epic := typedTask("epic1", 1, "Epic")
	task := typedTask("t1", 2, "child task")
	parentID := "epic1"
	task.ParentID = &parentID
	projA := newProject("proj-a", "Alpha", epic, task)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	if _, err := svc.Reparent("t1", nil); err != nil {
		t.Fatalf("Reparent to root: %v", err)
	}
	if task.ParentID != nil {
		t.Errorf("ParentID = %v, want nil (root)", task.ParentID)
	}
}

func TestReparentUsesLeanHierarchyWithoutTypeRestrictions(t *testing.T) {
	leaf := typedTask("leaf1", 1, "leaf")
	task := typedTask("t1", 2, "another leaf")
	projA := newProject("proj-a", "Alpha", leaf, task)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	parentID := "leaf1"
	if _, err := svc.Reparent("t1", &parentID); err != nil {
		t.Fatalf("Reparent under generic task: %v", err)
	}
	if task.ParentID == nil || *task.ParentID != leaf.ID {
		t.Fatalf("ParentID = %v, want %s", task.ParentID, leaf.ID)
	}
	if len(repo.commits) != 1 {
		t.Errorf("reparent committed %d times, want 1", len(repo.commits))
	}
}

func TestReparentRejectsCycle(t *testing.T) {
	epic := typedTask("epic1", 1, "Epic")
	story := typedTask("story1", 2, "Story")
	epicID := "epic1"
	story.ParentID = &epicID // story is a child of epic
	projA := newProject("proj-a", "Alpha", epic, story)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	// Making epic a child of its own descendant would create a cycle.
	if _, err := svc.Reparent("epic1", &[]string{"story1"}[0]); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput (cycle)", err)
	}
}

func TestReparentUnknownParent(t *testing.T) {
	task := typedTask("t1", 1, "task")
	repo := newFakeRepo(newProject("proj-a", "Alpha", task))
	svc := newTestService(repo)

	ghost := "no-such-parent"
	if _, err := svc.Reparent("t1", &ghost); !errors.Is(err, app.ErrTaskNotFound) {
		t.Fatalf("err = %v, want ErrTaskNotFound", err)
	}
}

func TestReparentRejectsSubtreeThatWouldExceedDepth(t *testing.T) {
	var chain []*models.Task
	for i := 0; i < models.MaxNestingDepth; i++ {
		task := typedTask(string(rune('a'+i)), i+1, "Ancestor")
		if i > 0 {
			parentID := chain[i-1].ID
			task.ParentID = &parentID
		}
		chain = append(chain, task)
	}
	moved := typedTask("moved", 20, "Moved")
	child := typedTask("child", 21, "Child")
	child.ParentID = &moved.ID
	project := newProject("proj-a", "Alpha", append(chain, moved, child)...)
	repo := newFakeRepo(project)
	service := newTestService(repo)

	parentID := chain[len(chain)-1].ID
	if _, err := service.Reparent("moved", &parentID); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("Reparent error = %v, want ErrInvalidInput", err)
	}
	if moved.ParentID != nil {
		t.Fatalf("rejected reparent changed parent to %v", moved.ParentID)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("rejected reparent committed %d times", len(repo.commits))
	}
}

func TestMoveTaskUpReordersSiblings(t *testing.T) {
	first := newTask("t1", 1, "first")
	second := newTask("t2", 2, "second")
	projA := newProject("proj-a", "Alpha", first, second)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	if _, err := svc.MoveTaskUp("t2"); err != nil {
		t.Fatalf("MoveTaskUp: %v", err)
	}
	if projA.Tasks[0].ID != "t2" || projA.Tasks[1].ID != "t1" {
		t.Fatalf("order = [%s %s], want [t2 t1]", projA.Tasks[0].ID, projA.Tasks[1].ID)
	}
	if len(repo.commits) != 1 {
		t.Fatalf("Commit called %d times, want 1", len(repo.commits))
	}
}

func TestMoveTaskUpAtTopRejected(t *testing.T) {
	first := newTask("t1", 1, "first")
	second := newTask("t2", 2, "second")
	projA := newProject("proj-a", "Alpha", first, second)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	if _, err := svc.MoveTaskUp("t1"); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("rejected move committed %d times, want 0", len(repo.commits))
	}
}

func TestMoveTaskDownReordersSiblings(t *testing.T) {
	first := newTask("t1", 1, "first")
	second := newTask("t2", 2, "second")
	projA := newProject("proj-a", "Alpha", first, second)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	if _, err := svc.MoveTaskDown("t1"); err != nil {
		t.Fatalf("MoveTaskDown: %v", err)
	}
	if projA.Tasks[0].ID != "t2" || projA.Tasks[1].ID != "t1" {
		t.Fatalf("order = [%s %s], want [t2 t1]", projA.Tasks[0].ID, projA.Tasks[1].ID)
	}
}

func TestMoveTaskDownAtBottomRejected(t *testing.T) {
	first := newTask("t1", 1, "first")
	second := newTask("t2", 2, "second")
	projA := newProject("proj-a", "Alpha", first, second)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	if _, err := svc.MoveTaskDown("t2"); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("rejected move committed %d times, want 0", len(repo.commits))
	}
}

func TestReorderSiblingTasksUsesExplicitVisibleTarget(t *testing.T) {
	parentA := typedTask("parent-a", 1, "Parent A")
	parentB := typedTask("parent-b", 2, "Parent B")
	a1 := newTask("a1", 3, "A1")
	a2 := newTask("a2", 4, "A2")
	b1 := newTask("b1", 5, "B1")
	b2 := newTask("b2", 6, "B2")
	a1.ParentID = &parentA.ID
	a2.ParentID = &parentA.ID
	b1.ParentID = &parentB.ID
	b2.ParentID = &parentB.ID
	project := newProject("proj-a", "Alpha", parentA, a1, parentB, b1, a2, b2)
	repo := newFakeRepo(project)
	service := newTestService(repo)

	if _, err := service.ReorderSiblingTasks("a1", "a2"); err != nil {
		t.Fatalf("ReorderSiblingTasks: %v", err)
	}
	assertTaskIDs(t, project.GetSiblings(a1), "a2", "a1")
	assertTaskIDs(t, project.GetSiblings(b1), "b1", "b2")
	if len(repo.commits) != 1 {
		t.Fatalf("commits = %d, want 1", len(repo.commits))
	}
}

func TestReorderSiblingTasksRejectsDifferentParents(t *testing.T) {
	parentA := typedTask("parent-a", 1, "Parent A")
	parentB := typedTask("parent-b", 2, "Parent B")
	a1 := newTask("a1", 3, "A1")
	b1 := newTask("b1", 4, "B1")
	a1.ParentID = &parentA.ID
	b1.ParentID = &parentB.ID
	project := newProject("proj-a", "Alpha", parentA, a1, parentB, b1)
	repo := newFakeRepo(project)
	service := newTestService(repo)

	if _, err := service.ReorderSiblingTasks("a1", "b1"); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("ReorderSiblingTasks error = %v, want ErrInvalidInput", err)
	}
	assertTaskIDs(t, project.GetSiblings(a1), "a1")
	assertTaskIDs(t, project.GetSiblings(b1), "b1")
	if len(repo.commits) != 0 {
		t.Fatalf("rejected reorder committed %d times", len(repo.commits))
	}
}

func TestHierarchyReorderRollsBackSharedSnapshotOnCommitFailure(t *testing.T) {
	first := newTask("t1", 1, "first")
	second := newTask("t2", 2, "second")
	project := newProject("proj-a", "Alpha", first, second)
	repo := newFakeRepo(project)
	repo.commitErr = errors.New("commit boom")
	service := newTestService(repo)

	if _, err := service.MoveTaskDown("t1"); err == nil {
		t.Fatal("expected commit error")
	}
	assertTaskIDs(t, project.Tasks, "t1", "t2")
	if project.Tasks[0] != first || project.Tasks[1] != second {
		t.Fatal("rollback replaced repository-owned task pointers")
	}
}

func TestIndentNestsUnderTaskAbove(t *testing.T) {
	epic := typedTask("epic1", 1, "Epic")
	task := typedTask("t1", 2, "leaf")
	projA := newProject("proj-a", "Alpha", epic, task)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	if _, err := svc.IndentTask("t1"); err != nil {
		t.Fatalf("IndentTask: %v", err)
	}
	if task.ParentID == nil || *task.ParentID != "epic1" {
		t.Fatalf("ParentID = %v, want epic1", task.ParentID)
	}
}

func TestIndentRejectedWhenNoTaskAbove(t *testing.T) {
	task := typedTask("t1", 1, "only task")
	projA := newProject("proj-a", "Alpha", task)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	if _, err := svc.IndentTask("t1"); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("rejected indent committed %d times, want 0", len(repo.commits))
	}
}

func TestOutdentMovesToParentLevel(t *testing.T) {
	epic := typedTask("epic1", 1, "Epic")
	story := typedTask("story1", 2, "Story")
	epicID := "epic1"
	story.ParentID = &epicID
	projA := newProject("proj-a", "Alpha", epic, story)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	if _, err := svc.OutdentTask("story1"); err != nil {
		t.Fatalf("OutdentTask: %v", err)
	}
	if story.ParentID != nil {
		t.Fatalf("ParentID = %v, want nil (root)", story.ParentID)
	}
}

func TestOutdentRejectedAtRoot(t *testing.T) {
	task := typedTask("t1", 1, "root task")
	projA := newProject("proj-a", "Alpha", task)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	if _, err := svc.OutdentTask("t1"); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}
