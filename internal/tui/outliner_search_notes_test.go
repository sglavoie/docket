package tui

import (
	"errors"
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/query"
)

func noteIndexRow(location app.NoteLocation, label, title, filename string) LocatedNoteRow {
	return LocatedNoteRow{
		Note:       &models.Note{Title: title, Filename: filename},
		Location:   location,
		OwnerLabel: label,
	}
}

// noteTierOutliner builds a cross-project Outliner whose note index is the
// supplied snapshot, which is what the shell injects in production.
func noteTierOutliner(rows []LocatedNoteRow, projects ...*models.Project) *OutlinerModel {
	outliner := scopeTestOutliner(projects...)
	outliner.setNoteSearchIndex(func() []LocatedNoteRow { return rows })
	return outliner
}

func noteTierIdentities(matches []outlinerSearchNoteMatch) []NoteRowIdentity {
	identities := make([]NoteRowIdentity, len(matches))
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
	rows := []LocatedNoteRow{
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
	rows := []LocatedNoteRow{
		noteIndexRow(app.GlobalNoteLocation(), "Global", "Global plan", "global.md"),
		noteIndexRow(app.ProjectNoteLocation(alpha.ID), "alpha", "Alpha plan", "alpha.md"),
		noteIndexRow(app.ProjectNoteLocation(beta.ID), "beta", "Beta plan", "beta.md"),
	}
	outliner := noteTierOutliner(rows, alpha, beta)

	unscoped := noteTierIdentities(outliner.scoreNoteSearchTier("plan", fuzzy.NewMatcher(false), true))
	wantUnscoped := []NoteRowIdentity{
		{Location: app.GlobalNoteLocation(), Filename: "global.md"},
		{Location: app.ProjectNoteLocation(alpha.ID), Filename: "alpha.md"},
		{Location: app.ProjectNoteLocation(beta.ID), Filename: "beta.md"},
	}
	if !slices.Equal(unscoped, wantUnscoped) {
		t.Fatalf("unscoped tier = %+v, want Global plus every project %+v", unscoped, wantUnscoped)
	}

	outliner.pushProjectScope(alpha.ID)
	scoped := noteTierIdentities(outliner.scoreNoteSearchTier("plan", fuzzy.NewMatcher(false), true))
	wantScoped := []NoteRowIdentity{
		{Location: app.ProjectNoteLocation(alpha.ID), Filename: "alpha.md"},
	}
	if !slices.Equal(scoped, wantScoped) {
		t.Fatalf("scoped tier = %+v, want the scoped project's notes alone %+v", scoped, wantScoped)
	}
}

// Ranking ties fall to the index's own location-then-filename order, so the
// same query lists the same notes in the same order on every rebuild.
func TestNoteSearchTierRanksByScoreThenIndexOrder(t *testing.T) {
	rows := []LocatedNoteRow{
		noteIndexRow(app.GlobalNoteLocation(), "Global", "Nothing here", "a.md"),
		noteIndexRow(app.ProjectNoteLocation("alpha"), "alpha", "Widget rollout", "b.md"),
		noteIndexRow(app.ProjectNoteLocation("alpha"), "alpha", "Widget rollout", "a.md"),
	}
	rows[0].Note.Body = "a passing mention of widget in the body only"
	outliner := noteTierOutliner(rows)

	got := noteTierIdentities(outliner.scoreNoteSearchTier("widget", fuzzy.NewMatcher(false), true))
	want := []NoteRowIdentity{
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
	rows := []LocatedNoteRow{
		noteIndexRow(app.GlobalNoteLocation(), "Global", "widget rollout", "widget.md"),
	}
	outliner := noteTierOutliner(rows, alpha)

	searchRankedList(outliner, "Widget")
	// The notes tier now emits its own rows, so the projection is counted by row
	// kind: the note is on screen and no task is.
	tasks, notes := 0, 0
	for _, row := range outliner.rows {
		switch {
		case row.IsNoteRow:
			notes++
		case row.isTaskRow():
			tasks++
		}
	}
	if tasks != 0 || notes != 1 {
		t.Fatalf("projection = %d task rows and %d note rows, want the note alone", tasks, notes)
	}
	if !outliner.searchCaseRelaxed {
		t.Fatalf("searchCaseRelaxed = false, want the note-only hit to relax the query")
	}

	searchRankedList(outliner, "Widget !1")
	if outliner.searchCaseRelaxed {
		t.Fatalf("searchCaseRelaxed = true under a sigil, want the notes tier absent whole")
	}
}

func TestNoteOwnerCatalogAdmitsGlobalAndRefusesArchivedOwners(t *testing.T) {
	alpha := &models.Project{ID: "alpha", Name: "Alpha", Path: t.TempDir()}
	archived := &models.Project{ID: "archived", Name: "Archived", Path: t.TempDir(), Archived: true}
	blank := &models.Project{ID: "  ", Name: "Blank", Path: t.TempDir()}
	unnamed := &models.Project{ID: "unnamed", Path: t.TempDir()}

	owners, projects := noteOwnerCatalog("/notes", []*models.Project{alpha, archived, blank, unnamed, alpha})
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

func TestNoteSearchIndexCmdOrdersRowsAndLabelsOwners(t *testing.T) {
	global := app.GlobalNoteLocation()
	alpha := app.ProjectNoteLocation("alpha")
	service := &combinedNotesService{byLocation: map[app.NoteLocation][]*models.Note{
		global: {{Title: "Global", Filename: "z.md"}, nil},
		alpha:  {{Title: "Second", Filename: "b.md"}, {Title: "First", Filename: "a.md"}},
	}}
	service.errors = map[app.NoteLocation]error{
		app.ProjectNoteLocation("broken"): errors.New("unreadable"),
	}
	owners := []noteSearchIndexOwner{
		{Location: global, Label: "Global"},
		{Location: alpha, Label: "Alpha"},
		{Location: app.ProjectNoteLocation("broken"), Label: "Broken"},
	}

	msg, ok := noteSearchIndexCmd(service, 7, owners)().(noteSearchIndexLoadedMsg)
	if !ok {
		t.Fatalf("note index command produced %T, want noteSearchIndexLoadedMsg", msg)
	}
	if msg.generation != 7 || msg.err != nil {
		t.Fatalf("generation %d err %v, want the requested generation and no error", msg.generation, msg.err)
	}
	got := make([]NoteRowIdentity, len(msg.rows))
	labels := make([]string, len(msg.rows))
	for i, row := range msg.rows {
		got[i] = row.Identity()
		labels[i] = row.OwnerLabel
	}
	want := []NoteRowIdentity{
		{Location: global, Filename: "z.md"},
		{Location: alpha, Filename: "a.md"},
		{Location: alpha, Filename: "b.md"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("index rows = %+v, want location-then-filename order %+v", got, want)
	}
	if !slices.Equal(labels, []string{"Global", "Alpha", "Alpha"}) {
		t.Fatalf("owner labels = %v, want each row labeled by its own owner", labels)
	}

	// A snapshot failure is reported rather than published as an empty index.
	service.snapshotErr = errors.New("snapshot failed")
	failed := noteSearchIndexCmd(service, 8, owners)().(noteSearchIndexLoadedMsg)
	if failed.err == nil || failed.rows != nil {
		t.Fatalf("failed read = %+v, want an error and no rows", failed)
	}

	// A nil service is inert rather than an error: the tier is simply empty.
	inert := noteSearchIndexCmd(nil, 9, owners)().(noteSearchIndexLoadedMsg)
	if inert.err != nil || inert.rows != nil || inert.generation != 9 {
		t.Fatalf("nil-service read = %+v, want an inert empty result", inert)
	}
}

func TestNoteSearchIndexKeepsItsRowsAgainstStaleAndFailedReads(t *testing.T) {
	live := []LocatedNoteRow{noteIndexRow(app.GlobalNoteLocation(), "Global", "Live", "live.md")}
	stale := []LocatedNoteRow{noteIndexRow(app.GlobalNoteLocation(), "Global", "Stale", "stale.md")}
	model := &AppModel{noteIndexGeneration: 3, noteIndexRows: live}

	model.handleNoteSearchIndexLoaded(noteSearchIndexLoadedMsg{generation: 2, rows: stale})
	if len(model.noteIndexRows) != 1 || model.noteIndexRows[0].Note.Title != "Live" {
		t.Fatalf("index rows = %+v, want the superseded read discarded", model.noteIndexRows)
	}

	model.handleNoteSearchIndexLoaded(noteSearchIndexLoadedMsg{generation: 3, err: errors.New("read failed")})
	if len(model.noteIndexRows) != 1 || model.noteIndexRows[0].Note.Title != "Live" {
		t.Fatalf("index rows = %+v, want a failed refresh to leave the retained rows", model.noteIndexRows)
	}

	model.handleNoteSearchIndexLoaded(noteSearchIndexLoadedMsg{generation: 3, rows: stale})
	if len(model.noteIndexRows) != 1 || model.noteIndexRows[0].Note.Title != "Stale" {
		t.Fatalf("index rows = %+v, want the current generation adopted", model.noteIndexRows)
	}
	if got := model.noteSearchIndexRows(); !slices.Equal(got, model.noteIndexRows) {
		t.Fatalf("injected accessor = %+v, want the retained rows", got)
	}
}

// A refresh that lands while a search is already open must reach the screen on
// the next frame. The index is read through an injected accessor, so it is
// invisible to the rebuild fingerprint: the accepted result has to invalidate
// the projection itself or the memoized rows would keep the superseded tier.
func TestNoteSearchIndexRefreshRebuildsOpenProjection(t *testing.T) {
	task := primaryRowsTask("task", models.StateBacklog)
	task.Title = "Unrelated"
	alpha := primaryRowsProject("alpha", task)
	outliner := scopeTestOutliner(alpha)
	model := &AppModel{crossOutliner: outliner, noteIndexGeneration: 4}
	outliner.setNoteSearchIndex(model.noteSearchIndexRows)

	// The search opens against an index that has nothing to say about it, which
	// is exactly the state a first launch is in while the read is still running.
	searchRankedList(outliner, "widget")
	if !outliner.hasSearchProjection() {
		t.Fatalf("the query left no open search projection")
	}
	if got := countNoteRows(outliner); got != 0 {
		t.Fatalf("projection = %d note rows before the index landed, want none", got)
	}

	refreshed := []LocatedNoteRow{
		noteIndexRow(app.GlobalNoteLocation(), "Global", "Widget rollout", "widget.md"),
	}
	model.handleNoteSearchIndexLoaded(noteSearchIndexLoadedMsg{generation: 4, rows: refreshed})

	// The next frame, and no user action in between.
	outliner.crossProjectRebuildRows()
	if got := countNoteRows(outliner); got != 1 {
		t.Fatalf("projection = %d note rows after the refresh, want the refreshed note visible", got)
	}

	// A stale generation and a failed read leave the displayed projection alone
	// rather than replacing it with a superseded or empty answer.
	superseded := []LocatedNoteRow{
		noteIndexRow(app.GlobalNoteLocation(), "Global", "Nothing here", "other.md"),
	}
	model.handleNoteSearchIndexLoaded(noteSearchIndexLoadedMsg{generation: 3, rows: superseded})
	outliner.crossProjectRebuildRows()
	if got := countNoteRows(outliner); got != 1 {
		t.Fatalf("projection = %d note rows after a stale read, want the displayed note retained", got)
	}
	model.handleNoteSearchIndexLoaded(noteSearchIndexLoadedMsg{generation: 4, err: errors.New("read failed")})
	outliner.crossProjectRebuildRows()
	if got := countNoteRows(outliner); got != 1 {
		t.Fatalf("projection = %d note rows after a failed read, want the displayed note retained", got)
	}
}

// A refresh re-ranks the notes tier, so the row the cursor rested on can move.
// The rebuild must re-seat the cursor by the note's own identity: keeping the
// numeric row would silently select whichever result took its place.
func TestNoteSearchIndexRefreshPreservesCursorIdentity(t *testing.T) {
	task := primaryRowsTask("task", models.StateBacklog)
	task.Title = "Unrelated"
	alpha := primaryRowsProject("alpha", task)
	global := app.GlobalNoteLocation()

	// One title for all three, so the tier's score ties and its filename
	// tiebreak is what decides order. Inserting "widget-a.md" then pushes every
	// already-visible match down exactly one row.
	rows := []LocatedNoteRow{
		noteIndexRow(global, "Global", "Widget notes", "widget-b.md"),
		noteIndexRow(global, "Global", "Widget notes", "widget-c.md"),
	}
	outliner := scopeTestOutliner(alpha)
	model := &AppModel{crossOutliner: outliner, noteIndexGeneration: 2, noteIndexRows: rows}
	outliner.setNoteSearchIndex(model.noteSearchIndexRows)

	searchRankedList(outliner, "widget")
	if got := countNoteRows(outliner); got != 2 {
		t.Fatalf("projection = %d note rows, want both matching notes", got)
	}
	selected := NoteRowIdentity{Location: global, Filename: "widget-c.md"}
	index, ok := noteRowIndex(outliner, selected)
	if !ok {
		t.Fatalf("note rows = %+v, want the selected note on screen", noteRowIdentities(outliner))
	}
	outliner.Cursor = index

	// A stale generation and a failed read change neither the rows nor the
	// cursor, so nothing is re-seated on a refresh that was never accepted.
	model.handleNoteSearchIndexLoaded(noteSearchIndexLoadedMsg{generation: 1, rows: nil})
	model.handleNoteSearchIndexLoaded(noteSearchIndexLoadedMsg{generation: 2, err: errors.New("read failed")})
	outliner.crossProjectRebuildRows()
	if countNoteRows(outliner) != 2 || outliner.Cursor != index {
		t.Fatalf("cursor = %d over %d note rows, want an unaccepted refresh to change nothing",
			outliner.Cursor, countNoteRows(outliner))
	}

	inserted := []LocatedNoteRow{
		noteIndexRow(global, "Global", "Widget notes", "widget-a.md"),
		rows[0],
		rows[1],
	}
	model.handleNoteSearchIndexLoaded(noteSearchIndexLoadedMsg{generation: 2, rows: inserted})
	outliner.crossProjectRebuildRows()

	if got := countNoteRows(outliner); got != 3 {
		t.Fatalf("projection = %d note rows after the refresh, want the inserted match visible", got)
	}
	moved, ok := noteRowIndex(outliner, selected)
	if !ok {
		t.Fatalf("note rows = %+v, want the selected note still present", noteRowIdentities(outliner))
	}
	if moved == index {
		t.Fatalf("the inserted match did not move row %d, so the test proves nothing", index)
	}
	if outliner.Cursor != moved {
		t.Fatalf("cursor = %d, want %d: the selected note must keep focus across the refresh",
			outliner.Cursor, moved)
	}
	if identity, onNote := outliner.cursorNoteIdentity(); !onNote || identity != selected {
		t.Fatalf("cursor note = %+v (note row: %v), want %+v", identity, onNote, selected)
	}

	// The selected note leaving the index falls back to a valid navigable row
	// rather than an out-of-range cursor, and the render still owns the window.
	scrollOffset := outliner.ViewScrollOffset
	model.handleNoteSearchIndexLoaded(noteSearchIndexLoadedMsg{generation: 2, rows: inserted[:1]})
	outliner.crossProjectRebuildRows()
	if countNoteRows(outliner) != 1 {
		t.Fatalf("projection = %d note rows, want only the surviving match", countNoteRows(outliner))
	}
	if outliner.Cursor < 0 || outliner.Cursor >= len(outliner.rows) ||
		!outliner.rows[outliner.Cursor].isNavigable() {
		t.Fatalf("cursor = %d over %d rows, want a valid navigable landing row",
			outliner.Cursor, len(outliner.rows))
	}
	if outliner.ViewScrollOffset != scrollOffset {
		t.Fatalf("ViewScrollOffset = %d, want the rebuild to leave the window to the render",
			outliner.ViewScrollOffset)
	}
}

func noteRowIdentities(outliner *OutlinerModel) []NoteRowIdentity {
	var identities []NoteRowIdentity
	for _, row := range outliner.rows {
		if row.IsNoteRow {
			identities = append(identities, row.NoteRow)
		}
	}
	return identities
}

func noteRowIndex(outliner *OutlinerModel, identity NoteRowIdentity) (int, bool) {
	for index, row := range outliner.rows {
		if row.IsNoteRow && row.NoteRow == identity {
			return index, true
		}
	}
	return 0, false
}

func countNoteRows(outliner *OutlinerModel) int {
	notes := 0
	for _, row := range outliner.rows {
		if row.IsNoteRow {
			notes++
		}
	}
	return notes
}

// An availability message lands after every committed task mutation, so the
// index reloads only when the owner catalog actually moved.
func TestNoteSearchIndexReloadsOnlyForAnOwnerCatalogChange(t *testing.T) {
	model := &AppModel{}
	if cmd := model.requestNoteSearchIndexLoad(); cmd == nil {
		t.Fatalf("the first load produced no command")
	}
	generation := model.noteIndexGeneration
	if cmd := model.refreshNoteSearchIndexOwners(); cmd != nil {
		t.Fatalf("an unchanged owner catalog requested a reload")
	}
	if model.noteIndexGeneration != generation {
		t.Fatalf("generation moved to %d without a reload", model.noteIndexGeneration)
	}

	model.noteIndexOwners = "a different owner catalog"
	if cmd := model.refreshNoteSearchIndexOwners(); cmd == nil {
		t.Fatalf("a changed owner catalog requested no reload")
	}
	if model.noteIndexGeneration != generation+1 {
		t.Fatalf("generation = %d, want the reload to supersede the previous read", model.noteIndexGeneration)
	}
}
