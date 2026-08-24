package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/app"
)

func TestCaptureRecordsURLAndCodeReferenceAttachments(t *testing.T) {
	project := newProject("p1", "Alpha")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	result, err := svc.Capture(app.CaptureRequest{
		ProjectID:   project.ID,
		Title:       "Read the spec",
		Attachments: []string{"https://example.com/spec.pdf", "internal/app/add.go:19"},
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	task := project.GetTaskByID(result.TaskID)
	if task == nil {
		t.Fatal("captured task not found")
	}
	if len(task.Attachments) != 2 {
		t.Fatalf("attachments = %d, want 2", len(task.Attachments))
	}
	// Order is line order, so the task reads back the way the line was written.
	if got := task.Attachments[0].Filename; got != "https://example.com/spec.pdf" {
		t.Errorf("first attachment = %q, want the URL", got)
	}
	if got := task.Attachments[1].Filename; !strings.Contains(got, "add.go") {
		t.Errorf("second attachment = %q, want the code reference", got)
	}
}

// A missing file must abort the whole capture: the alternative is a task that
// exists while the attachment the line asked for does not.
func TestCaptureRefusesAnAttachmentNamingAMissingFile(t *testing.T) {
	project := newProject("p1", "Alpha")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.Capture(app.CaptureRequest{
		ProjectID:   project.ID,
		Title:       "Read the spec",
		Attachments: []string{filepath.Join(t.TempDir(), "absent.pdf")},
	})
	if err == nil {
		t.Fatal("Capture succeeded, want a refusal naming the missing file")
	}
	if !strings.Contains(err.Error(), "absent.pdf") {
		t.Errorf("error = %v, want it to name the value", err)
	}
	if len(project.Tasks) != 0 {
		t.Errorf("project holds %d tasks, want the capture to have aborted", len(project.Tasks))
	}
}

// Two different files whose base names collide would copy over each other in the
// attachments directory, so the line refuses by name instead.
func TestCaptureRefusesTwoAttachmentsResolvingToOneFilename(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	for _, dir := range []string{dirA, dirB} {
		if err := os.WriteFile(filepath.Join(dir, "spec.pdf"), []byte("x"), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
	}
	project := newProject("p1", "Alpha")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.Capture(app.CaptureRequest{
		ProjectID:   project.ID,
		Title:       "Read the spec",
		Attachments: []string{filepath.Join(dirA, "spec.pdf"), filepath.Join(dirB, "spec.pdf")},
	})
	if err == nil {
		t.Fatal("Capture succeeded, want a refusal naming the collision")
	}
	if !strings.Contains(err.Error(), "spec.pdf") {
		t.Errorf("error = %v, want it to name the destination filename", err)
	}
	if len(project.Tasks) != 0 {
		t.Errorf("project holds %d tasks, want the capture to have aborted", len(project.Tasks))
	}
}
