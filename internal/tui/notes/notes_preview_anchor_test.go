package notes

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/route"
	"github.com/sglavoie/docket/internal/tui/theme"
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
	view.SearchApplied = query
	view.searchBeforeEdit = query
	view.searchInput.SetValue(query)
	view.RederiveVisibleRows()
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
	start, _, _, _ := listsurface.ListPreviewScrollWindow(len(view.preview.lines), max(marker-1, 0), anchorPaneHeight)
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
		if len(view.VisibleRows) != 1 {
			t.Fatalf("fuzzy body query matched %d rows, want one", len(view.VisibleRows))
		}
		if !view.notesPreviewBodyMatched(&view.VisibleRows[0]) {
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
		if view.notesPreviewBodyMatched(&view.VisibleRows[0]) {
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
		theme.WarningIconStyle.Render("alpha"),
		theme.MutedStyle.Render("beta"),
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
	view := NewNotesViewModelWithService(t.TempDir(), route.AllNotesScope(), service, nil, project)
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

	reanchor(t, "note identity", func() { view.Cursor = 1 })
	reanchor(t, "accepted query", func() { applyNotesSearch(view, "KESTREL") })
	reanchor(t, "live draft", func() {
		view.searchEditing = true
		view.searchInput.SetValue("kestre")
		view.RederiveVisibleRows()
	})
	reanchor(t, "raw revision", func() { view.CurrentRow().Note.RawRevision = "rev-reloaded" })
	// Dropping the revision moves the key onto the body digest, and changing the
	// body then moves the digest: an in-memory edit that never reached disk still
	// re-renders and re-anchors.
	reanchor(t, "revision absent", func() { view.CurrentRow().Note.RawRevision = "" })
	reanchor(t, "body digest", func() {
		row := view.CurrentRow()
		row.Note.Body = "- prepended lead line\n" + row.Note.Body
	})
	reanchor(t, "scope", func() { view.SetScope(route.GlobalNotesScope()) })
	if view.CurrentRow().Note.Filename != "global.md" {
		t.Fatalf("scope change left the cursor on %q", view.CurrentRow().Note.Filename)
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
	if view.SearchApplied != "" {
		t.Fatalf("Ctrl+F accepted the draft as %q", view.SearchApplied)
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
	if view.SearchApplied != "kestrel" {
		t.Fatalf("Enter did not accept the draft: %q", view.SearchApplied)
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
