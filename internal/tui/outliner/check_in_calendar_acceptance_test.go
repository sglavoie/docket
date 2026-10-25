package outliner

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// The exact-midnight input is injected at the production projection boundary;
// unlike scenario wait timers, it controls the calendar used for membership.
func TestPersistedCheckInsAcrossMidnightAndDeferredWake(t *testing.T) {
	midnight := time.Date(2040, 4, 2, 0, 0, 0, 0, time.Local)
	wake := midnight.Add(time.Minute)
	project := primaryRowsProject(uuid.NewString())
	project.Path = t.TempDir()
	states := []models.WorkflowState{models.StateWaiting, models.StateDoing, models.StateBacklog, models.StateBacklog}
	for i, state := range states {
		task := primaryRowsTask(uuid.NewString(), state)
		task.ShortID = i + 1
		task.CreatedAt, task.UpdatedAt, task.StateChangedAt = midnight.AddDate(0, 0, -10), midnight.AddDate(0, 0, -10), midnight.AddDate(0, 0, -10)
		task.NextCheckIn = &midnight
		task.CheckInRepeatDays = 3
		if i == 3 {
			task.DeferredUntil = &wake
		}
		project.Tasks = append(project.Tasks, task)
	}
	project.NextShortID = 5
	if err := storage.SaveProject(project); err != nil {
		t.Fatal(err)
	}
	if err := storage.LoadProject(project); err != nil {
		t.Fatal(err)
	}
	out, runtime := primaryRowsOutliner(project)
	out.PrimaryMode = chrome.ModeAll
	planned := project.Tasks[2]
	runtime.Planning().SetToday([]config.ProjectTaskLocator{{ProjectID: project.ID, TaskID: planned.ID}})
	for _, tc := range []struct {
		name     string
		at       time.Time
		waiting  string
		deferred int
	}{
		{"before midnight", midnight.Add(-time.Second), "Waiting", 0},
		{"midnight", midnight, "Review", 0},
		{"wake", wake.Add(time.Second), "Review", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := out.buildOutlinerModeInput([]*models.Project{project})
			input.Now = tc.at
			input.TaskFilter = out.workModeTaskFilterAt(tc.at)
			out.Rows = nil
			out.buildPrimaryModeRowsWithInput([]*models.Project{project}, input, nil)
			for index, want := range map[int]string{0: tc.waiting, 1: "Doing", 2: "Today"} {
				task := project.Tasks[index]
				if got := primarySectionForTask(out, task.ID); got != want {
					t.Fatalf("task %d section %q, want %q", index, got, want)
				}
				if countTaskRows(out, task.ID) != 1 {
					t.Fatal("duplicate or missing task")
				}
			}
			if countTaskRows(out, project.Tasks[3].ID) != tc.deferred {
				t.Fatal("deferred reminder appeared before wake or stayed hidden")
			}
			// A Today lane suppresses its redundant Today suffix. Check the
			// underlying project census as well as the actual planning row.
			_, todayCount := out.projectGroupPressure(project.Tasks, project.ID)
			if todayCount != 1 {
				t.Fatalf("scheduled attention changed project Today count: %d", todayCount)
			}
			if len(runtime.Planning().GetToday()) != 1 || !input.TodayIDs[taskref.For(planned, project)] {
				t.Fatal("reminders changed planning membership")
			}
		})
	}
}
