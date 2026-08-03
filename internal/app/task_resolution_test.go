package app_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

func duplicateTaskFixture() (*models.Task, *models.Task, *fakeRepo, *app.Service) {
	alphaTask := newTask("shared", 1, "Alpha task")
	betaTask := newTask("shared", 1, "Beta task")
	repo := newFakeRepo(
		newProject("alpha", "Alpha", alphaTask),
		newProject("beta", "Beta", betaTask),
	)
	return alphaTask, betaTask, repo, newTestService(repo)
}

func requireAmbiguousTaskID(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, app.ErrAmbiguousTaskID) {
		t.Fatalf("error = %v, want ErrAmbiguousTaskID", err)
	}
	if !strings.Contains(err.Error(), "provide project ownership") {
		t.Fatalf("error = %q, want locator guidance", err)
	}
}

func TestSnapshotLookupTaskIDDistinguishesMissingUniqueAndAmbiguous(t *testing.T) {
	unique := newTask("unique", 1, "Unique")
	sharedA := newTask("shared", 2, "Shared A")
	sharedB := newTask("shared", 1, "Shared B")
	repo := newFakeRepo(
		newProject("alpha", "Alpha", unique, sharedA),
		newProject("beta", "Beta", sharedB),
	)
	snap, err := repo.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	ref, status := snap.LookupTaskID("unique")
	if status != app.TaskLookupUnique || ref.Task != unique || ref.Project.ID != "alpha" {
		t.Fatalf("LookupTaskID(unique) = (%+v, %v)", ref, status)
	}
	if ref, status = snap.LookupTaskID("missing"); status != app.TaskLookupMissing || ref.Task != nil {
		t.Fatalf("LookupTaskID(missing) = (%+v, %v)", ref, status)
	}
	if ref, status = snap.LookupTaskID("shared"); status != app.TaskLookupAmbiguous || ref.Task != nil {
		t.Fatalf("LookupTaskID(shared) = (%+v, %v)", ref, status)
	}
	ref, ok := snap.LookupLocator(app.TaskLocator{ProjectID: "beta", TaskID: "shared"})
	if !ok || ref.Task != sharedB {
		t.Fatalf("LookupLocator(beta/shared) = (%+v, %v)", ref, ok)
	}
}

func TestDirectMutationRejectsAmbiguousIDAndLocatorTargetsOneProject(t *testing.T) {
	alphaTask, betaTask, repo, service := duplicateTaskFixture()
	alphaPriority, betaPriority := alphaTask.Priority, betaTask.Priority

	_, err := service.SetPriority("shared", 1)
	requireAmbiguousTaskID(t, err)
	if len(repo.commits) != 0 {
		t.Fatalf("ambiguous mutation committed %d transactions", len(repo.commits))
	}
	if alphaTask.Priority != alphaPriority || betaTask.Priority != betaPriority {
		t.Fatalf("ambiguous mutation changed priorities: alpha=%d beta=%d", alphaTask.Priority, betaTask.Priority)
	}

	_, err = service.SetPriorityByLocator(app.TaskLocator{ProjectID: "beta", TaskID: "shared"}, 1)
	if err != nil {
		t.Fatalf("SetPriorityByLocator: %v", err)
	}
	if len(repo.commits) != 1 || len(repo.commits[0]) != 1 || repo.commits[0][0].Project.ID != "beta" {
		t.Fatalf("locator commit = %#v", repo.commits)
	}
	if alphaTask.Priority != alphaPriority || betaTask.Priority != 1 {
		t.Fatalf("locator mutation priorities: alpha=%d beta=%d", alphaTask.Priority, betaTask.Priority)
	}
}

