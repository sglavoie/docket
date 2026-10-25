package outliner

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/notes"
)

func TestProjectNotesSectionScopeOrderAndNavigation(t *testing.T) {
	for _, direct := range []bool{false, true} {
		t.Run(map[bool]string{false: "scoped", true: "direct"}[direct], func(t *testing.T) {
			project := primaryRowsProject("alpha", primaryRowsTask("task", models.StateBacklog))
			index := []notes.LocatedNoteRow{
				noteIndexRow(app.GlobalNoteLocation(), "Global", "Global note", "global.md"),
				noteIndexRow(app.ProjectNoteLocation("beta"), "beta", "Other note", "other.md"),
				noteIndexRow(app.ProjectNoteLocation("alpha"), "alpha", "Older note", "older.md"),
				noteIndexRow(app.ProjectNoteLocation("alpha"), "alpha", "Newest note", "newest.md"),
			}
			index[3].Note.ModifiedAt = time.Now().Add(time.Hour)
			out := noteTierOutliner(index, project)
			if direct {
				out.Project = project
			} else {
				out.PushProjectScope(project.ID)
			}
			out.RebuildRows()
			indexes := noteRowIndexes(out)
			if len(indexes) != 2 {
				t.Fatalf("note indexes = %v, want only alpha's two notes", indexes)
			}
			header := indexes[0] - 1
			if row := out.Rows[header]; !row.IsSectionHeader || row.SectionName != "Notes" || row.SectionCount != 2 {
				t.Fatalf("missing Notes section: %+v", row)
			}
			for i, row := range out.Rows {
				if row.IsTaskRow() && i > header {
					t.Fatal("a task appeared inside the Notes section")
				}
			}
			if got := out.Rows[indexes[0]].NoteRow.Filename; got != "newest.md" {
				t.Fatalf("first note = %s, want newest", got)
			}
			for _, i := range indexes {
				if out.Rows[i].MatchSource != MatchSourceNone {
					t.Fatal("a browsing note was labeled as a search match")
				}
				out.Cursor = i
				if _, ok := out.CursorNoteIdentity(); !ok {
					t.Fatal("note cannot open through the existing note route")
				}
				if task, owner := out.GetTaskRef(); task != nil || owner != nil {
					t.Fatal("note acquired a task action target")
				}
				plain := ansi.Strip(out.renderOutlinerNoteRow(out.Rows[i], false, 80, i, nil))
				if !strings.Contains(plain, out.Search.noteMatches[out.Rows[i].NoteRow].row.Note.Title) {
					t.Fatalf("note title missing: %q", plain)
				}
			}
			out.Cursor = header
			if !out.CollapseOrMoveToParent() || len(noteRowIndexes(out)) != 0 {
				t.Fatal("Notes section did not fold")
			}
			if !out.ExpandAtCursor() || len(noteRowIndexes(out)) != 2 {
				t.Fatal("Notes section did not unfold")
			}
			if !out.ZoomSectionAtCursor() || len(searchVisibleTaskIDs(out)) != 0 || len(noteRowIndexes(out)) != 2 {
				t.Fatal("Notes section zoom did not isolate notes")
			}
		})
	}
}

func TestProjectNotesWithoutTasksAndAfterSearch(t *testing.T) {
	project := primaryRowsProject("alpha")
	index := []notes.LocatedNoteRow{noteIndexRow(app.ProjectNoteLocation("alpha"), "alpha", "Runbook", "runbook.md")}
	out := noteTierOutliner(index, project)
	out.RebuildRows()
	if len(noteRowIndexes(out)) != 0 {
		t.Fatal("unscoped browsing acquired project notes")
	}
	out.PushProjectScope(project.ID)
	if len(noteRowIndexes(out)) != 1 {
		t.Fatal("a note-only project hid its notes")
	}
	searchRankedList(out, "unmatched")
	if len(noteRowIndexes(out)) != 0 {
		t.Fatal("unmatched browsing notes leaked into search")
	}
	out.ClearSearch()
	if len(noteRowIndexes(out)) != 1 {
		t.Fatal("clearing search did not restore the Notes section")
	}
	out.Filter.Pinned = true
	adoptFilterCarriers(out)
	out.RebuildRows()
	if len(noteRowIndexes(out)) != 0 {
		t.Fatal("notes appeared as matches for a task filter")
	}
	out.Filter.Pinned = false
	adoptFilterCarriers(out)
	project.Archived = true
	out.RebuildRows()
	if len(noteRowIndexes(out)) != 0 {
		t.Fatal("archived project notes are visible")
	}
}

func TestProjectNotesRefreshPreservesCursor(t *testing.T) {
	project := primaryRowsProject("alpha")
	index := []notes.LocatedNoteRow{noteIndexRow(app.ProjectNoteLocation("alpha"), "alpha", "Runbook", "runbook.md")}
	out := noteTierOutliner(nil, project)
	out.SetNoteSearchIndex(func() []notes.LocatedNoteRow { return index })
	out.PushProjectScope(project.ID)
	out.Cursor = noteRowIndexes(out)[0]
	identity, _ := out.CursorNoteIdentity()
	out.Search.CaptureNoteCursor(identity)
	newer := noteIndexRow(app.ProjectNoteLocation("alpha"), "alpha", "New Note", "new.md")
	newer.Note.ModifiedAt = time.Now().Add(time.Hour)
	index = append(index, newer)
	out.RebuildRows()
	if got, ok := out.CursorNoteIdentity(); !ok || got != identity {
		t.Fatalf("refresh moved cursor from %v to %v", identity, got)
	}
	index = nil
	out.RebuildRows()
	for _, row := range out.Rows {
		if row.IsNoteRow || row.CollapseKey == projectNotesSectionKey {
			t.Fatal("removing the last note left a stale Notes section")
		}
	}
}
