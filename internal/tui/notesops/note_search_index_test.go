package notesops

import (
	"errors"
	"slices"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/notes"
	"github.com/sglavoie/docket/internal/tui/outliner"
)

func TestNoteSearchIndexCmdOrdersRowsAndLabelsOwners(t *testing.T) {
	global := app.GlobalNoteLocation()
	alpha := app.ProjectNoteLocation("alpha")
	service := &indexNotesService{byLocation: map[app.NoteLocation][]*models.Note{
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

	msg, ok := noteSearchIndexCmd(service, 7, owners)().(NoteSearchIndexLoadedMsg)
	if !ok {
		t.Fatalf("note index command produced %T, want NoteSearchIndexLoadedMsg", msg)
	}
	if msg.generation != 7 || msg.err != nil {
		t.Fatalf("generation %d err %v, want the requested generation and no error", msg.generation, msg.err)
	}
	got := make([]notes.NoteRowIdentity, len(msg.rows))
	labels := make([]string, len(msg.rows))
	for i, row := range msg.rows {
		got[i] = row.Identity()
		labels[i] = row.OwnerLabel
	}
	want := []notes.NoteRowIdentity{
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
	failed := noteSearchIndexCmd(service, 8, owners)().(NoteSearchIndexLoadedMsg)
	if failed.err == nil || failed.rows != nil {
		t.Fatalf("failed read = %+v, want an error and no rows", failed)
	}

	// A nil service is inert rather than an error: the tier is simply empty.
	inert := noteSearchIndexCmd(nil, 9, owners)().(NoteSearchIndexLoadedMsg)
	if inert.err != nil || inert.rows != nil || inert.generation != 9 {
		t.Fatalf("nil-service read = %+v, want an inert empty result", inert)
	}
}

func TestNoteSearchIndexKeepsItsRowsAgainstStaleAndFailedReads(t *testing.T) {
	live := []notes.LocatedNoteRow{noteIndexRow(app.GlobalNoteLocation(), "Live", "live.md")}
	stale := []notes.LocatedNoteRow{noteIndexRow(app.GlobalNoteLocation(), "Stale", "stale.md")}
	controller := &Controller{index: noteIndex{generation: 3, rows: live}}
	shell := &testShell{}

	controller.HandleNoteSearchIndexLoaded(shell, NoteSearchIndexLoadedMsg{generation: 2, rows: stale})
	if len(controller.index.rows) != 1 || controller.index.rows[0].Note.Title != "Live" {
		t.Fatalf("index rows = %+v, want the superseded read discarded", controller.index.rows)
	}

	controller.HandleNoteSearchIndexLoaded(shell, NoteSearchIndexLoadedMsg{generation: 3, err: errors.New("read failed")})
	if len(controller.index.rows) != 1 || controller.index.rows[0].Note.Title != "Live" {
		t.Fatalf("index rows = %+v, want a failed refresh to leave the retained rows", controller.index.rows)
	}

	controller.HandleNoteSearchIndexLoaded(shell, NoteSearchIndexLoadedMsg{generation: 3, rows: stale})
	if len(controller.index.rows) != 1 || controller.index.rows[0].Note.Title != "Stale" {
		t.Fatalf("index rows = %+v, want the current generation adopted", controller.index.rows)
	}
	if got := controller.NoteSearchIndexRows(shell); !slices.Equal(got, controller.index.rows) {
		t.Fatalf("injected accessor = %+v, want the retained rows", got)
	}
}

// A refresh that lands while a search is already open must reach the screen on
// the next frame. The index is read through an injected accessor, so it is
// invisible to the rebuild fingerprint: the accepted result has to invalidate
// the projection itself or the memoized rows would keep the superseded tier.
func TestNoteSearchIndexRefreshRebuildsOpenProjection(t *testing.T) {
	task := testTask("task", models.StateBacklog)
	task.Title = "Unrelated"
	alpha := testProject("alpha", task)
	outlinerView := testOutliner(alpha)
	controller := &Controller{index: noteIndex{generation: 4}}
	shell := &testShell{outlinerView: outlinerView}
	outlinerView.SetNoteSearchIndex(func() []notes.LocatedNoteRow { return controller.NoteSearchIndexRows(shell) })

	// The search opens against an index that has nothing to say about it, which
	// is exactly the state a first launch is in while the read is still running.
	searchRankedList(outlinerView, "widget")
	if !outlinerView.HasSearchProjection() {
		t.Fatalf("the query left no open search projection")
	}
	if got := countNoteRows(outlinerView); got != 0 {
		t.Fatalf("projection = %d note rows before the index landed, want none", got)
	}

	refreshed := []notes.LocatedNoteRow{
		noteIndexRow(app.GlobalNoteLocation(), "Widget rollout", "widget.md"),
	}
	controller.HandleNoteSearchIndexLoaded(shell, NoteSearchIndexLoadedMsg{generation: 4, rows: refreshed})

	// The next frame, and no user action in between.
	outlinerView.CrossProjectRebuildRows()
	if got := countNoteRows(outlinerView); got != 1 {
		t.Fatalf("projection = %d note rows after the refresh, want the refreshed note visible", got)
	}

	// A stale generation and a failed read leave the displayed projection alone
	// rather than replacing it with a superseded or empty answer.
	superseded := []notes.LocatedNoteRow{
		noteIndexRow(app.GlobalNoteLocation(), "Nothing here", "other.md"),
	}
	controller.HandleNoteSearchIndexLoaded(shell, NoteSearchIndexLoadedMsg{generation: 3, rows: superseded})
	outlinerView.CrossProjectRebuildRows()
	if got := countNoteRows(outlinerView); got != 1 {
		t.Fatalf("projection = %d note rows after a stale read, want the displayed note retained", got)
	}
	controller.HandleNoteSearchIndexLoaded(shell, NoteSearchIndexLoadedMsg{generation: 4, err: errors.New("read failed")})
	outlinerView.CrossProjectRebuildRows()
	if got := countNoteRows(outlinerView); got != 1 {
		t.Fatalf("projection = %d note rows after a failed read, want the displayed note retained", got)
	}
}

// A refresh re-ranks the notes tier, so the row the cursor rested on can move.
// The rebuild must re-seat the cursor by the note's own identity: keeping the
// numeric row would silently select whichever result took its place.
func TestNoteSearchIndexRefreshPreservesCursorIdentity(t *testing.T) {
	task := testTask("task", models.StateBacklog)
	task.Title = "Unrelated"
	alpha := testProject("alpha", task)
	global := app.GlobalNoteLocation()

	// One title for all three, so the tier's score ties and its filename
	// tiebreak is what decides order. Inserting "widget-a.md" then pushes every
	// already-visible match down exactly one row.
	rows := []notes.LocatedNoteRow{
		noteIndexRow(global, "Widget notes", "widget-b.md"),
		noteIndexRow(global, "Widget notes", "widget-c.md"),
	}
	outlinerView := testOutliner(alpha)
	controller := &Controller{index: noteIndex{generation: 2, rows: rows}}
	shell := &testShell{outlinerView: outlinerView}
	outlinerView.SetNoteSearchIndex(func() []notes.LocatedNoteRow { return controller.NoteSearchIndexRows(shell) })

	searchRankedList(outlinerView, "widget")
	if got := countNoteRows(outlinerView); got != 2 {
		t.Fatalf("projection = %d note rows, want both matching notes", got)
	}
	selected := notes.NoteRowIdentity{Location: global, Filename: "widget-c.md"}
	index, ok := noteRowIndex(outlinerView, selected)
	if !ok {
		t.Fatalf("note rows = %+v, want the selected note on screen", noteRowIdentities(outlinerView))
	}
	outlinerView.Cursor = index

	// A stale generation and a failed read change neither the rows nor the
	// cursor, so nothing is re-seated on a refresh that was never accepted.
	controller.HandleNoteSearchIndexLoaded(shell, NoteSearchIndexLoadedMsg{generation: 1, rows: nil})
	controller.HandleNoteSearchIndexLoaded(shell, NoteSearchIndexLoadedMsg{generation: 2, err: errors.New("read failed")})
	outlinerView.CrossProjectRebuildRows()
	if countNoteRows(outlinerView) != 2 || outlinerView.Cursor != index {
		t.Fatalf("cursor = %d over %d note rows, want an unaccepted refresh to change nothing",
			outlinerView.Cursor, countNoteRows(outlinerView))
	}

	inserted := []notes.LocatedNoteRow{
		noteIndexRow(global, "Widget notes", "widget-a.md"),
		rows[0],
		rows[1],
	}
	controller.HandleNoteSearchIndexLoaded(shell, NoteSearchIndexLoadedMsg{generation: 2, rows: inserted})
	outlinerView.CrossProjectRebuildRows()

	if got := countNoteRows(outlinerView); got != 3 {
		t.Fatalf("projection = %d note rows after the refresh, want the inserted match visible", got)
	}
	moved, ok := noteRowIndex(outlinerView, selected)
	if !ok {
		t.Fatalf("note rows = %+v, want the selected note still present", noteRowIdentities(outlinerView))
	}
	if moved == index {
		t.Fatalf("the inserted match did not move row %d, so the test proves nothing", index)
	}
	if outlinerView.Cursor != moved {
		t.Fatalf("cursor = %d, want %d: the selected note must keep focus across the refresh",
			outlinerView.Cursor, moved)
	}
	if identity, onNote := outlinerView.CursorNoteIdentity(); !onNote || identity != selected {
		t.Fatalf("cursor note = %+v (note row: %v), want %+v", identity, onNote, selected)
	}

	// The selected note leaving the index falls back to a valid navigable row
	// rather than an out-of-range cursor, and the render still owns the window.
	scrollOffset := outlinerView.ViewScrollOffset
	controller.HandleNoteSearchIndexLoaded(shell, NoteSearchIndexLoadedMsg{generation: 2, rows: inserted[:1]})
	outlinerView.CrossProjectRebuildRows()
	if countNoteRows(outlinerView) != 1 {
		t.Fatalf("projection = %d note rows, want only the surviving match", countNoteRows(outlinerView))
	}
	if outlinerView.Cursor < 0 || outlinerView.Cursor >= len(outlinerView.Rows) ||
		!outlinerView.Rows[outlinerView.Cursor].IsNavigable() {
		t.Fatalf("cursor = %d over %d rows, want a valid navigable landing row",
			outlinerView.Cursor, len(outlinerView.Rows))
	}
	if outlinerView.ViewScrollOffset != scrollOffset {
		t.Fatalf("ViewScrollOffset = %d, want the rebuild to leave the window to the render",
			outlinerView.ViewScrollOffset)
	}
}

// An availability message lands after every committed task mutation, so the
// index reloads only when the owner catalog actually moved.
func TestNoteSearchIndexReloadsOnlyForAnOwnerCatalogChange(t *testing.T) {
	var controller Controller
	shell := &testShell{}
	if cmd := controller.RequestNoteSearchIndexLoad(shell); cmd == nil {
		t.Fatalf("the first load produced no command")
	}
	generation := controller.index.generation
	if cmd := controller.RefreshNoteSearchIndexOwners(shell); cmd != nil {
		t.Fatalf("an unchanged owner catalog requested a reload")
	}
	if controller.index.generation != generation {
		t.Fatalf("generation moved to %d without a reload", controller.index.generation)
	}

	controller.index.owners = "a different owner catalog"
	if cmd := controller.RefreshNoteSearchIndexOwners(shell); cmd == nil {
		t.Fatalf("a changed owner catalog requested no reload")
	}
	if controller.index.generation != generation+1 {
		t.Fatalf("generation = %d, want the reload to supersede the previous read", controller.index.generation)
	}
}

// noteIndexRow seeds one global index row. The shell's tests only ever own the
// index itself, so every row here is global; the project-owned rows and their
// labels belong to the tests beside the outliner, whose copy takes a label.
func noteIndexRow(location app.NoteLocation, title, filename string) notes.LocatedNoteRow {
	return notes.LocatedNoteRow{
		Note:       &models.Note{Title: title, Filename: filename},
		Location:   location,
		OwnerLabel: "Global",
	}
}

func noteRowIdentities(outlinerView *outliner.OutlinerModel) []notes.NoteRowIdentity {
	var identities []notes.NoteRowIdentity
	for _, row := range outlinerView.Rows {
		if row.IsNoteRow {
			identities = append(identities, row.NoteRow)
		}
	}
	return identities
}

func noteRowIndex(outlinerView *outliner.OutlinerModel, identity notes.NoteRowIdentity) (int, bool) {
	for index, row := range outlinerView.Rows {
		if row.IsNoteRow && row.NoteRow == identity {
			return index, true
		}
	}
	return 0, false
}

func countNoteRows(outlinerView *outliner.OutlinerModel) int {
	notes := 0
	for _, row := range outlinerView.Rows {
		if row.IsNoteRow {
			notes++
		}
	}
	return notes
}
