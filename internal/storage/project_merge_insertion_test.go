package storage

import (
	"slices"
	"testing"

	"github.com/sglavoie/docket/internal/models"
)

func TestMergePreservesInsertedSiblingPositions(t *testing.T) {
	for _, nested := range []bool{false, true} {
		for _, order := range [][]string{
			{"x", "y", "a", "b"},
			{"a", "x", "y", "b"},
			{"a", "b", "x", "y"},
		} {
			name := order[0] + order[1] + order[2] + order[3]
			if nested {
				name += "/children"
			}
			t.Run(name, func(t *testing.T) {
				makeDoc := func(ids []string) *ProjectDocument {
					var tasks []*models.Task
					if nested {
						tasks = append(tasks, mergeTestTask("parent", "Parent"))
					}
					for _, id := range ids {
						task := mergeTestTask(id, id)
						if nested {
							task.ParentID = &tasks[0].ID
						}
						tasks = append(tasks, task)
					}
					return mergeTestDoc("p", tasks...)
				}
				base := makeDoc([]string{"a", "b"})
				edited := makeDoc(order)
				want := mergeTaskIDs(edited.Tasks)
				for _, swap := range []bool{false, true} {
					local, current := edited, base
					if swap {
						local, current = current, local
					}
					result := mustMerge(t, base, local, current)
					if result.HasConflicts() || result.Merged == nil {
						t.Fatalf("unopposed insertion conflicted: %+v", result.Conflicts)
					}
					if got := mergeTaskIDs(result.Merged.Tasks); !slices.Equal(got, want) {
						t.Errorf("merge (swap=%t) order = %v, want %v", swap, got, want)
					}
					reconciled, conflicts := mustReconcile(t, base, local, current)
					if len(conflicts) != 0 {
						t.Fatalf("unopposed insertion reconciled with conflicts: %+v", conflicts)
					}
					if got := mergeTaskIDs(reconciled.Tasks); !slices.Equal(got, want) {
						t.Errorf("reconcile (swap=%t) order = %v, want %v", swap, got, want)
					}
				}
			})
		}
	}
}

func TestCommitMergePreservesInsertionBesideConcurrentEdit(t *testing.T) {
	env := newMergeCommitEnv(t)
	project := env.project("p1", "Alpha", "alpha")
	base := env.commitDoc(t, project, mergeTestDoc("Alpha",
		mergeTestTask("a", "Alpha"), mergeTestTask("b", "Beta")), "", nil)
	current := mergeTestDoc("Alpha", cloneDocTasks(base.Tasks)...)
	current.Tasks[1].Title = "Updated Beta"
	env.commitDoc(t, project, current, base.Revision, base)

	local := mergeTestDoc("Alpha", cloneDocTasks(base.Tasks)...)
	local.Tasks = []*models.Task{local.Tasks[0], mergeTestTask("x", "Inserted"), local.Tasks[1]}
	merged := env.commitDoc(t, project, local, base.Revision, base)
	if got, want := mergeTaskIDs(merged.Tasks), []string{"a", "x", "b"}; !slices.Equal(got, want) {
		t.Fatalf("reloaded merged order = %v, want %v", got, want)
	}
	if merged.Tasks[2].Title != "Updated Beta" {
		t.Fatalf("concurrent title edit lost: %+v", merged.Tasks[2])
	}
}

func TestMergePreservesConcurrentInsertionsAroundSurvivingSiblings(t *testing.T) {
	makeDoc := func(ids ...string) *ProjectDocument {
		tasks := make([]*models.Task, 0, len(ids))
		for _, id := range ids {
			tasks = append(tasks, mergeTestTask(id, id))
		}
		return mergeTestDoc("p", tasks...)
	}
	for _, tc := range []struct {
		name                 string
		base, local, current []string
		want                 []string
	}{
		{
			name: "independent gaps", base: []string{"a", "b", "c"},
			local: []string{"a", "x", "b", "c"}, current: []string{"a", "b", "y", "c"},
			want: []string{"a", "x", "b", "y", "c"},
		},
		{
			name: "same gap keeps side order", base: []string{"a", "b"},
			local: []string{"a", "x", "b"}, current: []string{"a", "y", "b"},
			want: []string{"a", "x", "y", "b"},
		},
		{
			name: "deleted following sibling", base: []string{"a", "b", "c"},
			local: []string{"a", "x", "b", "c"}, current: []string{"a", "c"},
			want: []string{"a", "x", "c"},
		},
		{
			name: "reorder retains authority", base: []string{"a", "b", "c"},
			local: []string{"c", "a", "b"}, current: []string{"a", "x", "b", "c"},
			want: []string{"c", "a", "x", "b"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base, local, current := makeDoc(tc.base...), makeDoc(tc.local...), makeDoc(tc.current...)
			result := mustMerge(t, base, local, current)
			if result.HasConflicts() || result.Merged == nil {
				t.Fatalf("insertions conflicted: %+v", result.Conflicts)
			}
			if got := mergeTaskIDs(result.Merged.Tasks); !slices.Equal(got, tc.want) {
				t.Errorf("merged order = %v, want %v", got, tc.want)
			}
			reconciled, conflicts := mustReconcile(t, base, local, current)
			if len(conflicts) != 0 {
				t.Fatalf("insertions reconciled with conflicts: %+v", conflicts)
			}
			if got := mergeTaskIDs(reconciled.Tasks); !slices.Equal(got, tc.want) {
				t.Errorf("reconciled order = %v, want %v", got, tc.want)
			}
		})
	}
}
