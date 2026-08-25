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
	// OriginalPath must carry the URL exactly as AttachURL stores it: the panel's
	// original-path action copies that field, not Filename.
	if got := task.Attachments[0].OriginalPath; got != "https://example.com/spec.pdf" {
		t.Errorf("first attachment OriginalPath = %q, want the URL", got)
	}
}

// A captured URL and a panel-attached URL must be indistinguishable on the task,
// or the panel's original-path action behaves differently depending on where the
// attachment came from.
func TestCaptureURLMatchesAttachURLStorage(t *testing.T) {
	project := newProject("p1", "Alpha")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	const link = "https://example.com/spec.pdf"
	captured, err := svc.Capture(app.CaptureRequest{
		ProjectID:   project.ID,
		Title:       "Captured",
		Attachments: []string{link},
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	plain, err := svc.Capture(app.CaptureRequest{ProjectID: project.ID, Title: "Attached"})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if _, err := svc.AttachURL(plain.TaskID, link); err != nil {
		t.Fatalf("AttachURL: %v", err)
	}

	capturedTask := project.GetTaskByID(captured.TaskID)
	attachedTask := project.GetTaskByID(plain.TaskID)
	if capturedTask == nil || attachedTask == nil {
		t.Fatal("tasks not found")
	}
	if len(capturedTask.Attachments) != 1 || len(attachedTask.Attachments) != 1 {
		t.Fatalf("attachment counts = %d and %d, want 1 each",
			len(capturedTask.Attachments), len(attachedTask.Attachments))
	}
	got, want := capturedTask.Attachments[0], attachedTask.Attachments[0]
	if got.Filename != want.Filename || got.OriginalPath != want.OriginalPath || got.IsURL != want.IsURL {
		t.Errorf("captured attachment = %+v, want it to match the AttachURL one %+v", got, want)
	}
}

// A value the grammar accepts by its http(s) prefix is not yet a URL; the
// canonical validation must refuse it before the capture commits, or the task
// persists a URL the Attachments panel rejects on read.
func TestCaptureRefusesAMalformedURLAttachment(t *testing.T) {
	for _, malformed := range []string{"http://", "https://"} {
		t.Run(malformed, func(t *testing.T) {
			project := newProject("p1", "Alpha")
			repo := newFakeRepo(project)
			svc := newTestService(repo)

			_, err := svc.Capture(app.CaptureRequest{
				ProjectID:   project.ID,
				Title:       "Read the spec",
				Attachments: []string{malformed},
			})
			if err == nil {
				t.Fatal("Capture succeeded, want a refusal naming the URL shape")
			}
			if !strings.Contains(err.Error(), "absolute http or https URL") {
				t.Errorf("error = %v, want the canonical URL diagnostic", err)
			}
			if len(project.Tasks) != 0 {
				t.Errorf("project holds %d tasks, want the capture to have aborted", len(project.Tasks))
			}
		})
	}
}

// The expanded form's creation path is the same resolver, so it must refuse the
// same value and retain OriginalPath on the valid one.
func TestCaptureURLValidationCoversAddTask(t *testing.T) {
	project := newProject("p1", "Alpha")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.AddTask(app.AddTaskRequest{
		ProjectID:   project.ID,
		Title:       "Read the spec",
		Attachments: []string{"https://"},
	})
	if err == nil {
		t.Fatal("AddTask succeeded, want a refusal naming the URL shape")
	}
	if !strings.Contains(err.Error(), "absolute http or https URL") {
		t.Errorf("error = %v, want the canonical URL diagnostic", err)
	}
	if len(project.Tasks) != 0 {
		t.Fatalf("project holds %d tasks, want the add to have aborted", len(project.Tasks))
	}

	result, err := svc.AddTask(app.AddTaskRequest{
		ProjectID:   project.ID,
		Title:       "Read the spec",
		Attachments: []string{"https://example.com/spec.pdf"},
	})
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}
	task := project.GetTaskByID(result.TaskID)
	if task == nil || len(task.Attachments) != 1 {
		t.Fatal("added task has no attachment")
	}
	if got := task.Attachments[0].OriginalPath; got != "https://example.com/spec.pdf" {
		t.Errorf("OriginalPath = %q, want the URL", got)
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

// A file whose name holds spaces is an ordinary attachment. The grammar hands
// the path on whole once a quoted or escaped span is normalized, and a space is
// not a separator to SafeChildName, so the copy keeps the name the file had.
func TestCaptureCopiesAFileWhoseNameHoldsSpaces(t *testing.T) {
	source := filepath.Join(t.TempDir(), "Design Notes v2.pdf")
	if err := os.WriteFile(source, []byte("x"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	project := newProject("p1", "Alpha")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	result, err := svc.Capture(app.CaptureRequest{
		ProjectID:   project.ID,
		Title:       "Read the spec",
		Attachments: []string{source},
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	task := project.GetTaskByID(result.TaskID)
	if task == nil || len(task.Attachments) != 1 {
		t.Fatalf("captured task = %+v, want exactly one attachment", task)
	}
	if got := task.Attachments[0].Filename; got != "Design Notes v2.pdf" {
		t.Errorf("Filename = %q, want the name the file already had", got)
	}
	if got := task.Attachments[0].OriginalPath; got != source {
		t.Errorf("OriginalPath = %q, want %q", got, source)
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
