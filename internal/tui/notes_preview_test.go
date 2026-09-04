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
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/route"
)

// notesPreviewFixture mounts a Notes manager over one Global owner plus one
// project owner, so the list column always has an owner label wide enough to
// compete with the title for the pane's columns.
func notesPreviewFixture(t *testing.T, notes ...*models.Note) *NotesViewModel {
	t.Helper()
	project := &models.Project{
		ID:   "atlas",
		Name: "Atlas Rewrite With A Deliberately Long Owner Label",
		Path: t.TempDir(),
	}
	service := &combinedNotesService{byLocation: map[app.NoteLocation][]*models.Note{
		app.ProjectNoteLocation(project.ID): notes,
	}}
	view := NewNotesViewModelWithService(t.TempDir(), route.AllNotesScope(), service, nil, project)
	mountNotesManager(t, view)
	return view
}

func notesScreen(t *testing.T, view *NotesViewModel) string {
	t.Helper()
	content, auditIssues := withRenderAudit(func() string { return view.View().Content })
	plain, issues := normalizeScreen(content, view.Width(), view.Height())
	issues = append(issues, auditIssues...)
	if len(issues) != 0 {
		t.Fatalf("layout issues at %dx%d = %#v\n%s", view.Width(), view.Height(), issues, content)
	}
	return plain
}

// notesPreviewPaneDrawn reports whether the wide layout is on screen. The pane
// joiner is the only thing on this surface that draws a vertical rule, so its
// presence is the honest signal and no width arithmetic is repeated here.
func notesPreviewPaneDrawn(screen string) bool {
	return strings.Contains(screen, " │ ")
}

func longNoteBody(lines int) string {
	items := make([]string, 0, lines)
	for i := range lines {
		items = append(items, fmt.Sprintf("- entry %02d", i))
	}
	return strings.Join(items, "\n")
}

func TestNotesPreviewAppearsAtSeventyTwoColumnsAndSurvivesResizeCycles(t *testing.T) {
	view := notesPreviewFixture(t, &models.Note{
		Title:       "Atlas migration notes",
		Filename:    "atlas.md",
		Body:        "# Heading\n\nSome **body** copy that is long enough to wrap in a narrow pane.",
		ModifiedAt:  time.Unix(1_700_000_000, 0),
		RawRevision: "rev-1",
	})

	for _, width := range []int{40, 60, 71} {
		view.SetSize(width, 20)
		if screen := notesScreen(t, view); notesPreviewPaneDrawn(screen) {
			t.Fatalf("width %d drew the preview pane below the shared threshold:\n%s", width, screen)
		}
	}
	for _, width := range []int{72, 90, 140} {
		view.SetSize(width, 20)
		screen := notesScreen(t, view)
		if !notesPreviewPaneDrawn(screen) {
			t.Fatalf("width %d did not draw the preview pane:\n%s", width, screen)
		}
		if !strings.Contains(screen, "Atlas migration notes") {
			t.Fatalf("width %d preview omitted the note title:\n%s", width, screen)
		}
	}

	// Repeated transitions must not leave the surface in either state's geometry.
	for range 3 {
		view.SetSize(120, 20)
		if !notesPreviewPaneDrawn(notesScreen(t, view)) {
			t.Fatal("wide render after a narrow one lost the preview pane")
		}
		view.SetSize(50, 20)
		if notesPreviewPaneDrawn(notesScreen(t, view)) {
			t.Fatal("narrow render after a wide one kept the preview pane")
		}
	}
}

func TestNotesPreviewUsesSharedGeometryAndChargesTheListPaneWidth(t *testing.T) {
	view := notesPreviewFixture(t, &models.Note{
		Title:       "A note title long enough to be truncated inside the list pane column",
		Filename:    "wide.md",
		Body:        "needle appears here",
		ModifiedAt:  time.Unix(1_700_000_000, 0),
		RawRevision: "rev-1",
	})
	view.SetSize(100, 20)

	listWidth, previewWidth := view.notesPreviewGeometry(100)
	sharedList, sharedPreview := listsurface.ListSurfacePreviewGeometry(listsurface.ListSurfaceRenderConfig{Width: 100, Height: 20})
	if listWidth != sharedList || previewWidth != sharedPreview {
		t.Fatalf(
			"Notes geometry = (%d, %d), shared list-surface geometry = (%d, %d)",
			listWidth, previewWidth, sharedList, sharedPreview,
		)
	}
	if listWidth != 55 || previewWidth != 42 {
		t.Fatalf("55/45 split at 100 columns = (%d, %d), want (55, 42)", listWidth, previewWidth)
	}

	// The body-match indicator is a list-pane column, so it must be charged
	// against the pane rather than the terminal: it survives beside the owner and
	// the truncated title instead of being pushed past the pane's right edge.
	view.searchApplied = "needle"
	view.rederiveVisibleRows()
	if len(view.visibleRows) != 1 {
		t.Fatalf("body search rows = %d, want one", len(view.visibleRows))
	}
	pane := ansi.Strip(view.renderListPane(listWidth, 5))
	for _, line := range strings.Split(strings.TrimRight(pane, "\n"), "\n") {
		if got := ansi.StringWidth(line); got > listWidth {
			t.Fatalf("list pane line width = %d, want at most %d: %q", got, listWidth, line)
		}
	}
	if !strings.Contains(pane, "· body") {
		t.Fatalf("list pane dropped the body-match indicator at its pane width:\n%s", pane)
	}
	if !strings.Contains(pane, "Atlas Rewrite") {
		t.Fatalf("list pane dropped the owner label at its pane width:\n%s", pane)
	}
}

