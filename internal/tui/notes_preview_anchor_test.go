package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

const (
	anchorPaneWidth  = 42
	anchorPaneHeight = 10
)

// markedNoteBody builds a body whose filler lines share no letter run with any
// marker a test searches for, so the rendered line a marker lands on is the only
// line the anchor can honestly choose.
func markedNoteBody(total int, markers map[int]string) string {
	lines := make([]string, 0, total)
	for i := range total {
		if marker, ok := markers[i]; ok {
			lines = append(lines, "- "+marker)
			continue
		}
		lines = append(lines, fmt.Sprintf("- padding row %02d", i))
	}
	return strings.Join(lines, "\n")
}

// renderedLineContaining locates a marker in the cached preview lines. The
// Markdown renderer owns wrapping and list decoration, so a test must ask where
// a marker actually landed rather than assume a source line maps to one row.
func renderedLineContaining(t *testing.T, view *NotesViewModel, needle string) int {
	t.Helper()
	for index, line := range view.preview.lines {
		if strings.Contains(ansi.Strip(line), needle) {
			return index
		}
	}
	t.Fatalf("rendered preview never drew %q:\n%s", needle, ansi.Strip(strings.Join(view.preview.lines, "\n")))
	return -1
}

// applyNotesSearch accepts a query the way Enter does, so the visible rows, the
// match map, and the relaxed-case flag are all the ones the surface would show.
func applyNotesSearch(view *NotesViewModel, query string) {
	view.searchApplied = query
	view.searchBeforeEdit = query
	view.searchInput.SetValue(query)
	view.rederiveVisibleRows()
}

func anchorPage(view *NotesViewModel) string {
	return ansi.Strip(strings.Join(
		view.renderNotesPreviewBody(anchorPaneWidth, anchorPaneHeight), "\n",
	))
}

// wantAnchorOffset is the offset a marker one line below the top of the pane
// should settle at. The render owns the tail clamp, so a match near the end of a
// note anchors at the last full window rather than past it; the clamp is asked
// of the shared window authority rather than recomputed here.
func wantAnchorOffset(t *testing.T, view *NotesViewModel, needle string) int {
	t.Helper()
	marker := renderedLineContaining(t, view, needle)
	start, _, _, _ := listPreviewScrollWindow(len(view.preview.lines), max(marker-1, 0), anchorPaneHeight)
	return start
}

func TestNotesPreviewAnchorsBodyMatchesWithOnePrecedingContextLine(t *testing.T) {
	for _, tc := range []struct {
		name string
		at   int
	}{
		{"top", 0},
		{"middle", 20},
		{"bottom", 39},
	} {
		t.Run(tc.name, func(t *testing.T) {
			view := notesPreviewFixture(t, &models.Note{
				Title:       "Long note",
				Filename:    "long.md",
				Body:        markedNoteBody(40, map[int]string{tc.at: "kestrel"}),
				ModifiedAt:  time.Unix(1_700_000_000, 0),
				RawRevision: "rev-long",
			})
			view.SetSize(100, 24)
			applyNotesSearch(view, "kestrel")

			page := anchorPage(view)
			if want := wantAnchorOffset(t, view, "kestrel"); view.preview.offset != want {
				t.Fatalf("anchor offset = %d, want %d", view.preview.offset, want)
			}
			if !strings.Contains(page, "kestrel") {
				t.Fatalf("anchored page did not show the match:\n%s", page)
			}
			// Every match but the first rendered line has a line above it, and the
			// anchor spends it on the note's own preceding row.
			if tc.at > 0 && !strings.Contains(page, fmt.Sprintf("padding row %02d", tc.at-1)) {
				t.Fatalf("anchored page dropped the preceding context line:\n%s", page)
			}
		})
	}
}

