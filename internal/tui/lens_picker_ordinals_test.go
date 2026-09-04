package tui

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/lens"
)

// lensPickerOrdinalSlots returns the fixed-width number column of every row, in
// display order. The slot is read off PrimaryMarker rather than off the rendered
// line because that is the seam the marker column is charged to: whatever the
// list draws around it, this is the two columns the digit owns.
func lensPickerOrdinalSlots(t *testing.T, model *AppModel) []string {
	t.Helper()
	if model.lensPicker == nil {
		t.Fatal("no lens picker is mounted")
	}
	items := model.lensPicker.Surface.Items()
	slots := make([]string, 0, len(items))
	for _, item := range items {
		plain := []rune(ansi.Strip(item.PrimaryMarker))
		if len(plain) < lens.LensPickerOrdinalMarkerWidth {
			t.Fatalf("row %q has no number column: %q", item.PrimaryText, string(plain))
		}
		slot := string(plain[:lens.LensPickerOrdinalMarkerWidth])
		if got := lipgloss.Width(slot); got != lens.LensPickerOrdinalMarkerWidth {
			t.Fatalf("row %q number column is %d columns: %q", item.PrimaryText, got, slot)
		}
		slots = append(slots, slot)
	}
	return slots
}

func TestLensPickerDigitAppliesTheNthListedLensAndCloses(t *testing.T) {
	model := lensPickerApp(t)

	// The two action rows head the list but are unnumbered, so 2 is the second
	// saved lens rather than the second row.
	pressLens(t, model, "2")

	if model.lensPicker != nil {
		t.Error("a digit apply left the picker mounted")
	}
	if !model.crossOutliner.hasActiveLens() {
		t.Fatal("a digit apply installed nothing")
	}
	if got := model.crossOutliner.lens.active.name; got != "Stale" {
		t.Errorf("active lens = %q, want the second listed lens Stale", got)
	}
}

func TestLensPickerOrdinalsNumberOnlyTheListedSavedLenses(t *testing.T) {
	model := lensPickerApp(t)

	want := []string{"  ", "  ", "1 ", "2 ", "3 "}
	if got := lensPickerOrdinalSlots(t, model); !slices.Equal(got, want) {
		t.Fatalf("number columns = %q, want %q", got, want)
	}
	if got := model.lensPicker.Ordinals; len(got) != 3 {
		t.Fatalf("ordinal assignment holds %d lenses, want 3", len(got))
	}
	if got := model.lensPicker.Ordinals[0]; got != lensNamed(t, model, "High Priority").ID {
		t.Errorf("ordinal 1 = %s, want the first listed lens", got)
	}
}

func TestLensPickerNarrowedListRenumbersFromOneAndTheDigitAppliesIt(t *testing.T) {
	model := lensPickerApp(t)
	waiting := lensNamed(t, model, "Waiting")

	pressLens(t, model, "/")
	typeLens(t, model, "wait")
	want := []string{lens.LensPickerCreateLabel, lens.LensPickerAddTemplateLabel, "Waiting"}
	if got := lensPickerRowTitles(t, model); !slices.Equal(got, want) {
		t.Fatalf("filtered rows = %v, want %v", got, want)
	}
	// Waiting is the third lens unfiltered; narrowed it is the first, and the row
	// says so rather than keeping a number nothing on screen answers to.
	if got := lensPickerOrdinalSlots(t, model); !slices.Equal(got, []string{"  ", "  ", "1 "}) {
		t.Fatalf("narrowed number columns = %q, want the survivor renumbered to 1", got)
	}

	// Enter leaves search with the query retained, so the digit acts on the
	// narrowing that is still on screen.
	pressLens(t, model, "enter")
	if model.lensPicker.Searching || model.lensPicker.Query != "wait" {
		t.Fatalf("leaving search dropped the query: %q", model.lensPicker.Query)
	}
	pressLens(t, model, "1")

	if model.lensPicker != nil {
		t.Error("a digit apply left the picker mounted")
	}
	if !model.crossOutliner.hasActiveLens() ||
		model.crossOutliner.lens.active.lensID != waiting.ID {
		t.Fatal("1 applied something other than the narrowed row")
	}
}

func TestLensPickerDigitsTypedIntoTheQueryStayLiteral(t *testing.T) {
	model := lensPickerApp(t)

	pressLens(t, model, "/")
	typeLens(t, model, "1")

	if model.lensPicker == nil {
		t.Fatal("a digit typed into the query closed the picker")
	}
	if got := model.lensPicker.Query; got != "1" {
		t.Fatalf("query = %q, want the literal digit", got)
	}
	if model.crossOutliner.hasActiveLens() {
		t.Error("a digit typed into the query applied a lens")
	}
	rendered := ansi.Strip(model.lensPicker.Render(80))
	if !strings.Contains(rendered, "Search: 1") {
		t.Errorf("the search line does not show the typed digit:\n%s", rendered)
	}
}

