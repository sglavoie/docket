package outliner

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/util"
)

// This file covers what a whole row does when the width runs out: which signals
// survive, which are shed and in what order, that the title is never eaten
// before the ladder is exhausted, and that the assembled row never overruns the
// columns it was given. The indicators themselves are in
// outliner_render_indicators_test.go and the reserved segments in
// outliner_render_rows_test.go.

// TestLoadedRowSpendsItsWidthOnTheTitle is the row simplification stated as an
// outcome rather than as a list of deletions. A task carrying every signal the
// row used to draw — context, tags, a long Doing age, repeated deferrals, a
// checklist, comments, attachments — shows its whole title at 60 columns
// instead, because none of those reach the row any more.
func TestLoadedRowSpendsItsWidthOnTheTitle(t *testing.T) {
	now := time.Now()
	due := util.StartOfDay(now.AddDate(0, 0, -4))
	title := "Investigate the sync timeout"
	task := &models.Task{
		ID:             "combined-signals",
		Title:          title,
		Context:        "abcdefghijklmnop",
		Tags:           []string{"release", "bug"},
		WorkflowState:  models.StateDoing,
		StateChangedAt: now.AddDate(0, 0, -120),
		UpdatedAt:      now,
		DueDate:        &due,
		DeferCount:     3,
		Comments:       []*models.Comment{{Text: "One"}, {Text: "Two"}},
		Attachments:    []models.Attachment{{Filename: "quote.pdf"}},
		Checklist:      []models.ChecklistItem{{Text: "Chase", Done: true}, {Text: "Escalate"}},
	}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	row := OutlinerRow{Task: task, ProjectID: project.ID, Prefix: "│   ├── "}

	for _, width := range []int{60, 80} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			plain := ansi.Strip(outliner.RenderOutlinerTaskRow(row, false, width, 0, nil))
			if got := lipgloss.Width(plain); got > width {
				t.Fatalf("%d-column row assembled %d columns: %q", width, got, plain)
			}
			if !strings.Contains(plain, title) {
				t.Fatalf("%d-column row clipped a title the simplified budget fits: %q", width, plain)
			}
			if !strings.Contains(plain, ansi.Strip(query.FormatDueDate(task))) {
				t.Fatalf("%d-column row dropped the deadline: %q", width, plain)
			}
			for _, gone := range []string{"abcdefghijk", "release", "doing 120d", "deferred ×3", "1/2", "▪ 2", "⊕ 1"} {
				if strings.Contains(plain, gone) {
					t.Fatalf("%d-column row still carries %q: %q", width, gone, plain)
				}
			}

			// Every one of them is still a keypress away. Peek is the whole document,
			// which is what makes dropping them from the row a relocation.
			peek := ansi.Strip(strings.Join(peekDocumentBody(outliner, row, width), "\n"))
			for _, want := range []string{"abcdefghijklmnop", "release", "doing 120d", "×3", "1/2 done", "quote.pdf", "2"} {
				if !strings.Contains(peek, want) {
					t.Fatalf("%d-column Peek lost %q:\n%s", width, want, peek)
				}
			}
		})
	}
}

// TestLongTitleStillClipsToTheRowAndRecoversInPeek keeps the one rule the wider
// budget does not repeal: a title longer than any budget is still clipped on the
// row, and the clipped tail is recovered by reading Peek rather than by widening
// the terminal.
func TestLongTitleStillClipsToTheRowAndRecoversInPeek(t *testing.T) {
	now := time.Now()
	title := "Investigate intermittent timeout when syncing the customer portal release archive"
	task := &models.Task{
		ID:             "long-title",
		Title:          title,
		WorkflowState:  models.StateBacklog,
		StateChangedAt: now.AddDate(0, 0, -3),
		UpdatedAt:      now,
	}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	row := OutlinerRow{Task: task, ProjectID: project.ID, Prefix: "│   ├── "}

	plain := ansi.Strip(outliner.RenderOutlinerTaskRow(row, false, 60, 0, nil))
	if got := lipgloss.Width(plain); got > 60 {
		t.Fatalf("60-column row assembled %d columns: %q", got, plain)
	}
	if !strings.Contains(plain, "…") {
		t.Fatalf("60-column row did not clip an 80-column title: %q", plain)
	}
	peek := ansi.Strip(strings.Join(peekDocumentBody(outliner, row, 60), "\n"))
	if got := strings.Join(strings.Fields(peek), " "); !strings.HasPrefix(got, title) {
		t.Fatalf("60-column Peek did not name the whole title:\n%s", peek)
	}
}

