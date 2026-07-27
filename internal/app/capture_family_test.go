package app_test

import (
	"errors"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// TestService_Capture_ExtendedFields exercises the capture-family surface the TUI
// capture flows depend on: a parsed status sigil applied as a workflow-state
// state, recurrence, URL/Source
// metadata, tag add + remove, and prepend positioning. It asserts the committed
// task carries every field and lands at the FRONT of the project's tasks.
func TestService_Capture_ExtendedFields(t *testing.T) {
	existing := newTask("t-1", 1, "existing")
	project := newProject("proj-1", "Proj", existing)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	res, err := svc.Capture(app.CaptureRequest{
		ProjectID:  project.ID,
		Title:      "Captured task",
		Priority:   2,
		Tags:       []string{"keep", "drop"},
		RemoveTags: []string{"drop"},
		URL:        "https://example.com",
		Source:     "slack",
		Recurrence: models.ParseRecurrence("daily"),
		State:      models.StateInProgress,
		Prepend:    true,
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}

	var task *models.Task
	for _, tk := range project.Tasks {
		if tk.ID == res.TaskID {
			task = tk
		}
	}
	if task == nil {
		t.Fatal("captured task not found in project")
	}
	if task.Priority != 2 {
		t.Errorf("priority = %d, want 2", task.Priority)
	}
	if !task.HasTag("keep") || task.HasTag("drop") {
		t.Errorf("tags = %v, want keep only (drop removed)", task.Tags)
	}
	if task.URL != "https://example.com" || task.Source != "slack" {
		t.Errorf("url/source = %q/%q", task.URL, task.Source)
	}
	if task.Recurrence == nil {
		t.Error("recurrence not set")
	}
	if task.State() != models.StateInProgress {
		t.Errorf("state = %q, want in_progress", task.State())
	}
	// Prepend places the new task at the front.
	if project.Tasks[0].ID != res.TaskID {
		t.Errorf("prepended task not first: %q", project.Tasks[0].Title)
	}
}

// TestService_Capture_DefaultInboxState verifies that a captured task with an
// omitted state defaults to Inbox.
func TestService_Capture_DefaultInboxState(t *testing.T) {
	project := newProject("proj-1", "Proj")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	res, err := svc.Capture(app.CaptureRequest{
		ProjectID: project.ID,
		Title:     "Inbox task",
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	task := project.GetTaskByID(res.TaskID)
	if task == nil {
		t.Fatal("captured task not found")
	}
	if task.State() != models.StateInbox {
		t.Errorf("state = %q, want inbox", task.State())
	}
}

// TestService_Capture_RejectsInvalidState verifies an invalid canonical state is
// rejected before any commit.
func TestService_Capture_RejectsInvalidState(t *testing.T) {
	project := newProject("proj-1", "Proj")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.Capture(app.CaptureRequest{ProjectID: project.ID, Title: "x", State: models.WorkflowState("Custom")})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("rejected capture committed %d changesets", len(repo.commits))
	}
}

// TestService_CaptureMany_OneAtomicCommit verifies Bulk Capture semantics: every
// task is created in ONE ChangeSet, prepended newest-first ahead of the existing
// tasks, so the batch has a single undo entry.
func TestService_CaptureMany_OneAtomicCommit(t *testing.T) {
	existing := newTask("t-1", 1, "existing")
	project := newProject("proj-1", "Proj", existing)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	reqs := []app.CaptureRequest{
		{ProjectID: project.ID, Title: "First", State: models.StateInbox, Prepend: true},
		{ProjectID: project.ID, Title: "Second", State: models.StateInbox, Prepend: true},
	}
	results, err := svc.CaptureMany(reqs)
	if err != nil {
		t.Fatalf("CaptureMany: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	if len(repo.commits) != 1 {
		t.Fatalf("expected exactly 1 commit for the batch, got %d", len(repo.commits))
	}
	// Newest-first prepend: [First, Second, existing].
	if len(project.Tasks) != 3 {
		t.Fatalf("tasks = %d, want 3", len(project.Tasks))
	}
	if project.Tasks[0].Title != "First" || project.Tasks[1].Title != "Second" || project.Tasks[2].Title != "existing" {
		t.Errorf("order = [%q %q %q], want [First Second existing]", project.Tasks[0].Title, project.Tasks[1].Title, project.Tasks[2].Title)
	}
	cs, ok := repo.lastCommit()
	if !ok || cs.UndoAction != "Add Tasks" {
		t.Errorf("undo action = %q, want %q", cs.UndoAction, "Add Tasks")
	}
}

// TestService_CaptureMany_MixedProjectsRejected verifies a batch spanning
// projects is rejected before any commit.
func TestService_CaptureMany_MixedProjectsRejected(t *testing.T) {
	p1 := newProject("proj-1", "One")
	p2 := newProject("proj-2", "Two")
	repo := newFakeRepo(p1, p2)
	svc := newTestService(repo)

	_, err := svc.CaptureMany([]app.CaptureRequest{
		{ProjectID: p1.ID, Title: "a"},
		{ProjectID: p2.ID, Title: "b"},
	})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("rejected batch committed %d changesets", len(repo.commits))
	}
}

// TestService_CreateLinkedTask_Relations exercises every link kind: the new task
// and the mutated source commit as ONE ChangeSet, and the relationship is
// established on the correct side.
func TestService_CreateLinkedTask_Relations(t *testing.T) {
	cases := []struct {
		name string
		link app.LinkKind
		// verify inspects the committed project after the link is created.
		verify func(t *testing.T, project *models.Project, source, created *models.Task)
	}{
		{
			name: "blocker adds dependency to the source",
			link: app.LinkBlocker,
			verify: func(t *testing.T, _ *models.Project, source, created *models.Task) {
				if !source.HasDependency(created.ID) {
					t.Errorf("source should depend on the new blocker")
				}
			},
		},
		{
			name: "dependent depends on the source",
			link: app.LinkDependent,
			verify: func(t *testing.T, _ *models.Project, source, created *models.Task) {
				if !created.HasDependency(source.ID) {
					t.Errorf("new task should depend on the source")
				}
			},
		},
		{
			name: "subtask nests under the source and promotes it",
			link: app.LinkSubtask,
			verify: func(t *testing.T, _ *models.Project, source, created *models.Task) {
				if created.ParentID == nil || *created.ParentID != source.ID {
					t.Errorf("subtask parent = %v, want source %s", created.ParentID, source.ID)
				}
				if source.Type != models.TaskTypeStory {
					t.Errorf("leaf source not promoted to story: %s", source.Type)
				}
			},
		},
		{
			name: "follow-up relates both ways and copies tags",
			link: app.LinkFollowUp,
			verify: func(t *testing.T, _ *models.Project, source, created *models.Task) {
				if !source.HasRelatedTo(created.ID) || !created.HasRelatedTo(source.ID) {
					t.Errorf("follow-up should relate both ways")
				}
				if !created.HasTag("carried") {
					t.Errorf("follow-up should inherit the source's tags, got %v", created.Tags)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := newTask("src-1", 1, "source")
			source.AddTag("carried")
			project := newProject("proj-1", "Proj", source)
			repo := newFakeRepo(project)
			svc := newTestService(repo)

			res, err := svc.CreateLinkedTask(app.LinkedTaskRequest{
				Task:         app.CaptureRequest{ProjectID: project.ID, Title: "linked"},
				SourceTaskID: source.ID,
				Link:         tc.link,
			})
			if err != nil {
				t.Fatalf("CreateLinkedTask: %v", err)
			}
			if len(repo.commits) != 1 {
				t.Fatalf("expected 1 atomic commit, got %d", len(repo.commits))
			}
			created := project.GetTaskByID(res.New.TaskID)
			if created == nil {
				t.Fatal("created linked task not found")
			}
			if created.State() != models.StateInbox {
				t.Fatalf("linked task state = %q, want canonical Inbox default", created.State())
			}
			tc.verify(t, project, project.GetTaskByID(source.ID), created)
		})
	}
}

// TestService_CreateLinkedTask_MissingSource verifies a missing source aborts
// before any commit.
func TestService_CreateLinkedTask_MissingSource(t *testing.T) {
	project := newProject("proj-1", "Proj")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.CreateLinkedTask(app.LinkedTaskRequest{
		Task:         app.CaptureRequest{ProjectID: project.ID, Title: "x"},
		SourceTaskID: "nope",
		Link:         app.LinkBlocker,
	})
	if !errors.Is(err, app.ErrTaskNotFound) {
		t.Fatalf("error = %v, want ErrTaskNotFound", err)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("rejected linked-task create committed %d changesets", len(repo.commits))
	}
}