func TestLensPickerOutOfRangeDigitRefusesAndMovesNothing(t *testing.T) {
	model := lensPickerApp(t)
	before := lensPickerCursorTitle(t, model)

	pressLens(t, model, "7")

	if model.lensPicker == nil {
		t.Fatal("an out-of-range digit closed the picker")
	}
	if got := model.lensPicker.Notice(); got != "Only 3 lenses are numbered — press 1-3" {
		t.Errorf("refusal notice = %q", got)
	}
	if got := lensPickerCursorTitle(t, model); got != before {
		t.Errorf("cursor moved to %q, want it left on %q", got, before)
	}
	if model.crossOutliner.hasActiveLens() {
		t.Error("an out-of-range digit applied a lens")
	}
	if got := len(machineLenses(t, model)); got != len(config.StarterNamedLenses()) {
		t.Errorf("an out-of-range digit wrote %d lenses", got)
	}
}

func TestLensPickerActionRowsAndTheTenthLensCarryNoOrdinal(t *testing.T) {
	model := lensPickerApp(t)
	lenses := make(config.NamedLensList, 0, 12)
	for index := range 12 {
		lenses = append(lenses, config.NamedLens{
			// The names sort in the same order as the index, so the twelfth row is
			// the twelfth listed row and the ordinal cap is observable.
			ID:   fmt.Sprintf("3333333%d-3333-4333-8333-333333333333", index%10),
			Name: fmt.Sprintf("Lens %02d", index),
			Mode: config.LensModeAll,
			Sort: config.LensSort{Field: config.LensSortDefault},
		})
	}
	// Two records must not share a UUID, so the cycled digit above is corrected
	// for the two rows past ten.
	lenses[10].ID = "44444444-4444-4444-8444-444444444444"
	lenses[11].ID = "55555555-5555-4555-8555-555555555555"
	// The population is installed in the machine as well as in the picker, so the
	// digit below dispatches a real apply rather than a resolve failure.
	setMachineLenses(t, model, lenses)
	if err := model.lensPicker.Refresh(model.namedLensPickerSnapshot()); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	want := []string{"  ", "  ", "1 ", "2 ", "3 ", "4 ", "5 ", "6 ", "7 ", "8 ", "9 ", "  ", "  ", "  "}
	if got := lensPickerOrdinalSlots(t, model); !slices.Equal(got, want) {
		t.Fatalf("number columns = %q, want %q", got, want)
	}
	if got := len(model.lensPicker.Ordinals); got != lens.LensPickerMaxOrdinal {
		t.Errorf("ordinal assignment holds %d lenses, want the %d cap", got, lens.LensPickerMaxOrdinal)
	}
	// A digit past the cap is out of range even though the row is on screen, and
	// says so in the numbering's own terms.
	pressLens(t, model, "9")
	if model.lensPicker != nil {
		t.Fatal("9 did not apply the ninth listed lens")
	}
	if got := model.crossOutliner.lens.active.name; got != "Lens 08" {
		t.Errorf("9 applied %q, want the ninth listed lens", got)
	}
}

func TestLensPickerActiveDotDoesNotShiftTheNumberColumn(t *testing.T) {
	model := lensPickerApp(t)
	stale := lensNamed(t, model, "Stale")

	unmarked := lensPickerOrdinalSlots(t, model)
	for _, modified := range []bool{false, true} {
		if err := model.lensPicker.Refresh(lens.NamedLensPickerSnapshot{
			Lenses:   machineLenses(t, model),
			ActiveID: stale.ID,
			Modified: modified,
		}); err != nil {
			t.Fatalf("refresh: %v", err)
		}
		if got := lensPickerOrdinalSlots(t, model); !slices.Equal(got, unmarked) {
			t.Fatalf("modified=%v shifted the number columns to %q, want %q",
				modified, got, unmarked)
		}
		marked := lensPickerItemNamed(t, model, "Stale")
		plain := ansi.Strip(marked.PrimaryMarker)
		wantDot := "2 ●"
		if modified {
			wantDot = "2 ●*"
		}
		// The dot lives strictly behind the digit, so its two spellings differ only
		// after the number column has already been drawn.
		if !strings.HasPrefix(plain, wantDot) {
			t.Fatalf("marker = %q, want it to lead with %q", plain, wantDot)
		}
	}

	// The narrowest branch of contentWidth is 20, not the 42 a comfortable
	// terminal reaches: the ordinal plus either dot must still fit there.
	const narrowWidth = 30
	if got := max(lens.NamedLensPickerInnerWidth(narrowWidth)-4, 12); got != 20 {
		t.Fatalf("contentWidth at %d columns = %d, want the narrow branch 20", narrowWidth, got)
	}
	for _, width := range []int{narrowWidth, chrome.MinTerminalWidth, 80} {
		rendered := ansi.Strip(model.lensPicker.Render(width))
		assertNoLensPickerOverflow(t, rendered, width)
		if !strings.Contains(rendered, "2 ●* Stale") {
			t.Errorf("the numbered active row is not intact at width %d:\n%s", width, rendered)
		}
	}
}

