package storage

import (
	"reflect"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

func TestMergeEmptyCollectionsDoNotCountAsEdits(t *testing.T) {
	for _, field := range []string{"Tags", "Checklist", "Dependencies", "Comments", "Attachments"} {
		for _, emptyBase := range []bool{false, true} {
			for _, deletion := range []bool{false, true} {
				name := field
				if emptyBase {
					name += "/empty base"
				} else {
					name += "/nil base"
				}
				if deletion {
					name += "/delete"
				} else {
					name += "/edit"
				}
				t.Run(name, func(t *testing.T) {
					doc := func(empty bool) *ProjectDocument {
						task := mergeTestTask("a", "Task")
						value := reflect.ValueOf(task).Elem().FieldByName(field)
						if empty {
							value.Set(reflect.MakeSlice(value.Type(), 0, 0))
						} else {
							value.SetZero()
						}
						return mergeTestDoc("p", task)
					}
					base, local, current := doc(emptyBase), doc(emptyBase), doc(!emptyBase)
					local.Tasks[0].Title = "Edited"
					switch field {
					case "Tags":
						local.Tasks[0].Tags = []string{"added"}
					case "Checklist":
						local.Tasks[0].Checklist = []models.ChecklistItem{{Text: "Added"}}
					case "Dependencies":
						local.Tasks[0].Dependencies = []models.TaskReference{{ProjectID: "p", TaskID: "target"}}
					case "Comments":
						local.Tasks[0].Comments = []*models.Comment{{Text: "Added", CreatedAt: local.Tasks[0].CreatedAt}}
					case "Attachments":
						local.Tasks[0].Attachments = []models.Attachment{{Filename: "added.txt", AddedAt: local.Tasks[0].CreatedAt}}
					}
					if deletion {
						local.Tasks = nil
					}
					two := mustMerge(t, base, local, current)
					many, conflicts := mustReconcile(t, base, local, current)
					if two.HasConflicts() || len(conflicts) > 0 {
						t.Fatalf("empty collection caused a conflict: merge=%+v reconcile=%+v", two.Conflicts, conflicts)
					}
					for _, output := range []*ProjectDocument{two.Merged, many} {
						if deletion {
							if len(output.Tasks) != 0 {
								t.Fatal("empty collection resurrected a deleted task")
							}
						} else if output.Tasks[0].Title != "Edited" {
							t.Fatal("independent title edit was lost")
						} else if value := reflect.ValueOf(output.Tasks[0]).Elem().FieldByName(field); value.Len() != 1 {
							t.Fatal("collection edit was lost")
						}
					}
				})
			}
		}
	}
}

func TestReconcileReturnedVersionDetachesFromInput(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		name := "single version"
		if fallback {
			name = "whole document fallback"
		}
		t.Run(name, func(t *testing.T) {
			parent := mergeTestTask("parent", "Parent")
			base := mergeTestDoc("p", parent)
			primary := base.clone()
			primary.Comments = []*models.ProjectComment{{
				ID: "11111111-1111-4111-8111-111111111111", Text: "Original", CreatedAt: parent.CreatedAt,
			}}
			sides := []*ProjectDocument{primary}
			if fallback {
				if err := primary.Tasks[0].ApplyStateTransitionAt(models.StateDone, parent.UpdatedAt.Add(time.Hour)); err != nil {
					t.Fatal(err)
				}
				other := base.clone()
				child := mergeTestTask("child", "Child")
				child.ParentID = &parent.ID
				other.Tasks = append(other.Tasks, child)
				sides = append(sides, other)
			}
			output, conflicts, err := ReconcileProjectDocuments(base, sides)
			if err != nil || (len(conflicts) > 0) != fallback {
				t.Fatalf("reconcile: conflicts=%+v err=%v", conflicts, err)
			}
			if fallback && len(output.Tasks) != 1 {
				t.Fatal("expected the primary document fallback")
			}
			output.Tasks[0].Title = "Changed"
			output.Comments[0].Text = "Changed"
			if primary.Tasks[0].Title != "Parent" || primary.Comments[0].Text != "Original" {
				t.Fatal("editing the returned version changed the input")
			}
		})
	}
}

func TestMergedTaskCollectionsDetachFromInputs(t *testing.T) {
	for _, scenario := range []string{"unchanged", "local edit", "current edit", "identical edits", "addition"} {
		for _, reconcile := range []bool{false, true} {
			name := scenario + "/merge"
			if reconcile {
				name = scenario + "/reconcile"
			}
			t.Run(name, func(t *testing.T) {
				doc := func() *ProjectDocument {
					task := mergeTestTask("a", "Task")
					task.Tags = []string{"original", "second"}
					task.Checklist = []models.ChecklistItem{{Text: "First"}, {Text: "Second"}}
					task.Comments = []*models.Comment{{Text: "First", CreatedAt: task.CreatedAt}, {Text: "Second", CreatedAt: task.CreatedAt}}
					task.Attachments = []models.Attachment{{Filename: "first.txt", AddedAt: task.CreatedAt}, {Filename: "second.txt", AddedAt: task.CreatedAt}}
					return mergeTestDoc("p", task)
				}
				base, local, current := doc(), doc(), doc()
				edit := func(task *models.Task) {
					task.Tags[0] = "edited"
					task.Checklist[0].Text = "Edited"
					task.Comments[0].Text = "Edited"
					task.Attachments[0].Filename = "edited.txt"
				}
				switch scenario {
				case "local edit":
					edit(local.Tasks[0])
					current.Tasks[0].Title = "Other edit"
				case "current edit":
					edit(current.Tasks[0])
					local.Tasks[0].Title = "Other edit"
				case "identical edits":
					edit(local.Tasks[0])
					edit(current.Tasks[0])
				case "addition":
					base.Tasks = nil
					current.Tasks = nil
				}
				inputs := []*ProjectDocument{base, local, current}
				before := make([][]*models.Task, len(inputs))
				for i, input := range inputs {
					for _, task := range input.Tasks {
						before[i] = append(before[i], models.CloneTaskDeep(task))
					}
				}
				var output *ProjectDocument
				if reconcile {
					var conflicts []MergeConflict
					output, conflicts = mustReconcile(t, base, local, current)
					if len(conflicts) > 0 {
						t.Fatalf("unexpected conflicts: %+v", conflicts)
					}
				} else {
					result := mustMerge(t, base, local, current)
					if result.HasConflicts() {
						t.Fatalf("unexpected conflicts: %+v", result.Conflicts)
					}
					output = result.Merged
				}
				task := output.Tasks[0]
				task.Tags[0] = "mutated"
				task.Checklist[0].Done = true
				task.Comments[0].Text = "Mutated"
				task.RemoveComment(0)
				task.RemoveAttachment(0)
				for i, input := range inputs {
					for j, original := range input.Tasks {
						if !reflect.DeepEqual(original, before[i][j]) {
							t.Errorf("mutating merged task changed input %d task %d", i, j)
						}
					}
				}
			})
		}
	}
}
