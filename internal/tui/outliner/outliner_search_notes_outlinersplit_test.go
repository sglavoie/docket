package outliner

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/query"
)

func noteTierIdentities(matches []outlinerSearchNoteMatch) []notes.NoteRowIdentity {
	identities := make([]notes.NoteRowIdentity, len(matches))
	for i, match := range matches {
		identities[i] = match.row.Identity()
	}
	return identities
}

// The sigil gate is a fact about the parse, not about the conjuncts it
// compiled, so a destination or pin sigil counts the same as a tag. A sigil the
// parser refuses outright is caught by the invalid() half of the same decision.
func TestSearchPredicatesReportAnySigil(t *testing.T) {
	outliner := scopeTestOutliner()
	cases := []struct {
		query string
		want  bool
	}{
		{query: "", want: false},
		{query: "needle", want: false},
		{query: "release notes", want: false},
		{query: "needle #auth", want: true},
		{query: "!1", want: true},
		{query: "needle @today", want: true},
		{query: "needle $doing", want: true},
		{query: "needle ##errand", want: true},
		{query: "needle *pin", want: true},
	}
	for _, testCase := range cases {
		_, predicates := outliner.compileSearchQuery(testCase.query, nil)
		if got := predicates.HasAnySigil(); got != testCase.want {
			t.Fatalf("hasAnySigil(%q) = %v, want %v", testCase.query, got, testCase.want)
		}
	}
	var absent *query.SearchPredicates
	if absent.HasAnySigil() {
		t.Fatalf("a nil gate reported a sigil")
	}

	// A retired destination never becomes a token, so it is the guidance arm
	// rather than this flag that keeps the notes tier out of a refused query.
	_, refused := outliner.compileSearchQuery("needle $ready", nil)
	if !refused.Invalid() {
		t.Fatalf("a retired sigil compiled to a usable gate")
	}
}

func TestNoteSearchTierIsAbsentWithoutAnInjectedIndex(t *testing.T) {
	// The Project Hub's preview painter constructs an Outliner with no service
	// and no index, and every struct-literal test model does the same.
	outliner := scopeTestOutliner()
	if rows := outliner.noteSearchRowsInScope(); rows != nil {
		t.Fatalf("scoped rows = %+v, want none without an injected index", rows)
	}
	matches := outliner.scoreNoteSearchTier("needle", fuzzy.NewMatcher(false), true)
	if matches != nil {
		t.Fatalf("note matches = %+v, want none without an injected index", matches)
	}
}

func TestNoteSearchTierRefusedByTheSigilDecision(t *testing.T) {
	rows := []notes.LocatedNoteRow{
		noteIndexRow(app.GlobalNoteLocation(), "Global", "Needle note", "needle.md"),
	}
	outliner := noteTierOutliner(rows)
	if got := outliner.scoreNoteSearchTier("needle", fuzzy.NewMatcher(false), true); len(got) != 1 {
		t.Fatalf("admitted tier = %d matches, want the one note", len(got))
	}
	if got := outliner.scoreNoteSearchTier("needle", fuzzy.NewMatcher(false), false); got != nil {
		t.Fatalf("refused tier = %+v, want nothing scored at all", got)
	}
}

// A single-project scope yields that project's notes alone — not Global —
// exactly as a scoped Outliner shows only that project's work.
func TestNoteSearchTierRespectsProjectScope(t *testing.T) {
	alpha := primaryRowsProject("alpha")
	beta := primaryRowsProject("beta")
	rows := []notes.LocatedNoteRow{
		noteIndexRow(app.GlobalNoteLocation(), "Global", "Global plan", "global.md"),
		noteIndexRow(app.ProjectNoteLocation(alpha.ID), "alpha", "Alpha plan", "alpha.md"),
		noteIndexRow(app.ProjectNoteLocation(beta.ID), "beta", "Beta plan", "beta.md"),
	}
	outliner := noteTierOutliner(rows, alpha, beta)

	unscoped := noteTierIdentities(outliner.scoreNoteSearchTier("plan", fuzzy.NewMatcher(false), true))
	wantUnscoped := []notes.NoteRowIdentity{
		{Location: app.GlobalNoteLocation(), Filename: "global.md"},
		{Location: app.ProjectNoteLocation(alpha.ID), Filename: "alpha.md"},
		{Location: app.ProjectNoteLocation(beta.ID), Filename: "beta.md"},
	}
	if !slices.Equal(unscoped, wantUnscoped) {
		t.Fatalf("unscoped tier = %+v, want Global plus every project %+v", unscoped, wantUnscoped)
	}

	outliner.PushProjectScope(alpha.ID)
	scoped := noteTierIdentities(outliner.scoreNoteSearchTier("plan", fuzzy.NewMatcher(false), true))
	wantScoped := []notes.NoteRowIdentity{
		{Location: app.ProjectNoteLocation(alpha.ID), Filename: "alpha.md"},
	}
	if !slices.Equal(scoped, wantScoped) {
		t.Fatalf("scoped tier = %+v, want the scoped project's notes alone %+v", scoped, wantScoped)
	}
}

