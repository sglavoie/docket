package outliner

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

// resumeSearchOutliner seeds one project holding a task whose only match for
// "callback" is its resume note, beside a task carrying nothing of the sort. The
// bland titles are the point: every assertion below is about the note.
func resumeSearchOutliner(seed ...func(*models.Task)) (*OutlinerModel, *models.Task) {
	noted := primaryRowsTask("noted", models.StateBacklog)
	noted.Title = "Untitled row"
	noted.ResumeNote = "stopped inside the callback handler"
	other := primaryRowsTask("other", models.StateBacklog)
	other.Title = "Unrelated row"
	other.Tags = []string{"backend"}
	for _, mutate := range seed {
		mutate(noted)
	}
	project := primaryRowsProject("alpha", noted, other)
	return scopeTestOutliner(project), noted
}

// TestResumeNoteSearchIsGatedBySigilsLikeEveryOtherField holds the note inside
// the AND gate rather than beside it: a sigil the task fails removes it however
// well the note scores, and a sigil it passes leaves the note-only hit standing.
func TestResumeNoteSearchIsGatedBySigilsLikeEveryOtherField(t *testing.T) {
	outliner, noted := resumeSearchOutliner(func(task *models.Task) {
		task.Tags = []string{"frontend"}
	})

	searchRankedList(outliner, "callback")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{noted.ID}) {
		t.Fatalf("ungated resume search = %v, want the noted task", got)
	}
	searchRankedList(outliner, "#frontend callback")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{noted.ID}) {
		t.Fatalf("passing gate = %v, want the noted task", got)
	}
	searchRankedList(outliner, "#backend callback")
	if got := searchVisibleTaskIDs(outliner); len(got) != 0 {
		t.Fatalf("failing gate = %v, want the note gated out", got)
	}
	searchRankedList(outliner, "!9 callback")
	if got := searchVisibleTaskIDs(outliner); len(got) != 0 {
		t.Fatalf("invalid gate = %v, want nothing admitted", got)
	}
}

// TestResumeNoteSearchRelaxesCaseWithEveryOtherField proves the note rides the
// one case reading rather than a rule of its own: a case-sensitive pass that
// finds nothing retries relaxed and reports the relaxation, and a query whose
// case the note already matches never relaxes.
func TestResumeNoteSearchRelaxesCaseWithEveryOtherField(t *testing.T) {
	outliner, noted := resumeSearchOutliner()

	searchRankedList(outliner, "Callback")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{noted.ID}) {
		t.Fatalf("relaxed resume search = %v, want the noted task", got)
	}
	if !outliner.Search.caseRelaxed {
		t.Fatal("the resume-only retry did not report a relaxed pass")
	}
	if outliner.Rows[0].MatchSource != MatchSourceResume {
		t.Fatalf("relaxed hit reported source %d, want resume", outliner.Rows[0].MatchSource)
	}

	searchRankedList(outliner, "callback")
	if outliner.Search.caseRelaxed {
		t.Fatal("a query the note already matched reported a relaxed pass")
	}

	// Relaxation widens case and nothing else: the gate still refuses the task.
	searchRankedList(outliner, "#backend Callback")
	if got := searchVisibleTaskIDs(outliner); len(got) != 0 {
		t.Fatalf("relaxed gated search = %v, want the gate still refusing", got)
	}
}

// TestResumeNoteSearchObeysTheTerminalPopulationRule keeps the note out of the
// one place a new field could quietly widen a population: a done task answers a
// note query only when the query opens the terminal population.
func TestResumeNoteSearchObeysTheTerminalPopulationRule(t *testing.T) {
	finished := primaryRowsTask("finished", models.StateDone)
	finished.Title = "Untitled row"
	finished.ResumeNote = "stopped inside the callback handler"
	outliner := scopeTestOutliner(primaryRowsProject("alpha", finished))

	searchRankedList(outliner, "callback")
	if got := searchVisibleTaskIDs(outliner); len(got) != 0 {
		t.Fatalf("plain resume search = %v, want the done task excluded", got)
	}
	searchRankedList(outliner, "$done callback")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{finished.ID}) {
		t.Fatalf("$done resume search = %v, want the done task revealed", got)
	}
}