func TestNotesPreviewAnchorFollowsTheEffectiveQueryAndCaseMode(t *testing.T) {
	body := markedNoteBody(40, map[int]string{
		5:  "widget lowercase",
		10: "bravo term",
		25: "Widget uppercase",
		30: "alpha term",
		35: "kestrel bird",
	})
	newView := func(t *testing.T) *NotesViewModel {
		t.Helper()
		view := notesPreviewFixture(t, &models.Note{
			Title:       "Long note",
			Filename:    "long.md",
			Body:        body,
			ModifiedAt:  time.Unix(1_700_000_000, 0),
			RawRevision: "rev-long",
		})
		view.SetSize(100, 24)
		return view
	}

	t.Run("multi-term anchors on the earliest term", func(t *testing.T) {
		view := newView(t)
		applyNotesSearch(view, "alpha bravo")
		anchorPage(view)
		if want := wantAnchorOffset(t, view, "bravo term"); view.preview.offset != want {
			t.Fatalf("multi-term anchor = %d, want the earlier term's line %d", view.preview.offset, want)
		}
	})

	t.Run("fuzzy body match anchors on the line it is locatable in", func(t *testing.T) {
		view := newView(t)
		applyNotesSearch(view, "kstrl")
		if len(view.visibleRows) != 1 {
			t.Fatalf("fuzzy body query matched %d rows, want one", len(view.visibleRows))
		}
		if !view.notesPreviewBodyMatched(&view.visibleRows[0]) {
			t.Fatal("fuzzy body query did not report a body match")
		}
		anchorPage(view)
		if want := wantAnchorOffset(t, view, "kestrel bird"); view.preview.offset != want {
			t.Fatalf("fuzzy anchor = %d, want %d", view.preview.offset, want)
		}
	})

	t.Run("smart case narrows the anchor to the cased occurrence", func(t *testing.T) {
		view := newView(t)
		applyNotesSearch(view, "Widget")
		if view.searchCaseRelaxed {
			t.Fatal("a query with a case-sensitive reading fell back to the relaxed retry")
		}
		anchorPage(view)
		if want := wantAnchorOffset(t, view, "Widget uppercase"); view.preview.offset != want {
			t.Fatalf("case-sensitive anchor = %d, want the cased line %d", view.preview.offset, want)
		}
	})

	t.Run("relaxed fallback anchors with the case mode that returned the rows", func(t *testing.T) {
		view := newView(t)
		applyNotesSearch(view, "WIDGET")
		if !view.searchCaseRelaxed {
			t.Fatal("the all-caps query did not fall back to the case-relaxed retry")
		}
		if view.effectiveSearchCaseSensitive() {
			t.Fatal("the anchor kept a case mode the visible rows were not built with")
		}
		anchorPage(view)
		if want := wantAnchorOffset(t, view, "widget lowercase"); view.preview.offset != want {
			t.Fatalf("relaxed anchor = %d, want the first occurrence at %d", view.preview.offset, want)
		}
	})

	t.Run("title-only match and no query both open at the top", func(t *testing.T) {
		view := newView(t)
		applyNotesSearch(view, "Long")
		if view.notesPreviewBodyMatched(&view.visibleRows[0]) {
			t.Fatal("a title hit reported a body match")
		}
		anchorPage(view)
		if view.preview.offset != 0 {
			t.Fatalf("title-only anchor = %d, want the top", view.preview.offset)
		}

		applyNotesSearch(view, "")
		anchorPage(view)
		if view.preview.offset != 0 {
			t.Fatalf("no-query anchor = %d, want the top", view.preview.offset)
		}
	})
}

// TestNotesPreviewAnchorFallsBackToTheTopWhenTheRegionIsUnlocatable covers the
// deterministic escape hatch: search runs over the raw body, so a subsequence
// that only exists across a wrap point has no rendered line to open at.
func TestNotesPreviewAnchorFallsBackToTheTopWhenTheRegionIsUnlocatable(t *testing.T) {
	lines := []string{
		WarningIconStyle.Render("alpha"),
		MutedStyle.Render("beta"),
	}
	if got := notesPreviewAnchor(lines, "gamma", false); got != 0 {
		t.Fatalf("unlocatable region anchored at %d, want the top", got)
	}
	if got := notesPreviewAnchor(nil, "alpha", false); got != 0 {
		t.Fatalf("empty body anchored at %d, want the top", got)
	}
	if got := notesPreviewAnchor(lines, "   ", false); got != 0 {
		t.Fatalf("blank query anchored at %d, want the top", got)
	}
	// Styling must not hide the match: the anchor searches the stripped text.
	if got := notesPreviewAnchor(lines, "beta", false); got != 0 {
		t.Fatalf("styled second line anchored at %d, want one line of context above it", got)
	}
}

