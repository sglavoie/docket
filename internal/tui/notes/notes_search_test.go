package notes

import (
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/fuzzy"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/route"
	"github.com/sglavoie/docket/internal/tui/theme"
)

func TestNotesSearchBodyMatchingIsCaseInsensitiveAndRequiresEveryTerm(t *testing.T) {
	body := "The ALPHA decision is documented here.\nLater, beta shipped."
	if score := scoreNoteBodyTerms(body, "alpha BETA", fuzzy.Matcher{}); score <= 0 {
		t.Fatalf("case-insensitive multi-term body score = %d, want a match", score)
	}
	if score := scoreNoteBodyTerms(body, "alpha missing", fuzzy.Matcher{}); score != 0 {
		t.Fatalf("body score with a missing term = %d, want no match", score)
	}
	if score := scoreNoteBodyTerms("release roadmap", "rls rmap", fuzzy.Matcher{}); score <= 0 {
		t.Fatalf("dense fuzzy body fallback score = %d, want a match", score)
	}
	sensitive := fuzzy.NewMatcher(true)
	if score := scoreNoteBodyTerms(body, "alpha BETA", sensitive); score != 0 {
		t.Fatalf("case-sensitive body score on folded terms = %d, want no match", score)
	}
	if score := scoreNoteBodyTerms(body, "ALPHA beta", sensitive); score <= 0 {
		t.Fatalf("case-sensitive body score on exact-case terms = %d, want a match", score)
	}
}

func TestNotesSearchRankingPrefersTitlesAndUsesDeterministicTies(t *testing.T) {
	alpha := &models.Project{ID: "alpha", Name: "Alpha", Path: t.TempDir()}
	beta := &models.Project{ID: "beta", Name: "Beta", Path: t.TempDir()}
	when := time.Unix(10, 0)
	titleMatch := &models.Note{
		Title: "Needle plan", Filename: "title.md", ModifiedAt: time.Unix(1, 0),
	}
	newerBody := &models.Note{
		Title: "Quartz", Filename: "newer.md", Body: "needle", ModifiedAt: time.Unix(11, 0),
	}
	alphaA := &models.Note{
		Title: "Apple", Filename: "a.md", Body: "needle", ModifiedAt: when,
	}
	alphaB := &models.Note{
		Title: "Apple", Filename: "b.md", Body: "needle", ModifiedAt: when,
	}
	alphaZ := &models.Note{
		Title: "Zulu", Filename: "z.md", Body: "needle", ModifiedAt: when,
	}
	betaA := &models.Note{
		Title: "Apple", Filename: "beta.md", Body: "needle", ModifiedAt: when,
	}
	service := &combinedNotesService{byLocation: map[app.NoteLocation][]*models.Note{
		app.GlobalNoteLocation():          {titleMatch},
		app.ProjectNoteLocation(alpha.ID): {alphaZ, alphaB, alphaA},
		app.ProjectNoteLocation(beta.ID):  {betaA, newerBody},
	}}
	view := NewNotesViewModelWithService(
		t.TempDir(),
		route.AllNotesScope(),
		service,
		nil,
		alpha,
		beta,
	)
	mountNotesManager(t, view)

	view.SearchApplied = "needle"
	view.RederiveVisibleRows()

	var got []string
	for _, row := range view.VisibleRows {
		got = append(got, row.Note.Filename)
	}
	want := []string{"title.md", "newer.md", "a.md", "b.md", "z.md", "beta.md"}
	if !slices.Equal(got, want) {
		t.Fatalf("search ranking = %v, want %v", got, want)
	}
	if match := view.SearchMatches[titleMatchIdentity(titleMatch)]; len(match.TitlePositions) == 0 {
		t.Fatal("title match did not retain fuzzy highlight positions")
	}
}