func TestLensPickerDigitApplyFailsExactlyAsEnterDoes(t *testing.T) {
	// The digit is not a second apply path: it parks the cursor and reuses the
	// tail of Enter, so a refusal must be the same refusal, on the same row, with
	// the same records left behind.
	refuse := func(t *testing.T, keys ...string) (*AppModel, string, string) {
		t.Helper()
		model := lensPickerApp(t)
		waiting := lensNamed(t, model, "Waiting")
		// The picker still lists the row; the machine no longer holds it, which is
		// the one refusal the read-only apply can raise.
		survivors := make(config.NamedLensList, 0, 2)
		for _, saved := range machineLenses(t, model) {
			if saved.ID != waiting.ID {
				survivors = append(survivors, saved)
			}
		}
		setMachineLenses(t, model, survivors)
		pressLens(t, model, keys...)
		if model.lensPicker == nil {
			t.Fatal("a refused apply closed the picker")
		}
		return model, waiting.ID, model.lensPicker.Notice()
	}

	byDigit, _, digitNotice := refuse(t, "3")
	if !strings.Contains(digitNotice, "no longer available") {
		t.Fatalf("digit refusal notice = %q", digitNotice)
	}
	// The cursor is left on the row the digit named, so the corrected retry is
	// one Enter away exactly as a refused Enter would have left it.
	if got := lensPickerCursorTitle(t, byDigit); got != "Waiting" {
		t.Errorf("cursor = %q, want the row the digit raised the reason from", got)
	}
	if byDigit.crossOutliner.hasActiveLens() {
		t.Error("a refused apply installed a lens")
	}
	if got := len(machineLenses(t, byDigit)); got != len(config.StarterNamedLenses())-1 {
		t.Errorf("a refused apply wrote %d lenses", got)
	}

	byEnter, waitingID, _ := refuse(t)
	if !byEnter.lensPicker.SelectLens(waitingID) {
		t.Fatal("the picker no longer lists the row the digit named")
	}
	pressLens(t, byEnter, "enter")
	if got := byEnter.lensPicker.Notice(); got != digitNotice {
		t.Errorf("Enter refusal = %q, digit refusal = %q; the two paths disagree",
			got, digitNotice)
	}
}

func TestLensPickerGuidanceAdvertisesTheOrdinalsOnlyWhenLensesAreListed(t *testing.T) {
	model := lensPickerApp(t)
	footerOf := func() string {
		t.Helper()
		return ansi.Strip(chrome.RenderGuidanceText(model.lensPicker.GuidanceSpec(), 200))
	}

	got := footerOf()
	if !strings.Contains(got, "1-9") {
		t.Errorf("browse guidance = %q, want the 1-9 hint", got)
	}
	if !strings.Contains(got, "apply by number") {
		t.Errorf("browse guidance = %q, want the ordinal label", got)
	}

	// While typing, digits are characters, so the footer must not promise they
	// apply anything.
	pressLens(t, model, "/")
	if got := footerOf(); strings.Contains(got, "1-9") {
		t.Errorf("search guidance = %q, still advertises the ordinals", got)
	}

	// A query that matches nothing leaves nothing to number.
	typeLens(t, model, "zzz")
	pressLens(t, model, "enter")
	if model.lensPicker.Listed != 0 {
		t.Fatalf("the no-match query still lists %d lenses", model.lensPicker.Listed)
	}
	if got := footerOf(); strings.Contains(got, "1-9") {
		t.Errorf("no-match guidance = %q, advertises numbers no row carries", got)
	}
	// And the digit refuses in the empty population's own words.
	pressLens(t, model, "1")
	if got := model.lensPicker.Notice(); got != lens.LensPickerOrdinalEmptyRefusal {
		t.Errorf("empty-population refusal = %q, want %q", got, lens.LensPickerOrdinalEmptyRefusal)
	}
}

// --- create -----------------------------------------------------------------
