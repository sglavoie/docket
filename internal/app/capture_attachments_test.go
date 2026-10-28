package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
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
	if _, err := svc.AttachURL(repo.loc(plain.TaskID), link); err != nil {
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

const mailLink = "message://%3Cabc@example.com%3E"

// Mail's message: link is a URL attachment, though url.Parse refuses it: the
// scheme and a non-empty remainder decide it, and it is stored as given.
func TestResolveAttachmentInputClassifiesAMailLinkAsAURL(t *testing.T) {
	for _, value := range []string{mailLink, "MESSAGE://%3Cabc@example.com%3E"} {
		resolved, err := app.ResolveAttachmentInput(value)
		if err != nil {
			t.Fatalf("ResolveAttachmentInput(%q): %v", value, err)
		}
		if resolved.Kind != app.AttachmentInputURL || resolved.URL != value {
			t.Errorf("ResolveAttachmentInput(%q) = %+v, want a URL stored as given", value, resolved)
		}
	}
}

// A message: link naming no message is refused as a link, by name, never read
// as a missing file.
func TestCaptureRefusesAMailLinkNamingNoMessage(t *testing.T) {
	for _, value := range []string{"message:", "message://", "message:%3Cabc%3E"} {
		t.Run(value, func(t *testing.T) {
			project := newProject("p1", "Alpha")
			svc := newTestService(newFakeRepo(project))
			_, err := svc.Capture(app.CaptureRequest{ProjectID: project.ID, Title: "Reply", Attachments: []string{value}})
			if err == nil {
				t.Fatal("Capture succeeded, want a refusal")
			}
			if !strings.Contains(err.Error(), value+":") || !strings.Contains(err.Error(), "message: link") {
				t.Errorf("error = %v, want it to name %q and the message: link", err, value)
			}
			if strings.Contains(err.Error(), "file not found") {
				t.Errorf("error = %v, want a link refusal, not a missing file", err)
			}
			if len(project.Tasks) != 0 {
				t.Errorf("project holds %d tasks, want the capture to have aborted", len(project.Tasks))
			}
		})
	}
}

// A captured Mail link survives the canonical writer: stored, reloaded, and
// written again byte for byte.
func TestCapturedMailLinkRoundTripsByteIdentical(t *testing.T) {
	dataDir := t.TempDir()
	commitSvc := storage.NewCommitService(filepath.Join(dataDir, ".docket", "transactions"), []string{dataDir})
	project := realTestProject(t, dataDir, "alpha", "seed task")
	repo := newStorageRepo(commitSvc, project)
	repo.seed(t)
	svc := app.NewServiceWithArtifacts(repo, nil, nil, nil)

	if _, err := svc.Capture(app.CaptureRequest{ProjectID: project.ID, Title: "Reply to Ana", Attachments: []string{mailLink}}); err != nil {
		t.Fatalf("Capture: %v", err)
	}
	stored, err := os.ReadFile(storage.TasksFilePath(project.Path))
	if err != nil {
		t.Fatalf("read tasks.md: %v", err)
	}
	reloaded := &models.Project{ID: project.ID, Slug: project.Slug, Name: project.Name, Path: project.Path}
	if err = storage.LoadProject(reloaded); err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	var found *models.Task
	for _, task := range reloaded.Tasks {
		if task.Title == "Reply to Ana" {
			found = task
		}
	}
	if found == nil || len(found.Attachments) != 1 {
		t.Fatalf("reloaded task = %+v, want one attachment", found)
	}
	if got := found.Attachments[0]; !got.IsURL || got.Filename != mailLink || got.OriginalPath != mailLink {
		t.Errorf("reloaded attachment = %+v, want the Mail link as a URL", got)
	}
	rewritten, err := storage.WriteProjectMarkdown(reloaded)
	if err != nil {
		t.Fatalf("WriteProjectMarkdown: %v", err)
	}
	if rewritten != string(stored) {
		t.Errorf("round trip changed tasks.md:\n--- stored\n%s\n--- rewritten\n%s", stored, rewritten)
	}
}

// A folder is not a file: it is refused by name before anything is staged.
func TestCaptureRefusesAnAttachmentNamingAFolder(t *testing.T) {
	dir := t.TempDir()
	project := newProject("p1", "Alpha")
	svc := newTestService(newFakeRepo(project))
	_, err := svc.Capture(app.CaptureRequest{ProjectID: project.ID, Title: "Read", Attachments: []string{dir}})
	if err == nil || !strings.Contains(err.Error(), "not a file: "+dir) {
		t.Fatalf("Capture error = %v, want a refusal naming the folder", err)
	}
	if len(project.Tasks) != 0 {
		t.Errorf("project holds %d tasks, want the capture to have aborted", len(project.Tasks))
	}
}

// A copy refused for permission names the file and the macOS setting that
// grants it, and the commit stages nothing.
func TestCaptureRefusesAnUnreadableFileWithThePrivacyWording(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a 0o000 file")
	}
	source := filepath.Join(t.TempDir(), "locked.pdf")
	if err := os.WriteFile(source, []byte("secret"), 0o000); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	dataDir := t.TempDir()
	commitSvc := storage.NewCommitService(filepath.Join(dataDir, ".docket", "transactions"), []string{dataDir})
	project := realTestProject(t, dataDir, "alpha", "seed task")
	repo := newStorageRepo(commitSvc, project)
	repo.seed(t)
	before, err := os.ReadFile(storage.TasksFilePath(project.Path))
	if err != nil {
		t.Fatalf("read tasks.md: %v", err)
	}
	svc := app.NewServiceWithArtifacts(repo, nil, nil, nil)

	_, err = svc.Capture(app.CaptureRequest{ProjectID: project.ID, Title: "Read the locked file", Attachments: []string{source}})
	want := "Docket can't read " + source + ": allow Docket in Privacy & Security › Files and Folders"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("Capture error = %v, want %q", err, want)
	}
	if strings.Contains(err.Error(), "permission denied") {
		t.Errorf("error = %v, want the raw errno kept out of the wording", err)
	}
	after, err := os.ReadFile(storage.TasksFilePath(project.Path))
	if err != nil {
		t.Fatalf("read tasks.md: %v", err)
	}
	if string(after) != string(before) {
		t.Error("tasks.md changed, want nothing committed")
	}
	if entries, _ := os.ReadDir(filepath.Join(project.Path, ".attachments")); len(entries) != 0 {
		t.Errorf("attachments directory holds %d entries, want nothing staged", len(entries))
	}
}