// TestResumeNoteNarrowsThroughALensCarrierToo covers the other carrier of the
// same query. A lens holds its query on the canonical rebuild path, so the note
// has to answer there as well or a saved lens and a typed query would disagree.
func TestResumeNoteNarrowsThroughALensCarrierToo(t *testing.T) {
	outliner, noted := resumeSearchOutliner()
	outliner.Lens.Active = &activeLensFrame{LensID: "lens", Name: "Resume"}
	outliner.SetLensQuery("callback")
	outliner.Population.Invalidate()
	outliner.CrossProjectRebuildRows()

	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{noted.ID}) {
		t.Fatalf("lens-narrowed rows = %v, want the resume-note match", got)
	}
	row, ok := findPlanningTaskRow(outliner, noted.ID)
	if !ok {
		t.Fatal("the narrowed projection dropped the noted row")
	}
	if row.MatchSource != MatchSourceResume {
		t.Fatalf("lens-narrowed row reported source %d, want resume", row.MatchSource)
	}
}

// TestResumeNoteSearchAttributionFollowsEveryRebuild is the invalidation
// contract: attribution is recomputed with membership, so writing the term into
// the note, moving it to the description, clearing the note, and clearing the
// query each produce the reading the current workspace supports rather than a
// cue left over from the last one.
func TestResumeNoteSearchAttributionFollowsEveryRebuild(t *testing.T) {
	outliner, noted := resumeSearchOutliner(func(task *models.Task) { task.ResumeNote = "" })

	rebuild := func() {
		outliner.Population.Invalidate()
		outliner.CrossProjectRebuildRows()
	}
	cue := func() string {
		t.Helper()
		if len(outliner.Rows) == 0 || outliner.Rows[0].Task != noted {
			t.Fatalf("search rows = %v, want the noted task", searchVisibleTaskIDs(outliner))
		}
		return ansi.Strip(outliner.RenderOutlinerTaskRow(outliner.Rows[0], false, 120, 120, 0, nil))
	}

	// Nothing matches until the note carries the term.
	searchRankedList(outliner, "callback")
	if got := searchVisibleTaskIDs(outliner); len(got) != 0 {
		t.Fatalf("search before the note was written = %v, want nothing", got)
	}

	noted.ResumeNote = "stopped inside the callback handler"
	rebuild()
	if plain := cue(); !strings.Contains(plain, "· resume") {
		t.Fatalf("resume hit did not explain itself: %q", plain)
	}

	// The same term in the description outranks the note, so the cue moves.
	noted.Description = "the callback path"
	rebuild()
	if plain := cue(); !strings.Contains(plain, "· description") {
		t.Fatalf("description hit kept the resume cue: %q", plain)
	}

	// Clearing the note leaves the description hit alone, and clearing both
	// empties the result set rather than stranding a row.
	noted.ResumeNote = ""
	rebuild()
	if plain := cue(); !strings.Contains(plain, "· description") {
		t.Fatalf("cleared note disturbed the description hit: %q", plain)
	}
	noted.Description = ""
	rebuild()
	if got := searchVisibleTaskIDs(outliner); len(got) != 0 {
		t.Fatalf("search after clearing every match = %v, want nothing", got)
	}

	// A cleared query restores the mode's own rows, cue-free.
	noted.ResumeNote = "stopped inside the callback handler"
	rebuild()
	outliner.ClearSearch()
	for index, row := range outliner.Rows {
		if !row.IsTaskRow() || row.Task == nil {
			continue
		}
		if row.MatchSource != MatchSourceNone {
			t.Fatalf("restored row carried source %d, want none", row.MatchSource)
		}
		plain := ansi.Strip(outliner.RenderOutlinerTaskRow(row, false, 120, 120, index, nil))
		if strings.Contains(plain, "· resume") {
			t.Fatalf("restored row wore the resume cue: %q", plain)
		}
	}
}
