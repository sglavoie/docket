package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
)

func TestNotesPreviewPagingRefusesWhereNoPaneExists(t *testing.T) {
	parseError := &models.Note{
		Filename: "broken.md", ParseError: "yaml: line 2: bad mapping",
		ModifiedAt: time.Unix(1_000, 0), RawRevision: "rev-broken",
	}
	view := notesPreviewFixture(t, &models.Note{
		Title: "Long note", Filename: "long.md", Body: longNoteBody(60),
		ModifiedAt: time.Unix(2_000, 0), RawRevision: "rev-long",
	}, parseError)
	view.SetSize(100, 24)
	notesScreen(t, view)

	// Narrow layout: the pane does not exist, so neither press may move an offset.
	view.SetSize(60, 24)
	_, _ = view.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	if view.preview.offset != 0 {
		t.Fatalf("narrow layout paged to %d", view.preview.offset)
	}
	_, _ = view.Update(syntheticKey('/'))
	_, _ = view.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	if view.preview.offset != 0 {
		t.Fatalf("narrow search editing paged to %d", view.preview.offset)
	}
	if view.searchInput.Value() != "" {
		t.Fatalf("a refused paging key still reached the text input: %q", view.searchInput.Value())
	}
	_, _ = view.Update(syntheticSpecialKey(tea.KeyEsc))

	// No current row: a query matching nothing leaves the pane with no note.
	view.SetSize(100, 24)
	applyNotesSearch(view, "zzzznotanote")
	if len(view.visibleRows) != 0 {
		t.Fatalf("the impossible query matched %d rows", len(view.visibleRows))
	}
	if view.notesPreviewVisible() {
		t.Fatal("an empty result set reported a visible preview")
	}
	_, _ = view.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	if view.preview.offset != 0 {
		t.Fatalf("paging with no current row moved the offset to %d", view.preview.offset)
	}
	applyNotesSearch(view, "")

	// A parse-error row previews its warning and never scrolls off it.
	view.cursor = 1
	if view.visibleRows[view.cursor].Note.Filename != "broken.md" {
		t.Fatalf("cursor row = %q, want broken.md", view.visibleRows[view.cursor].Note.Filename)
	}
	notesScreen(t, view)
	_, _ = view.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	notesScreen(t, view)
	if view.preview.offset != 0 || len(view.preview.lines) != 0 {
		t.Fatalf("parse-error preview paged: %#v", view.preview)
	}

	// Minimum heights keep both the cursor row and the footer on screen while the
	// paging keys stay live.
	view.cursor = 0
	for height := 7; height <= 12; height++ {
		view.SetSize(100, height)
		notesScreen(t, view)
		_, _ = view.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
		screen := notesScreen(t, view)
		if !strings.Contains(screen, "Long note") {
			t.Fatalf("height %d clipped the cursor row:\n%s", height, screen)
		}
	}
}

// TestNotesPreviewPagingGuidanceRejectsParseErrors covers the state the pane's
// geometry admits but its body cannot honor: a note whose frontmatter failed to
// parse draws a fixed warning, so Ctrl+F/B would move nothing and neither the
// footer, `?` help, nor the paging state may pretend otherwise. The pageable
// predicate is what separates it from mere pane visibility, so the same fixture
// asserts a parseable neighbor keeps both controls.
func TestNotesPreviewPagingGuidanceRejectsParseErrors(t *testing.T) {
	view := notesPreviewFixture(t, &models.Note{
		Title: "Long note", Filename: "long.md", Body: longNoteBody(60),
		ModifiedAt: time.Unix(2_000, 0), RawRevision: "rev-long",
	}, &models.Note{
		Filename: "broken.md", ParseError: "yaml: line 2: bad mapping",
		ModifiedAt: time.Unix(1_000, 0), RawRevision: "rev-broken",
	})
	view.SetSize(100, 24)

	advertises := func(t *testing.T, state string) bool {
		t.Helper()
		spec, ok := view.ContextualGuidance()
		if !ok {
			t.Fatalf("Notes guidance is unavailable during %s", state)
		}
		return strings.Contains(guidanceEntryText(spec), "Ctrl+F/B preview")
	}
	helpAdvertises := func() bool {
		for _, item := range view.GetHelpItems() {
			if item.Key == "Ctrl+F/B" {
				return true
			}
		}
		return false
	}
	// searchEdit runs one assertion inside a live search draft, since the two
	// guidance specs are built by separate branches and only one of them is
	// reachable at a time.
	searchEdit := func(t *testing.T, state string, assert func(t *testing.T, state string)) {
		t.Helper()
		_, _ = view.Update(syntheticKey('/'))
		if !view.searchEditing {
			t.Fatalf("/ did not open the search draft for %s", state)
		}
		assert(t, state)
		_, _ = view.Update(syntheticSpecialKey(tea.KeyEsc))
	}

	// The parseable neighbor is the control: both routes advertise the keys.
	notesScreen(t, view)
	if !advertises(t, "browse on a parseable note") {
		t.Fatal("browse guidance omitted the paging hint on a pageable note")
	}
	if !helpAdvertises() {
		t.Fatal("Notes help omitted preview paging on a pageable note")
	}
	searchEdit(t, "search editing on a parseable note", func(t *testing.T, state string) {
		if !advertises(t, state) {
			t.Fatalf("%s omitted the paging hint", state)
		}
	})

	view.cursor = 1
	if view.visibleRows[view.cursor].Note.Filename != "broken.md" {
		t.Fatalf("cursor row = %q, want broken.md", view.visibleRows[view.cursor].Note.Filename)
	}
	notesScreen(t, view)
	if view.notesPreviewPageable() {
		t.Fatal("a parse-error row reported a pageable preview")
	}
	if !view.notesPreviewVisible() {
		t.Fatal("a parse-error row draws a pane, so visibility must stay true and only pageability change")
	}
	if advertises(t, "browse on a parse-error note") {
		t.Fatal("browse guidance advertised paging a warning that cannot scroll")
	}
	if helpAdvertises() {
		t.Fatal("Notes help advertised paging a warning that cannot scroll")
	}
	searchEdit(t, "search editing on a parse-error note", func(t *testing.T, state string) {
		if advertises(t, state) {
			t.Fatalf("%s advertised paging a warning that cannot scroll", state)
		}
		_, _ = view.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
		if view.preview.offset != 0 || view.preview.pageSize != 0 {
			t.Fatalf("Ctrl+F during %s mutated paging state: %#v", state, view.preview)
		}
		if view.searchInput.Value() != "" {
			t.Fatalf("a refused paging key reached the text input during %s: %q", state, view.searchInput.Value())
		}
	})

	// Browse presses are refused the same way, and the render leaves the warning
	// at the top with no page to spend.
	for _, code := range []rune{'f', 'b'} {
		_, _ = view.Update(tea.KeyPressMsg{Code: code, Mod: tea.ModCtrl})
		notesScreen(t, view)
		if view.preview.offset != 0 || view.preview.pageSize != 0 || len(view.preview.lines) != 0 {
			t.Fatalf("Ctrl+%c on a parse-error row mutated paging state: %#v", code, view.preview)
		}
	}

	// Returning to the parseable note restores both controls and a working page.
	view.cursor = 0
	notesScreen(t, view)
	if !advertises(t, "browse back on the parseable note") {
		t.Fatal("browse guidance did not restore the paging hint")
	}
	if !helpAdvertises() {
		t.Fatal("Notes help did not restore preview paging")
	}
	_, _ = view.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	notesScreen(t, view)
	if view.preview.offset == 0 {
		t.Fatal("Ctrl+F on the restored pageable note did not page")
	}
}

