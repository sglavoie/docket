package app_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/textgrammar"
)

// TestService_Capture_ExtendedFields exercises the capture-family surface the TUI
// capture flows depend on: a parsed destination applied through workflow state,
// tag add + remove and prepend positioning. It asserts the committed
// task carries every field and lands at the FRONT of the project's tasks.
func TestService_Capture_ExtendedFields(t *testing.T) {
	existing := newTask("t-1", 1, "existing")
	project := newProject("proj-1", "Proj", existing)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	res, err := svc.Capture(app.CaptureRequest{
		ProjectID:   project.ID,
		Title:       "Captured task",
		Priority:    2,
		Tags:        []string{"keep", "drop"},
		RemoveTags:  []string{"drop"},
		Destination: textgrammar.DestinationDoing,
		Prepend:     true,
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
	if task.State() != models.StateDoing {
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

func TestService_Capture_ChildDefaultsBacklog(t *testing.T) {
	parent := newTask("parent", 1, "Parent")
	project := newProject("proj-1", "Proj", parent)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	parentID := parent.ID
	res, err := svc.Capture(app.CaptureRequest{
		ProjectID: project.ID,
		Title:     "Backlog child",
		ParentID:  &parentID,
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	task := project.GetTaskByID(res.TaskID)
	if task == nil || task.State() != models.StateBacklog {
		t.Fatalf("child = %+v, want Backlog", task)
	}
}

// TestService_Capture_RejectsInvalidDestination verifies an invalid destination is
// rejected before any commit.
func TestService_Capture_RejectsInvalidDestination(t *testing.T) {
	project := newProject("proj-1", "Proj")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.Capture(app.CaptureRequest{ProjectID: project.ID, Title: "x", Destination: app.CaptureDestination("custom")})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("rejected capture committed %d changesets", len(repo.commits))
	}
}

func TestService_Capture_RejectsRetiredReviewDestination(t *testing.T) {
	project := newProject("proj-1", "Proj")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.Capture(app.CaptureRequest{
		ProjectID:   project.ID,
		Title:       "review is legacy-only",
		Destination: app.CaptureDestination("review"),
	})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
	if len(repo.commits) != 0 || len(project.Tasks) != 0 {
		t.Fatalf("rejected Review capture mutated state: commits=%d tasks=%d", len(repo.commits), len(project.Tasks))
	}
}

// newChildHierarchyProject builds a task plus a chain that reaches the
// nesting cap, so capture can be probed against every child placement the domain
// accepts or rejects. Each case gets a fresh project because a successful capture
// mutates it.
func newChildHierarchyProject() *models.Project {
	tasks := []*models.Task{newTask("leaf", 1, "leaf")}
	var previous *models.Task
	for level := 0; level <= models.MaxNestingDepth; level++ {
		story, err := models.NewGenericTask(fmt.Sprintf("story-%d", level), level+2, "story")
		if err != nil {
			panic(err)
		}
		if previous != nil {
			parentID := previous.ID
			story.ParentID = &parentID
		}
		tasks = append(tasks, story)
		previous = story
	}
	return newProject("proj-1", "Proj", tasks...)
}

// TestService_Capture_ChildHierarchyRules verifies capture enforces the same
// child placement rules as AddTask, so the TUI's child actions cannot persist a
// relationship the domain rejects or later detaches back to the root.
func TestService_Capture_ChildHierarchyRules(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		parentID string
		wantErr  error
	}{
		{name: "generic task parent is accepted", parentID: "leaf"},
		{name: "parent at the nesting cap is refused", parentID: "story-6", wantErr: app.ErrInvalidInput},
		{name: "parent one below the cap is accepted", parentID: "story-5"},
		{name: "valid parent is accepted", parentID: "story-0"},
		{name: "deleted parent is reported missing", parentID: "gone", wantErr: app.ErrTaskNotFound},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			project := newChildHierarchyProject()
			repo := newFakeRepo(project)
			svc := newTestService(repo)

			parentID := testCase.parentID
			res, err := svc.Capture(app.CaptureRequest{
				ProjectID: project.ID,
				Title:     "child",
				ParentID:  &parentID,
			})
			if testCase.wantErr != nil {
				if !errors.Is(err, testCase.wantErr) {
					t.Fatalf("error = %v, want %v", err, testCase.wantErr)
				}
				if len(repo.commits) != 0 {
					t.Fatalf("rejected capture committed %d changesets", len(repo.commits))
				}
				if project.GetTaskByID("leaf").Level != 0 {
					t.Error("a rejected capture changed the existing hierarchy")
				}
				return
			}
			if err != nil {
				t.Fatalf("Capture: %v", err)
			}
			created := project.GetTaskByID(res.TaskID)
			if created == nil {
				t.Fatal("captured task not found")
			}
			if created.ParentID == nil || *created.ParentID != parentID {
				t.Errorf("parent = %v, want %s", created.ParentID, parentID)
			}
		})
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
		{ProjectID: project.ID, Title: "First", Destination: app.CaptureDestinationInbox, Prepend: true},
		{ProjectID: project.ID, Title: "Second", Destination: app.CaptureDestinationInbox, Prepend: true},
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

// TestService_CaptureMany_MixedProjectsCommitsAtomically verifies a routed
// composer batch writes every touched project through one repository transaction.
func TestService_CaptureMany_MixedProjectsCommitsAtomically(t *testing.T) {
	p1 := newProject("proj-1", "One")
	p2 := newProject("proj-2", "Two")
	repo := newFakeRepo(p1, p2)
	svc := newTestService(repo)

	results, err := svc.CaptureMany([]app.CaptureRequest{
		{ProjectID: p1.ID, Title: "a"},
		{ProjectID: p2.ID, Title: "b"},
	})
	if err != nil {
		t.Fatalf("CaptureMany: %v", err)
	}
	if len(results) != 2 || results[0].ProjectID != p1.ID || results[1].ProjectID != p2.ID {
		t.Fatalf("results = %+v, want input-order project routing", results)
	}
	if len(repo.commits) != 1 || len(repo.commits[0]) != 2 {
		t.Fatalf("commits = %+v, want one transaction with two changesets", repo.commits)
	}
	if p1.GetTaskByID(results[0].TaskID) == nil || p2.GetTaskByID(results[1].TaskID) == nil {
		t.Fatal("routed tasks were not added to both projects")
	}
}

func TestService_CaptureMany_LineFailureLeavesEveryProjectUntouched(t *testing.T) {
	p1 := newProject("proj-1", "One")
	p2 := newProject("proj-2", "Two")
	repo := newFakeRepo(p1, p2)
	svc := newTestService(repo)

	_, err := svc.CaptureMany([]app.CaptureRequest{
		{SourceLine: 1, ProjectID: p1.ID, Title: "valid"},
		{SourceLine: 4, ProjectID: p2.ID, Title: "invalid", Priority: 9},
	})
	if !errors.Is(err, app.ErrInvalidInput) || !strings.Contains(err.Error(), "line 4") {
		t.Fatalf("error = %v, want line 4 ErrInvalidInput", err)
	}
	if len(repo.commits) != 0 || len(p1.Tasks) != 0 || len(p2.Tasks) != 0 {
		t.Fatalf("failed batch was partial: commits=%d p1=%d p2=%d", len(repo.commits), len(p1.Tasks), len(p2.Tasks))
	}
}

func TestService_CaptureMany_CommitFailureRestoresEveryProject(t *testing.T) {
	p1 := newProject("proj-1", "One")
	p2 := newProject("proj-2", "Two")
	repo := newFakeRepo(p1, p2)
	repo.commitErr = errors.New("commit unavailable")
	svc := newTestService(repo)

	_, err := svc.CaptureMany([]app.CaptureRequest{
		{ProjectID: p1.ID, Title: "one", Prepend: true},
		{ProjectID: p2.ID, Title: "two", Prepend: true},
	})
	if err == nil || !strings.Contains(err.Error(), "commit unavailable") {
		t.Fatalf("error = %v, want commit failure", err)
	}
	if len(repo.commits) != 0 || len(p1.Tasks) != 0 || len(p2.Tasks) != 0 {
		t.Fatalf("failed commit was partial: commits=%d p1=%d p2=%d", len(repo.commits), len(p1.Tasks), len(p2.Tasks))
	}
	if p1.NextShortID != 1 || p2.NextShortID != 1 {
		t.Fatalf("failed commit consumed short IDs: p1=%d p2=%d", p1.NextShortID, p2.NextShortID)
	}
}
