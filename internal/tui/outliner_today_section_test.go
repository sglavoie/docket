package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestTodaySectionInAllIsSyntheticAndDeduplicated(t *testing.T) {
	project, tasks := planningOrderProject(
		t,
		t.TempDir(),
		"today-section",
		"today-section",
		"Open planned",
		"Completed planned",
		"Snoozed planned",
		"Unplanned",
	)
	tasks[0].ApplyWorkflowState(models.StateReady)
	tasks[1].ApplyWorkflowState(models.StateDone)
	tasks[2].ApplyWorkflowState(models.StateReady)
	tasks[3].ApplyWorkflowState(models.StateReady)
	future := time.Now().AddDate(0, 0, 2)
	tasks[2].SnoozedUntil = &future

	app, cross, _ := newPlanningOrderApp(t, project)
	seedDailyPlanOrder(t, app,
		locatorOf(project, tasks[0]),
		locatorOf(project, tasks[1]),
		locatorOf(project, tasks[2]),
	)
	enterCrossMode(cross, ModeAll)

	if len(cross.rows) == 0 {
		t.Fatal("All rendered no rows")
	}
	header := cross.rows[0]
	if !header.IsSectionHeader || !header.IsTodaySection || header.SectionName != todaySectionName {
		t.Fatalf("first row = %+v, want synthetic Today header", header)
	}
	if header.SectionCount != 2 {
		t.Fatalf("Today count = %d, want 2 visible explicit plan entries", header.SectionCount)
	}

	for _, tc := range []struct {
		task        *models.Task
		wantCount   int
		wantInToday bool
	}{
		{tasks[0], 1, true},
		{tasks[1], 1, true},
		{tasks[2], 0, false},
		{tasks[3], 1, false},
	} {
		if got := countTaskRows(cross, tc.task.ID); got != tc.wantCount {
			t.Errorf("%q row count = %d, want %d", tc.task.Title, got, tc.wantCount)
			continue
		}
		if tc.wantCount == 0 {
			continue
		}
		row, _ := findPlanningTaskRow(cross, tc.task.ID)
		if row.IsTodaySection != tc.wantInToday {
			t.Errorf("%q IsTodaySection = %v, want %v", tc.task.Title, row.IsTodaySection, tc.wantInToday)
		}
	}

	if got := len(app.runtime.Planning().GetDailyPlan()); got != 3 {
		t.Fatalf("hiding a snoozed entry changed Daily Plan membership: got %d, want 3", got)
	}
}

func TestTodaySectionIsHiddenWhenEmptyAndCollapsibleWhenPopulated(t *testing.T) {
	project, tasks := planningOrderProject(t, t.TempDir(), "today-fold", "today-fold", "Planned", "Unplanned")
	for _, task := range tasks {
		task.ApplyWorkflowState(models.StateReady)
	}
	app, cross, _ := newPlanningOrderApp(t, project)

	enterCrossMode(cross, ModeAll)
	if _, visible := todaySectionCount(cross); visible {
		t.Fatal("empty Today section should be hidden")
	}

	seedDailyPlanOrder(t, app, locatorOf(project, tasks[0]))
	enterCrossMode(cross, ModeAll)
	if countTaskRows(cross, tasks[0].ID) != 1 {
		t.Fatalf("planned task rows before collapse = %v", visibleTaskIDs(cross))
	}
	cross.cursor = 0
	if !cross.toggleSectionFoldAtCursor() {
		t.Fatal("Today header did not toggle")
	}
	if !cross.collapsedSections[todaySectionCollapseKey] {
		t.Fatal("Today collapse state was not stored under its synthetic key")
	}
	if countTaskRows(cross, tasks[0].ID) != 0 {
		t.Fatalf("planned task remained visible under collapsed Today: %v", visibleTaskIDs(cross))
	}
	if countTaskRows(cross, tasks[1].ID) != 1 {
		t.Fatalf("collapsing Today hid an unrelated workflow task: %v", visibleTaskIDs(cross))
	}
}

func TestOutlinerTitleDropsTodayProgressButKeepsOtherOpenLoops(t *testing.T) {
	m := newWorkModeOutliner(t)
	m.selectOutlinerMode(ModeAll)
	m.openLoops = OpenLoops{
		Inbox:      3,
		TodayDone:  1,
		TodayTotal: 4,
		Overdue:    2,
		HasTasks:   true,
	}

	rendered := ansi.Strip(m.renderOutlinerTreeContent(160))
	if strings.Contains(strings.ToLower(rendered), "today:") ||
		strings.Contains(strings.ToLower(rendered), "today 1/4") ||
		strings.Contains(rendered, "→ next:") {
		t.Fatalf("Outliner retained verbose Today chrome: %q", rendered)
	}
	for _, want := range []string{"inbox 3", "overdue 2"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("Outliner title omitted retained counter %q: %q", want, rendered)
		}
	}
}
