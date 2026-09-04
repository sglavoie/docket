package taskdetail

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/coordinator"
)

func newTaskToolGuidanceFixture(t *testing.T) (*models.Project, *models.Task) {
	t.Helper()
	project, err := models.NewProject("project", "Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	task, err := models.NewGenericTask("task", 1, "Task")
	if err != nil {
		t.Fatalf("NewGenericTask: %v", err)
	}
	project.Tasks = []*models.Task{task}
	return project, task
}

func assertGuidanceExcludes(t *testing.T, state string, spec chrome.GuidanceSpec, excluded ...string) {
	t.Helper()
	got := guidanceEntryText(spec)
	for _, entry := range excluded {
		if strings.Contains(got, entry) {
			t.Errorf("%s guidance = %q, unexpectedly contains %q", state, got, entry)
		}
	}
}

func TestCommentsGuidanceTracksBrowseEmptyInputExpandedAndConfirmation(t *testing.T) {
	project, task := newTaskToolGuidanceFixture(t)
	task.Comments = []*models.Comment{{Text: "comment", CreatedAt: time.Now()}}
	model := NewTaskCommentsModel(task, project)

	spec, ok := model.guidanceSpec()
	assertGuidanceContains(t, "comments browse", spec, ok,
		"Enter expand", "a add", "e edit", "Space actions", "y copy", "Y copy all", "Esc close")
	assertGuidanceExcludes(t, "comments browse", spec, "delete")
	if spec.Rows != 1 {
		t.Fatalf("comments browse rows = %d, want 1", spec.Rows)
	}

	model.ExpandedView = true
	spec, ok = model.guidanceSpec()
	assertRequiredGuidance(t, "comments expanded", spec, ok, "Enter close", "Esc close")
	assertGuidanceExcludes(t, "comments expanded", spec, "add", "edit", "delete", "copy")

	model.ExpandedView = false
	model.AddMode = true
	spec, ok = model.guidanceSpec()
	assertRequiredGuidance(t, "comments add", spec, ok, "Ctrl+S save", "Esc cancel")
	assertGuidanceContains(t, "comments add", spec, ok, "Ctrl+E editor", "Shift+Enter newline")

	model.Confirm.Show("discard-add", "Discard comment?")
	spec, ok = model.guidanceSpec()
	assertRequiredGuidance(t, "comments confirmation", spec, ok, "y confirm", "n/Esc cancel")

	model.Confirm.Cancel()
	model.AddMode = false
	task.Comments = nil
	spec, ok = model.guidanceSpec()
	assertGuidanceContains(t, "comments empty", spec, ok, "a add", "Esc close")
	assertGuidanceExcludes(t, "comments empty", spec, "expand", "edit", "delete", "copy", "actions", "navigate")
}

func TestAttachmentGuidanceTracksFileCodeURLAndEmptyVariants(t *testing.T) {
	project, task := newTaskToolGuidanceFixture(t)
	task.Attachments = []models.Attachment{
		{Filename: "file.txt", OriginalPath: "/tmp/file.txt"},
		{Filename: "code.go", OriginalPath: "/tmp/code.go", IsCodeRef: true, LineNumber: 12},
		{Filename: "https://example.com", OriginalPath: "https://example.com", IsURL: true},
	}
	model := NewTaskAttachmentsModel(task, project, nil, coordinator.AttachmentCoordinator{})

	spec, ok := model.guidanceSpec()
	assertGuidanceContains(t, "file attachment", spec, ok,
		"Enter open", "E editor", "y copy", "Space actions", "J/K reorder", "Esc close")
	assertGuidanceExcludes(t, "file attachment", spec, "copy path:line", "delete", "unlink", "original")

	model.Cursor = 1
	spec, ok = model.guidanceSpec()
	assertGuidanceContains(t, "code attachment", spec, ok, "Enter open", "E editor", "Y copy path:line", "J/K reorder")
	assertGuidanceExcludes(t, "code attachment", spec, "original")

	model.Cursor = 2
	spec, ok = model.guidanceSpec()
	assertGuidanceContains(t, "URL attachment", spec, ok, "Enter open", "y copy", "Space actions")
	assertGuidanceExcludes(t, "URL attachment", spec, "E editor", "original", "copy path:line", "delete", "unlink")

	task.Attachments = nil
	model.Cursor = 0
	spec, ok = model.guidanceSpec()
	assertGuidanceContains(t, "attachments empty", spec, ok, "a add", "Esc close")
	assertGuidanceExcludes(t, "attachments empty", spec, "open", "edit", "copy", "actions", "reorder", "navigate")

	model.AddMode = true
	spec, ok = model.guidanceSpec()
	assertRequiredGuidance(t, "attachment add", spec, ok, "Enter add", "Esc cancel")
}

func TestTaskToolGuidanceFitsNarrowBudgetsAndViews(t *testing.T) {
	project, task := newTaskToolGuidanceFixture(t)
	for i := 0; i < 20; i++ {
		task.Comments = append(task.Comments, &models.Comment{Text: "comment", CreatedAt: time.Now()})
		task.Attachments = append(task.Attachments, models.Attachment{
			Filename:     "https://example.com",
			OriginalPath: "https://example.com",
			IsURL:        true,
		})
	}

	comments := NewTaskCommentsModel(task, project)
	comments.SetSize(36, 16)
	comments.Cursor = len(task.Comments) - 1
	attachments := NewTaskAttachmentsModel(task, project, nil, coordinator.AttachmentCoordinator{})
	attachments.SetSize(36, 16)
	attachments.Cursor = len(task.Attachments) - 1

	for name, rendered := range map[string]string{
		"comments":    comments.View().Content,
		"attachments": attachments.View().Content,
	} {
		assertFitsWithin(t, rendered, 36, 16)
		plain := ansi.Strip(rendered)
		if strings.Contains(plain, "Press 'a'") || strings.Contains(plain, "Enter:") || strings.Contains(plain, "Esc:") {
			t.Errorf("%s retained quoted-key or colon footer grammar:\n%s", name, plain)
		}
	}

	for _, tc := range []struct {
		name string
		spec chrome.GuidanceSpec
	}{
		{"comments", func() chrome.GuidanceSpec { spec, _ := comments.guidanceSpec(); return spec }()},
		{"attachments", func() chrome.GuidanceSpec { spec, _ := attachments.guidanceSpec(); return spec }()},
	} {
		if tc.spec.Rows != 1 {
			t.Errorf("%s browse row budget = %d, want 1", tc.name, tc.spec.Rows)
		}
		assertGuidanceFits(t, tc.spec, 24)
	}
}
