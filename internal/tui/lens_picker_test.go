package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/lens"
)

// The picker is driven exclusively through key messages here, because the shell
// routing, the nested overlays, and the retention rules only exist together: an
// assertion made by calling the model's own methods would prove nothing about
// which overlay a key actually reaches.

// --- population and ordering ------------------------------------------------

func TestLensPickerListsBothActionRowsFirstThenLensesAlphabetically(t *testing.T) {
	model := lensPickerApp(t)

	want := []string{
		lens.LensPickerCreateLabel, lens.LensPickerAddTemplateLabel,
		"High Priority", "Stale", "Waiting",
	}
	if got := lensPickerRowTitles(t, model); !slices.Equal(got, want) {
		t.Fatalf("picker rows = %v, want %v", got, want)
	}
	if got := lensPickerCursorTitle(t, model); got != lens.LensPickerCreateLabel {
		t.Errorf("opening cursor = %q, want the save row", got)
	}
	items := model.lenses.Picker().Surface.Items()
	if got := items[1].ID; got != lens.LensPickerAddTemplateItemID {
		t.Errorf("second row ID = %q, want %q", got, lens.LensPickerAddTemplateItemID)
	}
	if got := items[1].SecondaryText; got != lens.LensPickerAddTemplateSecondary {
		t.Errorf("add row secondary = %q, want %q", got, lens.LensPickerAddTemplateSecondary)
	}
	for _, alias := range []string{"template", "preset", "built-in view"} {
		if !containsString(items[1].Aliases, alias) {
			t.Errorf("add row aliases = %v, want %q among them", items[1].Aliases, alias)
		}
	}
	if got := items[1].OperationIDs; len(got) != 1 || got[0] != lens.OperationLensAddTemplate {
		t.Errorf("add row operations = %v, want just %q", got, lens.OperationLensAddTemplate)
	}
}

func TestLensPickerOpensOnTheActiveLensAndMarksIt(t *testing.T) {
	model := lensOperationApp(t)
	waiting := lensNamed(t, model, "Waiting")
	if _, err := model.crossOutliner.ApplyNamedLens(waiting); err != nil {
		t.Fatalf("apply lens: %v", err)
	}
	if cmd := model.openNamedLensPicker(); cmd != nil {
		drainLensPickerCmd(t, model, cmd)
	}

	if got := lensPickerCursorTitle(t, model); got != "Waiting" {
		t.Fatalf("cursor = %q, want the active lens", got)
	}
	item, _ := model.lenses.Picker().Surface.CurrentItem()
	if !strings.Contains(ansi.Strip(item.PrimaryMarker), "●") {
		t.Errorf("the active lens row carries no active marker: %q", ansi.Strip(item.PrimaryMarker))
	}
}

// --- navigation -------------------------------------------------------------

func TestLensPickerDisplaysAndSearchesProjectScope(t *testing.T) {
	model := lensOperationApp(t)
	model.domain.runtime = testConfigWithProjects(&models.Project{ID: "project-id", Slug: "web", Name: "Website"})
	model.domain.runtime.Machine().NamedLenses = config.NamedLensList{
		{ID: "scoped", Name: "Planning", Mode: config.LensModeAll, Scope: "project-id"},
		{ID: "unscoped", Name: "Everywhere", Mode: config.LensModeAll},
	}
	model.openNamedLensPicker()
	pressLens(t, model, "/")
	typeLens(t, model, "web")
	want := []string{lens.LensPickerCreateLabel, lens.LensPickerAddTemplateLabel, "Planning"}
	if got := lensPickerRowTitles(t, model); !slices.Equal(got, want) {
		t.Fatalf("scope search rows = %v, want %v", got, want)
	}
	items := model.lenses.Picker().Surface.Items()
	if got := items[2].SecondaryText; !strings.Contains(got, "Project: web") {
		t.Fatalf("scoped lens summary = %q", got)
	}
}

func TestLensPickerEmptySearchGuidanceFollowsEditingState(t *testing.T) {
	model := lensPickerApp(t)
	pressLens(t, model, "/")
	typeLens(t, model, "zzzzzzzz")
	if got := model.lenses.Picker().StateLine(); got != "No matching lenses — Esc clears the search." {
		t.Fatalf("editing guidance = %q", got)
	}
	pressLens(t, model, "enter")
	if got := model.lenses.Picker().StateLine(); got != "No matching lenses — / edits the search." {
		t.Fatalf("browsing guidance = %q", got)
	}
	pressLens(t, model, "/", "esc")
	if model.lenses.Picker().Query != "" || model.lenses.Picker().Listed == 0 {
		t.Fatal("following the guidance did not restore the lens list")
	}
	pressLens(t, model, "enter", "esc")
	if model.lenses.Picker() != nil {
		t.Fatal("Esc in browse mode did not close the picker")
	}
}

