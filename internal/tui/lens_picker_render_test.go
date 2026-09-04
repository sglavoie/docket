package tui

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/lens"
	"github.com/sglavoie/tt/internal/tui/listsurface"
)

// lensPickerItemNamed returns one built row by its primary text, which is how a
// color assertion names its target without depending on the row's position.
func lensPickerItemNamed(t *testing.T, model *AppModel, name string) listsurface.ListItem {
	t.Helper()
	if model.lenses.picker == nil {
		t.Fatal("no lens picker is mounted")
	}
	for _, item := range model.lenses.picker.Surface.Items() {
		if item.PrimaryText == name {
			return item
		}
	}
	t.Fatalf("no picker row named %q in %v", name, lensPickerRowTitles(t, model))
	return listsurface.ListItem{}
}

// assertLensRowStyle holds one row to the style its own definition resolves to,
// which is the whole contract: the picker never names a color, it asks.
func assertLensRowStyle(t *testing.T, item listsurface.ListItem, saved config.NamedLens) {
	t.Helper()
	want, ok := lens.NamedLensPrimaryStyle(saved)
	if !ok {
		if item.PrimaryStyle != nil {
			t.Fatalf("row %q wears %v for a neutral definition",
				item.PrimaryText, item.PrimaryStyle.GetForeground())
		}
		return
	}
	if item.PrimaryStyle == nil {
		t.Fatalf("row %q carries no style, want %v", item.PrimaryText, want.GetForeground())
	}
	if got := item.PrimaryStyle.GetForeground(); got != want.GetForeground() {
		t.Fatalf("row %q foreground = %v, want %v", item.PrimaryText, got, want.GetForeground())
	}
}

func TestLensPickerColorsSavedRowsAndLeavesActionRowsNeutral(t *testing.T) {
	model := lensPickerApp(t)

	for _, name := range []string{"High Priority", "Stale", "Waiting"} {
		item := lensPickerItemNamed(t, model, name)
		assertLensRowStyle(t, item, lensNamed(t, model, name))
		if item.PrimaryStyle == nil {
			t.Errorf("the %q fixture resolves no accent, so this test proves nothing", name)
		}
		// The accent rides PrimaryStyle alone: the searched, sorted, and truncated
		// text stays the plain name.
		if item.PrimaryText != ansi.Strip(item.PrimaryText) {
			t.Errorf("row %q embeds styling in its primary text: %q", name, item.PrimaryText)
		}
	}
	for _, item := range model.lenses.picker.Surface.Items() {
		if item.Data != nil {
			continue
		}
		if item.PrimaryStyle != nil {
			t.Errorf("action row %q wears %v", item.PrimaryText, item.PrimaryStyle.GetForeground())
		}
	}
}

// A query change rebuilds every row, so the filtered population must keep the
// color the unfiltered one had, and the ordering must not move.
func TestLensPickerSearchKeepsSavedRowColorAndOrder(t *testing.T) {
	model := lensPickerApp(t)
	pressLens(t, model, "/")
	typeLens(t, model, "stal")

	want := []string{lens.LensPickerCreateLabel, lens.LensPickerAddTemplateLabel, "Stale"}
	if got := lensPickerRowTitles(t, model); !slices.Equal(got, want) {
		t.Fatalf("rows under a query = %v, want %v", got, want)
	}
	stale := lensPickerItemNamed(t, model, "Stale")
	assertLensRowStyle(t, stale, lensNamed(t, model, "Stale"))
	if stale.PrimaryStyle == nil {
		t.Error("a filtered row lost the accent its definition resolves")
	}
}

func TestLensPickerColorSurvivesRenameAndFollowsReplacement(t *testing.T) {
	model := lensPickerApp(t)
	before := selectLensRow(t, model, "Waiting")
	beforeStyle, ok := lens.NamedLensPrimaryStyle(before)
	if !ok {
		t.Fatal("the Waiting fixture resolves no accent")
	}

	pressLens(t, model, "space")
	pressLens(t, model, "enter") // Rename
	for range len("Waiting") {
		pressLens(t, model, "backspace")
	}
	typeLens(t, model, "Blocked")
	pressLens(t, model, "enter")

	// A rename changes no facet, so the same definition must still resolve to the
	// same accent under its new name.
	renamed := lensPickerItemNamed(t, model, "Blocked")
	assertLensRowStyle(t, renamed, lensNamed(t, model, "Blocked"))
	if renamed.PrimaryStyle == nil ||
		renamed.PrimaryStyle.GetForeground() != beforeStyle.GetForeground() {
		t.Fatalf("rename changed the accent to %v, want %v", renamed.PrimaryStyle, beforeStyle.GetForeground())
	}

	// Replacing under the same UUID writes a new definition, which is exactly what
	// the row must recompute from.
	model.crossOutliner.Filter.Pinned = true
	model.crossOutliner.AdoptFilterCarriers()
	pressLens(t, model, "space")
	pressLens(t, model, "j")
	pressLens(t, model, "enter")
	pressLens(t, model, "y")

	replaced := lensNamed(t, model, "Blocked")
	if replaced.ID != before.ID {
		t.Fatal("the replace minted a new identity")
	}
	// The role ladder itself is lens-internal, so the divergence is asserted
	// through the accent it resolves to, which is the only part of it a row shows.
	got, gotOK := lens.NamedLensPrimaryStyle(replaced)
	was, wasOK := lens.NamedLensPrimaryStyle(before)
	if gotOK == wasOK && (!gotOK || got.GetForeground() == was.GetForeground()) {
		t.Fatal("the replacement resolves the same accent, so recomputation is unobservable")
	}
	assertLensRowStyle(t, lensPickerItemNamed(t, model, "Blocked"), replaced)
}