func TestNotesPreviewPagingIsAdvertisedOnlyWhereItIsActionable(t *testing.T) {
	view := notesPreviewFixture(t, &models.Note{
		Title: "Long note", Filename: "long.md", Body: longNoteBody(60),
		ModifiedAt: time.Unix(1_700_000_000, 0), RawRevision: "rev-long",
	})

	guidanceHas := func(t *testing.T, spec chrome.GuidanceSpec, ok bool) bool {
		t.Helper()
		if !ok {
			t.Fatal("Notes guidance is unavailable")
		}
		return strings.Contains(guidanceEntryText(spec), "Ctrl+F/B preview")
	}

	view.SetSize(100, 24)
	spec, ok := view.ContextualGuidance()
	if !guidanceHas(t, spec, ok) {
		t.Fatalf("wide browse guidance omitted the paging hint: %+v", spec.Hints)
	}
	for _, hint := range spec.Hints {
		if hint.Key == "Ctrl+F/B" && hint.Required {
			t.Fatal("the paging hint claimed a required slot")
		}
	}
	assertGuidanceContains(t, "notes browse with preview", spec, ok, "Enter open/edit", "/ search")

	_, _ = view.Update(syntheticKey('/'))
	spec, ok = view.ContextualGuidance()
	if !guidanceHas(t, spec, ok) {
		t.Fatalf("search-edit guidance omitted the paging hint: %+v", spec.Hints)
	}
	assertRequiredGuidance(t, "notes search with preview", spec, ok,
		"Enter apply search", "Esc cancel edit")

	// Narrow: no pane, so neither the footer nor `?` help may name the keys.
	view.SetSize(60, 24)
	spec, ok = view.ContextualGuidance()
	if guidanceHas(t, spec, ok) {
		t.Fatalf("narrow search-edit guidance advertised a pane that is not there: %+v", spec.Hints)
	}
	_, _ = view.Update(syntheticSpecialKey(tea.KeyEsc))
	spec, ok = view.ContextualGuidance()
	if guidanceHas(t, spec, ok) {
		t.Fatalf("narrow browse guidance advertised a pane that is not there: %+v", spec.Hints)
	}
	for _, item := range view.GetHelpItems() {
		if item.Key == "Ctrl+F/B" {
			t.Fatal("narrow Notes help advertised preview paging")
		}
	}

	// No previewable note: the pane's geometry fits, but there is no row to draw.
	view.SetSize(100, 24)
	applyNotesSearch(view, "zzzznotanote")
	spec, ok = view.ContextualGuidance()
	if guidanceHas(t, spec, ok) {
		t.Fatalf("empty-result guidance advertised preview paging: %+v", spec.Hints)
	}
	assertRequiredGuidance(t, "notes empty result", spec, ok, "Esc clear search")
	for _, item := range view.GetHelpItems() {
		if item.Key == "Ctrl+F/B" {
			t.Fatal("Notes help advertised preview paging with no previewable note")
		}
	}
}
