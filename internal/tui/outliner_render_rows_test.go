package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestSnoozeIndicatorSeparatesPendingFromWoken(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		snoozed time.Time
		want    string
	}{
		{"pending", now.AddDate(0, 0, 3), "zzz "},
		{"woken inside the window", now.Add(-30 * time.Hour), "woke "},
		{"woken past the window", now.Add(-snoozeWakeWindow - time.Hour), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{ID: "task", Title: "Task", SnoozedUntil: &tc.snoozed}
			plain := ansi.Strip(formatSnoozeIndicator(task, now))
			if tc.want == "" {
				if plain != "" {
					t.Fatalf("lapsed snooze past the wake window still rendered %q", plain)
				}
				return
			}
			if !strings.HasPrefix(plain, tc.want) {
				t.Fatalf("snooze indicator %q does not start with %q", plain, tc.want)
			}
		})
	}
}

func TestProjectHeaderPressureDegradesBeforeTruncation(t *testing.T) {
	const line = "▾ [project] (4)"
	row := outlinerRow{OverdueCount: 2, TodayCount: 3}

	cases := []struct {
		name     string
		rowWidth int
		want     string
	}{
		{"both counts fit", 60, "▾ [project] (4)  overdue 2 · today 3"},
		{"today drops first", 30, "▾ [project] (4)  overdue 2"},
		{"both drop", 20, "▾ [project] (4)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ansi.Strip(appendProjectHeaderPressure(line, row, tc.rowWidth))
			if got != tc.want {
				t.Fatalf("pressure suffix at width %d = %q, want %q", tc.rowWidth, got, tc.want)
			}
		})
	}

	quiet := ansi.Strip(appendProjectHeaderPressure(line, outlinerRow{}, 60))
	if quiet != line {
		t.Fatalf("a group with no pressure wore a suffix: %q", quiet)
	}
}

func TestCrossProjectTaskRowsOmitProjectBadge(t *testing.T) {
	alpha := primaryRowsProject("alpha", primaryRowsTask("alpha-task", models.StateBacklog))
	beta := primaryRowsProject("beta", primaryRowsTask("beta-task", models.StateBacklog))
	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.selectOutlinerMode(ModeAll)

	headerLabels := make(map[string]bool)
	for index, row := range outliner.rows {
		if row.IsProjectHeader {
			headerLabels[row.ProjectID] = strings.Contains(
				ansi.Strip(outliner.renderOutlinerProjectHeaderRow(row, false, 120, index)),
				"["+row.ProjectID+"]",
			)
			continue
		}
		if !row.isTaskRow() || row.task == nil {
			continue
		}

		line := ansi.Strip(outliner.renderOutlinerTaskRow(row, false, 120, 120, index, nil))
		if strings.Contains(line, "["+row.ProjectID+"]") {
			t.Errorf("task row %q repeated project badge in %q", row.task.ID, line)
		}
	}

	for _, projectID := range []string{alpha.ID, beta.ID} {
		if !headerLabels[projectID] {
			t.Errorf("project header %q did not retain its label", projectID)
		}
	}
}

func TestOutlinerTaskRowShowsCompactCommentAndAttachmentCounts(t *testing.T) {
	task := &models.Task{
		ID:            "task",
		Title:         "Review compact metadata",
		WorkflowState: models.StateBacklog,
		Comments:      []*models.Comment{{Text: "One"}, {Text: "Two"}},
		Attachments:   []models.Attachment{{Filename: "design.pdf"}},
	}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	row := outlinerRow{task: task, ProjectID: project.ID}

	plain := ansi.Strip(outliner.renderOutlinerTaskRow(row, false, 120, 120, 0, nil))
	for _, want := range []string{"▪ 2", "⊕ 1"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("task row missing %q: %q", want, plain)
		}
	}
}

func TestOutlinerTaskRowShowsSemanticTitleWithoutMarkerPrefix(t *testing.T) {
	task := &models.Task{
		ID:            "bug-task",
		Title:         "Fix capture grammar",
		WorkflowState: models.StateBacklog,
		Tags:          []string{"bug", "composer"},
	}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	row := outlinerRow{task: task, ProjectID: project.ID}

	plain := ansi.Strip(outliner.renderOutlinerTaskRow(row, false, 120, 120, 0, nil))
	for _, want := range []string{"Fix capture grammar", "#bug", "#composer"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("task row missing %q: %q", want, plain)
		}
	}
	if strings.Contains(plain, " B Fix capture grammar") {
		t.Fatalf("task row retained retired bug marker prefix: %q", plain)
	}
}

func TestOutlinerTaskRowShowsAncestorMarkerOnlyForPromotedRoot(t *testing.T) {
	task := &models.Task{ID: "child", Title: "Child", WorkflowState: models.StateBacklog}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())

	promoted := ansi.Strip(outliner.renderOutlinerTaskRow(outlinerRow{
		task: task, Breadcrumb: "Parent ›",
	}, false, 120, 120, 0, nil))
	if !strings.Contains(promoted, "Child  ‹ Parent") {
		t.Fatalf("promoted task row omitted ancestor marker: %q", promoted)
	}

	nested := ansi.Strip(outliner.renderOutlinerTaskRow(outlinerRow{
		task: task, prefix: connectorSpace + connectorLastItem,
	}, false, 120, 120, 0, nil))
	if strings.Contains(nested, "‹") {
		t.Fatalf("nested task row retained ancestor marker: %q", nested)
	}
}

func TestWaitingAgeIndicatorReadsTheStateClock(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	waiting := &models.Task{
		ID:             "blocked",
		Title:          "Blocked",
		WorkflowState:  models.StateWaiting,
		StateChangedAt: now.AddDate(0, 0, -12),
		UpdatedAt:      now,
	}
	if got := ansi.Strip(formatWaitingAgeIndicator(waiting, now)); got != "waiting 12d" {
		t.Fatalf("waiting age = %q, want %q; a comment must not reset it", got, "waiting 12d")
	}

	for _, state := range []models.WorkflowState{models.StateBacklog, models.StatePaused, models.StateDone} {
		other := &models.Task{
			ID:             "other",
			Title:          "Other",
			WorkflowState:  state,
			StateChangedAt: now.AddDate(0, 0, -12),
		}
		if got := formatWaitingAgeIndicator(other, now); got != "" {
			t.Fatalf("state %q wore a waiting age: %q", state, ansi.Strip(got))
		}
	}
}

func TestCheckBackIndicatorCountsTowardChasing(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		due  *time.Time
		want string
	}{
		{"no date to chase", nil, ""},
		{"overdue", timePtr(now.AddDate(0, 0, -5)), "check +5d"},
		{"today", timePtr(now.AddDate(0, 0, 0)), "check today"},
		{"upcoming", timePtr(now.AddDate(0, 0, 3)), "check 3d"},
		{"late in the day still counts as that day", timePtr(now.Add(11 * time.Hour)), "check today"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{
				ID:             "blocked",
				Title:          "Blocked",
				WorkflowState:  models.StateWaiting,
				StateChangedAt: now.AddDate(0, 0, -1),
				DueDate:        tc.due,
			}
			if got := ansi.Strip(formatCheckBackIndicator(task, now)); got != tc.want {
				t.Fatalf("check-back = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWaitingRowReplacesThePlainDueDateWithTheCheckBack(t *testing.T) {
	now := time.Now()
	due := now.AddDate(0, 0, 4)
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
	due := now.AddDate(0, 0, 4)
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

func timePtr(value time.Time) *time.Time {
	return &value
}
