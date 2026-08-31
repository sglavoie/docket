package tui

import (
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// noteRowProjection builds the one situation the tier exists for: a sigil-free
// query that both a task and a note answer, so the ordering between the two
// kinds is observable rather than asserted about an empty list.
func noteRowProjection(t *testing.T) *OutlinerModel {
	t.Helper()
	task := primaryRowsTask("needle-task", models.StateBacklog)
	task.Title = "Needle in the task"
	other := primaryRowsTask("plain-task", models.StateBacklog)
	other.Title = "Unrelated work"
	project := primaryRowsProject("alpha", task, other)

	outliner := noteTierOutliner([]LocatedNoteRow{
		noteIndexRow(app.GlobalNoteLocation(), "", "Needle notes", "needle.md"),
		noteIndexRow(app.ProjectNoteLocation("alpha"), "alpha", "Needle again", "again.md"),
		noteIndexRow(app.GlobalNoteLocation(), "", "Nothing here", "quiet.md"),
	}, project)
	searchRankedList(outliner, "needle")
	return outliner
}

func noteRowIndexes(outliner *OutlinerModel) []int {
	indexes := make([]int, 0)
	for i, row := range outliner.rows {
		if row.IsNoteRow {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

// TestSearchProjectionSeatsNoteRowsBelowEveryTaskMatch pins the tier's rank: a
// task the query names is what the Outliner is for, so no note may outrank one
// however strongly it scored. The rows also carry the identity and the source
// the renderer reads, and the payload map is keyed by that same identity.
func TestSearchProjectionSeatsNoteRowsBelowEveryTaskMatch(t *testing.T) {
	outliner := noteRowProjection(t)

	noteIndexes := noteRowIndexes(outliner)
	if len(noteIndexes) != 2 {
		t.Fatalf("note rows = %d, want the two matching notes", len(noteIndexes))
	}
	for i, row := range outliner.rows {
		if row.isTaskRow() && i > noteIndexes[0] {
			t.Fatalf("task row at %d outranked the notes tier seated at %d", i, noteIndexes[0])
		}
	}
	if got := searchVisibleTaskIDs(outliner); !slices.Contains(got, "needle-task") {
		t.Fatalf("task matches = %v, want the note tier appended beside them", got)
	}

	for _, index := range noteIndexes {
		row := outliner.rows[index]
		if row.MatchSource != matchSourceNote {
			t.Fatalf("note row %d source = %v, want matchSourceNote", index, row.MatchSource)
		}
		if row.task != nil {
			t.Fatalf("note row %d carried a task payload", index)
		}
		if row.NoteRow.Filename == "" {
			t.Fatalf("note row %d carried no note identity", index)
		}
		if _, ok := outliner.searchNoteMatches[row.NoteRow]; !ok {
			t.Fatalf("note row %d identity %+v is absent from the retained payloads",
				index, row.NoteRow)
		}
	}

	// The rows are seated by score and tiebroken by sourceIndex, which is the
	// index's own location-then-filename order, so the same query lists the same
	// notes in the same order on every rebuild.
	for i := 1; i < len(noteIndexes); i++ {
		prev := outliner.searchNoteMatches[outliner.rows[noteIndexes[i-1]].NoteRow]
		curr := outliner.searchNoteMatches[outliner.rows[noteIndexes[i]].NoteRow]
		if prev.match.score < curr.match.score {
			t.Fatalf("note row scores ran uphill: %d before %d",
				prev.match.score, curr.match.score)
		}
		if prev.match.score == curr.match.score && prev.sourceIndex >= curr.sourceIndex {
			t.Fatalf("tied note rows ordered %d before %d, want the index's own order",
				prev.sourceIndex, curr.sourceIndex)
		}
	}
}

// noteRowWithOwner returns the projection's project-owned note row, which is
// the one whose render carries an owner label.
func noteRowWithOwner(t *testing.T, outliner *OutlinerModel) outlinerRow {
	t.Helper()
	for _, index := range noteRowIndexes(outliner) {
		row := outliner.rows[index]
		if outliner.searchNoteMatches[row.NoteRow].row.OwnerLabel != "" {
			return row
		}
	}
	t.Fatal("the projection emitted no project-owned note row")
	return outlinerRow{}
}

// TestNoteRowIsNavigableButNeverATaskRow is the row-kind contract session 15
// turns on: a note row is the first kind the cursor may rest on while every
// task-only guard must refuse it. The accessors below are the ones every entity
// operation resolves its target through, so a note-row cursor yielding nothing
// from all of them is what makes the whole registry refuse by row kind.
func TestNoteRowIsNavigableButNeverATaskRow(t *testing.T) {
	outliner := noteRowProjection(t)
	noteIndexes := noteRowIndexes(outliner)
	if len(noteIndexes) == 0 {
		t.Fatal("the projection emitted no note row to point at")
	}
	row := outliner.rows[noteIndexes[0]]
	if row.isTaskRow() {
		t.Fatal("a note row reported itself as a task row")
	}
	if !row.isNavigable() {
		t.Fatal("a note row is unreachable by the cursor")
	}
	if row.isHeaderRow() {
		t.Fatal("a note row reported itself as a foldable header")
	}

	outliner.cursor = noteIndexes[0]
	if task, project := outliner.getTaskRef(); task != nil || project != nil {
		t.Fatalf("getTaskRef on a note row = %v/%v, want nothing", task, project)
	}
	if task := outliner.getCursorTask(); task != nil {
		t.Fatalf("getCursorTask on a note row = %v, want nothing", task)
	}
	if identity := outliner.currentTaskIdentity(); identity.Valid() {
		t.Fatalf("currentTaskIdentity on a note row = %+v, want an invalid identity", identity)
	}
	if identity, ok := outliner.cursorTaskIdentity(); ok {
		t.Fatalf("cursorTaskIdentity on a note row = %+v, want a refusal", identity)
	}
}

// TestNoteRowRendersWithoutTaskFurniture holds the renderer to the session's
// rule: a note keeps the task row's rhythm and columns without borrowing a
// state badge, a priority, or a due column it has no value for.
func TestNoteRowRendersWithoutTaskFurniture(t *testing.T) {
	outliner := noteRowProjection(t)
	row := noteRowWithOwner(t, outliner)
	entry := outliner.searchNoteMatches[row.NoteRow]

	const rowWidth = 80
	line := outliner.renderOutlinerNoteRow(row, false, rowWidth, 0, nil)
	plain := ansi.Strip(line)
	for _, want := range []string{noteRowGlyph, entry.row.Note.Title,
		"[" + entry.row.OwnerLabel + "]", matchSourceLabels[matchSourceNote]} {
		if !strings.Contains(plain, want) {
			t.Fatalf("note row lost %q:\n%q", want, plain)
		}
	}
	for _, state := range []models.WorkflowState{
		models.StateBacklog, models.StateDoing, models.StateInbox,
	} {
		if badge := ansi.Strip(GetStateBadge(state)); strings.Contains(plain, strings.TrimSpace(badge)) {
			t.Fatalf("note row faked the %s state badge:\n%q", state, plain)
		}
	}

	// A cursor row must fill the width exactly, which is what keeps the
	// highlight from tearing where the notes block meets the task rows above it.
	cursorLine := outliner.renderOutlinerNoteRow(row, true, rowWidth, 0, nil)
	if got := lipgloss.Width(cursorLine); got != rowWidth {
		t.Fatalf("cursor note row width = %d, want exactly %d", got, rowWidth)
	}
	if got := lipgloss.Width(line); got > rowWidth {
		t.Fatalf("note row width = %d, want no more than %d", got, rowWidth)
	}
}

// TestClearSearchDropsTheRetainedNotePayloads keeps the widest thing the tier
// holds — every matched note's body — from outliving the rows that named it.
func TestClearSearchDropsTheRetainedNotePayloads(t *testing.T) {
	outliner := noteRowProjection(t)
	if len(outliner.searchNoteMatches) == 0 {
		t.Fatal("the projection retained no note payloads to drop")
	}
	outliner.ClearSearch()
	if outliner.searchNoteMatches != nil {
		t.Fatalf("cleared search retained %d note payloads", len(outliner.searchNoteMatches))
	}
	if indexes := noteRowIndexes(outliner); len(indexes) != 0 {
		t.Fatalf("cleared search left note rows at %v", indexes)
	}
}