func TestNotesSearchSmartCaseNarrowsThenFallsBackWithLabel(t *testing.T) {
	project := &models.Project{ID: "alpha", Name: "Alpha", Path: t.TempDir()}
	service := &combinedNotesService{byLocation: map[app.NoteLocation][]*models.Note{
		app.GlobalNoteLocation(): {
			{Title: "API rollout", Filename: "upper.md", ModifiedAt: time.Unix(2, 0)},
			{Title: "api cleanup", Filename: "lower.md", ModifiedAt: time.Unix(1, 0)},
		},
	}}
	view := NewNotesViewModelWithService(t.TempDir(), route.AllNotesScope(), service, nil, project)
	mountNotesManager(t, view)
	view.SetSize(120, 24)

	view.SearchApplied = "api"
	view.RederiveVisibleRows()
	if len(view.VisibleRows) != 2 || view.searchCaseRelaxed {
		t.Fatalf("lowercase search = %d rows relaxed=%v, want both notes unrelaxed",
			len(view.VisibleRows), view.searchCaseRelaxed)
	}

	view.SearchApplied = "API"
	view.RederiveVisibleRows()
	if len(view.VisibleRows) != 1 || view.VisibleRows[0].Note.Filename != "upper.md" || view.searchCaseRelaxed {
		t.Fatalf("uppercase search = %#v relaxed=%v, want only upper.md unrelaxed",
			view.VisibleRows, view.searchCaseRelaxed)
	}

	view.SearchApplied = "Cleanup"
	view.RederiveVisibleRows()
	if len(view.VisibleRows) != 1 || view.VisibleRows[0].Note.Filename != "lower.md" || !view.searchCaseRelaxed {
		t.Fatalf("fallback search = %#v relaxed=%v, want lower.md via a relaxed retry",
			view.VisibleRows, view.searchCaseRelaxed)
	}
	if got := view.scopeCountLabel(); !strings.Contains(got, `"Cleanup" (case ignored) in`) {
		t.Fatalf("relaxed count label = %q, want a case-ignored badge", got)
	}

	view.SearchApplied = "Zebra"
	view.RederiveVisibleRows()
	if len(view.VisibleRows) != 0 || view.searchCaseRelaxed {
		t.Fatalf("unmatchable search = %d rows relaxed=%v, want an ordinary empty state",
			len(view.VisibleRows), view.searchCaseRelaxed)
	}
	if got := view.scopeCountLabel(); strings.Contains(got, "case ignored") {
		t.Fatalf("empty retry count label = %q, want no badge", got)
	}
}

func titleMatchIdentity(note *models.Note) NoteRowIdentity {
	return NoteRowIdentity{Location: app.GlobalNoteLocation(), Filename: note.Filename}
}

func TestNotesSearchLiveInputNavigationAndEscTransitions(t *testing.T) {
	t.Setenv("DOCKET_EDITOR", "true")
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	project := &models.Project{ID: "alpha", Name: "Alpha", Path: t.TempDir()}
	service := &combinedNotesService{byLocation: map[app.NoteLocation][]*models.Note{
		app.GlobalNoteLocation(): {
			{Title: "Unrelated", Filename: "other.md", Body: "ordinary"},
			{Title: "Body result", Filename: "body.md", Body: "contains the NEEDLE here"},
		},
		app.ProjectNoteLocation(project.ID): {
			{Title: "Needle title", Filename: "title.md"},
		},
	}}
	view := NewNotesViewModelWithService(
		t.TempDir(),
		route.AllNotesScope(),
		service,
		nil,
		project,
	)
	shell := mountNotesManager(t, view)
	view.SetSize(80, 24)

	_, cmd := view.Update(syntheticKey('/'))
	if cmd == nil || !view.HasTextInput() || !view.ShouldCaptureEsc() {
		t.Fatal("/ did not open a focused search input that owns Esc")
	}
	for _, r := range "needle" {
		_, _ = view.Update(syntheticKey(r))
	}
	if view.LocationPicker != nil {
		t.Fatal("search text propagated a to the note creation control")
	}
	if view.SearchQuery() != "" || len(view.VisibleRows) != 2 {
		t.Fatalf(
			"live draft changed accepted query or results: accepted=%q rows=%d",
			view.SearchQuery(),
			len(view.VisibleRows),
		)
	}

	view.Cursor = 0
	_, _ = view.Update(syntheticSpecialKey(tea.KeyDown))
	if view.Cursor != 1 {
		t.Fatalf("Down during search moved cursor to %d, want 1", view.Cursor)
	}
	_, _ = view.Update(syntheticCtrlKey('k'))
	if view.Cursor != 0 {
		t.Fatalf("Ctrl+K during search moved cursor to %d, want 0", view.Cursor)
	}

	_, _ = view.Update(syntheticSpecialKey(tea.KeyEnter))
	if view.HasTextInput() || view.SearchQuery() != "needle" || len(view.VisibleRows) != 2 {
		t.Fatalf(
			"accepted search = input %t query %q rows %d",
			view.HasTextInput(),
			view.SearchQuery(),
			len(view.VisibleRows),
		)
	}
	rendered := view.View().Content
	plain := ansi.Strip(rendered)
	for _, want := range []string{`Search: "needle"`, "2 of 3 notes match", "· body"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("accepted search render missing %q:\n%s", want, plain)
		}
	}
	if !strings.Contains(rendered, theme.HighlightStyle.Render("N")) {
		t.Fatalf("title match was not highlighted:\n%s", rendered)
	}

	if _, openCmd := view.Update(syntheticSpecialKey(tea.KeyEnter)); openCmd == nil {
		t.Fatal("applied search result could not be opened")
	}
	_, _ = view.Update(syntheticKey('e'))
	if !view.renaming {
		t.Fatal("applied search result could not be renamed")
	}
	_, _ = view.Update(syntheticSpecialKey(tea.KeyEsc))
	_, _ = view.Update(syntheticKey(' '))
	if !shell.actionSheetOpened {
		t.Fatal("applied search result could not reach its action sheet")
	}
	shell.actionSheetOpened = false

	_, _ = view.Update(syntheticKey('/'))
	_, _ = view.Update(syntheticKey('z'))
	if len(view.VisibleRows) != 0 || view.SearchQuery() != "needle" {
		t.Fatalf("draft edit = rows %d accepted %q, want zero rows and prior query", len(view.VisibleRows), view.SearchQuery())
	}
	_, _ = view.Update(syntheticSpecialKey(tea.KeyEsc))
	if view.HasTextInput() || view.SearchQuery() != "needle" || len(view.VisibleRows) != 2 {
		t.Fatalf(
			"editing Esc did not restore accepted search: input=%t query=%q rows=%d",
			view.HasTextInput(),
			view.SearchQuery(),
			len(view.VisibleRows),
		)
	}

	_, clearCmd := view.Update(syntheticSpecialKey(tea.KeyEsc))
	if clearCmd != nil || view.SearchQuery() != "" || len(view.VisibleRows) != 3 {
		t.Fatalf("normal Esc did not clear search first: cmd=%v query=%q rows=%d", clearCmd, view.SearchQuery(), len(view.VisibleRows))
	}
	_, closeCmd := view.Update(syntheticSpecialKey(tea.KeyEsc))
	if closeCmd == nil {
		t.Fatal("second normal Esc did not close Notes")
	}
	if _, ok := closeCmd().(CloseNotesManagerMsg); !ok {
		t.Fatalf("second normal Esc emitted %T, want CloseNotesManagerMsg", closeCmd())
	}
}

