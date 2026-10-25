package taskdetail

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/coordinator"
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
	model := newTestCommentsPanel(task, project)

	spec, ok := model.guidanceSpec()
	assertGuidanceContains(t, "comments browse", spec, ok,
		"Enter expand", "a add", "e edit", "Space actions", "y copy", "Y copy all", "Esc close", "? help")
	// The panel's keys are the shell footer, so no breadcrumb row is left to
	// name the exit or the help: both are required here instead.
	assertRequiredGuidance(t, "comments browse", spec, ok, "Esc close", "? help")
	assertGuidanceExcludes(t, "comments browse", spec, "delete")
	if spec.Rows != taskToolGuidanceRows {
		t.Fatalf("comments browse rows = %d, want %d", spec.Rows, taskToolGuidanceRows)
	}

	model.ExpandedView = true
	spec, ok = model.guidanceSpec()
	assertRequiredGuidance(t, "comments expanded", spec, ok, "Enter close", "Esc close")
	assertGuidanceExcludes(t, "comments expanded", spec, "add", "edit", "delete", "copy")

	model.ExpandedView = false
	model.AddMode = true
	spec, ok = model.guidanceSpec()
	assertRequiredGuidance(t, "comments add", spec, ok, "Ctrl+S save", "Esc cancel")
	assertGuidanceContains(t, "comments add", spec, ok, "Ctrl+E editor", "Alt+Enter newline")

	// Esc over typed text raises the discard question instead of closing, so the
	// entry that key owns changes with the draft rather than with the mode.
	model.TextInput.SetValue("half a thought")
	spec, ok = model.guidanceSpec()
	assertRequiredGuidance(t, "comments drafted add", spec, ok, "Ctrl+S save", "Esc discard")

	model.Confirm.Show("discard-add", "Discard comment?")
	spec, ok = model.guidanceSpec()
	assertRequiredGuidance(t, "comments confirmation", spec, ok, "Y discard", "N/Esc keep editing")

	// A destructive question keeps the ordinary confirmation chrome: declining it
	// leaves the list alone rather than returning to a composer.
	model.Confirm.Show("delete", "Delete comment [0]?")
	spec, ok = model.guidanceSpec()
	assertRequiredGuidance(t, "comments delete confirmation", spec, ok, "y confirm", "n/Esc cancel")

	model.Confirm.Cancel()
	model.TextInput.SetValue("")
	model.AddMode = false
	// The panel reads its owner's entries rather than the task pointer, so an
	// out-of-band removal reaches it the way the shell delivers one: through the
	// reload hook.
	task.Comments = nil
	model.RefreshAfterReload()
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
		"Enter open", "E editor", "y copy path", "Space actions", "J/K reorder", "Esc close", "? help")
	assertRequiredGuidance(t, "file attachment", spec, ok, "Esc close", "? help")
	assertGuidanceExcludes(t, "file attachment", spec, "copy path:line", "delete", "unlink", "original")

	model.Cursor = 1
	spec, ok = model.guidanceSpec()
	assertGuidanceContains(t, "code attachment", spec, ok, "Enter open", "E editor", "Y copy path:line", "J/K reorder")
	assertGuidanceExcludes(t, "code attachment", spec, "original")

	model.Cursor = 2
	spec, ok = model.guidanceSpec()
	assertGuidanceContains(t, "URL attachment", spec, ok, "Enter open", "y copy path", "Space actions")
	assertGuidanceExcludes(t, "URL attachment", spec, "E editor", "original", "copy path:line", "delete", "unlink")

	task.Attachments = nil
	model.Cursor = 0
	spec, ok = model.guidanceSpec()
	assertGuidanceContains(t, "attachments empty", spec, ok, "a add", "Esc close")
	assertGuidanceExcludes(t, "attachments empty", spec, "open", "edit", "copy", "actions", "reorder", "navigate")

	model.AddMode = true
	spec, ok = model.guidanceSpec()
	assertRequiredGuidance(t, "attachment add", spec, ok, "Enter add", "Esc cancel")

	model.TextInput.SetValue("/tmp/file.txt")
	spec, ok = model.guidanceSpec()
	assertRequiredGuidance(t, "drafted attachment add", spec, ok, "Enter add", "Esc discard")
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

	comments := newTestCommentsPanel(task, project)
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
		if tc.spec.Rows != taskToolGuidanceRows {
			t.Errorf("%s browse row budget = %d, want %d", tc.name, tc.spec.Rows, taskToolGuidanceRows)
		}
		assertGuidanceFits(t, tc.spec, 24)
		// At the app's floor the wrapped footer still carries the menu that is
		// the only route to Delete, beside the exit and the help.
		floor := ansi.Strip(chrome.RenderGuidanceText(tc.spec, chrome.MinTerminalWidth))
		for _, entry := range []string{"Space actions", "Esc close", "? help"} {
			if !strings.Contains(floor, entry) {
				t.Errorf("%s footer at the floor sheds %q:\n%s", tc.name, entry, floor)
			}
		}
	}
}
