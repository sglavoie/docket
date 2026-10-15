package engine

import (
	"errors"
	"os"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/storage"
)

// TestCaptureCreatingProjectsIsOneTransaction files two tasks into a project
// the workspace did not hold. The project, its catalog record, and both tasks
// land as one transaction and one undo entry; undoing it removes the project
// and its tasks together, and redoing it brings both back.
func TestCaptureCreatingProjectsIsOneTransaction(t *testing.T) {
	env := newHeadlessEnvironment(t)
	runtime := openHeadless(t, env.config(true))
	unfiled := runtime.State().SystemUnfiledProject()
	ledger := runtime.Ledger()
	before, err := ledger.UndoCount(storage.LedgerScope{})
	if err != nil {
		t.Fatalf("UndoCount: %v", err)
	}

	creation, err := app.PrepareProjectCreation(runtime.State(), []app.NewProject{{ID: "11111111-1111-4111-8111-111111111111", Name: "garden"}})
	if err != nil {
		t.Fatalf("PrepareProjectCreation: %v", err)
	}
	garden := creation.Projects[0]
	results, err := runtime.Service().CaptureManyCreatingProjects(creation, []app.CaptureRequest{
		{ProjectID: garden.ID, Title: "Plant the beans"},
		{ProjectID: unfiled.ID, Title: "Buy compost"},
		{ProjectID: garden.ID, Title: "Water the beans"},
	})
	if err != nil {
		t.Fatalf("CaptureManyCreatingProjects: %v", err)
	}
	if len(results) != 3 || results[0].TransactionID == "" ||
		results[0].TransactionID != results[1].TransactionID || results[1].TransactionID != results[2].TransactionID {
		t.Fatalf("the capture did not commit as one transaction: %+v", results)
	}
	if after, _ := ledger.UndoCount(storage.LedgerScope{}); after != before+1 {
		t.Fatalf("undo entries = %d, want one more than %d", after, before)
	}
	if refreshErr := runtime.Refresh(); refreshErr != nil {
		t.Fatalf("Refresh: %v", refreshErr)
	}
	stored := runtime.State().GetProject("garden")
	if stored == nil || stored.ID != garden.ID || stored.Name != "garden" {
		t.Fatalf("the catalog does not hold the new project: %+v", stored)
	}
	requireGardenTasks(t, runtime, garden.ID, "Plant the beans", "Water the beans")

	if _, undoErr := ledger.Undo(storage.GlobalScope()); undoErr != nil {
		t.Fatalf("Undo: %v", undoErr)
	}
	if refreshErr := runtime.Refresh(); refreshErr != nil {
		t.Fatalf("Refresh after undo: %v", refreshErr)
	}
	if runtime.State().GetProject("garden") != nil {
		t.Fatal("undo left the created project in the catalog")
	}
	if _, statErr := os.Stat(storage.TasksFilePath(garden.Path)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("undo left the created project's tasks.md behind (stat err %v)", statErr)
	}
	snap, err := runtime.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	for _, task := range snap.ProjectByID(unfiled.ID).Tasks {
		if task.Title == "Buy compost" {
			t.Fatal("undo left the companion task in Unfiled")
		}
	}

	if _, redoErr := ledger.Redo(storage.GlobalScope()); redoErr != nil {
		t.Fatalf("Redo: %v", redoErr)
	}
	if refreshErr := runtime.Refresh(); refreshErr != nil {
		t.Fatalf("Refresh after redo: %v", refreshErr)
	}
	if runtime.State().GetProject("garden") == nil {
		t.Fatal("redo did not restore the created project")
	}
	requireGardenTasks(t, runtime, garden.ID, "Plant the beans", "Water the beans")
}

// TestCaptureCreatingProjectsRefusesASlugTakenSinceThePlan holds the race the
// confirmation opens: another writer creates the same slug between preparing
// the creation and committing it. The capture refuses and writes nothing.
func TestCaptureCreatingProjectsRefusesASlugTakenSinceThePlan(t *testing.T) {
	env := newHeadlessEnvironment(t)
	runtime := openHeadless(t, env.config(true))
	ledger := runtime.Ledger()

	creation, err := app.PrepareProjectCreation(runtime.State(), []app.NewProject{{ID: "11111111-1111-4111-8111-111111111111", Name: "garden"}})
	if err != nil {
		t.Fatalf("PrepareProjectCreation: %v", err)
	}
	other := openHeadless(t, env.config(false))
	if _, createErr := other.CreateProject("garden"); createErr != nil {
		t.Fatalf("concurrent CreateProject: %v", createErr)
	}
	before, err := ledger.UndoCount(storage.LedgerScope{})
	if err != nil {
		t.Fatalf("UndoCount: %v", err)
	}

	_, err = runtime.Service().CaptureManyCreatingProjects(creation, []app.CaptureRequest{
		{ProjectID: creation.Projects[0].ID, Title: "Plant the beans"},
	})
	if err == nil {
		t.Fatal("a capture creating a slug another writer took since its plan committed")
	}
	if after, _ := ledger.UndoCount(storage.LedgerScope{}); after != before {
		t.Fatalf("a refused capture recorded an undo entry: %d, want %d", after, before)
	}
	if refreshErr := runtime.Refresh(); refreshErr != nil {
		t.Fatalf("Refresh: %v", refreshErr)
	}
	winner := runtime.State().GetProject("garden")
	if winner == nil || winner.ID == creation.Projects[0].ID {
		t.Fatalf("the catalog lost the concurrent project or took the refused one: %+v", winner)
	}
	snap, err := runtime.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if tasks := snap.ProjectByID(winner.ID).Tasks; len(tasks) != 0 {
		t.Fatalf("the refused capture wrote into the concurrent project: %d tasks", len(tasks))
	}
}

// TestCaptureCreatingProjectsRefusesAProjectWithNoTask keeps the catalog from
// recording a project whose tasks.md this transaction would never write.
func TestCaptureCreatingProjectsRefusesAProjectWithNoTask(t *testing.T) {
	env := newHeadlessEnvironment(t)
	runtime := openHeadless(t, env.config(true))
	unfiled := runtime.State().SystemUnfiledProject()

	creation, err := app.PrepareProjectCreation(runtime.State(), []app.NewProject{{ID: "11111111-1111-4111-8111-111111111111", Name: "garden"}})
	if err != nil {
		t.Fatalf("PrepareProjectCreation: %v", err)
	}
	if _, captureErr := runtime.Service().CaptureManyCreatingProjects(creation, []app.CaptureRequest{
		{ProjectID: unfiled.ID, Title: "Buy compost"},
	}); captureErr == nil {
		t.Fatal("a created project receiving no task committed")
	}
	if refreshErr := runtime.Refresh(); refreshErr != nil {
		t.Fatalf("Refresh: %v", refreshErr)
	}
	if runtime.State().GetProject("garden") != nil {
		t.Fatal("the refused capture recorded the project")
	}
}

func requireGardenTasks(t *testing.T, runtime *Runtime, projectID string, want ...string) {
	t.Helper()
	snap, err := runtime.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	project := snap.ProjectByID(projectID)
	if project == nil {
		t.Fatalf("the snapshot does not load project %s", projectID)
	}
	var titles []string
	for _, task := range project.Tasks {
		titles = append(titles, task.Title)
	}
	if len(titles) != len(want) {
		t.Fatalf("project tasks = %q, want %q", titles, want)
	}
	for i := range want {
		if titles[i] != want[i] {
			t.Fatalf("project tasks = %q, want %q", titles, want)
		}
	}
}
