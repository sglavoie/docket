package outliner

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/fuzzy"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/notes"
	"github.com/sglavoie/docket/internal/tui/route"
)

// TestNoteRowDropsItsCueThenItsLabelAsWidthFalls holds the note row to the task
// row's drop rule: the two optional segments are dropped whole, in a fixed
// order, and never truncated. The cue goes first because it explains a result
// whose title has already become unreadable, and a cue cut mid-word would read
// as a different word entirely.
func TestNoteRowDropsItsCueThenItsLabelAsWidthFalls(t *testing.T) {
	outliner := noteRowProjection(t)
	row := noteRowWithOwner(t, outliner)
	cue := "· " + matchSourceLabels[MatchSourceNote]
	owner := "[alpha]"

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

// noteLabelProjection is a ranked result holding a task, a global note, and a
// project note whose body alone carries the query, so the label column, the
// global spelling, and the body excerpt are all observable in one list.
func noteLabelProjection(t *testing.T) *OutlinerModel {
	t.Helper()
	task := primaryRowsTask("needle-task", models.StateBacklog)
	task.Title = "Needle in the task"
	project := primaryRowsProject("alpha", task)
	body := noteIndexRow(app.ProjectNoteLocation("alpha"), "alpha", "Unrelated title", "body.md")
	body.Note.Body = "An opening paragraph that says nothing. Then the needle sits here."
	outliner := noteTierOutliner([]notes.LocatedNoteRow{
		noteIndexRow(app.GlobalNoteLocation(), "Global", "Needle notes", "needle.md"),
		body,
	}, project)
	searchRankedList(outliner, "needle")
	return outliner
}

func noteRowTitled(t *testing.T, outliner *OutlinerModel, title string) OutlinerRow {
	t.Helper()
	for _, index := range noteRowIndexes(outliner) {
		row := outliner.Rows[index]
		if outliner.Search.noteMatches[row.NoteRow].row.Note.Title == title {
			return row
		}
	}
	t.Fatalf("the projection emitted no note row titled %q", title)
	return OutlinerRow{}
}

// TestNoteRowLeadsWithTheRankedTaskLabel holds a note row to the label a ranked
// task row draws: the project's slug or a global marker, ahead of the title and
// padded into the same column, so a note title starts where a task title does.
func TestNoteRowLeadsWithTheRankedTaskLabel(t *testing.T) {
	outliner := noteLabelProjection(t)
	const rowWidth = 100
	layout := outliner.ComputeOutlinerRowLayout(outliner.Rows, rowWidth)
	if layout == nil || layout.ProjectWidth != lipgloss.Width("[global]") {
		t.Fatalf("layout = %+v, want the project column widened to the global label", layout)
	}

	var taskTitleColumn int
	for _, row := range outliner.Rows {
		if row.IsTaskRow() && row.Task != nil {
			plain := ansi.Strip(outliner.RenderOutlinerTaskRow(row, false, rowWidth, 0, layout))
			taskTitleColumn = lipgloss.Width(plain[:strings.Index(plain, row.Task.Title)])
		}
	}
	for title, label := range map[string]string{"Needle notes": "[global]", "Unrelated title": "[alpha]"} {
		plain := ansi.Strip(outliner.renderOutlinerNoteRow(noteRowTitled(t, outliner, title), false, rowWidth, 0, layout))
		labelAt, titleAt := strings.Index(plain, label), strings.Index(plain, title)
		if labelAt < 0 || titleAt < labelAt {
			t.Fatalf("note row does not lead %q with %q:\n%q", title, label, plain)
		}
		if got := lipgloss.Width(plain[:titleAt]); got != taskTitleColumn {
			t.Fatalf("note title %q starts at column %d, task titles at %d:\n%q", title, got, taskTitleColumn, plain)
		}
	}
}

// TestNotesOnlyResultStillMeasuresItsLabelColumn covers the result set whose
// every hit is a note: no task row exists to measure, yet the labels still have
// to share one column.
func TestNotesOnlyResultStillMeasuresItsLabelColumn(t *testing.T) {
	outliner := noteTierOutliner([]notes.LocatedNoteRow{
		noteIndexRow(app.GlobalNoteLocation(), "Global", "Needle notes", "needle.md"),
		noteIndexRow(app.ProjectNoteLocation("alpha"), "alpha", "Needle again", "again.md"),
	}, primaryRowsProject("alpha", primaryRowsTask("plain", models.StateBacklog)))
	searchRankedList(outliner, "needle")
	if got := len(noteRowIndexes(outliner)); got != 2 {
		t.Fatalf("note rows = %d, want both notes", got)
	}
	layout := outliner.ComputeOutlinerRowLayout(outliner.Rows, 100)
	if layout == nil || layout.ProjectWidth != lipgloss.Width("[global]") {
		t.Fatalf("notes-only layout = %+v, want the label column measured", layout)
	}
}

// TestNoteBodyHitShowsAnExcerptShedFirst pins the explanation a body hit owes:
// a muted excerpt around the match in the width the title leaves, dropped whole
// before the cue or the label when the row narrows, and never on a note whose
// body did not match.
func TestNoteBodyHitShowsAnExcerptShedFirst(t *testing.T) {
	outliner := noteLabelProjection(t)
	row := noteRowTitled(t, outliner, "Unrelated title")
	cue := "· " + matchSourceLabels[MatchSourceNote]

	plain := ansi.Strip(outliner.renderOutlinerNoteRow(row, false, 120, 0, nil))
	if !strings.Contains(plain, "needle sits here") {
		t.Fatalf("body hit drew no excerpt:\n%q", plain)
	}
	if strings.Index(plain, "needle sits") < strings.Index(plain, "Unrelated title") ||
		strings.Index(plain, "needle sits") > strings.Index(plain, cue) {
		t.Fatalf("excerpt is not between the title and the cue:\n%q", plain)
	}

	sawExcerptOnly := false
	for width := 120; width >= 20; width-- {
		plain := strings.TrimRight(ansi.Strip(outliner.renderOutlinerNoteRow(row, false, width, 0, nil)), " ")
		excerpt, _ := renderedNoteExcerpt(t, plain, "Unrelated title", cue, width)
		hasExcerpt := excerpt != ""
		if hasExcerpt && !strings.Contains(excerpt, "needle") {
			t.Fatalf("width %d drew an excerpt without its match: %q", width, excerpt)
		}
		if hasExcerpt && (!strings.Contains(plain, cue) || !strings.Contains(plain, "[alpha]")) {
			t.Fatalf("width %d kept the excerpt after dropping the cue or the label:\n%q", width, plain)
		}
		if hasExcerpt && !strings.Contains(plain, "Unrelated title") {
			t.Fatalf("width %d spent the title's columns on the excerpt:\n%q", width, plain)
		}
		if !hasExcerpt && strings.Contains(plain, cue) {
			sawExcerptOnly = true
		}
	}
	if !sawExcerptOnly {
		t.Fatal("no width dropped the excerpt while the cue survived")
	}

	quiet := noteRowTitled(t, outliner, "Needle notes")
	if entry := outliner.Search.noteMatches[quiet.NoteRow]; entry.match.BodyMatched {
		t.Fatal("fixture note without a body reported a body match")
	}
	if got := outliner.noteBodyExcerpt(outliner.Search.noteMatches[quiet.NoteRow], 120); got != "" {
		t.Fatalf("title-only hit drew an excerpt %q", got)
	}
}

// renderedNoteExcerpt reads the excerpt a rendered note row actually drew: the
// text between the end of its title — whole, or cut at the title's own ellipsis
// — and the cue, or the line's end when the cue was dropped. It also reports the
// width the row left that excerpt, measured off the same rendered line.
func renderedNoteExcerpt(t *testing.T, plain, title, cue string, rowWidth int) (string, int) {
	t.Helper()
	titleAt := strings.Index(plain, title[:3])
	if titleAt < 0 {
		t.Fatalf("row drew no title:\n%q", plain)
	}
	titleEnd := titleAt + len(title)
	if !strings.HasPrefix(plain[titleAt:], title) {
		cut := strings.Index(plain[titleAt:], "…")
		if cut < 0 {
			t.Fatalf("title is neither whole nor cut at an ellipsis:\n%q", plain)
		}
		titleEnd = titleAt + cut + len("…")
	}
	rest := plain[titleEnd:]
	cueWidth := 0
	if at := strings.Index(rest, cue); at >= 0 {
		rest = rest[:at]
		cueWidth = lipgloss.Width("  " + cue)
	}
	budget := rowWidth - lipgloss.Width(plain[:titleEnd]) - lipgloss.Width(noteExcerptSeparator) - cueWidth
	return strings.TrimSpace(rest), budget
}

// TestNoteBodyExcerptKeepsMatchWithinRowBudget holds a late body hit — the
// needle sits behind more than forty runes of unrelated prose — to the one
// promise an excerpt makes: whatever width the row leaves it, the drawn excerpt
// either carries the matched word or is absent. A right cut that keeps leading
// context and loses the match is the failure, and it is read off the rendered
// text rather than inferred from a missing keyword.
func TestNoteBodyExcerptKeepsMatchWithinRowBudget(t *testing.T) {
	outliner := noteLabelProjection(t)
	row := noteRowTitled(t, outliner, "Unrelated title")
	body := outliner.Search.noteMatches[row.NoteRow].row.Note.Body
	if at := strings.Index(body, "needle"); at < 40 {
		t.Fatalf("fixture needle sits at %d, want at least 40 nonmatching runes ahead of it", at)
	}
	const title, cue, label = "Unrelated title", "· note", "[alpha]"

	budgets := map[int]bool{}
	sawExcerpt, sawAbsent := false, false
	for width := 80; width >= 20; width-- {
		plain := strings.TrimRight(ansi.Strip(outliner.renderOutlinerNoteRow(row, false, width, 0, nil)), " ")
		if got := lipgloss.Width(outliner.renderOutlinerNoteRow(row, true, width, 0, nil)); got != width {
			t.Fatalf("cursor note row at width %d = %d columns", width, got)
		}
		excerpt, budget := renderedNoteExcerpt(t, plain, title, cue, width)
		budgets[budget] = true
		if excerpt == "" {
			sawAbsent = true
			continue
		}
		sawExcerpt = true
		if !strings.Contains(excerpt, "needle") {
			t.Fatalf("width %d (excerpt budget %d) drew an excerpt without the match: %q\n%q", width, budget, excerpt, plain)
		}
		if w := lipgloss.Width(excerpt); w > budget {
			t.Fatalf("width %d drew a %d-column excerpt in a %d-column budget: %q", width, w, budget, excerpt)
		}
		if !strings.Contains(plain, title) || !strings.Contains(plain, label) || !strings.Contains(plain, cue) {
			t.Fatalf("width %d kept the excerpt at the cost of the title, label, or cue:\n%q", width, plain)
		}
	}
	for budget := noteExcerptMinWidth; budget <= 40; budget++ {
		if !budgets[budget] {
			t.Fatalf("no row width left the excerpt a %d-column budget; saw %v", budget, budgets)
		}
	}
	if !sawExcerpt || !sawAbsent {
		t.Fatalf("the sweep never observed both rungs: excerpt=%v absent=%v", sawExcerpt, sawAbsent)
	}

	// The fitter itself, at each budget below the row's floor gate: the match or
	// nothing, never more columns than it was given.
	positions := []int{strings.Index(body, "needle")}
	for i := 1; i < len("needle"); i++ {
		positions = append(positions, positions[0]+i)
	}
	for width := 0; width <= 80; width++ {
		got := ansi.Strip(searchMatchExcerptWithin(body, positions, width))
		if lipgloss.Width(got) > width {
			t.Fatalf("budget %d drew %q", width, got)
		}
		if got != "" && !strings.Contains(got, "needle") {
			t.Fatalf("budget %d drew context without the match: %q", width, got)
		}
		if width >= lipgloss.Width("…needle…") && got == "" {
			t.Fatalf("budget %d could hold the match yet drew nothing", width)
		}
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
