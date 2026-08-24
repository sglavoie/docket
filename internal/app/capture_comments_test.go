package app_test

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/app"
)

func TestCaptureRecordsCommentsInLineOrder(t *testing.T) {
	project := newProject("p1", "Alpha")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	result, err := svc.Capture(app.CaptureRequest{
		ProjectID: project.ID,
		Title:     "Call Dana",
		Comments:  []string{"  ask about the SSO deadline  ", "then confirm the date"},
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	task := project.GetTaskByID(result.TaskID)
	if task == nil {
		t.Fatal("captured task not found")
	}
	if len(task.Comments) != 2 {
		t.Fatalf("comments = %d, want 2", len(task.Comments))
	}
	// The same normalization an interactively added comment goes through, so the
	// two entry points cannot disagree about stored text.
	if got := task.Comments[0].Text; got != "ask about the SSO deadline" {
		t.Errorf("first comment = %q, want it trimmed", got)
	}
	if got := task.Comments[1].Text; got != "then confirm the date" {
		t.Errorf("second comment = %q, want line order preserved", got)
	}
	if task.Comments[0].CreatedAt.IsZero() {
		t.Error("comment has no timestamp")
	}
}

// A blank comment must abort the whole capture rather than being dropped: the
// task Markdown treats empty text as a flush sentinel, so a silently discarded
// comment is a value the user typed and never sees again.
func TestCaptureRefusesABlankComment(t *testing.T) {
	project := newProject("p1", "Alpha")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.Capture(app.CaptureRequest{
		ProjectID: project.ID,
		Title:     "Call Dana",
		Comments:  []string{"   "},
	})
	if err == nil {
		t.Fatal("Capture succeeded, want a refusal naming the empty comment")
	}
	if !strings.Contains(err.Error(), "comment") {
		t.Errorf("error = %v, want it to name the field", err)
	}
	if len(project.Tasks) != 0 {
		t.Errorf("project holds %d tasks, want the capture to have aborted", len(project.Tasks))
	}
}

// CaptureMany is the multiline composer's path, and a comment must ride the same
// single transaction there as it does on one line.
func TestCaptureManyRecordsCommentsPerLine(t *testing.T) {
	project := newProject("p1", "Alpha")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	results, err := svc.CaptureMany([]app.CaptureRequest{
		{ProjectID: project.ID, Title: "First", Comments: []string{"one"}},
		{ProjectID: project.ID, Title: "Second"},
	})
	if err != nil {
		t.Fatalf("CaptureMany: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	first := project.GetTaskByID(results[0].TaskID)
	second := project.GetTaskByID(results[1].TaskID)
	if first == nil || second == nil {
		t.Fatal("captured tasks not found")
	}
	if len(first.Comments) != 1 || first.Comments[0].Text != "one" {
		t.Errorf("first comments = %+v, want exactly the line's comment", first.Comments)
	}
	if len(second.Comments) != 0 {
		t.Errorf("second comments = %+v, want none", second.Comments)
	}
}
