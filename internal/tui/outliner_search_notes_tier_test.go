package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
)

// TestNoteRowDropsItsCueThenItsOwnerAsWidthFalls holds the note row to the task
// row's drop rule: the two optional segments are dropped whole, in a fixed
// order, and never truncated. The cue goes first because it explains a result
// whose title has already become unreadable, and a cue cut mid-word would read
// as a different word entirely.
func TestNoteRowDropsItsCueThenItsOwnerAsWidthFalls(t *testing.T) {
	outliner := noteRowProjection(t)
	row := noteRowWithOwner(t, outliner)
	entry := outliner.searchNoteMatches[row.NoteRow]
	cue := "· " + matchSourceLabels[matchSourceNote]
	owner := "[" + entry.row.OwnerLabel + "]"

	// Rendered at each width from the widest a row is ever asked for down to the
	// point where nothing optional survives, so the ladder is observed as a
	// ladder rather than at three hand-picked widths.
	sawBoth, sawOwnerOnly, sawNeither := 0, 0, 0
	lastCue, lastOwner := true, true
	for width := 120; width >= 8; width-- {
		plain := ansi.Strip(outliner.renderOutlinerNoteRow(row, false, width, 0, nil))
		hasCue := strings.Contains(plain, cue)
		hasOwner := strings.Contains(plain, owner)
		if hasCue && !lastCue {
			t.Fatalf("width %d brought the cue back after it was dropped:\n%q", width, plain)
		}
		if hasOwner && !lastOwner {
			t.Fatalf("width %d brought the owner back after it was dropped:\n%q", width, plain)
		}
		if hasCue && !hasOwner {
			t.Fatalf("width %d dropped the owner before the cue:\n%q", width, plain)
		}
		// A partial segment is the failure the whole ladder exists to prevent:
		// "· not" is a word the row never meant to print.
		if !hasCue && strings.Contains(plain, "·") {
			t.Fatalf("width %d printed a fragment of the dropped cue:\n%q", width, plain)
		}
		if !hasOwner && strings.Contains(plain, "[") {
			t.Fatalf("width %d printed a fragment of the dropped owner:\n%q", width, plain)
		}
		if rendered := lipgloss.Width(outliner.renderOutlinerNoteRow(row, true, width, 0, nil)); rendered != width {
			t.Fatalf("cursor note row at width %d = %d columns", width, rendered)
		}
		switch {
		case hasCue && hasOwner:
			sawBoth++
		case hasOwner:
			sawOwnerOnly++
		default:
			sawNeither++
		}
		lastCue, lastOwner = hasCue, hasOwner
	}
	if sawBoth == 0 || sawOwnerOnly == 0 || sawNeither == 0 {
		t.Fatalf("the ladder has no three rungs: both=%d owner-only=%d neither=%d",
			sawBoth, sawOwnerOnly, sawNeither)
	}
}

// TestNoteTierAndNotesManagerShareOneScorer pins the reuse the tier is built
// on. Both surfaces admit notes through scoreNoteSearch, so the same query must
// admit the same notes with the same strength on both; a private scorer on
// either side would let one surface rank a note the other never showed.
func TestNoteTierAndNotesManagerShareOneScorer(t *testing.T) {
	rows := []LocatedNoteRow{
		{
			Note:     &models.Note{Title: "Needle notes", Filename: "needle.md", Body: "the needle is in the title"},
			Location: app.GlobalNoteLocation(),
		},
		{
			Note:       &models.Note{Title: "Unrelated", Filename: "body.md", Body: "buried needle in the body"},
			Location:   app.ProjectNoteLocation("alpha"),
			OwnerLabel: "alpha",
		},
		{
			Note:     &models.Note{Title: "Quiet", Filename: "quiet.md", Body: "nothing to find here"},
			Location: app.GlobalNoteLocation(),
		},
	}

	const query = "needle"
	outliner := noteTierOutliner(rows)
	tier := outliner.scoreNoteSearchTier(query, fuzzy.SmartCase(query), true)
	if len(tier) == 0 {
		t.Fatal("the tier admitted no note to compare against")
	}

	// The manager derives its own visible rows from the same snapshot. Only the
	// fields deriveVisibleRows reads are set: the scorer is what is under test,
	// not the file read that ordinarily fills loadedRows.
	manager := &NotesViewModel{scope: route.AllNotesScope(), loadedRows: rows, searchApplied: query}
	manager.deriveVisibleRows()
	if len(manager.searchMatches) != len(tier) {
		t.Fatalf("manager admitted %d notes, tier admitted %d",
			len(manager.searchMatches), len(tier))
	}
	for _, match := range tier {
		managed, ok := manager.searchMatches[match.row.Identity()]
		if !ok {
			t.Fatalf("the tier admitted %+v and the manager did not", match.row.Identity())
		}
		if managed.score != match.match.score || managed.bodyMatched != match.match.bodyMatched {
			t.Fatalf("note %+v scored %+v on the tier and %+v in the manager",
				match.row.Identity(), match.match, managed)
		}
	}
}