// Color is read from the definition, never from the record's origin: a built-in
// identity with nothing semantic left in it goes neutral, and a user-created
// UUID carrying a recognized facet is colored like any other.
func TestLensPickerColorFollowsTheDefinitionNotTheBuiltinOrigin(t *testing.T) {
	model := lensPickerApp(t)
	builtinID := lensNamed(t, model, "Waiting").ID

	neutralBuiltin := config.NamedLens{
		ID:   builtinID,
		Name: "Everything",
		Mode: config.LensModeAll,
		Sort: config.LensSort{Field: config.LensSortDefault},
	}
	custom := config.NamedLens{
		ID:    "22222222-2222-4222-8222-222222222222",
		Name:  "Blocked Work",
		Mode:  config.LensModeAll,
		Query: "$" + string(models.StateWaiting),
		Sort:  config.LensSort{Field: config.LensSortDefault},
	}
	if err := model.lenses.picker.Refresh(lens.NamedLensPickerSnapshot{
		Lenses: config.NamedLensList{neutralBuiltin, custom},
	}); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	if got := lensPickerItemNamed(t, model, "Everything").PrimaryStyle; got != nil {
		t.Errorf("a neutral built-in identity wears %v", got.GetForeground())
	}
	colored := lensPickerItemNamed(t, model, "Blocked Work")
	assertLensRowStyle(t, colored, custom)
	if colored.PrimaryStyle == nil {
		t.Error("a custom lens with an explicit Waiting state received no accent")
	}
}

func TestLensPickerRenderColorsOrdinaryRowsAndYieldsUnderTheCursor(t *testing.T) {
	model := lensPickerApp(t)
	waiting := lensNamed(t, model, "Waiting")
	style, ok := lens.NamedLensPrimaryStyle(waiting)
	if !ok {
		t.Fatal("the Waiting fixture resolves no accent")
	}
	if _, err := model.crossOutliner.ApplyNamedLens(waiting); err != nil {
		t.Fatalf("apply lens: %v", err)
	}
	if err := model.lenses.picker.Refresh(model.namedLensPickerSnapshot()); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	// The cursor opens on the save row, so the active lens is an ordinary row:
	// colored, and still carrying its active marker.
	rendered := model.lenses.picker.Render(80)
	if !strings.Contains(rendered, style.Render("Waiting")) {
		t.Errorf("an ordinary saved row is not colored:\n%q", rendered)
	}
	if !strings.Contains(ansi.Strip(rendered), "● Waiting") {
		t.Errorf("the active marker no longer precedes the colored name:\n%s", ansi.Strip(rendered))
	}

	// Under the cursor SelectedLine owns the whole line, so the accent must be
	// gone while the text stays exactly where it was.
	if !model.lenses.picker.SelectLens(waiting.ID) {
		t.Fatal("the active lens is not listed")
	}
	cursored := model.lenses.picker.Render(80)
	if strings.Contains(cursored, style.Render("Waiting")) {
		t.Errorf("the cursor row kept its accent:\n%q", cursored)
	}
	if !strings.Contains(ansi.Strip(cursored), "● Waiting") {
		t.Errorf("the cursor row lost the name or its marker:\n%s", ansi.Strip(cursored))
	}
}

// --- rendering --------------------------------------------------------------

