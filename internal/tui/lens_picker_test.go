package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
)

// The picker is driven exclusively through key messages here, because the shell
// routing, the nested overlays, and the retention rules only exist together: an
// assertion made by calling the model's own methods would prove nothing about
// which overlay a key actually reaches.

func lensPickerApp(t *testing.T) *AppModel {
	t.Helper()
	model := lensOperationApp(t)
	if cmd := model.openNamedLensPicker(); cmd != nil {
		drainLensPickerCmd(t, model, cmd)
	}
	if model.lensPicker == nil {
		t.Fatalf("openNamedLensPicker left no picker mounted: %v", model.err)
	}
	return model
}

// drainLensPickerCmd runs the overlay protocol to quiescence. Only messages the
// operation-overlay plumbing owns are dispatched: a status or reload message is
// the routed view's business and would drag unrelated machinery into a test
// about overlay nesting.
func drainLensPickerCmd(t *testing.T, model *AppModel, cmd tea.Cmd) {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for steps := 0; len(queue) > 0; steps++ {
		if steps > 64 {
			t.Fatal("lens overlay messages did not settle")
		}
		next := queue[0]
		queue = queue[1:]
		if next == nil {
			continue
		}
		switch msg := next().(type) {
		case nil:
		case tea.BatchMsg:
			queue = append(queue, []tea.Cmd(msg)...)
		default:
			if handled, follow := model.handleOperationOverlayMessage(msg); handled {
				queue = append(queue, follow)
			}
		}
	}
}

// pressLens sends each named key through the shell's global router, exactly as
// a keystroke arrives in the running app.
func pressLens(t *testing.T, model *AppModel, keys ...string) {
	t.Helper()
	for _, name := range keys {
		key, err := parseUITestKey(name)
		if err != nil {
			t.Fatalf("parse key %q: %v", name, err)
		}
		handled, cmd := model.handleGlobalKeyPress(key)
		if !handled {
			t.Fatalf("key %q was not handled by any overlay", name)
		}
		drainLensPickerCmd(t, model, cmd)
	}
}

func typeLens(t *testing.T, model *AppModel, text string) {
	t.Helper()
	for _, r := range text {
		pressLens(t, model, string(r))
	}
}

func lensPickerRowTitles(t *testing.T, model *AppModel) []string {
	t.Helper()
	if model.lensPicker == nil {
		t.Fatal("no lens picker is mounted")
	}
	items := model.lensPicker.surface.Items()
	titles := make([]string, 0, len(items))
	for _, item := range items {
		titles = append(titles, item.PrimaryText)
	}
	return titles
}

func lensPickerCursorTitle(t *testing.T, model *AppModel) string {
	t.Helper()
	if model.lensPicker == nil {
		t.Fatal("no lens picker is mounted")
	}
	item, ok := model.lensPicker.surface.CurrentItem()
	if !ok {
		t.Fatal("the lens picker has no current row")
	}
	return item.PrimaryText
}

func sheetRowTitles(t *testing.T, model *AppModel) []string {
	t.Helper()
	if model.contextActionSheet == nil {
		t.Fatal("no action sheet is mounted")
	}
	items := model.contextActionSheet.surface.Items()
	titles := make([]string, 0, len(items))
	for _, item := range items {
		titles = append(titles, item.PrimaryText)
	}
	return titles
}

// selectLensRow parks the picker cursor on one saved lens by name so a test can
// name its target without depending on how many keys reach it.
func selectLensRow(t *testing.T, model *AppModel, name string) config.NamedLens {
	t.Helper()
	lens := lensNamed(t, model, name)
	if !model.lensPicker.SelectLens(lens.ID) {
		t.Fatalf("lens %q is not listed in the picker", name)
	}
	return lens
}

// --- population and ordering ------------------------------------------------

