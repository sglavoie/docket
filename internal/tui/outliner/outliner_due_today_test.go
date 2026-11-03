package outliner

import (
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

func TestTodayIncludesDueTasksWithoutChangingPlanning(t *testing.T) {
	now := time.Now()
	planned := primaryRowsTask("planned", models.StateBacklog)
	due := primaryRowsTask("due", models.StateBacklog)
	due.DueDate = &now
	overdue := primaryRowsTask("overdue", models.StateBacklog)
	yesterday := now.AddDate(0, 0, -1)
	overdue.DueDate = &yesterday
	next := primaryRowsTask("next-due", models.StateWaiting)
	next.DueDate = &yesterday
	next.Review = true
	child := primaryRowsTask("undated-child", models.StateBacklog)
	child.ParentID = &due.ID
	hidden := primaryRowsTask("hidden-due", models.StateBacklog)
	hidden.DueDate, hidden.Hidden = &yesterday, true
	project := primaryRowsProject("alpha", planned, due, overdue, next, child, hidden)
	otherDue := primaryRowsTask(due.ID, models.StateDoing)
	otherDue.DueDate = &now
	other := primaryRowsProject("beta", otherDue)
	out, runtime := primaryRowsOutliner(project, other)
	todayOrder := []config.ProjectTaskLocator{{ProjectID: project.ID, TaskID: planned.ID}}
	nextOrder := []config.ProjectTaskLocator{{ProjectID: project.ID, TaskID: next.ID}}
	runtime.Planning().SetToday(todayOrder)
	runtime.Planning().SetNext(nextOrder)

	for _, mode := range []chrome.OutlinerModeID{chrome.ModeToday, chrome.ModeAll} {
		out.SelectOutlinerMode(mode)
		for _, identity := range []taskref.TaskIdentity{
			taskref.For(planned, project), taskref.For(due, project),
			taskref.For(overdue, project),
			taskref.For(next, project), taskref.For(otherDue, other),
		} {
			if countTaskRowsForIdentity(out, identity) != 1 {
				t.Fatalf("%s: missing or duplicate task %v", mode, identity)
			}
			for _, row := range out.Rows {
				if row.IsTaskRow() && out.TaskIdentityForRow(row) == identity && !row.IsTodaySection {
					t.Fatalf("%s: task %v escaped the Today section: %+v", mode, identity, row)
				}
			}
		}
		if countTaskRows(out, hidden.ID) != 0 {
			t.Fatalf("%s: due date admitted hidden task", mode)
		}
		if mode == chrome.ModeToday && countTaskRows(out, child.ID) != 0 {
			t.Fatal("Today admitted an undated, unplanned descendant")
		}
	}
	if !slices.Equal(runtime.Planning().GetToday(), todayOrder) ||
		!slices.Equal(runtime.Planning().GetNext(), nextOrder) || next.State() != models.StateWaiting || !next.Review {
		t.Fatal("viewing due tasks changed their plan, lifecycle, or review flag")
	}

	out.PushProjectScope(other.ID)
	out.SelectOutlinerMode(chrome.ModeToday)
	if len(out.VisibleTaskIdentities()) != 1 || countTaskRowsForIdentity(out, taskref.For(otherDue, other)) != 1 {
		t.Fatal("Today did not respect project scope")
	}
}

func TestTodayDueMembershipUsesCalendarDayAndVisibility(t *testing.T) {
	// Date-only storage can retain UTC while the display clock is west of UTC.
	now := time.Date(2040, 4, 2, 23, 30, 0, 0, time.FixedZone("local", -6*60*60))
	sameDayUTC := time.Date(2040, 4, 2, 0, 0, 0, 0, time.UTC)
	earlierToday := sameDayUTC.Add(9 * time.Hour)
	yesterday, tomorrow := now.AddDate(0, 0, -1), now.AddDate(0, 0, 1)
	for _, tc := range []struct {
		name     string
		state    models.WorkflowState
		due      *time.Time
		deferred bool
		want     int
	}{
		{"date only in UTC", models.StateBacklog, &sameDayUTC, false, 1},
		{"timed deadline already passed today", models.StateDoing, &earlierToday, false, 1},
		{"inbox", models.StateInbox, &sameDayUTC, false, 1},
		{"waiting", models.StateWaiting, &sameDayUTC, false, 1},
		{"paused", models.StatePaused, &sameDayUTC, false, 1},
		{"someday", models.StateSomeday, &sameDayUTC, false, 1},
		{"yesterday", models.StateBacklog, &yesterday, false, 1},
		{"tomorrow", models.StateBacklog, &tomorrow, false, 0},
		{"undated", models.StateBacklog, nil, false, 0},
		{"done", models.StateDone, &sameDayUTC, false, 0},
		{"canceled", models.StateCanceled, &sameDayUTC, false, 0},
		{"deferred", models.StateBacklog, &sameDayUTC, true, 0},
		{"overdue done", models.StateDone, &yesterday, false, 0},
		{"overdue canceled", models.StateCanceled, &yesterday, false, 0},
		{"overdue deferred", models.StateBacklog, &yesterday, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := primaryRowsTask("task", tc.state)
			task.DueDate = tc.due
			if tc.deferred {
				task.DeferredUntil = &tomorrow
			}
			project := primaryRowsProject("alpha", task)
			input := OutlinerModeInput{Projects: []*models.Project{project}, Now: now}
			if rows := BuildTodayModeRows(input); len(rows) != tc.want {
				t.Fatalf("Today rows = %v, want %d", rows, tc.want)
			}
			if tc.want == 0 {
				return
			}
			input.TodayOrder = []PlanningTask{{Task: task, Project: project}, {Task: task, Project: project}}
			if rows := BuildTodayModeRows(input); len(rows) != 1 {
				t.Fatalf("task both planned and due appears %d times", len(rows))
			}
			input.CandidateTasks = map[string][]*models.Task{project.ID: {}}
			if len(BuildTodayModeRows(input)) != 0 {
				t.Fatal("Today ignored focus candidates")
			}
			input.CandidateTasks = nil
			input.TaskFilter = func(*models.Task, *models.Project) bool { return false }
			if len(BuildTodayModeRows(input)) != 0 {
				t.Fatal("Today ignored the task filter")
			}
			input.TaskFilter = nil
			input.TaskExclude = func(*models.Task, *models.Project) bool { return true }
			if len(BuildTodayModeRows(input)) != 0 {
				t.Fatal("Today ignored task exclusions")
			}
		})
	}
}

func TestTodayDueMembershipChangesAtMidnight(t *testing.T) {
	midnight := time.Date(2040, 4, 2, 0, 0, 0, 0, time.Local)
	due := primaryRowsTask("due", models.StateBacklog)
	due.DueDate = &midnight
	project := primaryRowsProject("alpha", due)
	input := OutlinerModeInput{Projects: []*models.Project{project}}
	for _, tc := range []struct {
		at   time.Time
		want int
	}{
		{midnight.Add(-time.Second), 0},
		{midnight, 1},
		{midnight.AddDate(0, 0, 1), 1},
	} {
		input.Now = tc.at
		if rows := BuildTodayModeRows(input); len(rows) != tc.want {
			t.Fatalf("at %s: Today rows = %v, want %d", tc.at, rows, tc.want)
		}
	}
}