func TestLensPickerRendersEmptyNoMatchAndOversizedRows(t *testing.T) {
	model := lensPickerApp(t)

	long := strings.Repeat("Quarterly Planning ", 12)
	oversized := config.NamedLens{
		ID:    "11111111-1111-4111-8111-111111111111",
		Name:  long,
		Mode:  config.LensModeAll,
		Query: "*pin ?stale ?deferred",
		Sort:  config.LensSort{Field: config.LensSortDefault},
	}
	if _, ok := lens.NamedLensPrimaryStyle(oversized); !ok {
		t.Fatal("the oversized fixture is neutral, so it cannot test a colored truncation")
	}
	snapshot := lens.NamedLensPickerSnapshot{
		Lenses:         config.NamedLensList{oversized},
		CurrentSummary: strings.Repeat("summary segment · ", 20),
	}
	if err := model.lenses.picker.Refresh(snapshot); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	// The row really is colored while it is being truncated, so the width
	// assertions below are about a styled row rather than a plain one.
	if got := lensPickerItemNamed(t, model, long).PrimaryStyle; got == nil {
		t.Fatal("the oversized row carries no accent, so its truncation is untested")
	}
	for _, width := range []int{chrome.MinTerminalWidth, 80, 200} {
		rendered := ansi.Strip(model.lenses.picker.Render(width))
		assertNoLensPickerOverflow(t, rendered, width)
		// An accent is styling, never columns: the name survives truncation to the
		// same visible text a neutral row would have shown.
		if !strings.Contains(rendered, "Quarterly") {
			t.Errorf("the colored row lost its name at width %d:\n%s", width, rendered)
		}
	}
	// The minimum supported terminal must still show the modal's own chrome,
	// which is what renderWithin's reservation buys.
	rendered := ansi.Strip(model.lenses.picker.RenderWithin(chrome.MinTerminalWidth, chrome.MinTerminalHeight))
	assertNoLensPickerOverflow(t, rendered, chrome.MinTerminalWidth)
	if lines := strings.Count(rendered, "\n") + 1; lines > chrome.MinTerminalHeight {
		t.Errorf("minimum-terminal render is %d lines, taller than %d", lines, chrome.MinTerminalHeight)
	}

	if err := model.lenses.picker.Refresh(lens.NamedLensPickerSnapshot{}); err != nil {
		t.Fatalf("refresh empty: %v", err)
	}
	empty := ansi.Strip(model.lenses.picker.Render(80))
	if !strings.Contains(empty, "No saved lenses yet") {
		t.Errorf("empty render names no empty state:\n%s", empty)
	}
	if !strings.Contains(empty, "Save current view") {
		t.Errorf("empty render dropped the save row:\n%s", empty)
	}
	if !strings.Contains(empty, "Add built-in view") {
		t.Errorf("empty render dropped the add row:\n%s", empty)
	}
}

func assertNoLensPickerOverflow(t *testing.T, rendered string, width int) {
	t.Helper()
	for _, line := range strings.Split(rendered, "\n") {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("line is %d columns wide at width %d: %q", got, width, line)
		}
	}
}

// --- teardown ---------------------------------------------------------------

func TestLensPickerIsClearedByNavigationCollapse(t *testing.T) {
	model := lensPickerApp(t)
	selectLensRow(t, model, "Stale")
	pressLens(t, model, "space")
	pressLens(t, model, "j", "j")
	pressLens(t, model, "enter")

	model.clearTransientNavigationState()

	if model.lenses.picker != nil || model.operations.actionSheet != nil ||
		model.operations.confirm.IsActive() {
		t.Error("a navigation collapse left a lens overlay mounted")
	}
	if model.operations.pendingRequest.ID != "" {
		t.Error("a navigation collapse left a pending operation")
	}
	if model.Overlays().Active() || model.Overlays().HasActiveTextInput() {
		t.Error("the overlay manager still reports the picker")
	}
}

func TestLensPickerDoesNotSwallowQuit(t *testing.T) {
	model := lensPickerApp(t)
	pressLens(t, model, "/")
	typeLens(t, model, "st")

	// Quit outranks every overlay, search mode included: the picker must not
	// absorb Ctrl+C as one more character.
	handled, cmd := model.handleGlobalKeyPress(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !handled || cmd == nil {
		t.Fatalf("ctrl+c handled=%v cmd=%v, want the quit command", handled, cmd)
	}
	if model.lenses.picker.Query != "st" {
		t.Errorf("ctrl+c was typed into the search query: %q", model.lenses.picker.Query)
	}
}

func TestLensPickerCountsAsAnActiveOverlayAndOwnsTextInput(t *testing.T) {
	model := lensPickerApp(t)

	if !model.Overlays().Active() {
		t.Error("a mounted picker is not reported as an active overlay")
	}
	if !model.Overlays().HasActiveTextInput() {
		t.Error("a mounted picker does not claim text input, so root keys would leak into it")
	}
	model.Overlays().ClearAll()
	if model.lenses.picker != nil {
		t.Error("ClearAll left the picker mounted")
	}
}

// --- helpers ----------------------------------------------------------------

// lensNamedErr reports whether one saved lens exists by name. Only presence is
// ever asserted, so the record itself is deliberately not returned.
func lensNamedErr(model *AppModel, name string) error {
	machine := model.domain.runtime.Machine()
	if machine == nil {
		return errors.New("machine-local state is unavailable")
	}
	for _, saved := range machine.NamedLenses {
		if saved.Name == name {
			return nil
		}
	}
	return fmt.Errorf("no saved lens named %q", name)
}