// Ranking ties fall to the index's own location-then-filename order, so the
// same query lists the same notes in the same order on every rebuild.
func TestNoteSearchTierRanksByScoreThenIndexOrder(t *testing.T) {
	rows := []notes.LocatedNoteRow{
		noteIndexRow(app.GlobalNoteLocation(), "Global", "Nothing here", "a.md"),
		noteIndexRow(app.ProjectNoteLocation("alpha"), "alpha", "Widget rollout", "b.md"),
		noteIndexRow(app.ProjectNoteLocation("alpha"), "alpha", "Widget rollout", "a.md"),
	}
	rows[0].Note.Body = "a passing mention of widget in the body only"
	outliner := noteTierOutliner(rows)

	got := noteTierIdentities(outliner.scoreNoteSearchTier("widget", fuzzy.NewMatcher(false), true))
	want := []notes.NoteRowIdentity{
		{Location: app.ProjectNoteLocation("alpha"), Filename: "a.md"},
		{Location: app.ProjectNoteLocation("alpha"), Filename: "b.md"},
		{Location: app.GlobalNoteLocation(), Filename: "a.md"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("ranked tier = %+v, want title hits ahead of the body hit in index order %+v", got, want)
	}
}

// One query has one case reading. The relaxation decision is taken over the
// whole result set, so a query whose only hit is a note still relaxes — and a
// sigil, which removes the tier entirely, takes that hit away with it.
func TestSearchRelaxesItsCaseUniformlyAcrossTasksAndNotes(t *testing.T) {
	task := primaryRowsTask("task", models.StateBacklog)
	task.Title = "Unrelated"
	alpha := primaryRowsProject("alpha", task)
	rows := []notes.LocatedNoteRow{
		noteIndexRow(app.GlobalNoteLocation(), "Global", "widget rollout", "widget.md"),
	}
	outliner := noteTierOutliner(rows, alpha)

	searchRankedList(outliner, "Widget")
	// The notes tier now emits its own rows, so the projection is counted by row
	// kind: the note is on screen and no task is.
	tasks, notes := 0, 0
	for _, row := range outliner.Rows {
		switch {
		case row.IsNoteRow:
			notes++
		case row.IsTaskRow():
			tasks++
		}
	}
	if tasks != 0 || notes != 1 {
		t.Fatalf("projection = %d task rows and %d note rows, want the note alone", tasks, notes)
	}
	if !outliner.Search.caseRelaxed {
		t.Fatalf("searchCaseRelaxed = false, want the note-only hit to relax the query")
	}

	searchRankedList(outliner, "Widget !1")
	if outliner.Search.caseRelaxed {
		t.Fatalf("searchCaseRelaxed = true under a sigil, want the notes tier absent whole")
	}
}

func TestNoteOwnerCatalogAdmitsGlobalAndRefusesArchivedOwners(t *testing.T) {
	alpha := &models.Project{ID: "alpha", Name: "Alpha", Path: t.TempDir()}
	archived := &models.Project{ID: "archived", Name: "Archived", Path: t.TempDir(), Archived: true}
	blank := &models.Project{ID: "  ", Name: "Blank", Path: t.TempDir()}
	unnamed := &models.Project{ID: "unnamed", Path: t.TempDir()}

	owners, projects := notes.NoteOwnerCatalog("/notes", []*models.Project{alpha, archived, blank, unnamed, alpha})
	if len(projects) != 2 || projects[0] != alpha || projects[1] != unnamed {
		t.Fatalf("owned projects = %+v, want alpha and unnamed once each", projects)
	}
	if owner := owners[app.GlobalNoteLocation()]; owner.Label != "Global" || owner.Directory != "/notes" {
		t.Fatalf("global owner = %+v, want the supplied notes dir labeled Global", owner)
	}
	if _, ok := owners[app.ProjectNoteLocation("archived")]; ok {
		t.Fatalf("an archived project was admitted as a note owner")
	}
	if label := owners[app.ProjectNoteLocation("unnamed")].Label; label != "unnamed" {
		t.Fatalf("unnamed owner label = %q, want the project id as the fallback", label)
	}
}
