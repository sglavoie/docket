package app_test

import (
	"errors"
	"testing"

	"github.com/sglavoie/tt/internal/app"
)

// A capture that names its task identity creates the task under exactly that
// identity, and a second capture naming the same one is refused rather than
// creating a duplicate, which is what makes an importer's replay detectable.
func TestCaptureWithTaskIDTakesThatIdentityOnce(t *testing.T) {
	const id = "5b0c7a4e-9d1f-8a2b-b3c4-d5e6f7a8b9c0"
	project := newProject("p1", "Alpha")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	results, err := svc.CaptureMany([]app.CaptureRequest{{ProjectID: project.ID, Title: "Shared link", TaskID: id}})
	if err != nil {
		t.Fatalf("CaptureMany: %v", err)
	}
	if results[0].TaskID != id || project.GetTaskByID(id) == nil {
		t.Fatalf("captured task id = %q, want %q", results[0].TaskID, id)
	}

	_, err = svc.CaptureMany([]app.CaptureRequest{{ProjectID: project.ID, Title: "Shared link", TaskID: id}})
	var invalid *app.InvalidInputError
	if !errors.As(err, &invalid) {
		t.Fatalf("second capture with the same id: err = %v, want an invalid input refusal", err)
	}
	if count := len(project.Tasks); count != 1 {
		t.Fatalf("project holds %d tasks after the refused replay, want 1", count)
	}

	for _, bad := range []string{"not-a-uuid", "5B0C7A4E-9D1F-8A2B-B3C4-D5E6F7A8B9C0"} {
		if _, err := svc.Capture(app.CaptureRequest{ProjectID: project.ID, Title: "x", TaskID: bad}); !errors.As(err, &invalid) {
			t.Errorf("TaskID %q: err = %v, want an invalid input refusal", bad, err)
		}
	}
}
