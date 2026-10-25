package app_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

func TestCreationRequestsExposeTypedCaptureDestination(t *testing.T) {
	addType := reflect.TypeOf(app.AddTaskRequest{})
	if field, ok := addType.FieldByName("State"); !ok || field.Type != reflect.TypeOf(models.WorkflowState("")) {
		t.Fatalf("add request State field = %#v, want models.WorkflowState", field)
	}

	captureType := reflect.TypeOf(app.CaptureRequest{})
	if field, ok := captureType.FieldByName("Destination"); !ok || field.Type != reflect.TypeOf(app.CaptureDestination("")) {
		t.Fatalf("capture request Destination field = %#v, want app.CaptureDestination", field)
	}
	for _, removed := range []string{"State", "Recurrence", "PlanningArtifacts", "Section", "InitialState", "Status", "Type", "Attrs"} {
		if _, ok := captureType.FieldByName(removed); ok {
			t.Fatalf("capture request still exposes removed field %s", removed)
		}
	}
}

// TestService_AddTask_Parity exercises the full capture surface:
// priority, tags, description, state, parent placement,
// checklist, plus an auxiliary durable artifact
// (standing in for the config store with an incremented template use-count). It
// asserts the created task carries every field AND that the task write and the
// auxiliary artifact are committed as ONE atomic ChangeSet.
func TestService_AddTask_Parity(t *testing.T) {
	epic, err := models.NewGenericTask("epic-1", 1, "Epic Parent")
	if err != nil {
		t.Fatal(err)
	}
	project := newProject("proj-1", "Proj", epic)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	parentID := epic.ID
	configArtifact := storage.ArtifactChange{Path: "/tmp/tt-config-parity.json", Data: []byte(`{"use_count":1}`)}
	res, err := svc.AddTask(app.AddTaskRequest{
		ProjectID:   project.ID,
		Title:       "Child task",
		Priority:    1,
		Tags:        []string{"backend", "api"},
		Description: "body text",
		State:       models.StateBacklog,
		ParentID:    &parentID,
		Checklist:   []string{"step 1", "step 2"},
		Artifacts:   []storage.ArtifactChange{configArtifact},
	})
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}

	if res.Title != "Child task" || res.ProjectID == "" || res.ShortID == 0 || res.Revision == "" {
		t.Fatalf("unexpected result envelope: %+v", res)
	}

	var task *models.Task
	for _, tk := range project.Tasks {
		if tk.ID == res.TaskID {
			task = tk
		}
	}
	if task == nil {
		t.Fatal("added task not found in project")
	}
	if task.Priority != 1 {
		t.Errorf("priority = %d, want 1", task.Priority)
	}
	if !task.HasTag("backend") || !task.HasTag("api") {
		t.Errorf("tags = %v, want backend+api", task.Tags)
	}
	if task.Description != "body text" {
		t.Errorf("description = %q", task.Description)
	}
	if got := task.State(); got != models.StateBacklog {
		t.Errorf("state = %q, want %q", got, models.StateBacklog)
	}
	if task.ParentID == nil || *task.ParentID != epic.ID {
		t.Errorf("parent = %v, want %s", task.ParentID, epic.ID)
	}
	if len(task.Checklist) != 2 {
		t.Errorf("checklist = %d items, want 2", len(task.Checklist))
	}
	// Atomicity: exactly ONE changeset, carrying the project write AND the auxiliary
	// artifact together.
	if len(repo.commits) != 1 {
		t.Fatalf("expected exactly 1 commit, got %d", len(repo.commits))
	}
	cs, ok := repo.lastCommit()
	if !ok {
		t.Fatal("expected exactly one changeset in the commit")
	}
	if cs.Project.ID != project.ID {
		t.Errorf("changeset project = %s, want %s", cs.Project.ID, project.ID)
	}
	if len(cs.Artifacts) != 1 || cs.Artifacts[0].Path != configArtifact.Path {
		t.Fatalf("auxiliary artifact did not ride the same changeset: %+v", cs.Artifacts)
	}
	if cs.UndoAction != "Add Task" {
		t.Errorf("undo action = %q, want %q", cs.UndoAction, "Add Task")
	}
}

