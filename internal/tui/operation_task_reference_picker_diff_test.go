package tui

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// This file covers what a commit spells: the difference between the membership
// the picker opened with and the set the user left checked, including the merge
// against a task that moved underneath the overlay. Row assembly is in
// operation_task_reference_picker_source_test.go.

// TestReferencePickerDiffsTheCheckedSetAgainstWhatIsStored walks every shape the
// commit can take. Both halves are sorted, because both are read off a
// membership map rather than off an ordered list, and a patch is a set on either
// side.
func TestReferencePickerDiffsTheCheckedSetAgainstWhatIsStored(t *testing.T) {
	everyStored := []string{"remote:target", "home:closed", "home:ghost", "gone:gone-1", "attic:relic"}
	sortedStored := []string{"attic:relic", "gone:gone-1", "home:closed", "home:ghost", "remote:target"}

	for _, testCase := range []struct {
		name       string
		checked    []string
		wantAdd    []string
		wantRemove []string
	}{
		{
			name:    "unchanged",
			checked: everyStored,
		},
		{
			name:    "add only",
			checked: append(append([]string(nil), everyStored...), "home:sibling", "remote:other"),
			wantAdd: []string{"home:sibling", "remote:other"},
		},
		{
			name:       "remove only",
			checked:    []string{"remote:target", "home:ghost"},
			wantRemove: []string{"attic:relic", "gone:gone-1", "home:closed"},
		},
		{
			name:       "add and remove in one commit",
			checked:    []string{"remote:target", "home:ghost", "gone:gone-1", "attic:relic", "home:sibling"},
			wantAdd:    []string{"home:sibling"},
			wantRemove: []string{"home:closed"},
		},
		{
			// Nothing checked is a legal full removal, not a refused empty value, and
			// it is spelled as Remove rather than Clear: the application layer refuses
			// Clear beside Add or Remove, and removing every stored reference already
			// says the same thing.
			name:       "everything unchecked",
			checked:    nil,
			wantRemove: sortedStored,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			subject, _ := referenceSuiteWorkspace(t)
			patch := referencePickerCommit(subject, subject, testCase.checked...)
			if got := dependencyLocatorStrings(patch.Add); !slices.Equal(got, testCase.wantAdd) {
				t.Errorf("add = %v, want %v", got, testCase.wantAdd)
			}
			if got := dependencyLocatorStrings(patch.Remove); !slices.Equal(got, testCase.wantRemove) {
				t.Errorf("remove = %v, want %v", got, testCase.wantRemove)
			}
			if patch.Clear {
				t.Error("the picker set Clear; it only ever spells a removal")
			}
			if wantZero := testCase.wantAdd == nil && testCase.wantRemove == nil; patch.IsZero() != wantZero {
				t.Errorf("patch.IsZero() = %v, want %v: %+v", patch.IsZero(), wantZero, patch)
			}
		})
	}
}

// TestReferencePickerRecheckingAStoredTargetIsANoOp is the picker's half of the
// application layer's known-addition rule. Leaving a terminal or unreadable
// stored row checked is the ordinary case — it is how a user removes something
// else without touching it — so it has to fall out as no intent at all, never as
// an addition the resolver would then have to judge against the addition rules.
func TestReferencePickerRecheckingAStoredTargetIsANoOp(t *testing.T) {
	subject, _ := referenceSuiteWorkspace(t)
	for _, locator := range []string{"home:closed", "gone:gone-1", "attic:relic"} {
		t.Run(locator, func(t *testing.T) {
			patch := referencePickerCommit(
				subject, subject,
				"remote:target", "home:closed", "home:ghost", "gone:gone-1", "attic:relic",
			)
			if !patch.IsZero() {
				t.Fatalf("re-checking %s produced %+v, want no intent at all", locator, patch)
			}
		})
	}

	// A duplicate payload collapses rather than adding twice, which is the same
	// rule the parser applies to two aliases naming one task.
	patch := referencePickerCommit(
		subject, subject,
		"remote:target", "home:closed", "home:ghost", "gone:gone-1", "attic:relic",
		"home:sibling", "home:sibling",
	)
	if got := dependencyLocatorStrings(patch.Add); !slices.Equal(got, []string{"home:sibling"}) {
		t.Errorf("add = %v, want one entry per target", got)
	}
}

