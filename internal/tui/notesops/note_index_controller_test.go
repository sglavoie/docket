package notesops

import (
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/tui/notes"
)

// TestNoteIndexControllerSupersedesAReadInFlight pins the generation discipline
// the cluster exists for: a fresh load claims the next generation, so a slow
// read that lands afterwards is no longer the one being awaited.
func TestNoteIndexControllerSupersedesAReadInFlight(t *testing.T) {
	t.Parallel()

	var controller noteIndex
	first := controller.beginLoad("owners-a")
	if !controller.awaits(first) {
		t.Fatalf("the controller awaits generation %d it just started", first)
	}

	second := controller.beginLoad("owners-b")
	if second == first {
		t.Fatalf("beginLoad reused generation %d, so a superseded read stays acceptable", second)
	}
	if controller.awaits(first) {
		t.Fatalf("generation %d is superseded and must no longer be awaited", first)
	}
	if !controller.awaits(second) {
		t.Fatalf("the controller awaits generation %d", second)
	}
}

// TestNoteIndexControllerReloadsOnlyForAnOwnerCatalogChange pins what keeps a
// full notes read off an ordinary keystroke: the fingerprint the last read was
// started against is what an availability change is compared to.
func TestNoteIndexControllerReloadsOnlyForAnOwnerCatalogChange(t *testing.T) {
	t.Parallel()

	var controller noteIndex
	if !controller.ownersMoved("owners-a") {
		t.Fatal("a controller that has never read owes its first load")
	}
	controller.beginLoad("owners-a")
	if controller.ownersMoved("owners-a") {
		t.Fatal("an unchanged owner catalog owes no reload")
	}
	if !controller.ownersMoved("owners-b") {
		t.Fatal("a changed owner catalog owes a reload")
	}
}

// TestNoteIndexControllerAdoptsOnlyWhatItAwaits pins that the retained rows are
// the shell's answer to the tier: an accepted read replaces them and nothing
// else does.
func TestNoteIndexControllerAdoptsOnlyWhatItAwaits(t *testing.T) {
	t.Parallel()

	live := []notes.LocatedNoteRow{noteIndexRow(app.GlobalNoteLocation(), "Live", "live.md")}
	stale := []notes.LocatedNoteRow{noteIndexRow(app.GlobalNoteLocation(), "Stale", "stale.md")}

	controller := noteIndex{generation: 3, rows: live}
	if controller.awaits(2) {
		t.Fatal("a superseded generation is not awaited")
	}
	controller.adopt(stale)
	if len(controller.rows) != 1 || controller.rows[0].Note.Title != "Stale" {
		t.Fatalf("rows = %+v, want the adopted read", controller.rows)
	}
}
