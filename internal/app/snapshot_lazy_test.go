package app

import (
	"testing"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

// lazyTestProject is one project of a lazy snapshot whose stored version holds
// the given task IDs, counting how often it is loaded.
func lazyTestProject(t *testing.T, id string, loads map[string]int, taskIDs ...string) LazyProject {
	t.Helper()
	var stored []*models.Task
	for i, taskID := range taskIDs {
		task, err := models.NewGenericTask(taskID, i+1, "Stored "+taskID)
		if err != nil {
			t.Fatal(err)
		}
		stored = append(stored, task)
	}
	return LazyProject{
		Project:  &models.Project{ID: id, Name: id},
		Revision: "rev-" + id,
		TaskIDs:  taskIDs,
		Load: func(project *models.Project) (*storage.ProjectDocument, error) {
			loads[id]++
			project.Tasks = cloneTasks(stored)
			project.NextShortID = len(stored) + 1
			project.BuildTaskTree()
			return &storage.ProjectDocument{ProjectName: id, Tasks: cloneTasks(stored), Revision: "rev-" + id}, nil
		},
	}
}

// TestLazySnapshotLoadsOnlyWhatIsAskedFor pins NewLazySnapshot: revisions are
// known up front, a qualified lookup loads its own project, an unqualified one
// loads only the owner and reports a shared identity ambiguous without loading,
// and taking a rollback or a clone loads nothing.
func TestLazySnapshotLoadsOnlyWhatIsAskedFor(t *testing.T) {
	loads := map[string]int{}
	snap := NewLazySnapshot([]LazyProject{
		lazyTestProject(t, "a", loads, "a-1", "shared"),
		lazyTestProject(t, "b", loads, "b-1", "shared"),
		lazyTestProject(t, "c", loads, "c-1"),
	}, []string{"gone"})

	if snap.Revision("b") != "rev-b" || len(loads) != 0 {
		t.Fatalf("revision = %q after %v loads, want it without loading", snap.Revision("b"), loads)
	}
	_ = rollbackSnapshot(snap)
	_ = cloneSnapshot(snap)
	if len(loads) != 0 {
		t.Fatalf("taking rollback copies loaded %v", loads)
	}
	if ref, ok := snap.LookupLocator(TaskLocator{ProjectID: "a", TaskID: "a-1"}); !ok || ref.Task.Title != "Stored a-1" || ref.Base == nil {
		t.Fatal("a qualified lookup did not resolve against its loaded project")
	}
	if _, status := snap.LookupTaskID("shared"); status != TaskLookupAmbiguous {
		t.Fatalf("shared identity status = %v, want ambiguous", status)
	}
	if ref, status := snap.LookupTaskID("c-1"); status != TaskLookupUnique || ref.Project.ID != "c" {
		t.Fatal("an unqualified lookup did not resolve its one owner")
	}
	if loads["a"] != 1 || loads["b"] != 0 || loads["c"] != 1 {
		t.Fatalf("loads = %v, want a and c once and b never", loads)
	}
	if got := snap.UnavailableProjectIDs(); len(got) != 1 || got[0] != "gone" {
		t.Fatalf("unavailable = %v", got)
	}
	if len(snap.Projects()) != 3 || loads["b"] != 1 {
		t.Fatalf("Projects did not load the rest once: %v", loads)
	}
}

// TestLazyRollbackRestoresWhatTheMutationLoaded runs the failure path over a
// lazy snapshot: a rollback taken before anything loads still restores a
// project the mutation loaded and edited afterwards, short IDs included, and
// restoring loads nothing the mutation never reached.
func TestLazyRollbackRestoresWhatTheMutationLoaded(t *testing.T) {
	loads := map[string]int{}
	snap := NewLazySnapshot([]LazyProject{
		lazyTestProject(t, "a", loads, "a-1"),
		lazyTestProject(t, "b", loads, "b-1"),
	}, nil)
	before := rollbackSnapshot(snap)

	project := snap.ProjectByID("a")
	project.Tasks[0].Title = "Edited"
	project.Name = "Edited"
	project.NextShortID = 99

	restoreSnapshotProjects(snap, before)
	if project.Tasks[0].Title != "Stored a-1" || project.Name != "a" || project.NextShortID != 2 {
		t.Fatalf("restored %q %q next %d, want the stored version", project.Name, project.Tasks[0].Title, project.NextShortID)
	}
	if loads["b"] != 0 {
		t.Fatal("restoring loaded a project the mutation never reached")
	}
}
