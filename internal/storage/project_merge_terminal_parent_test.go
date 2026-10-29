package storage

import (
	"errors"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

func TestCommitMergeRefusesConcurrentTerminalParentAndOpenChild(t *testing.T) {
	for _, state := range []models.WorkflowState{models.StateDone, models.StateCanceled} {
		for _, terminalFirst := range []bool{false, true} {
			name := string(state) + "/child-first"
			if terminalFirst {
				name = string(state) + "/terminal-first"
			}
			t.Run(name, func(t *testing.T) {
				env := newMergeCommitEnv(t)
				project := env.project("p1", "Alpha", "alpha")
				base := env.commitDoc(t, project, mergeTestDoc("Alpha", mergeTestTask("parent", "Parent")), "", nil)
				terminal := base.clone()
				if err := terminal.Tasks[0].ApplyStateTransitionAt(state, base.Tasks[0].UpdatedAt.Add(time.Hour)); err != nil {
					t.Fatal(err)
				}
				withChild := base.clone()
				child := mergeTestTask("child", "Unfinished child")
				parentID := base.Tasks[0].ID
				child.ParentID = &parentID
				withChild.Tasks = append(withChild.Tasks, child)
				remote, local := withChild, terminal
				if terminalFirst {
					remote, local = terminal, withChild
				}
				onDisk := env.commitDoc(t, project, remote, base.Revision, base)
				_, err := env.svc.Commit(ChangeSet{
					Project: projectFromDocument(project, local), ExpectedRevision: base.Revision, Base: base,
				})
				if !errors.Is(err, ErrMergeConflict) {
					t.Fatalf("commit = %v, want a recorded merge conflict", err)
				}
				records, err := env.store.List()
				if err != nil || len(records) != 1 {
					t.Fatalf("conflicts = %v, err = %v", records, err)
				}
				found := false
				for _, conflict := range records[0].Conflicts {
					if conflict.Kind == MergeConflictInvariant && conflict.Field == "parent_id" && conflict.TaskTitle == child.Title {
						found = true
					}
				}
				if !found {
					t.Fatalf("missing terminal-parent invariant: %+v", records[0].Conflicts)
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
}

func TestMergeConcurrentTerminalParentAndChildAdoption(t *testing.T) {
	for _, childState := range []models.WorkflowState{models.StateBacklog, models.StateDone, models.StateCanceled} {
		t.Run(string(childState), func(t *testing.T) {
			parent := mergeTestTask("parent", "Parent")
			child := mergeTestTask("child", "Child")
			now := parent.UpdatedAt.Add(time.Hour)
			if err := child.ApplyStateTransitionAt(childState, now); err != nil {
				t.Fatal(err)
			}
			base := mergeTestDoc("Alpha", parent, child)
			local, current := base.clone(), base.clone()
			if err := local.Tasks[0].ApplyStateTransitionAt(models.StateDone, now); err != nil {
				t.Fatal(err)
			}
			parentID := parent.ID
			current.Tasks[1].ParentID = &parentID
			for _, sides := range [][2]*ProjectDocument{{local, current}, {current, local}} {
				result := mustMerge(t, base, sides[0], sides[1])
				if !childState.IsTerminal() {
					if result.Merged != nil || !hasConflictKind(result, MergeConflictInvariant) {
						t.Fatalf("adopting unfinished work into a completed parent must conflict: %+v", result)
					}
					continue
				}
				if result.HasConflicts() {
					t.Fatalf("terminal child must remain valid: %+v", result.Conflicts)
				}
				if !result.Merged.Tasks[0].IsTerminalState() || result.Merged.Tasks[1].ParentID == nil || *result.Merged.Tasks[1].ParentID != parentID {
					t.Fatal("valid merge lost the completion or adoption")
				}
			}
		})
	}
}
