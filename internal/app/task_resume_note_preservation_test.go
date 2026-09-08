package app_test

import (
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// resumeNoteFixture is the note every case in this file carries. It is prose no
// mutation in the file has any reason to touch, which is the whole point: a
// mutation that rebuilt the task from the fields it knows about would drop it.
const resumeNoteFixture = "stopped at the failing merge case"

// persistedResumeNote renders the project the repo last committed through the
// live canonical writer and reads the note back out of the parsed bytes, so the
// assertion is about what durable storage holds and not about the in-memory
// pointer the mutation happened to keep.
func persistedResumeNote(t *testing.T, repo *fakeRepo, projectID, taskID string) string {
	t.Helper()
	document, ok := repo.documents[projectID]
	if !ok {
		t.Fatalf("no committed document for project %q", projectID)
	}
	content, err := document.WriteCurrent()
	if err != nil {
		t.Fatalf("WriteCurrent: %v", err)
	}
	parsed, err := storage.ParseProjectDocument(content)
	if err != nil {
		t.Fatalf("ParseProjectDocument: %v", err)
	}
	for _, task := range parsed.Tasks {
		if task != nil && task.ID == taskID {
			return task.ResumeNote
		}
	}
	t.Fatalf("project %q no longer holds task %q", projectID, taskID)
	return ""
}

// TestUnrelatedMutationsPreserveTheResumeNote is the retention audit expressed as
// a test. Each case names a field the mutation does write; none of them names the
// resume note, so each one must leave it exactly where it was.
func TestUnrelatedMutationsPreserveTheResumeNote(t *testing.T) {
	newTitle := "Renamed"
	for name, mutate := range map[string]func(*app.Service, app.TaskLocator) error{
		"edit the title": func(service *app.Service, locator app.TaskLocator) error {
			_, err := service.Edit(app.EditRequest{Locator: locator, Title: &newTitle})
			return err
		},
		"transition the state": func(service *app.Service, locator app.TaskLocator) error {
			_, err := service.ExecuteTaskOperation(app.TaskOperationRequest{
				Operation: app.TaskOperationSetState,
				Targets:   []app.TaskLocator{locator},
				State:     models.StateDoing,
			})
			return err
		},
		"replace the tags": func(service *app.Service, locator app.TaskLocator) error {
			_, err := service.ExecuteTaskOperation(app.TaskOperationRequest{
				Operation: app.TaskOperationSetTags,
				Targets:   []app.TaskLocator{locator},
				Tags:      []string{"infra"},
			})
			return err
		},
		"reorder within the project": func(service *app.Service, locator app.TaskLocator) error {
			_, err := service.MoveTaskToBottom(locator)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			task := newTask("noted", 1, "Noted")
			task.ResumeNote = resumeNoteFixture
			project := newProject("project", "Project", task, newTask("other", 2, "Other"))
			repo := newFakeRepo(project)

			if err := mutate(newTestService(repo), repo.loc("noted")); err != nil {
				t.Fatalf("mutation: %v", err)
			}
			if got := persistedResumeNote(t, repo, "project", "noted"); got != resumeNoteFixture {
				t.Fatalf("resume note after the mutation = %q, want %q", got, resumeNoteFixture)
			}
		})
	}
}

// TestCrossProjectMovePreservesTheResumeNote covers the one mutation that writes
// two documents: the note has to survive being removed from the source bytes and
// written into the destination's, on the moved root and on a descendant that was
// never named by the selection.
func TestCrossProjectMovePreservesTheResumeNote(t *testing.T) {
	root := newTask("root", 1, "Root")
	root.ResumeNote = resumeNoteFixture
	child := childOf("child", 2, "Child", root)
	child.ResumeNote = "half of the child is done"
	source := newProject("source", "Source", root, child)
	destination := newProject("destination", "Destination")
	repo := newFakeRepo(source, destination)

	if _, err := newTestService(repo).BulkMoveToProject(repoLocators(repo, []string{"root"}), "destination"); err != nil {
		t.Fatalf("BulkMoveToProject: %v", err)
	}

	if got := persistedResumeNote(t, repo, "destination", "root"); got != resumeNoteFixture {
		t.Fatalf("moved root's resume note = %q, want %q", got, resumeNoteFixture)
	}
	if got := persistedResumeNote(t, repo, "destination", "child"); got != child.ResumeNote {
		t.Fatalf("moved child's resume note = %q, want %q", got, child.ResumeNote)
	}
}
