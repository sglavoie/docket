package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
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

func assertGuidanceExcludes(t *testing.T, state string, spec GuidanceSpec, excluded ...string) {
	t.Helper()
	got := guidanceEntryText(spec)
	for _, entry := range excluded {
		if strings.Contains(got, entry) {
			t.Errorf("%s guidance = %q, unexpectedly contains %q", state, got, entry)
		}
	}
}

func TestChecklistGuidanceTracksBrowseEmptyInputAndConfirmation(t *testing.T) {
	project, task := newTaskToolGuidanceFixture(t)
	task.Checklist = []models.ChecklistItem{{Text: "one"}, {Text: "two"}}
	model := NewTaskChecklistModel(task, project)

	spec, ok := model.guidanceSpec()
	assertGuidanceContains(t, "checklist browse", spec, ok,
		"Space/Enter toggle", "a add", "e edit", "d delete", "J/K reorder", "Esc close")
	if spec.Rows != 1 {
		t.Fatalf("checklist browse rows = %d, want 1", spec.Rows)
	}

	task.Checklist = nil
	spec, ok = model.guidanceSpec()
	assertGuidanceContains(t, "checklist empty", spec, ok, "a add", "Esc close")
	assertGuidanceExcludes(t, "checklist empty", spec, "toggle", "edit", "delete", "copy", "reorder", "navigate")

	model.addMode = true
	spec, ok = model.guidanceSpec()
	assertRequiredGuidance(t, "checklist add", spec, ok, "Enter save", "Esc cancel")
	if spec.Rows != 2 {
		t.Fatalf("checklist add rows = %d, want 2", spec.Rows)
	}
	model.err = errors.New("item is empty")
	spec, ok = model.guidanceSpec()
	assertRequiredGuidance(t, "checklist invalid add", spec, ok, "Enter retry", "Esc cancel")

	model.confirm.Show("discard-add", "Discard item?")
	spec, ok = model.guidanceSpec()
	assertRequiredGuidance(t, "checklist confirmation", spec, ok, "y confirm", "n/Esc cancel")
}

func TestCommentsGuidanceTracksBrowseEmptyInputExpandedAndConfirmation(t *testing.T) {
	project, task := newTaskToolGuidanceFixture(t)
	task.Comments = []*models.Comment{{Text: "comment", CreatedAt: time.Now()}}
	model := NewTaskCommentsModel(task, project)

	spec, ok := model.guidanceSpec()
	assertGuidanceContains(t, "comments browse", spec, ok,
		"Enter expand", "a add", "e edit", "d delete", "y copy", "Y copy all", "Esc close")
	if spec.Rows != 1 {
		t.Fatalf("comments browse rows = %d, want 1", spec.Rows)
	}

	model.expandedView = true
	spec, ok = model.guidanceSpec()
	assertRequiredGuidance(t, "comments expanded", spec, ok, "Enter close", "Esc close")
	assertGuidanceExcludes(t, "comments expanded", spec, "add", "edit", "delete", "copy")

	model.expandedView = false
	model.addMode = true
	spec, ok = model.guidanceSpec()
	assertRequiredGuidance(t, "comments add", spec, ok, "Ctrl+S save", "Esc cancel")
	assertGuidanceContains(t, "comments add", spec, ok, "Ctrl+E editor", "Shift+Enter newline")

	model.confirm.Show("discard-add", "Discard comment?")
	spec, ok = model.guidanceSpec()
	assertRequiredGuidance(t, "comments confirmation", spec, ok, "y confirm", "n/Esc cancel")

	model.confirm.Cancel()
	model.addMode = false
	task.Comments = nil
	spec, ok = model.guidanceSpec()
	assertGuidanceContains(t, "comments empty", spec, ok, "a add", "Esc close")
	assertGuidanceExcludes(t, "comments empty", spec, "expand", "edit", "delete", "copy", "navigate")
}

