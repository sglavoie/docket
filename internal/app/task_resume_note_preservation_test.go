package app_test

import (
	"path/filepath"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
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
	newBody := "Body rewritten in the external editor."
	for name, mutate := range map[string]func(*app.Service, *fakeRepo, app.TaskLocator) error{
		"edit the title": func(service *app.Service, _ *fakeRepo, locator app.TaskLocator) error {
			_, err := service.Edit(app.EditRequest{Locator: locator, Title: &newTitle})
			return err
		},
		"edit the description": func(service *app.Service, _ *fakeRepo, locator app.TaskLocator) error {
			_, err := service.Edit(app.EditRequest{Locator: locator, Description: &newBody})
			return err
		},
		"transition the state": func(service *app.Service, _ *fakeRepo, locator app.TaskLocator) error {
			_, err := service.ExecuteTaskOperation(app.TaskOperationRequest{
				Operation: app.TaskOperationSetState,
				Targets:   []app.TaskLocator{locator},
				State:     models.StateDoing,
			})
			return err
		},
		// Done and Canceled are the two states that end a task's life, which is
		// exactly when its note stops being reachable through Peek and therefore
		// when a silent loss would go unnoticed.
		"complete the task": func(service *app.Service, _ *fakeRepo, locator app.TaskLocator) error {
			_, err := service.ExecuteTaskOperation(app.TaskOperationRequest{
				Operation: app.TaskOperationSetState,
				Targets:   []app.TaskLocator{locator},
				State:     models.StateDone,
			})
			return err
		},
		"cancel the task": func(service *app.Service, _ *fakeRepo, locator app.TaskLocator) error {
			_, err := service.ExecuteTaskOperation(app.TaskOperationRequest{
				Operation: app.TaskOperationSetState,
				Targets:   []app.TaskLocator{locator},
				State:     models.StateCanceled,
			})
			return err
		},
		"replace the tags": func(service *app.Service, _ *fakeRepo, locator app.TaskLocator) error {
			_, err := service.ExecuteTaskOperation(app.TaskOperationRequest{
				Operation: app.TaskOperationSetTags,
				Targets:   []app.TaskLocator{locator},
				Tags:      []string{"infra"},
			})
			return err
		},
		"reorder within the project": func(service *app.Service, _ *fakeRepo, locator app.TaskLocator) error {
			_, err := service.MoveTaskToBottom(locator)
			return err
		},
		"reparent under a sibling": func(service *app.Service, repo *fakeRepo, locator app.TaskLocator) error {
			parent := repo.loc("other").TaskID
			_, err := service.Reparent(locator, &parent)
			return err
		},
		"detach back to a root": func(service *app.Service, repo *fakeRepo, locator app.TaskLocator) error {
			parent := repo.loc("other").TaskID
			if _, err := service.Reparent(locator, &parent); err != nil {
				return err
			}
			_, err := service.Reparent(locator, nil)
			return err
		},
		// The inline editor and the external-editor round trip are one request:
		// title, body, tags, and destination land together, and the note is not
		// among the fields that request can name.
		"edit the whole task line": func(service *app.Service, _ *fakeRepo, locator app.TaskLocator) error {
			destination := app.CaptureDestinationBacklog
			_, err := service.EditTaskLine(app.TaskLineEditRequest{
				Source:      locator,
				Title:       newTitle,
				Description: &newBody,
				Destination: &destination,
				TagsToAdd:   []string{"urgent"},
			})
			return err
		},
		// An outline buffer is declarative and complete for the tasks it dumped,
		// so an absent field is a clear rather than an omission. The buffer holds
		// no resume note at all, which is what makes this the case most likely to
		// erase one.
		"apply an outline buffer": func(service *app.Service, repo *fakeRepo, locator app.TaskLocator) error {
			_, err := service.ApplyOutlinePlan(app.OutlinePlanRequest{
				ProjectID:        locator.ProjectID,
				ExpectedRevision: repo.revisions[locator.ProjectID],
				ScopeTaskIDs:     []string{"noted", "other"},
				Lines: []app.OutlinePlanLine{
					{SourceLine: 1, TaskID: "other", Depth: 0, Title: "Other"},
					{SourceLine: 2, TaskID: "noted", Depth: 1, Title: newTitle, Tags: []string{"infra"}},
				},
			})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			task := newTask("noted", 1, "Noted")
			task.ResumeNote = resumeNoteFixture
			project := newProject("project", "Project", task, newTask("other", 2, "Other"))
			repo := newFakeRepo(project)

			if err := mutate(newTestService(repo), repo, repo.loc("noted")); err != nil {
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

// reloadedResumeNote reads the note back out of the project's on-disk bytes,
// through the same loader the running app uses at launch. Every assertion below
// is therefore about what a restart would show, never about the pointer the
// mutation happened to keep.
func reloadedResumeNote(t *testing.T, project *models.Project, taskID string) string {
	t.Helper()
	return reloadedTaskLineEditTask(t, project, taskID).ResumeNote
}

// TestResumeNoteSurvivesReloadAndUndoRedo drives the note through the whole
// durable path: it is written to disk, read back by a fresh load, taken back by
// the workspace ledger, and re-applied by redo. The clear is included because
// absence is a value the ledger has to restore exactly as a note is.
func TestResumeNoteSurvivesReloadAndUndoRedo(t *testing.T) {
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".docket", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".docket", "transactions"), []string{dataDir}, ledger)

	project := realTestProject(t, dataDir, "alpha", "Noted")
	task := project.Tasks[0]
	task.ResumeNote = resumeNoteFixture
	repo := newStorageRepo(commitSvc, project)
	repo.seed(t)

	service := app.NewServiceWithArtifacts(repo, nil, nil, nil)
	locator := app.TaskLocator{ProjectID: project.ID, TaskID: task.ID}

	if got := reloadedResumeNote(t, project, task.ID); got != resumeNoteFixture {
		t.Fatalf("seeded note after reload = %q, want %q", got, resumeNoteFixture)
	}

	const rewritten = "picked it back up at the retry loop"
	if _, err := service.SetResumeNote(locator, rewritten); err != nil {
		t.Fatalf("SetResumeNote: %v", err)
	}
	if got := reloadedResumeNote(t, project, task.ID); got != rewritten {
		t.Fatalf("rewritten note after reload = %q, want %q", got, rewritten)
	}

	if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if got := reloadedResumeNote(t, project, task.ID); got != resumeNoteFixture {
		t.Fatalf("undone note = %q, want the seeded one", got)
	}
	if _, err := ledger.Redo(storage.GlobalScope()); err != nil {
		t.Fatalf("Redo: %v", err)
	}
	if got := reloadedResumeNote(t, project, task.ID); got != rewritten {
		t.Fatalf("redone note = %q, want the rewritten one", got)
	}

	// Clearing is the other direction of the same round trip: the emptied note
	// leaves no block behind, and undo puts the words back.
	if _, err := service.SetResumeNote(locator, ""); err != nil {
		t.Fatalf("SetResumeNote clear: %v", err)
	}
	if got := reloadedResumeNote(t, project, task.ID); got != "" {
		t.Fatalf("cleared note after reload = %q, want it absent", got)
	}
	if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("Undo the clear: %v", err)
	}
	if got := reloadedResumeNote(t, project, task.ID); got != rewritten {
		t.Fatalf("undone clear = %q, want the note back", got)
	}
}

// TestUndoingAnUnrelatedEditLeavesTheResumeNoteAlone is the other half of the
// ledger contract. An undo restores whole documents rather than the fields the
// mutation named, so a note written between a commit and its undo must not be
// rolled back with it — and a note that predates both must come back unchanged.
func TestUndoingAnUnrelatedEditLeavesTheResumeNoteAlone(t *testing.T) {
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".docket", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".docket", "transactions"), []string{dataDir}, ledger)

	project := realTestProject(t, dataDir, "alpha", "Before")
	task := project.Tasks[0]
	task.ResumeNote = resumeNoteFixture
	repo := newStorageRepo(commitSvc, project)
	repo.seed(t)

	service := app.NewServiceWithArtifacts(repo, nil, nil, nil)
	renamed := "After"
	if _, err := service.Edit(app.EditRequest{
		Locator: app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Title:   &renamed,
	}); err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if got := reloadedResumeNote(t, project, task.ID); got != resumeNoteFixture {
		t.Fatalf("note after the unrelated edit = %q, want %q", got, resumeNoteFixture)
	}

	if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	undone := reloadedTaskLineEditTask(t, project, task.ID)
	if undone.Title != "Before" {
		t.Fatalf("undone title = %q, want the pre-edit title", undone.Title)
	}
	if undone.ResumeNote != resumeNoteFixture {
		t.Fatalf("undone note = %q, want %q", undone.ResumeNote, resumeNoteFixture)
	}
}