func TestNotesPreviewAnchorKeyResetsOnEveryAxisAndHoldsManualPagingOtherwise(t *testing.T) {
	body := markedNoteBody(60, map[int]string{40: "kestrel"})
	note := &models.Note{
		Title: "Newest", Filename: "newest.md", Body: body,
		ModifiedAt: time.Unix(2_000, 0), RawRevision: "rev-newest",
	}
	other := &models.Note{
		Title: "Older", Filename: "older.md", Body: body,
		ModifiedAt: time.Unix(1_000, 0), RawRevision: "rev-older",
	}
	global := &models.Note{
		Title: "Global", Filename: "global.md", Body: body,
		ModifiedAt: time.Unix(3_000, 0), RawRevision: "rev-global",
	}
	project := &models.Project{ID: "atlas", Name: "Atlas", Path: t.TempDir()}
	service := &combinedNotesService{byLocation: map[app.NoteLocation][]*models.Note{
		app.ProjectNoteLocation(project.ID): {note, other},
		app.GlobalNoteLocation():            {global},
	}}
	view := NewNotesViewModelWithService(t.TempDir(), AllNotesScope(), service, nil, project)
	mountNotesManager(t, view)
	view.SetSize(100, 24)
	applyNotesSearch(view, "kestrel")

	// The landing row is the newest note; every axis below is exercised from a
	// deliberately paged position, so a reset is visible as a return to the anchor.
	anchorPage(view)
	anchored := view.preview.offset
	if anchored == 0 {
		t.Fatal("a body match at line 40 anchored at the top")
	}

	// Manual paging survives any number of ordinary re-renders while the key holds.
	view.pageNotesPreview(1)
	anchorPage(view)
	paged := view.preview.offset
	if paged == anchored {
		t.Fatal("Ctrl+F did not move the offset off the anchor")
	}
	for range 5 {
		anchorPage(view)
	}
	if view.preview.offset != paged {
		t.Fatalf("ordinary re-renders moved a manual offset from %d to %d", paged, view.preview.offset)
	}

	// Each axis is exercised the same way: page off the anchor, move exactly one
	// key component, and require the next render to land back on the anchor the
	// new key computes. The paged position is asserted to differ from that anchor,
	// so "it reset" is never satisfied by an offset that never moved.
	reanchor := func(t *testing.T, axis string, mutate func()) {
		t.Helper()
		anchorPage(view)
		before := view.preview.key
		view.pageNotesPreview(1)
		anchorPage(view)
		paged := view.preview.offset
		mutate()
		anchorPage(view)
		if view.preview.key == before {
			t.Fatalf("%s: the cache key did not change", axis)
		}
		want := wantAnchorOffset(t, view, "kestrel")
		if paged == want {
			t.Fatalf("%s: paging never left the anchor, so a reset is unprovable", axis)
		}
		if view.preview.offset != want {
			t.Fatalf("%s: offset = %d after the key changed, want the recomputed anchor %d",
				axis, view.preview.offset, want)
		}
	}

	reanchor(t, "note identity", func() { view.cursor = 1 })
	reanchor(t, "accepted query", func() { applyNotesSearch(view, "KESTREL") })
	reanchor(t, "live draft", func() {
		view.searchEditing = true
		view.searchInput.SetValue("kestre")
		view.rederiveVisibleRows()
	})
	reanchor(t, "raw revision", func() { view.currentRow().Note.RawRevision = "rev-reloaded" })
	// Dropping the revision moves the key onto the body digest, and changing the
	// body then moves the digest: an in-memory edit that never reached disk still
	// re-renders and re-anchors.
	reanchor(t, "revision absent", func() { view.currentRow().Note.RawRevision = "" })
	reanchor(t, "body digest", func() {
		row := view.currentRow()
		row.Note.Body = "- prepended lead line\n" + row.Note.Body
	})
	reanchor(t, "scope", func() { view.SetScope(GlobalNotesScope()) })
	if view.currentRow().Note.Filename != "global.md" {
		t.Fatalf("scope change left the cursor on %q", view.currentRow().Note.Filename)
	}

	// Width is the one axis the key records from the render rather than the model,
	// so it is proven by rendering the same note into a different pane.
	anchorPage(view)
	view.pageNotesPreview(1)
	anchorPage(view)
	view.renderNotesPreviewBody(anchorPaneWidth-6, anchorPaneHeight)
	if view.preview.key.width != anchorPaneWidth-6 {
		t.Fatalf("preview key width = %d, want the pane it rendered into", view.preview.key.width)
	}
	narrowAnchor := view.preview.offset
	view.renderNotesPreviewBody(anchorPaneWidth-6, anchorPaneHeight)
	if view.preview.offset != narrowAnchor {
		t.Fatalf("re-render at the new width moved the anchor from %d to %d",
			narrowAnchor, view.preview.offset)
	}
	if want := wantAnchorOffset(t, view, "kestrel"); narrowAnchor != want {
		t.Fatalf("width change anchored at %d, want %d in the rewrapped lines", narrowAnchor, want)
	}
}

