package app_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
)

func TestService_CaptureMany_AdoptsChildrenAcrossProjectsInOneTransaction(t *testing.T) {
	a1 := newTask("a-1", 1, "A one")
	b1 := newTask("b-1", 1, "B one")
	p1 := newProject("proj-1", "One", a1)
	p2 := newProject("proj-2", "Two", b1)
	repo := newFakeRepo(p1, p2)
	svc := newTestService(repo)

	results, err := svc.CaptureMany([]app.CaptureRequest{
		{SourceLine: 1, ProjectID: p1.ID, Title: "One umbrella", ChildIDs: []string{"a-1"}},
		{SourceLine: 2, ProjectID: p2.ID, Title: "Two umbrella", ChildIDs: []string{"b-1"}},
	})
	if err != nil {
		t.Fatalf("CaptureMany: %v", err)
	}
	if len(repo.commits) != 1 || len(repo.commits[0]) != 2 {
		t.Fatalf("commits = %+v, want one transaction with two changesets", repo.commits)
	}
	if a1.ParentID == nil || *a1.ParentID != results[0].TaskID {
		t.Errorf("a-1 parent = %v, want %s", a1.ParentID, results[0].TaskID)
	}
	if b1.ParentID == nil || *b1.ParentID != results[1].TaskID {
		t.Errorf("b-1 parent = %v, want %s", b1.ParentID, results[1].TaskID)
	}
	if action := repo.commits[0][0].UndoAction; action != "Add Tasks" {
		t.Errorf("undo action = %q, want %q", action, "Add Tasks")
	}
}

// TestService_CaptureMany_RejectsCrossLineChildConflict verifies a child claimed
// by two lines rejects the whole batch and names both lines.
func TestService_CaptureMany_RejectsCrossLineChildConflict(t *testing.T) {
	contested := newTask("t-1", 7, "Contested")
	other := newTask("t-2", 8, "Other")
	p1 := newProject("proj-1", "One", contested, other)
	p2 := newProject("proj-2", "Two")
	repo := newFakeRepo(p1, p2)
	svc := newTestService(repo)

	_, err := svc.CaptureMany([]app.CaptureRequest{
		{SourceLine: 1, ProjectID: p1.ID, Title: "First claim", ChildIDs: []string{"t-1", "t-2"}},
		{SourceLine: 2, ProjectID: p2.ID, Title: "Unrelated"},
		{SourceLine: 4, ProjectID: p1.ID, Title: "Second claim", ChildIDs: []string{"t-1"}},
	})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
	for _, want := range []string{"line 4", "line 1", "#7"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err.Error(), want)
		}
	}
	if len(repo.commits) != 0 || len(p1.Tasks) != 2 || len(p2.Tasks) != 0 {
		t.Fatalf("rejected batch was partial: commits=%d p1=%d p2=%d", len(repo.commits), len(p1.Tasks), len(p2.Tasks))
	}
	if contested.ParentID != nil || other.ParentID != nil {
		t.Fatalf("rejected batch reparented: contested=%v other=%v", contested.ParentID, other.ParentID)
	}
}

// TestService_CaptureMany_RepeatedChildOnOneLineIsNotAConflict separates the
// defensive dedupe from the cross-line rule.
func TestService_CaptureMany_RepeatedChildOnOneLineIsNotAConflict(t *testing.T) {
	child := newTask("t-1", 1, "Child")
	project := newProject("proj-1", "Proj", child)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	results, err := svc.CaptureMany([]app.CaptureRequest{
		{SourceLine: 1, ProjectID: project.ID, Title: "Umbrella", ChildIDs: []string{"t-1", "t-1"}},
	})
	if err != nil {
		t.Fatalf("CaptureMany: %v", err)
	}
	if child.ParentID == nil || *child.ParentID != results[0].TaskID {
		t.Fatalf("child parent = %v, want %s", child.ParentID, results[0].TaskID)
	}
}

func TestService_CaptureMany_ChildFailureLeavesEveryProjectUntouched(t *testing.T) {
	a1 := newTask("a-1", 1, "A one")
	p1 := newProject("proj-1", "One", a1)
	p2 := newProject("proj-2", "Two")
	repo := newFakeRepo(p1, p2)
	svc := newTestService(repo)

	_, err := svc.CaptureMany([]app.CaptureRequest{
		{SourceLine: 1, ProjectID: p1.ID, Title: "valid", ChildIDs: []string{"a-1"}},
		{SourceLine: 4, ProjectID: p2.ID, Title: "invalid", ChildIDs: []string{"missing"}},
	})
	if !errors.Is(err, app.ErrTaskNotFound) || !strings.Contains(err.Error(), "line 4") {
		t.Fatalf("error = %v, want line 4 ErrTaskNotFound", err)
	}
	if len(repo.commits) != 0 || len(p1.Tasks) != 1 || len(p2.Tasks) != 0 || a1.ParentID != nil {
		t.Fatalf("failed batch was partial: commits=%d p1=%d p2=%d a-1 parent=%v",
			len(repo.commits), len(p1.Tasks), len(p2.Tasks), a1.ParentID)
	}
}

// TestService_CaptureAdoptedChildPreservesFlatOrder pins the ordering contract
// at its hardest case: the captured task nests under a completed root, which the
// post-mutation hooks reopen, and it adopts an open root that sits before an
// unrelated root in the slice. Nothing pre-existing may move.
func TestService_CaptureAdoptedChildPreservesFlatOrder(t *testing.T) {
	adopted := newTask("t-1", 1, "Adopted root")
	unrelated := newTask("t-2", 2, "Unrelated root")
	finished := newTask("t-3", 3, "Finished root")
	finished.Complete()
	project := newProject("proj-1", "Proj", adopted, unrelated, finished)
	repo := newFakeRepo(project)

	res, err := newTestService(repo).Capture(app.CaptureRequest{
		ProjectID: project.ID,
		Title:     "Nested",
		ParentID:  &finished.ID,
		ChildIDs:  []string{adopted.ID},
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if adopted.ParentID == nil || *adopted.ParentID != res.TaskID {
		t.Fatalf("adopted parent = %v, want %s", adopted.ParentID, res.TaskID)
	}
	if finished.IsComplete() {
		t.Fatalf("completed root kept its terminal state above an open child")
	}
	titles := projectTaskTitles(project)
	want := []string{"Adopted root", "Unrelated root", "Finished root", "Nested"}
	if strings.Join(titles, ",") != strings.Join(want, ",") {
		t.Errorf("task order = %v, want %v", titles, want)
	}
}

func projectTaskTitles(project *models.Project) []string {
	titles := make([]string, 0, len(project.Tasks))
	for _, task := range project.Tasks {
		titles = append(titles, task.Title)
	}
	return titles
}