func TestDoingRowRendersTitleAndDueDateAtSixtyColumns(t *testing.T) {
	now := time.Now()
	due := util.StartOfDay(now.AddDate(0, 0, 4))
	task := &models.Task{
		ID:             "active",
		Title:          "Ship the migration safely",
		WorkflowState:  models.StateDoing,
		StateChangedAt: now.AddDate(0, 0, -9),
		UpdatedAt:      now,
		DueDate:        &due,
	}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	row := OutlinerRow{Task: task, ProjectID: project.ID}

	segments := outliner.renderTaskIndicatorSegments(row, time.Now())
	if segments.due == "" {
		t.Fatal("Doing task with a due date omitted its due segment")
	}

	plain := ansi.Strip(outliner.RenderOutlinerTaskRow(row, false, 60, 0, nil))
	for _, want := range []string{"Ship the migration safely", ansi.Strip(query.FormatDueDate(task))} {
		if !strings.Contains(plain, want) {
			t.Fatalf("60-column Doing row missing %q: %q", want, plain)
		}
	}
	if strings.Contains(plain, "doing 9d") {
		t.Fatalf("60-column Doing row wore an age the state badge already implies: %q", plain)
	}
	if got := lipgloss.Width(plain); got > 60 {
		t.Fatalf("60-column Doing row assembled %d columns: %q", got, plain)
	}
}

func TestWaitingRowKeepsDeadlineIndependentFromCheckIn(t *testing.T) {
	now := time.Now()
	due := util.StartOfDay(now.AddDate(0, 0, 4))
	checkIn := util.StartOfDay(now.AddDate(0, 0, -2))
	task := &models.Task{
		ID:             "blocked",
		Title:          "Blocked on vendor",
		WorkflowState:  models.StateWaiting,
		StateChangedAt: now.AddDate(0, 0, -6),
		DueDate:        &due,
		NextCheckIn:    &checkIn,
	}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	row := OutlinerRow{Task: task, ProjectID: project.ID}

	plain := ansi.Strip(outliner.RenderOutlinerTaskRow(row, false, 120, 0, nil))
	for _, want := range []string{"review +2d", ansi.Strip(query.FormatDueDate(task))} {
		if !strings.Contains(plain, want) {
			t.Fatalf("waiting row missing %q: %q", want, plain)
		}
	}
	if strings.Contains(plain, "waiting 6d") {
		t.Fatalf("waiting row wore an age Peek now carries: %q", plain)
	}
}

func TestNarrowRowShedsIndicatorsBeforeTheTitle(t *testing.T) {
	now := time.Now()
	due := util.StartOfDay(now.AddDate(0, 0, 4))
	checkIn := util.StartOfDay(now.AddDate(0, 0, -2))
	task := &models.Task{
		ID:             "loaded",
		Title:          "Blocked on vendor",
		WorkflowState:  models.StateWaiting,
		StateChangedAt: now.AddDate(0, 0, -6),
		UpdatedAt:      now,
		DueDate:        &due,
		NextCheckIn:    &checkIn,
		Dependencies:   []models.TaskReference{{ProjectID: "project", TaskID: "missing"}},
	}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	row := OutlinerRow{Task: task, ProjectID: project.ID}

	wide := ansi.Strip(outliner.RenderOutlinerTaskRow(row, false, 120, 0, nil))
	for _, want := range []string{"review +2d", ansi.Strip(query.FormatDueDate(task)), "~"} {
		if !strings.Contains(wide, want) {
			t.Fatalf("wide row missing %q: %q", want, wide)
		}
	}

	narrow := ansi.Strip(outliner.RenderOutlinerTaskRow(row, false, 30, 0, nil))
	if !strings.Contains(narrow, "Blocke") {
		t.Fatalf("narrow row let indicators eat the title: %q", narrow)
	}
	if strings.Contains(narrow, "~") {
		t.Fatalf("narrow row kept the reference census, which the ladder sheds first: %q", narrow)
	}
	if !strings.Contains(narrow, ansi.Strip(query.FormatDueDate(task))) {
		t.Fatalf("narrow row shed the deadline before shallower signals: %q", narrow)
	}
}
