package tui

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/util"
)

// This file covers what a whole row does when the width runs out: which signals
// survive, which are shed and in what order, that the title is never eaten
// before the ladder is exhausted, and that the assembled row never overruns the
// columns it was given. The indicators themselves are in
// outliner_render_indicators_test.go and the reserved segments in
// outliner_render_rows_test.go.

func TestBehavioralSignalsRowReconcilesWidthsAndOverflow(t *testing.T) {
	now := time.Now()
	due := util.StartOfDay(now.AddDate(0, 0, -4))
	title := "Investigate intermittent timeout when syncing the customer portal release archive"
	task := &models.Task{
		ID:             "combined-signals",
		Title:          title,
		Context:        "abcdefghijklmnop",
		Tags:           []string{"release"},
		WorkflowState:  models.StateDoing,
		StateChangedAt: now.AddDate(0, 0, -120),
		UpdatedAt:      now,
		DueDate:        &due,
		DeferCount:     3,
	}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	row := outlinerRow{task: task, ProjectID: project.ID, prefix: "│   ├── "}

	for _, width := range []int{60, 80} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			segments := outliner.buildTaskRowSegments(row, width, nil, cursorPaint{})
			indicators := ansi.Strip(segments.indicators)
			plain := ansi.Strip(outliner.renderOutlinerTaskRow(row, false, width, width, 0, nil))
			if got := lipgloss.Width(plain); got > width {
				t.Fatalf("%d-column row assembled %d columns: %q", width, got, plain)
			}
			for _, want := range []string{"[abcdefghijk…]", "doing 120d", ansi.Strip(query.FormatDueDate(task))} {
				if !strings.Contains(plain, want) {
					t.Fatalf("%d-column row missing %q: %q", width, want, plain)
				}
			}

			wantDeferCount := width == 80
			if got := strings.Contains(indicators, "deferred ×3"); got != wantDeferCount {
				t.Fatalf("%d-column defer count present = %v, want %v: %q", width, got, wantDeferCount, indicators)
			}
			if !strings.Contains(indicators, "doing 120d") || !strings.Contains(indicators, ansi.Strip(query.FormatDueDate(task))) {
				t.Fatalf("%d-column row dropped due or Doing before defer count: %q", width, indicators)
			}

			titleWidth := segments.titleWidth(width)
			visible := textfit.TruncateToVisualWidth(title, titleWidth)
			overflow := outliner.taskRowTitleOverflow(row, width, nil)
			if overflow == "" {
				t.Fatalf("%d-column row did not expose its clipped title tail", width)
			}
			head := strings.TrimSuffix(visible, "…")
			if got := head + overflow; got != title {
				t.Fatalf("%d-column visible head plus overflow = %q, want %q", width, got, title)
			}
		})
	}
}

func TestDoingRowRendersAgeAndDueDateAtSixtyColumns(t *testing.T) {
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
	row := outlinerRow{task: task, ProjectID: project.ID}

	segments := outliner.renderTaskIndicatorSegments(row)
	if got := ansi.Strip(segments.doing); got != "doing 9d" {
		t.Fatalf("rendered doing segment = %q, want %q", got, "doing 9d")
	}
	if segments.due == "" {
		t.Fatal("Doing task with a due date omitted its due segment")
	}

	plain := ansi.Strip(outliner.renderOutlinerTaskRow(row, false, 60, 60, 0, nil))
	for _, want := range []string{"Ship the migration safely", "doing 9d", ansi.Strip(query.FormatDueDate(task))} {
		if !strings.Contains(plain, want) {
			t.Fatalf("60-column Doing row missing %q: %q", want, plain)
		}
	}
	if got := lipgloss.Width(plain); got > 60 {
		t.Fatalf("60-column Doing row assembled %d columns: %q", got, plain)
	}
}

func TestWaitingRowReplacesThePlainDueDateWithTheCheckBack(t *testing.T) {
	now := time.Now()
	due := util.StartOfDay(now.AddDate(0, 0, 4))
	task := &models.Task{
		ID:             "blocked",
		Title:          "Blocked on vendor",
		WorkflowState:  models.StateWaiting,
		StateChangedAt: now.AddDate(0, 0, -6),
		DueDate:        &due,
	}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	row := outlinerRow{task: task, ProjectID: project.ID}

	plain := ansi.Strip(outliner.renderOutlinerTaskRow(row, false, 120, 120, 0, nil))
	for _, want := range []string{"waiting 6d", "check 4d"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("waiting row missing %q: %q", want, plain)
		}
	}
	if strings.Contains(plain, "⊡") {
		t.Fatalf("waiting row restated its due date beside the check-back: %q", plain)
	}
}

func TestNarrowRowShedsIndicatorsBeforeTheTitle(t *testing.T) {
	now := time.Now()
	due := util.StartOfDay(now.AddDate(0, 0, 4))
	task := &models.Task{
		ID:             "loaded",
		Title:          "Blocked on vendor",
		WorkflowState:  models.StateWaiting,
		StateChangedAt: now.AddDate(0, 0, -6),
		UpdatedAt:      now,
		DueDate:        &due,
		Comments:       []*models.Comment{{Text: "One"}, {Text: "Two"}},
		Attachments:    []models.Attachment{{Filename: "quote.pdf"}},
		Checklist:      []models.ChecklistItem{{Text: "Chase", Done: true}, {Text: "Escalate"}},
	}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	row := outlinerRow{task: task, ProjectID: project.ID}

	wide := ansi.Strip(outliner.renderOutlinerTaskRow(row, false, 120, 120, 0, nil))
	for _, want := range []string{"check 4d", "waiting 6d", "1/2", "▪ 2", "⊕ 1"} {
		if !strings.Contains(wide, want) {
			t.Fatalf("wide row missing %q: %q", want, wide)
		}
	}

	narrow := ansi.Strip(outliner.renderOutlinerTaskRow(row, false, 30, 30, 0, nil))
	if !strings.Contains(narrow, "Blocke") {
		t.Fatalf("narrow row let indicators eat the title: %q", narrow)
	}
	for _, shed := range []string{"1/2", "▪ 2", "⊕ 1"} {
		if strings.Contains(narrow, shed) {
			t.Fatalf("narrow row kept %q, which the ladder sheds first: %q", shed, narrow)
		}
	}
	if !strings.Contains(narrow, "check 4d") {
		t.Fatalf("narrow row shed the check-back before shallower signals: %q", narrow)
	}
}