func TestNotesPreviewRendersMarkdownEmptyAndParseErrorStates(t *testing.T) {
	modified := time.Unix(1_700_000_000, 0)
	view := notesPreviewFixture(t,
		&models.Note{
			Title:    "Rendered note",
			Filename: "rendered.md",
			Body: "# Heading\n\nA paragraph with *emphasis* and a very long sentence that has to " +
				"wrap more than once inside a narrow preview pane column.\n\n" +
				"- first item\n- second item\n\n" +
				"| column | other |\n| --- | --- |\n| a value | another value |\n\n" +
				"supercalifragilisticexpialidociousandthensomemoreletters",
			ModifiedAt:  modified,
			RawRevision: "rev-rendered",
		},
		&models.Note{
			Title:       "Empty note",
			Filename:    "empty.md",
			Body:        "   \n\n  ",
			ModifiedAt:  modified.Add(-time.Hour),
			RawRevision: "rev-empty",
		},
		&models.Note{
			Filename:    "broken.md",
			ParseError:  "yaml: line 2: mapping values are not allowed in this context",
			ModifiedAt:  modified.Add(-2 * time.Hour),
			RawRevision: "rev-broken",
		},
	)
	view.SetSize(100, 24)

	// Newest first, so the rendered note is the landing row.
	if got := view.visibleRows[0].Note.Filename; got != "rendered.md" {
		t.Fatalf("landing row = %q, want rendered.md", got)
	}

	header := notesPreviewHeader(&view.visibleRows[0])
	if header.Title != "Rendered note" || header.Summary != view.visibleRows[0].OwnerLabel {
		t.Fatalf("preview header = %#v", header)
	}
	if len(header.Fields) != 1 || header.Fields[0].Label != "Modified" ||
		!strings.HasPrefix(header.Fields[0].Value, modified.Format("2006-01-02 15:04")) {
		t.Fatalf("preview modification field = %#v", header.Fields)
	}

	const paneWidth = 42
	body := view.renderNotesPreviewBody(paneWidth, 60)
	for _, line := range body {
		if got := ansi.StringWidth(line); got > paneWidth {
			t.Fatalf("preview body line width = %d, want at most %d: %q", got, paneWidth, ansi.Strip(line))
		}
	}
	rendered := ansi.Strip(strings.Join(body, "\n"))
	for _, want := range []string{"Heading", "emphasis", "first item", "second item", "column", "another value"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("preview body omitted %q:\n%s", want, rendered)
		}
	}

	view.cursor = 1
	empty := ansi.Strip(strings.Join(view.renderNotesPreviewBody(paneWidth, 20), "\n"))
	if empty != "No body." {
		t.Fatalf("empty-body preview = %q, want the explicit empty state", empty)
	}

	view.cursor = 2
	broken := &view.visibleRows[2]
	if got := notesPreviewHeader(broken).Title; got != "broken.md" {
		t.Fatalf("parse-error header title = %q, want the filename", got)
	}
	if fields := notesPreviewHeader(broken).Fields; len(fields) != 0 {
		t.Fatalf("parse-error header claimed unreliable metadata: %#v", fields)
	}
	warning := ansi.Strip(strings.Join(view.renderNotesPreviewBody(paneWidth, 20), "\n"))
	if !strings.HasPrefix(warning, "! Note could not be parsed.") {
		t.Fatalf("parse-error preview = %q, want the warning state", warning)
	}
	if !strings.Contains(warning, "mapping values") {
		t.Fatalf("parse-error preview dropped the reason:\n%s", warning)
	}
	if view.preview.offset != 0 || len(view.preview.lines) != 0 {
		t.Fatalf("parse-error preview kept renderable state: %#v", view.preview)
	}
}

