package models

import (
	"reflect"
	"testing"
)

func TestReplaceTagRenamesMergesAndPreservesOrder(t *testing.T) {
	cases := []struct {
		name        string
		tags        []string
		from        string
		to          string
		want        []string
		wantChanged bool
	}{
		{"renames in place", []string{"work", "urgent"}, "work", "effort", []string{"effort", "urgent"}, true},
		{"merges onto an existing tag", []string{"work", "urgent", "effort"}, "work", "effort", []string{"effort", "urgent"}, true},
		{"merges from a later position", []string{"effort", "urgent", "work"}, "work", "effort", []string{"effort", "urgent"}, true},
		{"renames a stored variant", []string{"Work Item"}, "work-item", "effort", []string{"effort"}, true},
		{"leaves an unrelated set alone", []string{"urgent", "ops"}, "work", "effort", []string{"urgent", "ops"}, false},
		{"leaves an empty set alone", nil, "work", "effort", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, changed := ReplaceTag(tc.tags, tc.from, tc.to)
			if changed != tc.wantChanged {
				t.Fatalf("changed = %v, want %v", changed, tc.wantChanged)
			}
			if len(got) != 0 || len(tc.want) != 0 {
				if !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("tags = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// TestReplaceTagDoesNotStampTheTask is the reason ReplaceTag exists as a pure
// list transform: AddTag and RemoveTag stamp UpdatedAt, and a vocabulary rename
// is not activity on the task it re-addresses.
func TestReplaceTagDoesNotStampTheTask(t *testing.T) {
	task, err := NewGenericTask("task-1", 1, "renamed")
	if err != nil {
		t.Fatalf("NewGenericTask: %v", err)
	}
	task.Tags = []string{"work"}
	updated := task.UpdatedAt

	renamed, changed := ReplaceTag(task.Tags, "work", "effort")
	if !changed {
		t.Fatal("ReplaceTag reported no change")
	}
	task.Tags = renamed
	if !task.UpdatedAt.Equal(updated) {
		t.Fatalf("UpdatedAt = %s, want the untouched %s", task.UpdatedAt, updated)
	}
}
