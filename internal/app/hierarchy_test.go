package app_test

import (
	"errors"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// typedTask builds a task of a specific type for hierarchy tests.
func typedTask(id string, shortID int, taskType models.TaskType, title string) *models.Task {
	tk, err := models.NewTask(id, shortID, taskType, title)
	if err != nil {
		panic(err)
	}
	return tk
}

func TestReparentUnderValidParent(t *testing.T) {
	epic := typedTask("epic1", 1, models.TaskTypeEpic, "Epic")
	task := typedTask("t1", 2, models.TaskTypeTask, "child task")
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
	epic := typedTask("epic1", 1, models.TaskTypeEpic, "Epic")
	task := typedTask("t1", 2, models.TaskTypeTask, "child task")
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

func TestReparentRejectsIncompatibleType(t *testing.T) {
	leaf := typedTask("leaf1", 1, models.TaskTypeTask, "leaf")
	task := typedTask("t1", 2, models.TaskTypeTask, "another leaf")
	projA := newProject("proj-a", "Alpha", leaf, task)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	// A task cannot be a child of another leaf task.
	parentID := "leaf1"
	if _, err := svc.Reparent("t1", &parentID); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("rejected reparent committed %d times, want 0", len(repo.commits))
	}
}

func TestReparentRejectsCycle(t *testing.T) {
	epic := typedTask("epic1", 1, models.TaskTypeEpic, "Epic")
	story := typedTask("story1", 2, models.TaskTypeStory, "Story")
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
	task := typedTask("t1", 1, models.TaskTypeTask, "task")
	repo := newFakeRepo(newProject("proj-a", "Alpha", task))
	svc := newTestService(repo)

	ghost := "no-such-parent"
	if _, err := svc.Reparent("t1", &ghost); !errors.Is(err, app.ErrTaskNotFound) {
		t.Fatalf("err = %v, want ErrTaskNotFound", err)
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

func TestIndentNestsUnderTaskAbove(t *testing.T) {
	epic := typedTask("epic1", 1, models.TaskTypeEpic, "Epic")
	task := typedTask("t1", 2, models.TaskTypeTask, "leaf")
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
	task := typedTask("t1", 1, models.TaskTypeTask, "only task")
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
	epic := typedTask("epic1", 1, models.TaskTypeEpic, "Epic")
	story := typedTask("story1", 2, models.TaskTypeStory, "Story")
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
	task := typedTask("t1", 1, models.TaskTypeTask, "root task")
	projA := newProject("proj-a", "Alpha", task)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	if _, err := svc.OutdentTask("t1"); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestChangeTaskTypePromote(t *testing.T) {
	task := typedTask("t1", 1, models.TaskTypeTask, "leaf")
	projA := newProject("proj-a", "Alpha", task)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	if _, err := svc.ChangeTaskType("t1", app.PromoteType); err != nil {
		t.Fatalf("ChangeTaskType promote: %v", err)
	}
	if task.Type != models.TaskTypeStory {
		t.Fatalf("Type = %s, want story", task.Type)
	}
}

func TestChangeTaskTypeDemote(t *testing.T) {
	epic := typedTask("e1", 1, models.TaskTypeEpic, "Epic")
	projA := newProject("proj-a", "Alpha", epic)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	if _, err := svc.ChangeTaskType("e1", app.DemoteType); err != nil {
		t.Fatalf("ChangeTaskType demote: %v", err)
	}
	if epic.Type != models.TaskTypeStory {
		t.Fatalf("Type = %s, want story", epic.Type)
	}
}

func TestChangeTaskTypeCycleRight(t *testing.T) {
	task := typedTask("t1", 1, models.TaskTypeTask, "leaf")
	projA := newProject("proj-a", "Alpha", task)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	if _, err := svc.ChangeTaskType("t1", app.CycleTypeRight); err != nil {
		t.Fatalf("ChangeTaskType cycle: %v", err)
	}
	if task.Type != models.TaskTypeBug {
		t.Fatalf("Type = %s, want bug", task.Type)
	}
}

func TestChangeTaskTypePromoteRejectedAtTop(t *testing.T) {
	epic := typedTask("e1", 1, models.TaskTypeEpic, "Epic")
	projA := newProject("proj-a", "Alpha", epic)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	if _, err := svc.ChangeTaskType("e1", app.PromoteType); err == nil {
		t.Fatal("expected error promoting an epic")
	}
	if len(repo.commits) != 0 {
		t.Errorf("rejected type change committed %d times, want 0", len(repo.commits))
	}
	if epic.Type != models.TaskTypeEpic {
		t.Errorf("Type = %s, want epic (unchanged)", epic.Type)
	}
}

func TestMoveToProjectCarriesAttachmentOpsAndArtifact(t *testing.T) {
	epic := typedTask("epic1", 1, models.TaskTypeEpic, "Epic")
	child := typedTask("t1", 2, models.TaskTypeTask, "child")
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
	task := typedTask("t1", 1, models.TaskTypeTask, "task")
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
}

func TestMoveToProjectMovesSubtreeInOneTransaction(t *testing.T) {
	epic := typedTask("epic1", 1, models.TaskTypeEpic, "Epic")
	child := typedTask("t1", 2, models.TaskTypeTask, "child")
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
	if res.DestProjectID != "proj-b" {
		t.Errorf("DestProjectID = %q, want proj-b", res.DestProjectID)
	}
	if len(projA.Tasks) != 0 {
		t.Errorf("source still has %d tasks, want 0", len(projA.Tasks))
	}
	if len(projB.Tasks) != 2 {
		t.Errorf("dest has %d tasks, want 2", len(projB.Tasks))
	}
	if res.Task.ProjectSlug != "beta" {
		t.Errorf("moved task project slug = %q, want beta", res.Task.ProjectSlug)
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
	task := typedTask("t1", 1, models.TaskTypeTask, "task")
	repo := newFakeRepo(newProject("proj-a", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.MoveToProject("t1", "proj-a"); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestMoveToProjectUnknownDestination(t *testing.T) {
	task := typedTask("t1", 1, models.TaskTypeTask, "task")
	repo := newFakeRepo(newProject("proj-a", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.MoveToProject("t1", "proj-gone"); !errors.Is(err, app.ErrProjectNotFound) {
		t.Fatalf("err = %v, want ErrProjectNotFound", err)
	}
}