func TestAttachmentGuidanceTracksFileCodeURLAndEmptyVariants(t *testing.T) {
	project, task := newTaskToolGuidanceFixture(t)
	task.Attachments = []models.Attachment{
		{Filename: "file.txt", OriginalPath: "/tmp/file.txt"},
		{Filename: "code.go", OriginalPath: "/tmp/code.go", IsCodeRef: true, LineNumber: 12},
		{Filename: "https://example.com", OriginalPath: "https://example.com", IsURL: true},
	}
	model := NewTaskAttachmentsModel(task, project, nil, AttachmentCoordinator{})

	spec, ok := model.guidanceSpec()
	assertGuidanceContains(t, "file attachment", spec, ok,
		"o open", "e edit", "Ctrl+O original", "y copy", "d delete", "u unlink", "J/K reorder", "Esc close")
	assertGuidanceExcludes(t, "file attachment", spec, "copy path:line")

	model.cursor = 1
	spec, ok = model.guidanceSpec()
	assertGuidanceContains(t, "code attachment", spec, ok, "o open", "e edit", "Y copy path:line", "J/K reorder")
	assertGuidanceExcludes(t, "code attachment", spec, "Ctrl+O original")

	model.cursor = 2
	spec, ok = model.guidanceSpec()
	assertGuidanceContains(t, "URL attachment", spec, ok, "o open", "y copy", "d delete", "u unlink")
	assertGuidanceExcludes(t, "URL attachment", spec, "e edit", "Ctrl+O original", "copy path:line")

	task.Attachments = nil
	model.cursor = 0
	spec, ok = model.guidanceSpec()
	assertGuidanceContains(t, "attachments empty", spec, ok, "a add", "Esc close")
	assertGuidanceExcludes(t, "attachments empty", spec, "open", "edit", "copy", "delete", "unlink", "reorder", "navigate")

	model.addMode = true
	spec, ok = model.guidanceSpec()
	assertRequiredGuidance(t, "attachment add", spec, ok, "Enter add", "Esc cancel")
}

func TestTaskToolGuidanceFitsNarrowBudgetsAndViews(t *testing.T) {
	project, task := newTaskToolGuidanceFixture(t)
	for i := 0; i < 20; i++ {
		task.Checklist = append(task.Checklist, models.ChecklistItem{Text: "checklist item"})
		task.Comments = append(task.Comments, &models.Comment{Text: "comment", CreatedAt: time.Now()})
		task.Attachments = append(task.Attachments, models.Attachment{
			Filename:     "https://example.com",
			OriginalPath: "https://example.com",
			IsURL:        true,
		})
	}

	checklist := NewTaskChecklistModel(task, project)
	checklist.SetSize(36, 12)
	checklist.cursor = len(task.Checklist) - 1
	comments := NewTaskCommentsModel(task, project)
	comments.SetSize(36, 16)
	comments.cursor = len(task.Comments) - 1
	attachments := NewTaskAttachmentsModel(task, project, nil, AttachmentCoordinator{})
	attachments.SetSize(36, 16)
	attachments.cursor = len(task.Attachments) - 1

	for name, rendered := range map[string]string{
		"checklist":   checklist.View().Content,
		"comments":    comments.View().Content,
		"attachments": attachments.View().Content,
	} {
		height := 16
		if name == "checklist" {
			height = 12
		}
		assertFitsWithin(t, rendered, 36, height)
		plain := ansi.Strip(rendered)
		if strings.Contains(plain, "Press 'a'") || strings.Contains(plain, "Enter:") || strings.Contains(plain, "Esc:") {
			t.Errorf("%s retained quoted-key or colon footer grammar:\n%s", name, plain)
		}
	}

	for _, tc := range []struct {
		name string
		spec GuidanceSpec
	}{
		{"checklist", func() GuidanceSpec { spec, _ := checklist.guidanceSpec(); return spec }()},
		{"comments", func() GuidanceSpec { spec, _ := comments.guidanceSpec(); return spec }()},
		{"attachments", func() GuidanceSpec { spec, _ := attachments.guidanceSpec(); return spec }()},
	} {
		if tc.spec.Rows != 1 {
			t.Errorf("%s browse row budget = %d, want 1", tc.name, tc.spec.Rows)
		}
		assertGuidanceFits(t, tc.spec, 24)
	}
}