// TestNotesPreviewRevisionFallsBackToTheBodyDigest proves the in-memory fallback
// distinguishes bodies a note carries without a canonical file revision.
func TestNotesPreviewRevisionFallsBackToTheBodyDigest(t *testing.T) {
	withRevision := &models.Note{RawRevision: " rev-1 ", Body: "one"}
	if got := notesPreviewRevision(withRevision); got != "rev-1" {
		t.Fatalf("revision = %q, want the trimmed canonical revision", got)
	}
	first := notesPreviewRevision(&models.Note{Body: "one"})
	second := notesPreviewRevision(&models.Note{Body: "two"})
	if first == second {
		t.Fatalf("two bodies shared the fallback revision %q", first)
	}
	if again := notesPreviewRevision(&models.Note{Body: "one"}); again != first {
		t.Fatalf("the fallback revision is not stable: %q then %q", first, again)
	}
}

func TestNotesPreviewPagingDuringSearchEditingLeavesTheDraftAlone(t *testing.T) {
	view := notesPreviewFixture(t, &models.Note{
		Title: "Long note", Filename: "long.md",
		Body:       markedNoteBody(60, map[int]string{40: "kestrel"}),
		ModifiedAt: time.Unix(1_700_000_000, 0), RawRevision: "rev-long",
	})
	view.SetSize(100, 24)
	notesScreen(t, view)

	_, _ = view.Update(syntheticKey('/'))
	for _, r := range "kestrel" {
		_, _ = view.Update(syntheticKey(r))
	}
	if !view.searchEditing || view.searchInput.Value() != "kestrel" {
		t.Fatalf("draft = %q editing=%t", view.searchInput.Value(), view.searchEditing)
	}
	notesScreen(t, view)
	anchored := view.preview.offset
	if anchored == 0 {
		t.Fatal("the live draft did not anchor the preview on its body match")
	}

	_, _ = view.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	notesScreen(t, view)
	if view.preview.offset <= anchored {
		t.Fatalf("Ctrl+F during search editing = %d, want a page past %d", view.preview.offset, anchored)
	}
	if !view.searchEditing {
		t.Fatal("Ctrl+F accepted or canceled the search draft")
	}
	if view.searchApplied != "" {
		t.Fatalf("Ctrl+F accepted the draft as %q", view.searchApplied)
	}
	if view.searchInput.Value() != "kestrel" {
		t.Fatalf("Ctrl+F reached the text input: draft = %q", view.searchInput.Value())
	}

	forward := view.preview.offset
	_, _ = view.Update(tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl})
	notesScreen(t, view)
	if view.preview.offset >= forward {
		t.Fatalf("Ctrl+B during search editing = %d, want a page back from %d", view.preview.offset, forward)
	}
	if !view.searchEditing || view.searchInput.Value() != "kestrel" {
		t.Fatalf("Ctrl+B disturbed the draft: %q editing=%t", view.searchInput.Value(), view.searchEditing)
	}

	// Accepting the draft keeps the same key, so the reader's page survives Enter.
	_, _ = view.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	notesScreen(t, view)
	paged := view.preview.offset
	_, _ = view.Update(syntheticSpecialKey(tea.KeyEnter))
	notesScreen(t, view)
	if view.searchApplied != "kestrel" {
		t.Fatalf("Enter did not accept the draft: %q", view.searchApplied)
	}
	if view.preview.offset != paged {
		t.Fatalf("accepting an unchanged draft reset the page from %d to %d", paged, view.preview.offset)
	}

	// An accepted search pages the same way from ordinary browse.
	_, _ = view.Update(tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl})
	notesScreen(t, view)
	if view.preview.offset >= paged {
		t.Fatalf("Ctrl+B under an accepted search = %d, want a page back from %d", view.preview.offset, paged)
	}
}

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

	guidanceHas := func(t *testing.T, spec GuidanceSpec, ok bool) bool {
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
