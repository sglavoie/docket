package outliner

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/notes"
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
	entry := outliner.Search.noteMatches[row.NoteRow]
	cue := "· " + matchSourceLabels[MatchSourceNote]
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
// on. Both surfaces admit notes through notes.ScoreNoteSearch, so the same query must
// admit the same notes with the same strength on both; a private scorer on
// either side would let one surface rank a note the other never showed.
func TestNoteTierAndNotesManagerShareOneScorer(t *testing.T) {
	rows := []notes.LocatedNoteRow{
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
	manager := &notes.NotesViewModel{Scope: route.AllNotesScope(), LoadedRows: rows, SearchApplied: query}
	manager.DeriveVisibleRows()
	if len(manager.SearchMatches) != len(tier) {
		t.Fatalf("manager admitted %d notes, tier admitted %d",
			len(manager.SearchMatches), len(tier))
	}
	for _, match := range tier {
		managed, ok := manager.SearchMatches[match.row.Identity()]
		if !ok {
			t.Fatalf("the tier admitted %+v and the manager did not", match.row.Identity())
		}
		if managed.Score != match.match.Score || managed.BodyMatched != match.match.BodyMatched {
			t.Fatalf("note %+v scored %+v on the tier and %+v in the manager",
				match.row.Identity(), match.match, managed)
		}
	}
}
