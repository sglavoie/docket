package storage

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

func TestLifecycleRestorePartitionsRootsWithoutReplayingOldOrder(t *testing.T) {
	ledger, root := newTestLedger(t)
	path := filepath.Join(root, "projects", "alpha", "tasks.md")
	before, _ := taskRestoreDocuments()
	a, b := before.Tasks[0], before.Tasks[1]
	child := mergeTestTask("task-child", "Closed child")
	child.ParentID = &a.ID
	if err := child.ApplyStateTransitionAt(models.StateDone, child.UpdatedAt); err != nil {
		t.Fatal(err)
	}
	c, d := mergeTestTask("task-c", "C"), mergeTestTask("task-d", "D")
	if err := d.ApplyStateTransitionAt(models.StateDone, d.UpdatedAt); err != nil {
		t.Fatal(err)
	}
	before.Tasks = []*models.Task{a, child, b, c, d}
	after := before.clone()
	if err := after.Tasks[0].ApplyStateTransitionAt(models.StateDone, a.UpdatedAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	partitioned := &models.Project{Tasks: after.Tasks}
	partitioned.ReorderRootTasksByCompletion()
	after.Tasks = partitioned.Tasks
	recordTaskRestore(t, ledger, path, before, after)
	current := after.clone()
	byID := indexTasks(current.Tasks)
	byID[a.ID].Title = "Peer title"
	added := mergeTestTask("task-e", "Peer addition")
	// The peer reorders B and C and adds E without changing A's lifecycle.
	current.Tasks = []*models.Task{byID[c.ID], byID[b.ID], added, byID[a.ID], byID[child.ID], byID[d.ID]}
	writeFile(t, path, mustWriteCurrentDocument(t, current))
	for _, redo := range []bool{false, true} {
		var err error
		if redo {
			_, err = ledger.Redo(GlobalScope())
		} else {
			_, err = ledger.Undo(GlobalScope())
		}
		if err != nil {
			t.Fatal(err)
		}
		got, err := LoadProjectDocument(&models.Project{Path: filepath.Dir(path)})
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, task := range got.Tasks {
			ids = append(ids, task.ID)
		}
		if !slices.Equal(ids, []string{c.ID, b.ID, added.ID, a.ID, child.ID, d.ID}) {
			t.Fatalf("redo=%v replayed old ordering: %v", redo, ids)
		}
		task := indexTasks(got.Tasks)[a.ID]
		if task.Title != "Peer title" || task.IsComplete() != redo {
			t.Fatalf("redo=%v: restored task %+v", redo, task)
		}
	}
}

func TestLifecycleRestoreRefusesAdditionalManualReordering(t *testing.T) {
	ledger, root := newTestLedger(t)
	path := filepath.Join(root, "projects", "alpha", "tasks.md")
	before, _ := taskRestoreDocuments()
	before.Tasks = append(before.Tasks, mergeTestTask("task-c", "C"))
	after := before.clone()
	if err := after.Tasks[0].ApplyStateTransitionAt(models.StateDone, after.Tasks[0].UpdatedAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	project := &models.Project{Tasks: after.Tasks}
	project.ReorderRootTasksByCompletion()
	after.Tasks = project.Tasks
	after.Tasks[0], after.Tasks[1] = after.Tasks[1], after.Tasks[0]
	recordTaskRestore(t, ledger, path, before, after)
	current := after.clone()
	indexTasks(current.Tasks)[before.Tasks[0].ID].Title = "Peer title"
	content := mustWriteCurrentDocument(t, current)
	writeFile(t, path, content)
	if _, err := ledger.Undo(GlobalScope()); !errors.Is(err, ErrStaleArtifact) {
		t.Fatalf("undo error = %v, want exact-image refusal", err)
	}
	assertLedgerRestoreContent(t, path, content)
}