func TestDeleteRejectsAmbiguousIDAndLocatorDeletesOneProject(t *testing.T) {
	alphaTask, betaTask, repo, service := duplicateTaskFixture()

	_, err := service.Delete(app.DeleteRequest{TaskID: "shared", Confirm: true})
	requireAmbiguousTaskID(t, err)
	if len(repo.commits) != 0 || repo.projects[0].GetTaskByID(alphaTask.ID) == nil || repo.projects[1].GetTaskByID(betaTask.ID) == nil {
		t.Fatal("ambiguous delete mutated a project or committed")
	}

	_, err = service.Delete(app.DeleteRequest{
		Locator: app.TaskLocator{ProjectID: "beta", TaskID: "shared"},
		Confirm: true,
	})
	if err != nil {
		t.Fatalf("Delete by locator: %v", err)
	}
	if repo.projects[0].GetTaskByID("shared") == nil || repo.projects[1].GetTaskByID("shared") != nil {
		t.Fatalf("locator delete targeted wrong project: alpha=%v beta=%v", repo.projects[0].GetTaskByID("shared"), repo.projects[1].GetTaskByID("shared"))
	}
}

func TestTwoTaskMutationResolvesBothTargetsBeforeMutation(t *testing.T) {
	unique := newTask("unique", 1, "Unique")
	sharedA := newTask("shared", 2, "Shared A")
	sharedB := newTask("shared", 1, "Shared B")
	repo := newFakeRepo(
		newProject("alpha", "Alpha", unique, sharedA),
		newProject("beta", "Beta", sharedB),
	)
	service := newTestService(repo)
	unique.Pinned, sharedA.Pinned, sharedB.Pinned = true, true, true
	uniqueTime := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	sharedTime := uniqueTime.Add(time.Hour)
	otherSharedTime := uniqueTime.Add(2 * time.Hour)
	unique.PinnedAt, sharedA.PinnedAt, sharedB.PinnedAt = &uniqueTime, &otherSharedTime, &sharedTime

	_, err := service.ReorderPinnedTasks("unique", "shared")
	requireAmbiguousTaskID(t, err)
	if len(repo.commits) != 0 || !unique.PinnedAt.Equal(uniqueTime) || !sharedB.PinnedAt.Equal(sharedTime) {
		t.Fatal("ambiguous two-task mutation changed timestamps or committed")
	}

	_, err = service.ReorderPinnedTasksByLocator(
		app.TaskLocator{ProjectID: "alpha", TaskID: "unique"},
		app.TaskLocator{ProjectID: "beta", TaskID: "shared"},
	)
	if err != nil {
		t.Fatalf("ReorderPinnedTasksByLocator: %v", err)
	}
	if !unique.PinnedAt.Equal(sharedTime) || !sharedB.PinnedAt.Equal(uniqueTime) || !sharedA.PinnedAt.Equal(otherSharedTime) {
		t.Fatalf("locator reorder timestamps: unique=%v sharedA=%v sharedB=%v", unique.PinnedAt, sharedA.PinnedAt, sharedB.PinnedAt)
	}
}

func TestTaskRequestTargetValidationFailsBeforeMutation(t *testing.T) {
	task := newTask("task", 1, "Task")
	repo := newFakeRepo(newProject("alpha", "Alpha", task))
	service := newTestService(repo)
	originalState := task.State()

	tests := []struct {
		name string
		req  app.StateRequest
		want error
	}{
		{
			name: "conflicting fields",
			req:  app.StateRequest{TaskID: "task", Locator: app.TaskLocator{ProjectID: "alpha", TaskID: "task"}, Target: models.StateBacklog},
			want: app.ErrInvalidInput,
		},
		{
			name: "partial locator",
			req:  app.StateRequest{Locator: app.TaskLocator{ProjectID: "alpha"}, Target: models.StateBacklog},
			want: app.ErrInvalidInput,
		},
		{
			name: "missing task",
			req:  app.StateRequest{TaskID: "missing", Target: models.StateBacklog},
			want: app.ErrTaskNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.Transition(tt.req)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Transition error = %v, want %v", err, tt.want)
			}
		})
	}
	if len(repo.commits) != 0 || task.State() != originalState {
		t.Fatalf("invalid targets committed or changed state: commits=%d state=%s", len(repo.commits), task.State())
	}
}