func TestNotesPreviewPagesWithCtrlFAndCtrlBAndClampsAtTheTail(t *testing.T) {
	view := notesPreviewFixture(t, &models.Note{
		Title:       "Long note",
		Filename:    "long.md",
		Body:        longNoteBody(40),
		ModifiedAt:  time.Unix(1_700_000_000, 0),
		RawRevision: "rev-long",
	})
	view.SetSize(100, 24)

	const (
		paneWidth  = 42
		paneHeight = 10
	)
	first := view.renderNotesPreviewBody(paneWidth, paneHeight)
	total := len(view.preview.lines)
	if total <= paneHeight {
		t.Fatalf("rendered body lines = %d, want more than one page", total)
	}
	if view.preview.pageSize != paneHeight-2 {
		t.Fatalf("render-derived page size = %d, want %d", view.preview.pageSize, paneHeight-2)
	}
	if strings.Contains(ansi.Strip(strings.Join(first, "\n")), "↑") {
		t.Fatalf("first page claimed rows above it:\n%s", ansi.Strip(strings.Join(first, "\n")))
	}
	if want := fmt.Sprintf("↓ %d more", total-len(first)+1); !strings.Contains(
		ansi.Strip(strings.Join(first, "\n")), want,
	) {
		t.Fatalf("first page lower continuation != %q:\n%s", want, ansi.Strip(strings.Join(first, "\n")))
	}

	view.pageNotesPreview(1)
	second := ansi.Strip(strings.Join(view.renderNotesPreviewBody(paneWidth, paneHeight), "\n"))
	if view.preview.offset != paneHeight-2 {
		t.Fatalf("offset after Ctrl+F = %d, want %d", view.preview.offset, paneHeight-2)
	}
	if want := fmt.Sprintf("↑ %d more", paneHeight-2); !strings.Contains(second, want) {
		t.Fatalf("second page upper continuation != %q:\n%s", want, second)
	}

	view.pageNotesPreview(-1)
	view.renderNotesPreviewBody(paneWidth, paneHeight)
	if view.preview.offset != 0 {
		t.Fatalf("offset after Ctrl+B = %d, want the top", view.preview.offset)
	}
	view.pageNotesPreview(-1)
	view.renderNotesPreviewBody(paneWidth, paneHeight)
	if view.preview.offset != 0 {
		t.Fatalf("Ctrl+B past the top = %d, want a clamp at zero", view.preview.offset)
	}

	// The render owns the tail clamp, so paging past the end settles on the last
	// full window rather than scrolling into blank rows.
	for range 20 {
		view.pageNotesPreview(1)
	}
	tail := ansi.Strip(strings.Join(view.renderNotesPreviewBody(paneWidth, paneHeight), "\n"))
	if strings.Contains(tail, "↓") {
		t.Fatalf("tail page still claimed rows below it:\n%s", tail)
	}
	if !strings.Contains(tail, "entry 39") {
		t.Fatalf("tail page did not reach the last body line:\n%s", tail)
	}
	if view.preview.offset != total-(paneHeight-1) {
		t.Fatalf("clamped offset = %d, want %d", view.preview.offset, total-(paneHeight-1))
	}

	// A short body never leaves the top and never draws a continuation.
	short := view.renderNotesPreviewBody(paneWidth, 200)
	if joined := ansi.Strip(strings.Join(short, "\n")); strings.Contains(joined, "↑") ||
		strings.Contains(joined, "↓") {
		t.Fatalf("body shorter than the pane drew continuations:\n%s", joined)
	}
}

func TestNotesPreviewOffsetResetsOnCursorMoveRevisionChangeAndResize(t *testing.T) {
	first := &models.Note{
		Title: "Newest", Filename: "newest.md", Body: longNoteBody(40),
		ModifiedAt: time.Unix(2_000, 0), RawRevision: "rev-newest",
	}
	second := &models.Note{
		Title: "Older", Filename: "older.md", Body: longNoteBody(40),
		ModifiedAt: time.Unix(1_000, 0), RawRevision: "rev-older",
	}
	view := notesPreviewFixture(t, first, second)
	view.SetSize(100, 24)

	const (
		paneWidth  = 42
		paneHeight = 10
	)
	page := func() { view.renderNotesPreviewBody(paneWidth, paneHeight) }

	page()
	view.pageNotesPreview(1)
	page()
	if view.preview.offset == 0 {
		t.Fatal("Ctrl+F did not move the offset")
	}

	view.cursor = 1
	page()
	if view.preview.offset != 0 || view.preview.key.identity.Filename != "older.md" {
		t.Fatalf("cursor move left a stale preview: %#v", view.preview.key)
	}

	view.pageNotesPreview(1)
	page()
	if view.preview.offset == 0 {
		t.Fatal("Ctrl+F did not move the offset on the second note")
	}
	// The same note reloaded with different bytes must re-render rather than keep
	// an offset into lines that no longer exist.
	second.Body = longNoteBody(60)
	second.RawRevision = "rev-older-2"
	page()
	if view.preview.offset != 0 || view.preview.key.revision != "rev-older-2" {
		t.Fatalf("revision change left a stale preview: %#v", view.preview.key)
	}

	view.pageNotesPreview(1)
	page()
	if view.preview.offset == 0 {
		t.Fatal("Ctrl+F did not move the offset after the reload")
	}
	view.SetSize(120, 24)
	if view.preview.ready || view.preview.offset != 0 {
		t.Fatalf("resize left a stale preview: %#v", view.preview)
	}

	// Narrow terminals own no preview, so the paging keys must refuse rather than
	// move an offset nothing will render.
	view.SetSize(60, 24)
	view.pageNotesPreview(1)
	if view.preview.offset != 0 {
		t.Fatalf("Ctrl+B/F moved the offset with no preview on screen: %d", view.preview.offset)
	}
	if view.notesPreviewVisible() {
		t.Fatal("narrow terminal reported a visible preview")
	}
}