// TestReferencePickerPreservesConcurrentReferenceChanges is the merge contract.
// The picker overlay is not modal against the workspace: an accepted
// cross-project load republishes task trees through adoptCrossProjectSnapshot
// while `~` is still open, and setTaskReferencesCmd then resolves the task again
// against that newer state. A commit that spelled the whole checked set would
// read every reference the user never touched as an assertion — dropping one the
// load added and resurrecting one it removed — so the picker emits only what the
// user changed and the command merges that into the fresh state.
func TestReferencePickerPreservesConcurrentReferenceChanges(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		republish  []string
		wantAdd    []string
		wantRemove []string
	}{
		{
			// `home:chore` arrives while the overlay is up. It was never on a row the
			// user could uncheck, so the commit must not spell it as a removal.
			name:       "a reference added under the picker survives the commit",
			republish:  []string{"remote:target", "remote:legacy", "home:chore"},
			wantAdd:    []string{"remote:other"},
			wantRemove: []string{"remote:legacy"},
		},
		{
			// `remote:target` leaves while the overlay is up. It is still checked
			// because the user never touched it, so the commit must not read that
			// stale check as an addition and put it back.
			name:       "a reference removed under the picker stays removed",
			republish:  []string{"remote:legacy"},
			wantAdd:    []string{"remote:other"},
			wantRemove: []string{"remote:legacy"},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			model, subject, home := concurrentReferencePickerFixture(t)
			identity := taskref.For(subject, home)

			_, cmd := model.handleGlobalKeyPress(tea.KeyPressMsg{Code: '~', Text: "~"})
			if cmd != nil {
				model.handleOperationOverlayMessage(cmd())
			}
			picker := model.operationValuePicker
			if picker == nil || picker.provider != ValueProviderTaskReference {
				t.Fatalf("~ did not open the reference picker: %#v", picker)
			}

			// The user's whole edit: check `remote:other`, uncheck `remote:legacy`,
			// leave `remote:target` exactly as the picker preselected it. Both moves
			// go through the toggle the keyboard uses, because membership rather than
			// the checked set is what the commit is a difference against.
			for _, id := range referenceIDsFor("remote:other", "remote:legacy") {
				if !picker.surface.SetCursorToID(id) {
					t.Fatalf("the picker has no row for %s", id)
				}
				if err := picker.toggleCurrentMultiValue(); err != nil {
					t.Fatalf("toggling %s: %v", id, err)
				}
			}

			// Only now does the load land, after the picker was built and edited and
			// before Enter — the window the whole contract is about.
			model.adoptCrossProjectSnapshot([]*models.Project{republishedReferenceHome(t, testCase.republish...)})
			fresh, project := model.taskAndProjectForIdentity(identity)
			if fresh == nil || project == nil {
				t.Fatal("the republished snapshot left the task unresolvable")
			}
			gotStored := make([]string, 0, len(fresh.Dependencies))
			for _, reference := range fresh.Dependencies {
				gotStored = append(gotStored, reference.Locator())
			}
			if !slices.Equal(gotStored, testCase.republish) {
				t.Fatalf("the fresh task stores %v, want the republished %v", gotStored, testCase.republish)
			}

			_, done, msg := picker.handleKey(syntheticSpecialKey(tea.KeyEnter))
			if !done {
				t.Fatal("Enter did not close the picker")
			}
			selected, ok := msg.(OperationValueSelectedMsg)
			if !ok {
				t.Fatalf("Enter emitted %T, want a value selection", msg)
			}
			value, ok := selected.Value.(OperationTaskReferenceValue)
			if !ok {
				t.Fatalf("the picker emitted %T, want the reference difference", selected.Value)
			}
			if !slices.Equal(value.Add, []string{"remote:other"}) {
				t.Errorf("emitted add = %v, want the one row the user checked", value.Add)
			}
			if !slices.Equal(value.Remove, []string{"remote:legacy"}) {
				t.Errorf("emitted remove = %v, want the one row the user unchecked", value.Remove)
			}

			patch := taskReferencePatch(fresh, value)
			if got := dependencyLocatorStrings(patch.Add); !slices.Equal(got, testCase.wantAdd) {
				t.Errorf("add = %v, want %v", got, testCase.wantAdd)
			}
			if got := dependencyLocatorStrings(patch.Remove); !slices.Equal(got, testCase.wantRemove) {
				t.Errorf("remove = %v, want %v", got, testCase.wantRemove)
			}
			if patch.Clear {
				t.Error("the picker set Clear; it only ever spells a removal")
			}
		})
	}
}
