package storage

import (
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

func TestReconcileKeepsLatestUpdateFromReorderedTasks(t *testing.T) {
	for _, editTitle := range []bool{false, true} {
		for _, swap := range []bool{false, true} {
			name := "reorder only"
			if editTitle {
				name = "reorder beside title edit"
			}
			if swap {
				name += "/reversed sides"
			}
			t.Run(name, func(t *testing.T) {
				base := mergeTestDoc("p", mergeTestTask("a", "Alpha"), mergeTestTask("b", "Beta"))
				reordered := mergeTestDoc("p", cloneDocTasks(base.Tasks)...)
				movedAt := base.Tasks[0].UpdatedAt.Add(2 * time.Hour)
				project := &models.Project{Tasks: reordered.Tasks}
				if err := project.ReorderSiblingTasksAt([]*models.Task{project.Tasks[1], project.Tasks[0]}, movedAt); err != nil {
					t.Fatal(err)
				}
				reordered.Tasks = project.Tasks
				other := mergeTestDoc("p", cloneDocTasks(base.Tasks)...)
				if editTitle {
					other.Tasks[0].Title = "Edited Alpha"
					other.Tasks[0].UpdatedAt = movedAt.Add(-time.Hour)
				}
				sides := []*ProjectDocument{reordered, other}
				if swap {
					slices.Reverse(sides)
				}
				merged, conflicts := mustReconcile(t, base, sides...)
				if len(conflicts) != 0 {
					t.Fatalf("unexpected conflicts: %+v", conflicts)
				}
				if got := mergeTaskIDs(merged.Tasks); !slices.Equal(got, []string{"b", "a"}) {
					t.Fatalf("order = %v, want [b a]", got)
				}
				for _, task := range merged.Tasks {
					if !task.UpdatedAt.Equal(movedAt) {
						t.Errorf("%s updated at %s, want %s", task.ID, task.UpdatedAt, movedAt)
					}
					if editTitle && task.ID == "a" && task.Title != "Edited Alpha" {
						t.Errorf("concurrent title edit lost: %q", task.Title)
					}
				}
			})
		}
	}
}
