package outliner

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
)

// focusedHeaderProject seeds one project holding a row for every mode: an inbox
// task, a doing task, a backlog task, a due task, and a done one, so the seven
// modes differ in what they list rather than in whether they list anything.
func focusedHeaderProject(id string) *models.Project {
	due := primaryRowsTask(id+"-due", models.StateBacklog)
	dueDate := time.Now().AddDate(0, 0, 1)
	due.DueDate = &dueDate
	return primaryRowsProject(
		id,
		primaryRowsTask(id+"-inbox", models.StateInbox),
		primaryRowsTask(id+"-doing", models.StateDoing),
		primaryRowsTask(id+"-backlog", models.StateBacklog),
		due,
		primaryRowsTask(id+"-done", models.StateDone),
	)
}

// TestFocusedProjectHeaderLeadsEveryModeAndTheRankedList is the acceptance
// sweep: the header is a property of the projection being single-project, not of
// the strategy that built its rows. Every mode and the ranked list are checked
// through the same scoped model, because the four exits from
// buildCanonicalModeRows used to finish their own rows and only a sweep can tell
// a shared finalization from one branch that happens to be right.
func TestFocusedProjectHeaderLeadsEveryModeAndTheRankedList(t *testing.T) {
	alpha := focusedHeaderProject("alpha")
	beta := focusedHeaderProject("beta")
	outliner := scopeTestOutliner(alpha, beta)
	outliner.PushProjectScope(alpha.ID)

	for _, mode := range chrome.OutlinerModes {
		outliner.SelectOutlinerMode(mode.ID)
		assertSoleFocusedProjectHeader(t, outliner, alpha.ID, mode.ID)
	}

	outliner.SelectOutlinerMode(chrome.ModeAll)
	searchRankedList(outliner, "backlog")
	assertSoleFocusedProjectHeader(t, outliner, alpha.ID, "ranked list")
	if !outliner.HasContentRows() {
		t.Fatal("the ranked list matched nothing, so the sweep proves nothing about its header")
	}

	// A query the ranked list cannot satisfy is the empty case that still reaches
	// the header through the search branch rather than the mode tail.
	searchRankedList(outliner, "nothingmatchesthis")
	assertSoleFocusedProjectHeader(t, outliner, alpha.ID, "empty ranked list")
	if outliner.HasContentRows() {
		t.Fatal("a query matching nothing still reported content rows")
	}
	outliner.ClearSearch()
}

// TestFocusedProjectHeaderSurvivesAnEmptyProjection covers the half the header
// exists for: a project with nothing to list is still a project the user can
// act on, so the header is context printed above the ordinary explanation and
// never a search result competing with it.
func TestFocusedProjectHeaderSurvivesAnEmptyProjection(t *testing.T) {
	empty := primaryRowsProject("empty")
	other := focusedHeaderProject("other")
	outliner := scopeTestOutliner(empty, other)
	outliner.PushProjectScope(empty.ID)

	for _, mode := range chrome.OutlinerModes {
		outliner.SelectOutlinerMode(mode.ID)
		assertSoleFocusedProjectHeader(t, outliner, empty.ID, mode.ID)
		if outliner.HasContentRows() {
			t.Fatalf("%s over an empty project reported content rows", mode.ID)
		}
		if len(outliner.Rows) != 1 {
			t.Fatalf("%s over an empty project built %d rows, want the header alone", mode.ID, len(outliner.Rows))
		}
		// With nothing else to stand on the cursor takes the header, which is what
		// makes an empty project reachable at all.
		if outliner.Cursor != 0 || !outliner.CursorNamesFocusedProjectHeader() {
			t.Fatalf("%s left the cursor at %d, want it on the header", mode.ID, outliner.Cursor)
		}
	}
}

// TestFocusedProjectHeaderNeedsAResolvableOwner holds the two negative halves
// together: an unscoped workspace gets ordinary group headers even when one
// project happens to be all there is, and a scope whose project left the
// population emits nothing rather than a header over a project nobody can name.
func TestFocusedProjectHeaderNeedsAResolvableOwner(t *testing.T) {
	alpha := focusedHeaderProject("alpha")
	outliner := scopeTestOutliner(alpha)
	outliner.SelectOutlinerMode(chrome.ModeAll)
	if rows, _ := focusedProjectHeaders(outliner); len(rows) != 0 {
		t.Fatalf("an unscoped workspace over one project built %d focused headers", len(rows))
	}

	outliner.PushProjectScope(alpha.ID)
	assertSoleFocusedProjectHeader(t, outliner, alpha.ID, "scoped")

	outliner.Population.Projects = nil
	outliner.Population.Invalidate()
	outliner.CrossProjectRebuildRows()
	if outliner.FocusedProjectHeaderOwner() != nil {
		t.Fatal("a scope the population no longer holds still resolved an owner")
	}
	if rows, _ := focusedProjectHeaders(outliner); len(rows) != 0 {
		t.Fatalf("an unavailable scoped project fabricated %d headers", len(rows))
	}
}

// TestFocusedProjectHeaderKeepsTheCursorAcrossTheScopeRoundTrip is the
// navigation half of the contract. The header is inserted on the way in and
// gone on the way out, so a cursor captured in one scope must not be read as a
// row index in the other.
func TestFocusedProjectHeaderKeepsTheCursorAcrossTheScopeRoundTrip(t *testing.T) {
	alpha := focusedHeaderProject("alpha")
	beta := focusedHeaderProject("beta")
	outliner := scopeTestOutliner(alpha, beta)
	outliner.SelectOutlinerMode(chrome.ModeBacklog)

	base := 0
	for index, row := range outliner.Rows {
		if row.IsTaskRow() && row.ProjectID == beta.ID {
			base = index
			break
		}
	}
	if base == 0 {
		t.Fatal("the base scope listed no beta task to stand on")
	}
	outliner.Cursor = base
	baseIdentity := outliner.TaskIdentityForRow(outliner.Rows[base])

	outliner.PushProjectScope(alpha.ID)
	assertSoleFocusedProjectHeader(t, outliner, alpha.ID, "scoped")
	if outliner.Cursor == 0 {
		t.Fatal("focusing a project holding work parked the cursor on its header")
	}
	// The cursor names a row of the scoped projection, and a rebuild inside that
	// scope leaves it there: the header is present on both sides of the pass.
	scoped := outliner.Cursor
	scopedIdentity := outliner.TaskIdentityForRow(outliner.Rows[scoped])
	outliner.RebuildRows()
	if outliner.Cursor != scoped {
		t.Fatalf("a rebuild inside the scope moved the cursor from %d to %d", scoped, outliner.Cursor)
	}
	if got := outliner.TaskIdentityForRow(outliner.Rows[outliner.Cursor]); got != scopedIdentity {
		t.Fatalf("a rebuild inside the scope re-anchored the cursor to %+v, want %+v", got, scopedIdentity)
	}

	if !outliner.PopProjectScope() {
		t.Fatal("the focused scope did not pop")
	}
	if rows, _ := focusedProjectHeaders(outliner); len(rows) != 0 {
		t.Fatalf("leaving the scope left %d focused headers behind", len(rows))
	}
	if outliner.Cursor != base {
		t.Fatalf("the pop restored cursor %d, want the captured %d", outliner.Cursor, base)
	}
	if got := outliner.TaskIdentityForRow(outliner.Rows[outliner.Cursor]); got != baseIdentity {
		t.Fatalf("the pop landed on %+v, want the row it was pressed from %+v", got, baseIdentity)
	}
}
