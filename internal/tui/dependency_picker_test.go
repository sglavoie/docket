package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/grammar"
)

// TestReferencePickerAgreesWithTheQuickEditRouteOnThePatchHalves is the
// two-routes contract. `~` on a row and `~`/`-~` in the edit buffer are two
// spellings of one request, so for the same intent they must build the same
// app.DependencyPatch — otherwise the picker would be a second, quietly
// divergent dependency grammar rather than a keyboard route into the existing
// one.
//
// Add is compared as a set and Remove in order, and that asymmetry is the
// honest reading rather than a weakened assertion: Remove is derived from
// task.Dependencies on both routes, so stored order is a real shared fact,
// while Add is token order in the buffer and sorted locator order out of the
// picker, because selectedValue() sorts the checked payloads. See
// TestReferencePickerAndQuickEditDifferOnlyInSpelling below, which pins both
// divergences rather than leaving them to be rediscovered as bugs.
func TestReferencePickerAgreesWithTheQuickEditRouteOnThePatchHalves(t *testing.T) {
	stored := []models.TaskReference{
		{ProjectID: "home-project", TaskID: "home-3"},
		{ProjectID: "gone-project", TaskID: "gone-9"},
	}

	for _, testCase := range []struct {
		name    string
		input   string
		checked []string
	}{
		{
			name:    "no intent on either route",
			input:   "Edited task",
			checked: []string{"home-project:home-3", "gone-project:gone-9"},
		},
		{
			name:    "addition",
			input:   "Edited task ~2",
			checked: []string{"home-project:home-3", "gone-project:gone-9", "home-project:home-2"},
		},
		{
			name:    "removal of a closed target",
			input:   "Edited task -~3",
			checked: []string{"gone-project:gone-9"},
		},
		{
			name:    "removal of an unavailable target",
			input:   "Edited task -~gone-project:gone-9",
			checked: []string{"home-project:home-3"},
		},
		{
			name:    "one commit adds and removes",
			input:   "Edited task -~3 ~routed:1",
			checked: []string{"gone-project:gone-9", "routed-project:routed-1"},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			home, routed := dependencyWorkspace()
			task := home.Tasks[0]
			task.Dependencies = models.CloneTaskReferences(stored)

			result, err := grammar.ParseQuickEdit(testCase.input, task, home, []*models.Project{home, routed})
			if err != nil {
				t.Fatalf("parseQuickEdit(%q): %v", testCase.input, err)
			}
			typed := result.DependencyPatch()
			picked := referencePickerCommit(task, task, testCase.checked...)

			typedAdd := slices.Sorted(slices.Values(dependencyLocatorStrings(typed.Add)))
			pickedAdd := slices.Sorted(slices.Values(dependencyLocatorStrings(picked.Add)))
			if !slices.Equal(typedAdd, pickedAdd) {
				t.Errorf("add: typed %v, picked %v", typedAdd, pickedAdd)
			}
			if typedRemove, pickedRemove := dependencyLocatorStrings(typed.Remove), dependencyLocatorStrings(picked.Remove); !slices.Equal(typedRemove, pickedRemove) {
				t.Errorf("remove: typed %v, picked %v", typedRemove, pickedRemove)
			}
			if typed.Clear != picked.Clear {
				t.Errorf("clear: typed %v, picked %v", typed.Clear, picked.Clear)
			}
			if typed.IsZero() != picked.IsZero() {
				t.Errorf("intent: typed %+v, picked %+v", typed, picked)
			}
		})
	}
}

// TestReferencePickerAndQuickEditDifferOnlyInSpelling records the two places the
// routes do not produce byte-identical patches, so neither is read as a defect
// later. Both differences are in how one request is spelled, not in what it
// asks for: the application layer resolves each pair to the same stored set.
func TestReferencePickerAndQuickEditDifferOnlyInSpelling(t *testing.T) {
	home, routed := dependencyWorkspace()
	projects := []*models.Project{home, routed}
	task := home.Tasks[0]
	task.Dependencies = []models.TaskReference{{ProjectID: "home-project", TaskID: "home-3"}}

	// Add order. The buffer preserves the order the tokens were typed in; the
	// picker's checked payloads arrive sorted by locator, so its Add is sorted
	// too. Nothing downstream reads Add order — a soft reference set is stored,
	// not ranked — which is why the agreement above is asserted on membership.
	result, err := grammar.ParseQuickEdit("Edited task ~routed:1 ~2", task, home, projects)
	if err != nil {
		t.Fatalf("parseQuickEdit: %v", err)
	}
	typedAdd := dependencyLocatorStrings(result.DependencyPatch().Add)
	if want := []string{"routed-project:routed-1", "home-project:home-2"}; !slices.Equal(typedAdd, want) {
		t.Errorf("typed add = %v, want token order %v", typedAdd, want)
	}
	pickedAdd := dependencyLocatorStrings(referencePickerCommit(task, task,
		"home-project:home-3", "routed-project:routed-1", "home-project:home-2",
	).Add)
	if want := []string{"home-project:home-2", "routed-project:routed-1"}; !slices.Equal(pickedAdd, want) {
		t.Errorf("picked add = %v, want sorted locator order %v", pickedAdd, want)
	}
	if slices.Equal(typedAdd, pickedAdd) {
		t.Error("the two routes now agree on add order; fold this case back into the agreement test")
	}

	// Emptying the set. `~0` is a Clear; unchecking every row is a Remove over
	// the whole stored set. The picker cannot spell Clear at all, because the
	// application layer refuses Clear beside Add or Remove and a picker commit is
	// always potentially both.
	cleared, err := grammar.ParseQuickEdit("Edited task ~0", task, home, projects)
	if err != nil {
		t.Fatalf("parseQuickEdit: %v", err)
	}
	typedClear := cleared.DependencyPatch()
	if !typedClear.Clear || len(typedClear.Remove) != 0 {
		t.Fatalf("~0 = %+v, want a bare clear", typedClear)
	}
	emptied := referencePickerCommit(task, task)
	if emptied.Clear {
		t.Fatalf("an empty checked set = %+v, want a removal rather than a clear", emptied)
	}
	if got := dependencyLocatorStrings(emptied.Remove); !slices.Equal(got, []string{"home-project:home-3"}) {
		t.Errorf("remove = %v, want every stored reference", got)
	}
}
