package app

import (
	"testing"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

func rollbackTestSnapshot(t *testing.T) (Snapshot, *models.Project) {
	t.Helper()
	task, err := models.NewGenericTask("task-1", 1, "Before")
	if err != nil {
		t.Fatal(err)
	}
	project := &models.Project{ID: "p", Name: "Before", Tasks: []*models.Task{task}}
	document := &storage.ProjectDocument{ProjectName: "Before", Tasks: []*models.Task{models.CloneTaskDeep(task)}, Revision: "rev"}
	snap := NewPartialSnapshot([]*models.Project{project}, map[string]string{"p": "rev"},
		map[string]*storage.ProjectDocument{"p": document}, nil)
	return snap, project
}

// TestRollbackSnapshotHoldsTheStateItWasTakenAt pins the lazy rollback: a
// project's fields are the ones standing when the rollback was taken, and its
// tasks are the document's, detached from everything a mutation edits, however
// late the rollback is first read.
func TestRollbackSnapshotHoldsTheStateItWasTakenAt(t *testing.T) {
	snap, live := rollbackTestSnapshot(t)
	before := rollbackSnapshot(snap)

	live.Name = "Edited"
	live.Tags = []string{"edited"}
	live.Tasks[0].Title = "Edited"

	saved := before.ProjectByID("p")
	if saved.Name != "Before" || len(saved.Tags) != 0 {
		t.Fatalf("rollback project = %q %v, want the fields standing when it was taken", saved.Name, saved.Tags)
	}
	if saved.Tasks[0].Title != "Before" || saved.Tasks[0] == live.Tasks[0] || saved.Tasks[0] == snap.Document("p").Tasks[0] {
		t.Fatal("rollback tasks are not a detached copy of the document")
	}
	if before.Document("p").Tasks[0] != saved.Tasks[0] || before.Revision("p") != "rev" {
		t.Fatal("rollback document does not hold the tasks its project was given")
	}
	if ref, ok := before.LookupLocator(TaskLocator{ProjectID: "p", TaskID: "task-1"}); !ok || ref.Task != saved.Tasks[0] {
		t.Fatal("rollback lookup does not resolve to the detached task")
	}

	restoreSnapshotProjects(snap, before)
	if live.Name != "Before" || live.Tasks[0].Title != "Before" {
		t.Fatalf("restore left %q / %q, want the rollback state", live.Name, live.Tasks[0].Title)
	}
}