// noteTierFixtureApp searches the rich fixture for a query both a task and a
// note answer and parks the cursor on the first note row, which is the only
// cursor Open Note is applicable over.
func noteTierFixtureApp(t *testing.T) (*AppModel, NoteRowIdentity) {
	t.Helper()
	app := headerTargetAuditApp(t)
	outliner := app.crossOutliner
	if outliner == nil {
		t.Fatal("fixture app has no Outliner")
	}
	searchRankedList(outliner, "launch")
	for index, row := range outliner.rows {
		if !row.IsNoteRow {
			continue
		}
		outliner.Cursor = index
		return app, row.NoteRow
	}
	t.Fatal("the fixture search returned no note row")
	return nil, NoteRowIdentity{}
}

// TestOpenNoteIsTheFourthEnterActOnANoteRow drives the descriptor the way the
// shell does: a note-row cursor resolves Enter to Open Note rather than to one
// of the three acts that read an entity, and dispatch answers with the message
// that both opens the manager and positions it on the row the cursor named.
func TestOpenNoteIsTheFourthEnterActOnANoteRow(t *testing.T) {
	model, identity := noteTierFixtureApp(t)
	ctx := model.operationCommandContext()
	ctx.View = presentation.ViewCrossProjectOutliner
	if !ctx.Entity.HasNoteCursor {
		t.Fatal("a note-row cursor did not report HasNoteCursor")
	}

	registry := model.GetOperationRegistry()
	binding, _, ok := registry.InputBindingForKey(operation.InputStrategySimplified, "enter", ctx)
	if !ok || binding.OperationID != operation.OperationCoreOpenNote {
		t.Fatalf("note row Enter = %s available=%t, want %s",
			binding.OperationID, ok, operation.OperationCoreOpenNote)
	}
	// The three acts that read an entity must all refuse, or the shared key
	// would resolve by declaration order rather than by cursor.
	for _, id := range []operation.OperationID{
		operation.OperationCoreFocusSubtree, operation.OperationCoreZoomSection, operation.OperationCoreActivateGroup,
	} {
		descriptor, found := registry.Descriptor(id)
		if !found {
			t.Fatalf("operation %s is not registered", id)
		}
		if descriptor.Applicability != nil && descriptor.Applicability(ctx) {
			t.Fatalf("%s stayed applicable over a note row", id)
		}
	}

	descriptor, found := registry.Descriptor(operation.OperationCoreOpenNote)
	if !found {
		t.Fatal("Open Note is not registered")
	}
	result := model.executeCoreInputOperation(operation.OperationInvocation{Descriptor: descriptor, Context: ctx})
	if result.Err != nil {
		t.Fatalf("Open Note refused a note row: %v", result.Err)
	}
	message, isShow := result.Message.(ShowNotesManagerMsg)
	if !isShow {
		t.Fatalf("Open Note emitted %T, want ShowNotesManagerMsg", result.Message)
	}
	if message.Scope != route.AllNotesScope() {
		t.Fatalf("Open Note opened scope %+v, want every note reachable", message.Scope)
	}
	if message.Focus != identity {
		t.Fatalf("Open Note focused %+v, want the row the cursor named %+v",
			message.Focus, identity)
	}

	// Off a note row the act is gone entirely, which is what keeps the footer
	// from advertising it over the ordinary projection.
	model.crossOutliner.ClearSearch()
	plain := model.operationCommandContext()
	plain.View = presentation.ViewCrossProjectOutliner
	if plain.Entity.HasNoteCursor {
		t.Fatal("a cleared search still reported a note cursor")
	}
	if descriptor.Applicability(plain) {
		t.Fatal("Open Note stayed applicable with no note row in sight")
	}
}