func TestService_AddTask_StateContract(t *testing.T) {
	t.Run("omitted state defaults to Inbox", func(t *testing.T) {
		project := newProject("proj-1", "Proj")
		svc := newTestService(newFakeRepo(project))

		res, err := svc.AddTask(app.AddTaskRequest{ProjectID: project.ID, Title: "triage me"})
		if err != nil {
			t.Fatalf("AddTask: %v", err)
		}
		if got := project.GetTaskByID(res.TaskID).State(); got != models.StateInbox {
			t.Fatalf("state = %q, want inbox", got)
		}
	})

	t.Run("explicit legacy default remains Ready", func(t *testing.T) {
		project := newProject("proj-1", "Proj")
		svc := newTestService(newFakeRepo(project))

		res, err := svc.AddTask(app.AddTaskRequest{
			ProjectID: project.ID,
			Title:     "legacy active task",
			State:     models.StateBacklog,
		})
		if err != nil {
			t.Fatalf("AddTask: %v", err)
		}
		if got := project.GetTaskByID(res.TaskID).State(); got != models.StateBacklog {
			t.Fatalf("state = %q, want ready", got)
		}
	})

	t.Run("invalid state is rejected atomically", func(t *testing.T) {
		project := newProject("proj-1", "Proj")
		repo := newFakeRepo(project)
		svc := newTestService(repo)

		_, err := svc.AddTask(app.AddTaskRequest{
			ProjectID: project.ID,
			Title:     "invalid",
			State:     models.WorkflowState("Backlog"),
		})
		if !errors.Is(err, app.ErrInvalidInput) {
			t.Fatalf("error = %v, want ErrInvalidInput", err)
		}
		if len(repo.commits) != 0 || len(project.Tasks) != 0 {
			t.Fatal("invalid state must not mutate or commit")
		}
	})
}

// TestService_AddTask_InvalidParentRef verifies that a parent reference to a task
// that does not exist in the target project is rejected and nothing is committed.
func TestService_AddTask_InvalidParentRef(t *testing.T) {
	project := newProject("proj-1", "Proj", newTask("t-1", 1, "existing"))
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	missing := "does-not-exist"
	_, err := svc.AddTask(app.AddTaskRequest{ProjectID: project.ID, Title: "orphan", ParentID: &missing})
	if !errors.Is(err, app.ErrTaskNotFound) {
		t.Fatalf("error = %v, want ErrTaskNotFound", err)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("a rejected add must not commit, got %d commits", len(repo.commits))
	}
}

// TestService_AddTask_MissingProject verifies an unknown project id is reported
// and nothing is committed.
func TestService_AddTask_MissingProject(t *testing.T) {
	project := newProject("proj-1", "Proj")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.AddTask(app.AddTaskRequest{ProjectID: "nope", Title: "x"})
	if !errors.Is(err, app.ErrProjectNotFound) {
		t.Fatalf("error = %v, want ErrProjectNotFound", err)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("expected no commit, got %d", len(repo.commits))
	}
}

// TestService_AddTask_AtomicFailure verifies that when the commit fails, AddTask
// surfaces the error and no changeset is recorded — the add and its auxiliary
// artifact are all-or-nothing.
func TestService_AddTask_AtomicFailure(t *testing.T) {
	project := newProject("proj-1", "Proj")
	repo := newFakeRepo(project)
	repo.commitErr = errors.New("disk full")
	svc := newTestService(repo)

	_, err := svc.AddTask(app.AddTaskRequest{
		ProjectID: project.ID,
		Title:     "doomed",
		Artifacts: []storage.ArtifactChange{{Path: "/tmp/tt-config-atomic.json", Data: []byte(`{}`)}},
	})
	if err == nil {
		t.Fatal("expected commit error to surface")
	}
	if len(repo.commits) != 0 {
		t.Fatalf("failed commit must not be recorded, got %d", len(repo.commits))
	}
}

// TestService_AddTask_RejectsBadInputBeforeCommit verifies validation happens
// before any snapshot/commit: a malformed tag aborts the add with nothing written.
func TestService_AddTask_RejectsBadInputBeforeCommit(t *testing.T) {
	project := newProject("proj-1", "Proj")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.AddTask(app.AddTaskRequest{ProjectID: project.ID, Title: "ok", Tags: []string{"!!!"}})
	if err == nil {
		t.Fatal("expected a malformed tag to be rejected")
	}
	if len(repo.commits) != 0 {
		t.Fatalf("a rejected add must not commit, got %d commits", len(repo.commits))
	}
}