func TestNotesSearchAcceptedQuerySurvivesScopeAndRefresh(t *testing.T) {
	project := &models.Project{ID: "alpha", Name: "Alpha", Path: t.TempDir()}
	service := &combinedNotesService{byLocation: map[app.NoteLocation][]*models.Note{
		app.GlobalNoteLocation(): {
			{Title: "Global", Filename: "global.md", Body: "needle"},
		},
		app.ProjectNoteLocation(project.ID): {
			{Title: "Needle project", Filename: "project.md"},
		},
	}}
	view := NewNotesViewModelWithService(
		t.TempDir(),
		route.AllNotesScope(),
		service,
		nil,
		project,
	)
	mountNotesManager(t, view)
	_, _ = view.Update(syntheticKey('/'))
	for _, r := range "needle" {
		_, _ = view.Update(syntheticKey(r))
	}
	_, _ = view.Update(syntheticSpecialKey(tea.KeyEnter))

	view.SetScope(route.GlobalNotesScope())
	if view.SearchQuery() != "needle" || len(view.VisibleRows) != 1 ||
		view.VisibleRows[0].Location != app.GlobalNoteLocation() {
		t.Fatalf("Global scope lost search: query=%q rows=%#v", view.SearchQuery(), view.VisibleRows)
	}
	view.SetScope(route.ProjectNotesScope(project.ID))
	if view.SearchQuery() != "needle" || len(view.VisibleRows) != 1 ||
		view.VisibleRows[0].Location != app.ProjectNoteLocation(project.ID) {
		t.Fatalf("project scope lost search: query=%q rows=%#v", view.SearchQuery(), view.VisibleRows)
	}

	service.byLocation[app.ProjectNoteLocation(project.ID)] = append(
		service.byLocation[app.ProjectNoteLocation(project.ID)],
		&models.Note{Title: "Another", Filename: "another.md", Body: "needle"},
	)
	view.RefreshWorkspace(project)
	if view.SearchQuery() != "needle" || len(view.VisibleRows) != 2 {
		t.Fatalf("refresh lost accepted search: query=%q rows=%d", view.SearchQuery(), len(view.VisibleRows))
	}
}
