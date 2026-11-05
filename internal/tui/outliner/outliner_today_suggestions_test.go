package outliner

import (
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

func suggestionTitles(rows []OutlinerModeRow) []string {
	titles := make([]string, 0, len(rows))
	for _, row := range rows {
		titles = append(titles, row.Task.Title)
	}
	return titles
}

// The lane offers open work by its strongest reason — Doing, owed attention, a
// deadline this week, Next, then priority 1 — and nothing else.
func TestTodaySuggestionsRankByReason(t *testing.T) {
	now := time.Now()
	soon := now.AddDate(0, 0, 3)
	later := now.AddDate(0, 0, 30)

	doing := primaryRowsTask("doing", models.StateDoing)
	flagged := primaryRowsTask("flagged", models.StateBacklog)
	flagged.Review = true
	due := primaryRowsTask("due-soon", models.StateBacklog)
	due.DueDate = &soon
	far := primaryRowsTask("due-later", models.StateBacklog)
	far.DueDate = &later
	next := primaryRowsTask("next", models.StateBacklog)
	urgent := primaryRowsTask("urgent", models.StateInbox)
	urgent.Priority = 1
	plain := primaryRowsTask("plain", models.StateBacklog)
	done := primaryRowsTask("done", models.StateDone)
	project := primaryRowsProject("alpha", plain, urgent, next, far, due, flagged, doing, done)

	input := OutlinerModeInput{
		Projects:  []*models.Project{project},
		NextOrder: []config.ProjectTaskLocator{{ProjectID: project.ID, TaskID: next.ID}},
		Now:       now,
	}
	got := suggestionTitles(BuildTodaySuggestions(input, nil, nil, todayPlanFloor))
	want := []string{"doing", "flagged", "due-soon", "next", "urgent"}
	if len(got) != len(want) {
		t.Fatalf("suggestions = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("suggestions = %v, want %v", got, want)
		}
	}
}

// What Today or Pinned already draws is never offered, and a day that holds its
// target draws no lane at all.
func TestTodaySuggestionsStepAsideAsTheDayFills(t *testing.T) {
	first := primaryRowsTask("first", models.StateDoing)
	second := primaryRowsTask("second", models.StateDoing)
	pinned := primaryRowsTask("pinned", models.StateDoing)
	project := primaryRowsProject("alpha", first, second, pinned)
	input := OutlinerModeInput{Projects: []*models.Project{project}, Now: time.Now()}

	planned := map[taskref.TaskIdentity]bool{taskref.For(first, project): true}
	pins := map[taskref.TaskIdentity]bool{taskref.For(pinned, project): true}
	if got := suggestionTitles(BuildTodaySuggestions(input, planned, pins, todayPlanFloor)); len(got) != 1 || got[0] != "second" {
		t.Fatalf("suggestions = %v, want only the unplanned, unpinned task", got)
	}
	planned[taskref.For(second, project)] = true
	planned[taskref.TaskIdentity{ProjectID: "alpha", TaskID: "elsewhere"}] = true
	if got := BuildTodaySuggestions(input, planned, pins, todayPlanFloor); len(got) != 0 {
		t.Fatalf("a full day still drew suggestions: %v", suggestionTitles(got))
	}
}

// The lane is a foldable computed section under Today's own rows, and only
// Today draws it.
func TestTodayModeDrawsTheSuggestedLane(t *testing.T) {
	project := primaryRowsProject("alpha", primaryRowsTask("doing", models.StateDoing))
	outliner, _ := primaryRowsOutliner(project)

	outliner.SelectOutlinerMode(chrome.ModeToday)
	header := -1
	for index, row := range outliner.Rows {
		if row.IsSectionHeader && row.CollapseKey == todaySuggestionsCollapseKey {
			header = index
		}
	}
	if header < 0 || outliner.Rows[header].SectionName != TodaySuggestionsSectionName {
		t.Fatalf("Today drew no Suggested lane: %+v", outliner.Rows)
	}
	if header+1 >= len(outliner.Rows) || outliner.Rows[header+1].Task == nil {
		t.Fatalf("the lane holds no task row: %+v", outliner.Rows)
	}

	outliner.SelectOutlinerMode(chrome.ModeAll)
	for _, row := range outliner.Rows {
		if row.CollapseKey == todaySuggestionsCollapseKey {
			t.Fatal("All drew Today's Suggested lane")
		}
	}
}

// visibleTaskIDsOutsideSuggestions is visibleTaskIDs without the Suggested
// lane's offers, for assertions about what Today itself holds.
func visibleTaskIDsOutsideSuggestions(m *OutlinerModel) []string {
	var ids []string
	for _, row := range m.Rows {
		if row.IsTaskRow() && row.Task != nil && row.Group.SectionKey != todaySuggestionsCollapseKey {
			ids = append(ids, row.Task.ID)
		}
	}
	return ids
}