func TestLensPickerListsBothActionRowsFirstThenLensesAlphabetically(t *testing.T) {
	model := lensPickerApp(t)

	want := []string{
		lensPickerCreateLabel, lensPickerAddTemplateLabel,
		"High Priority", "Stale", "Waiting",
	}
	if got := lensPickerRowTitles(t, model); !slices.Equal(got, want) {
		t.Fatalf("picker rows = %v, want %v", got, want)
	}
	if got := lensPickerCursorTitle(t, model); got != lensPickerCreateLabel {
		t.Errorf("opening cursor = %q, want the save row", got)
	}
	items := model.lensPicker.surface.Items()
	if got := items[1].ID; got != lensPickerAddTemplateItemID {
		t.Errorf("second row ID = %q, want %q", got, lensPickerAddTemplateItemID)
	}
	if got := items[1].SecondaryText; got != lensPickerAddTemplateSecondary {
		t.Errorf("add row secondary = %q, want %q", got, lensPickerAddTemplateSecondary)
	}
	for _, alias := range []string{"template", "preset", "built-in view"} {
		if !containsString(items[1].Aliases, alias) {
			t.Errorf("add row aliases = %v, want %q among them", items[1].Aliases, alias)
		}
	}
	if got := items[1].OperationIDs; len(got) != 1 || got[0] != operationLensAddTemplate {
		t.Errorf("add row operations = %v, want just %q", got, operationLensAddTemplate)
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
	item, _ := model.lensPicker.surface.CurrentItem()
	if !strings.Contains(ansi.Strip(item.PrimaryMarker), "●") {
		t.Errorf("the active lens row carries no active marker: %q", ansi.Strip(item.PrimaryMarker))
	}
}

// --- navigation -------------------------------------------------------------

func TestLensPickerNavigationKeysMoveTheCursor(t *testing.T) {
	model := lensPickerApp(t)

	// One move down reaches the second action row, not the first saved lens:
	// both fixed rows sit above the population.
	pressLens(t, model, "j")
	if got := lensPickerCursorTitle(t, model); got != lensPickerAddTemplateLabel {
		t.Fatalf("after j cursor = %q", got)
	}
	pressLens(t, model, "down")
	if got := lensPickerCursorTitle(t, model); got != "High Priority" {
		t.Fatalf("after down cursor = %q", got)
	}
	pressLens(t, model, "k", "up")
	if got := lensPickerCursorTitle(t, model); got != lensPickerCreateLabel {
		t.Fatalf("after k/up cursor = %q", got)
	}
	pressLens(t, model, "end")
	if got := lensPickerCursorTitle(t, model); got != "Waiting" {
		t.Fatalf("after end cursor = %q", got)
	}
	pressLens(t, model, "home")
	if got := lensPickerCursorTitle(t, model); got != lensPickerCreateLabel {
		t.Fatalf("after home cursor = %q", got)
	}
	pressLens(t, model, "pgdown")
	if got := lensPickerCursorTitle(t, model); got == lensPickerCreateLabel {
		t.Error("pgdown left the cursor on the save row")
	}
	pressLens(t, model, "pgup")
	if got := lensPickerCursorTitle(t, model); got != lensPickerCreateLabel {
		t.Fatalf("after pgup cursor = %q", got)
	}
}

// --- search cascade ---------------------------------------------------------

func TestLensPickerSearchNarrowsLensesAndKeepsBothActionRowsAvailable(t *testing.T) {
	model := lensPickerApp(t)

	pressLens(t, model, "/")
	typeLens(t, model, "stal")
	want := []string{lensPickerCreateLabel, lensPickerAddTemplateLabel, "Stale"}
	if got := lensPickerRowTitles(t, model); !slices.Equal(got, want) {
		t.Fatalf("filtered rows = %v, want %v", got, want)
	}

	typeLens(t, model, "zzz")
	actions := []string{lensPickerCreateLabel, lensPickerAddTemplateLabel}
	if got := lensPickerRowTitles(t, model); !slices.Equal(got, actions) {
		t.Fatalf("no-match rows = %v, want the two action rows alone", got)
	}
	if state := model.lensPicker.stateLine(); !strings.Contains(state, "No matching lenses") {
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
	if model.lensPicker == nil {
		t.Fatal("the first Esc closed the picker instead of clearing the query")
	}
	if model.lensPicker.query != "" || !model.lensPicker.searching {
		t.Fatalf("first Esc left query=%q searching=%v", model.lensPicker.query, model.lensPicker.searching)
	}

	pressLens(t, model, "esc")
	if model.lensPicker == nil {
		t.Fatal("the second Esc closed the picker instead of leaving search")
	}
	if model.lensPicker.searching {
		t.Error("the second Esc left the picker in search mode")
	}

	pressLens(t, model, "esc")
	if model.lensPicker != nil {
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
	if model.lensPicker.searching || model.lensPicker.query != "zzz" {
		t.Fatalf("leaving search dropped the query: %q", model.lensPicker.query)
	}
	pressLens(t, model, "enter")

	if model.operationTextPrompt == nil {
		t.Fatal("Enter on the save row raised no name prompt")
	}
	if model.lensPicker == nil {
		t.Error("the picker must stay mounted beneath the name prompt")
	}
}

// --- apply ------------------------------------------------------------------

func TestLensPickerEnterAppliesLensAndClosesThePicker(t *testing.T) {
	model := lensPickerApp(t)
	selectLensRow(t, model, "Waiting")

	pressLens(t, model, "enter")

	if model.lensPicker != nil {
		t.Error("applying a lens left the picker mounted")
	}
	if !model.crossOutliner.hasActiveLens() {
		t.Fatal("applying a lens installed nothing")
	}
	if got := model.crossOutliner.activeLens.name; got != "Waiting" {
		t.Errorf("active lens = %q, want Waiting", got)
	}
}

// --- ordinal quick-apply ----------------------------------------------------
