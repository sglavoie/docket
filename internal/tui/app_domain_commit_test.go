package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
)

// cloneMachineState is the boundary between the runtime snapshot and the copy a
// commit mutates. Lens definitions carry nested slices, so a shallow copy would
// let an uncommitted edit rewrite the snapshot in place.
func TestCloneMachineStateDeepCopiesNamedLenses(t *testing.T) {
	snapshot := config.NewMachineState()
	if len(snapshot.NamedLenses) == 0 {
		t.Fatal("expected seeded starter lenses")
	}
	original := snapshot.NamedLenses[0]
	if len(original.Tasks.States) == 0 {
		t.Fatalf("expected the Waiting starter to carry a state: %+v", original)
	}

	clone := cloneMachineState(snapshot)
	if &clone.NamedLenses[0] == &snapshot.NamedLenses[0] {
		t.Fatal("clone shares the lens backing array with the snapshot")
	}

	clone.Theme = "light"
	clone.NamedLenses[0].Name = "Renamed"
	clone.NamedLenses[0].Tasks.States[0] = "done"
	clone.NamedLenses = append(clone.NamedLenses, config.NamedLens{
		ID:   "9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a09",
		Name: "Added",
		Mode: config.LensModeAll,
	})

	if snapshot.Theme != "dark" {
		t.Errorf("clone wrote the snapshot theme: %q", snapshot.Theme)
	}
	if snapshot.NamedLenses[0].Name != original.Name {
		t.Errorf("clone wrote the snapshot lens name: %q", snapshot.NamedLenses[0].Name)
	}
	if snapshot.NamedLenses[0].Tasks.States[0] != original.Tasks.States[0] {
		t.Errorf("clone wrote the snapshot state slice: %v", snapshot.NamedLenses[0].Tasks.States)
	}
	if len(snapshot.NamedLenses) != len(config.StarterNamedLenses()) {
		t.Errorf("clone appended into the snapshot: %d lenses", len(snapshot.NamedLenses))
	}
}
