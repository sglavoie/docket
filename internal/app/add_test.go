package app_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func TestCreationRequestsExposeOnlyCanonicalState(t *testing.T) {
	for name, requestType := range map[string]reflect.Type{
		"add":     reflect.TypeOf(app.AddTaskRequest{}),
		"capture": reflect.TypeOf(app.CaptureRequest{}),
	} {
		t.Run(name, func(t *testing.T) {
			if field, ok := requestType.FieldByName("State"); !ok || field.Type != reflect.TypeOf(models.WorkflowState("")) {
				t.Fatalf("%s request State field = %#v, want models.WorkflowState", name, field)
			}
			for _, legacy := range []string{"Section", "InitialState", "Status"} {
				if _, ok := requestType.FieldByName(legacy); ok {
					t.Fatalf("%s request still exposes legacy creation field %s", name, legacy)
				}
			}
		})
	}
}

// TestService_AddTask_Parity exercises the full typed-capture surface: type,
// priority, tags, description, state, recurrence, parent placement,
// checklist, and type-specific attributes, plus an auxiliary durable artifact
// (standing in for the config store with an incremented template use-count). It
// asserts the created task carries every field AND that the task write and the
// auxiliary artifact are committed as ONE atomic ChangeSet.
func TestService_AddTask_Parity(t *testing.T) {
	epic, err := models.NewTask("epic-1", 1, models.TaskTypeEpic, "Epic Parent")
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
		Type:        models.TaskTypeTask,
		Priority:    1,
		Tags:        []string{"backend", "api"},
		Description: "body text",
		State:       models.StateNext,
		Recurrence:  models.ParseRecurrence("weekly"),
		ParentID:    &parentID,
		Checklist:   []string{"step 1", "step 2"},
		Attrs:       models.TypeAttrValues{TaskEffort: "small"},
		Artifacts:   []storage.ArtifactChange{configArtifact},
	})
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}

	if res.Title != "Child task" || res.ProjectSlug == "" || res.QualifiedRef == "" || res.Revision == "" {
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
	if got := models.SectionForState(task.State()); got != models.SectionNext {
		t.Errorf("section = %q, want %q", got, models.SectionNext)
	}
	if task.Recurrence == nil {
		t.Error("recurrence not set")
	}
	if task.ParentID == nil || *task.ParentID != epic.ID {
		t.Errorf("parent = %v, want %s", task.ParentID, epic.ID)
	}
	if len(task.Checklist) != 2 {
		t.Errorf("checklist = %d items, want 2", len(task.Checklist))
	}
	if task.TaskAttrs == nil || task.TaskAttrs.Effort != models.EffortSmall {
		t.Errorf("task attributes not applied: %+v", task.TaskAttrs)
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
			State:     models.StateReady,
		})
		if err != nil {
			t.Fatalf("AddTask: %v", err)
		}
		if got := project.GetTaskByID(res.TaskID).State(); got != models.StateReady {
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

// TestService_AddTask_InvalidParentType verifies hierarchy rules are enforced: an
// epic cannot be nested under a leaf task. The rejected request commits nothing.
func TestService_AddTask_InvalidParentType(t *testing.T) {
	leaf := newTask("t-1", 1, "leaf task")
	project := newProject("proj-1", "Proj", leaf)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	parentID := leaf.ID
	_, err := svc.AddTask(app.AddTaskRequest{
		ProjectID: project.ID,
		Title:     "an epic",
		Type:      models.TaskTypeEpic,
		ParentID:  &parentID,
	})
	if err == nil {
		t.Fatal("expected error nesting an epic under a task")
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

	_, err := svc.AddTask(app.AddTaskRequest{ProjectID: project.ID, Title: "ok", Tags: []string{"bad,tag"}})
	if err == nil {
		t.Fatal("expected a malformed tag to be rejected")
	}
	if len(repo.commits) != 0 {
		t.Fatalf("a rejected add must not commit, got %d commits", len(repo.commits))
	}
}
