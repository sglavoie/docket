package storage

import (
	"errors"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

func TestMergeRefusesConcurrentStateChangeAndDeferral(t *testing.T) {
	for _, state := range []models.WorkflowState{models.StateInbox, models.StateDoing, models.StateWaiting, models.StatePaused, models.StateSomeday, models.StateDone, models.StateCanceled} {
		for _, swap := range []bool{false, true} {
			name := string(state)
			if swap {
				name += "/reversed sides"
			}
			t.Run(name, func(t *testing.T) {
				base := mergeTestDoc("Alpha", mergeTestTask("a", "Task"), mergeTestTask("b", "Other"))
				transitioned, deferred := base.clone(), base.clone()
				now := base.Tasks[0].UpdatedAt.Add(time.Hour)
				if err := transitioned.Tasks[0].ApplyStateTransitionAt(state, now); err != nil {
					t.Fatal(err)
				}
				if err := deferred.Tasks[0].ApplyDeferAt(now.Add(24*time.Hour), now); err != nil {
					t.Fatal(err)
				}
				// A task conflict must not cost unrelated concurrent work.
				deferred.Tasks[1].Title = "Other, edited"
				local, current := transitioned, deferred
				if swap {
					local, current = current, local
				}
				two := mustMerge(t, base, local, current)
				if two.Merged != nil || !hasConflictKind(two, MergeConflictInvariant) {
					t.Fatalf("state and deferral must conflict: %+v", two)
				}
				many, conflicts := mustReconcile(t, base, local, current)
				if len(conflicts) == 0 {
					t.Fatal("sync must report the incompatible state and deferral")
				}
				if many.Tasks[0].DeferredUntil != nil && many.Tasks[0].State() != models.StateBacklog {
					t.Fatalf("sync composed deferred %s work", many.Tasks[0].State())
				}
				if many.Tasks[1].Title != "Other, edited" {
					t.Fatal("task conflict discarded unrelated edits")
				}
				if _, err := many.WriteCurrent(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestCommitRecordsConcurrentStartAndDeferral(t *testing.T) {
	for _, deferFirst := range []bool{false, true} {
		name := "start first"
		if deferFirst {
			name = "defer first"
		}
		t.Run(name, func(t *testing.T) {
			env := newMergeCommitEnv(t)
			project := env.project("p1", "Alpha", "alpha")
			base := env.commitDoc(t, project, mergeTestDoc("Alpha", mergeTestTask("a", "Task")), "", nil)
			started, deferred := base.clone(), base.clone()
			now := base.Tasks[0].UpdatedAt.Add(time.Hour)
			if err := started.Tasks[0].ApplyStateTransitionAt(models.StateDoing, now); err != nil {
				t.Fatal(err)
			}
			if err := deferred.Tasks[0].ApplyDeferAt(now.Add(time.Hour), now); err != nil {
				t.Fatal(err)
			}
			remote, local := started, deferred
			if deferFirst {
				remote, local = deferred, started
			}
			onDisk := env.commitDoc(t, project, remote, base.Revision, base)
			_, err := env.svc.Commit(ChangeSet{Project: projectFromDocument(project, local), ExpectedRevision: base.Revision, Base: base})
			if !errors.Is(err, ErrMergeConflict) {
				t.Fatalf("commit = %v, want recorded conflict", err)
			}
			records, err := env.store.List()
			if err != nil || len(records) != 1 {
				t.Fatalf("records = %+v, err = %v", records, err)
			}
			found := false
			for _, conflict := range records[0].Conflicts {
				found = found || conflict.Field == "deferred_until"
			}
			if !found {
				t.Fatalf("no deferral conflict: %+v", records[0].Conflicts)
			}
			after, err := LoadProjectDocument(project)
			if err != nil {
				t.Fatal(err)
			}
			if after.Revision != onDisk.Revision {
				t.Fatal("refused merge changed the stored project")
			}
		})
	}
}