func TestLensPickerNavigationKeysMoveTheCursor(t *testing.T) {
	model := lensPickerApp(t)

	// One move down reaches the second action row, not the first saved lens:
	// both fixed rows sit above the population.
	pressLens(t, model, "j")
	if got := lensPickerCursorTitle(t, model); got != lens.LensPickerAddTemplateLabel {
		t.Fatalf("after j cursor = %q", got)
	}
	pressLens(t, model, "down")
	if got := lensPickerCursorTitle(t, model); got != "High Priority" {
		t.Fatalf("after down cursor = %q", got)
	}
	pressLens(t, model, "k", "up")
	if got := lensPickerCursorTitle(t, model); got != lens.LensPickerCreateLabel {
		t.Fatalf("after k/up cursor = %q", got)
	}
	pressLens(t, model, "end")
	if got := lensPickerCursorTitle(t, model); got != "Waiting" {
		t.Fatalf("after end cursor = %q", got)
	}
	pressLens(t, model, "home")
	if got := lensPickerCursorTitle(t, model); got != lens.LensPickerCreateLabel {
		t.Fatalf("after home cursor = %q", got)
	}
	pressLens(t, model, "pgdown")
	if got := lensPickerCursorTitle(t, model); got == lens.LensPickerCreateLabel {
		t.Error("pgdown left the cursor on the save row")
	}
	pressLens(t, model, "pgup")
	if got := lensPickerCursorTitle(t, model); got != lens.LensPickerCreateLabel {
		t.Fatalf("after pgup cursor = %q", got)
	}
}

// --- search cascade ---------------------------------------------------------

func TestLensPickerSearchNarrowsLensesAndKeepsBothActionRowsAvailable(t *testing.T) {
	model := lensPickerApp(t)

	pressLens(t, model, "/")
	typeLens(t, model, "stal")
	want := []string{lens.LensPickerCreateLabel, lens.LensPickerAddTemplateLabel, "Stale"}
	if got := lensPickerRowTitles(t, model); !slices.Equal(got, want) {
		t.Fatalf("filtered rows = %v, want %v", got, want)
	}

	typeLens(t, model, "zzz")
	actions := []string{lens.LensPickerCreateLabel, lens.LensPickerAddTemplateLabel}
	if got := lensPickerRowTitles(t, model); !slices.Equal(got, actions) {
		t.Fatalf("no-match rows = %v, want the two action rows alone", got)
	}
	if state := model.lenses.Picker().StateLine(); !strings.Contains(state, "No matching lenses") {
		t.Errorf("no-match state line = %q", state)
	}

	pressLens(t, model, "backspace", "backspace", "backspace")
	if got := lensPickerRowTitles(t, model); !slices.Equal(got, want) {
		t.Fatalf("after backspace rows = %v, want %v", got, want)
	}
}

func TestLensPickerEscapeClearsSearchThenLeavesSearchThenCloses(t *testing.T) {
	model := lensPickerApp(t)
	pressLens(t, model, "/")
	typeLens(t, model, "stale")

	pressLens(t, model, "esc")
	if model.lenses.Picker() == nil {
		t.Fatal("the first Esc closed the picker instead of clearing the query")
	}
	if model.lenses.Picker().Query != "" || !model.lenses.Picker().Searching {
		t.Fatalf("first Esc left query=%q searching=%v", model.lenses.Picker().Query, model.lenses.Picker().Searching)
	}

	pressLens(t, model, "esc")
	if model.lenses.Picker() == nil {
		t.Fatal("the second Esc closed the picker instead of leaving search")
	}
	if model.lenses.Picker().Searching {
		t.Error("the second Esc left the picker in search mode")
	}

	pressLens(t, model, "esc")
	if model.lenses.Picker() != nil {
		t.Error("the third Esc left the picker mounted")
	}
}

func TestLensPickerSearchEnterOnSaveRowStillOpensTheNamePrompt(t *testing.T) {
	model := lensPickerApp(t)
	pressLens(t, model, "/")
	typeLens(t, model, "zzz")
	// Enter leaves search with the query retained, so the no-match narrowing is
	// still in force and the save row is the only survivor — which is the whole
	// point of exempting it from the filter.
	pressLens(t, model, "enter")
	if model.lenses.Picker().Searching || model.lenses.Picker().Query != "zzz" {
		t.Fatalf("leaving search dropped the query: %q", model.lenses.Picker().Query)
	}
	pressLens(t, model, "enter")

	if model.operations.TextPrompt == nil {
		t.Fatal("Enter on the save row raised no name prompt")
	}
	if model.lenses.Picker() == nil {
		t.Error("the picker must stay mounted beneath the name prompt")
	}
}

// --- apply ------------------------------------------------------------------

func TestLensPickerEnterAppliesLensAndClosesThePicker(t *testing.T) {
	model := lensPickerApp(t)
	selectLensRow(t, model, "Waiting")

	pressLens(t, model, "enter")

	if model.lenses.Picker() != nil {
		t.Error("applying a lens left the picker mounted")
	}
	if !model.crossOutliner.HasActiveLens() {
		t.Fatal("applying a lens installed nothing")
	}
	if got := model.crossOutliner.Lens.Active.Name; got != "Waiting" {
		t.Errorf("active lens = %q, want Waiting", got)
	}
}

// --- ordinal quick-apply ----------------------------------------------------
