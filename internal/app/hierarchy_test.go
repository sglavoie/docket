package app_test

import (
	"errors"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
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

func TestMoveToProjectCarriesAttachmentOpsAndArtifact(t *testing.T) {
	epic := typedTask("epic1", 1, "Epic")
	child := typedTask("t1", 2, "child")
	epicID := "epic1"
	child.ParentID = &epicID
	projA := newProject("proj-a", "Alpha", epic, child)
	projB := newProject("proj-b", "Beta")
	repo := newFakeRepo(projA, projB)
	svc := newTestService(repo)

	artifact := storage.ArtifactChange{Path: "/data/.tt/planning.json", Data: []byte("{}")}
	res, err := svc.MoveToProject("epic1", "proj-b", artifact)
	if err != nil {
		t.Fatalf("MoveToProject: %v", err)
	}
	if res.MovedCount != 2 {
		t.Fatalf("MovedCount = %d, want 2", res.MovedCount)
	}

	if len(repo.commits) != 1 || len(repo.commits[0]) != 2 {
		t.Fatalf("want one commit of two changesets, got %d commits", len(repo.commits))
	}
	// Locate the source changeset (the one carrying the move side effects).
	var srcCS *storage.ChangeSet
	for i := range repo.commits[0] {
		if repo.commits[0][i].Project.ID == "proj-a" {
			srcCS = &repo.commits[0][i]
		}
	}
	if srcCS == nil {
		t.Fatal("source changeset not found in committed transaction")
	}
	// One AttachmentMove per moved task (root + child), each source→dest.
	moves := 0
	for _, op := range srcCS.AttachmentOps {
		if op.Kind == storage.AttachmentMove {
			moves++
			if op.SourceProject == nil || op.SourceProject.ID != "proj-a" || op.DestProject == nil || op.DestProject.ID != "proj-b" {
				t.Errorf("attachment move op has wrong projects: src=%v dest=%v", op.SourceProject, op.DestProject)
			}
		}
	}
	if moves != 2 {
		t.Errorf("attachment move ops = %d, want 2 (root + child)", moves)
	}
	// The supplied durable planning artifact rides the same transaction.
	foundArtifact := false
	for _, art := range srcCS.Artifacts {
		if art.Path == artifact.Path {
			foundArtifact = true
		}
	}
	if !foundArtifact {
		t.Errorf("durable planning artifact %q not carried in the move transaction", artifact.Path)
	}
}

func TestMoveToProjectAbortsOnCommitError(t *testing.T) {
	task := typedTask("t1", 1, "task")
	projA := newProject("proj-a", "Alpha", task)
	projB := newProject("proj-b", "Beta")
	repo := newFakeRepo(projA, projB)
	repo.commitErr = errors.New("commit boom")
	svc := newTestService(repo)

	if _, err := svc.MoveToProject("t1", "proj-b"); err == nil {
		t.Fatal("expected commit error to propagate")
	}
	if len(repo.commits) != 0 {
		t.Errorf("failed commit recorded %d successful transactions, want 0", len(repo.commits))
	}
	if len(projA.Tasks) != 1 || projA.Tasks[0] != task {
		t.Fatal("failed move did not restore the source project and task pointer")
	}
	if len(projB.Tasks) != 0 {
		t.Fatal("failed move left the task in the destination project")
	}
	if task.ShortID != 1 || task.ParentID != nil {
		t.Fatalf("failed move left task mutated: shortID=%d parent=%v", task.ShortID, task.ParentID)
	}
}

func TestMoveToProjectMovesSubtreeInOneTransaction(t *testing.T) {
	epic := typedTask("epic1", 1, "Epic")
	child := typedTask("t1", 2, "child")
	epicID := "epic1"
	child.ParentID = &epicID
	projA := newProject("proj-a", "Alpha", epic, child)
	projB := newProject("proj-b", "Beta")
	repo := newFakeRepo(projA, projB)
	svc := newTestService(repo)

	res, err := svc.MoveToProject("epic1", "proj-b")
	if err != nil {
		t.Fatalf("MoveToProject: %v", err)
	}
	if res.MovedCount != 2 {
		t.Errorf("MovedCount = %d, want 2 (epic + child)", res.MovedCount)
	}
	if len(projA.Tasks) != 0 {
		t.Errorf("source still has %d tasks, want 0", len(projA.Tasks))
	}
	if len(projB.Tasks) != 2 {
		t.Errorf("dest has %d tasks, want 2", len(projB.Tasks))
	}

	// The whole move must be a single transaction: exactly one Commit call
	// carrying both projects' changesets.
	if len(repo.commits) != 1 {
		t.Fatalf("Commit called %d times, want 1 (atomic move)", len(repo.commits))
	}
	if len(repo.commits[0]) != 2 {
		t.Errorf("transaction carried %d changesets, want 2 (source + dest)", len(repo.commits[0]))
	}
}

func TestMoveToProjectRejectsSameProject(t *testing.T) {
	task := typedTask("t1", 1, "task")
	repo := newFakeRepo(newProject("proj-a", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.MoveToProject("t1", "proj-a"); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestMoveToProjectUnknownDestination(t *testing.T) {
	task := typedTask("t1", 1, "task")
	repo := newFakeRepo(newProject("proj-a", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.MoveToProject("t1", "proj-gone"); !errors.Is(err, app.ErrProjectNotFound) {
		t.Fatalf("err = %v, want ErrProjectNotFound", err)
	}
}

func TestBulkMoveToProjectMovesSelectedSubtreesInOneTransaction(t *testing.T) {
	parent := typedTask("parent", 1, "Parent")
	child := typedTask("child", 2, "Child")
	parentID := parent.ID
	child.ParentID = &parentID
	other := typedTask("other", 1, "Other")
	alpha := newProject("alpha", "Alpha", parent, child)
	beta := newProject("beta", "Beta", other)
	destination := newProject("destination", "Destination")
	repo := newFakeRepo(alpha, beta, destination)
	svc := newTestService(repo)

	result, err := svc.BulkMoveToProject(app.BulkMoveRequest{
		TaskLocators: []app.TaskLocator{
			{ProjectID: alpha.ID, TaskID: parent.ID},
			// A selected descendant is carried by its selected ancestor exactly once.
			{ProjectID: alpha.ID, TaskID: child.ID},
			{ProjectID: beta.ID, TaskID: other.ID},
		},
		DestinationProjectID: destination.ID,
	})
	if err != nil {
		t.Fatalf("BulkMoveToProject: %v", err)
	}
	if result.MovedCount != 3 {
		t.Fatalf("moved=%d, want 3 tasks", result.MovedCount)
	}
	if len(alpha.Tasks) != 0 || len(beta.Tasks) != 0 || len(destination.Tasks) != 3 {
		t.Fatalf("post-move task counts alpha=%d beta=%d destination=%d", len(alpha.Tasks), len(beta.Tasks), len(destination.Tasks))
	}
	if len(repo.commits) != 1 || len(repo.commits[0]) != 3 {
		t.Fatalf("commits = %#v, want one atomic transaction with three projects", repo.commits)
	}
	attachmentMoves := 0
	for _, change := range repo.commits[0] {
		attachmentMoves += len(change.AttachmentOps)
	}
	if attachmentMoves != 3 {
		t.Fatalf("attachment moves = %d, want one per relocated task", attachmentMoves)
	}
}

func TestBulkMoveToProjectValidatesEverySourceBeforeMutation(t *testing.T) {
	alphaTask := typedTask("alpha-task", 1, "Alpha task")
	destinationTask := typedTask("destination-task", 1, "Destination task")
	alpha := newProject("alpha", "Alpha", alphaTask)
	destination := newProject("destination", "Destination", destinationTask)
	repo := newFakeRepo(alpha, destination)
	svc := newTestService(repo)

	_, err := svc.BulkMoveToProject(app.BulkMoveRequest{
		TaskLocators: []app.TaskLocator{
			{ProjectID: alpha.ID, TaskID: alphaTask.ID},
			{ProjectID: destination.ID, TaskID: destinationTask.ID},
		},
		DestinationProjectID: destination.ID,
	})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
	if len(alpha.Tasks) != 1 || len(destination.Tasks) != 1 || len(repo.commits) != 0 {
		t.Fatalf("failed validation mutated state: alpha=%d destination=%d commits=%d", len(alpha.Tasks), len(destination.Tasks), len(repo.commits))
	}
}