func TestNotesPreviewPagingKeysReachTheModelAndAreDocumented(t *testing.T) {
	view := notesPreviewFixture(t, &models.Note{
		Title: "Long note", Filename: "long.md", Body: longNoteBody(40),
		ModifiedAt: time.Unix(1_700_000_000, 0), RawRevision: "rev-long",
	})
	view.SetSize(100, 24)
	notesScreen(t, view)

	_, _ = view.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	if view.preview.offset == 0 {
		t.Fatal("Ctrl+F did not page the preview through Update")
	}
	_, _ = view.Update(tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl})
	if view.preview.offset != 0 {
		t.Fatalf("Ctrl+B did not page back through Update: %d", view.preview.offset)
	}

	var documented bool
	for _, item := range view.GetHelpItems() {
		if item.Key == "Ctrl+F/B" {
			documented = true
		}
	}
	if !documented {
		t.Fatalf("Notes help omitted the preview paging row: %#v", view.GetHelpItems())
	}

	// The gate follows the pane, so a narrow terminal documents neither press.
	view.SetSize(60, 24)
	for _, item := range view.GetHelpItems() {
		if item.Key == "Ctrl+F/B" {
			t.Fatal("narrow Notes help advertised a preview that is not on screen")
		}
	}
}

func TestNotesPreviewKeepsTheCursorRowAndFooterAtMinimumHeights(t *testing.T) {
	notes := make([]*models.Note, 0, 12)
	for i := range 12 {
		notes = append(notes, &models.Note{
			Title:       fmt.Sprintf("Note %02d with a deliberately long title for column pressure", i),
			Filename:    fmt.Sprintf("note-%02d.md", i),
			Body:        longNoteBody(40),
			ModifiedAt:  time.Unix(int64(1_000+i), 0),
			RawRevision: fmt.Sprintf("rev-%02d", i),
		})
	}
	view := notesPreviewFixture(t, notes...)

	// notesChromeFloor is the shortest terminal the surface's own chrome fits in:
	// the title, the scope count, the search line, the divider, the guidance row,
	// and the status row are all unconditional here, and the content block cannot
	// shrink below one row. Below it the view overflows with or without a preview,
	// which is a pre-existing property of the Notes chrome rather than anything
	// the pane introduces.
	const notesChromeFloor = 7
	for _, width := range []int{71, 72, 80, 120} {
		for height := notesChromeFloor; height <= 12; height++ {
			view.SetSize(width, height)
			// A search line, a status line, and an accepted query all take rows from
			// the same budget the list and preview draw into.
			view.searchApplied = "note"
			view.rederiveVisibleRows()
			view.setErrorStatus(strings.Repeat("a visible and actionable error summary ", 3))
			view.cursor = min(7, len(view.visibleRows)-1)

			screen := notesScreen(t, view)
			lines := strings.Split(strings.TrimRight(screen, "\n"), "\n")
			if len(lines) > height {
				t.Fatalf("%dx%d rendered %d rows:\n%s", width, height, len(lines), screen)
			}
			// The list rows land newest-first, so the cursor's title is the one this
			// index names; it must survive however few content rows are left.
			cursorTitle := fmt.Sprintf("Note %02d ", 11-view.cursor)
			if !strings.Contains(screen, cursorTitle) {
				t.Fatalf("%dx%d clipped the cursor row %q:\n%s", width, height, cursorTitle, screen)
			}
			if !strings.Contains(screen, "clear search") {
				t.Fatalf("%dx%d clipped the footer guidance:\n%s", width, height, screen)
			}
		}
	}
}
