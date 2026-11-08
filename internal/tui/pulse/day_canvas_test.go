package pulse

import (
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
)

func TestDayCanvasReadsTodaysDoneTheWeekAheadAndTheWeekBehind(t *testing.T) {
	now := time.Date(2026, 11, 6, 15, 0, 0, 0, time.Local) // a Friday
	at := func(days, hour int) *time.Time {
		v := time.Date(2026, 11, 6+days, hour, 0, 0, 0, time.Local)
		return &v
	}
	tasks := []*models.Task{
		{ID: "a", Title: "Earlier", WorkflowState: models.StateDone, CompletedAt: at(0, 9)},
		{ID: "b", Title: "Latest", WorkflowState: models.StateDone, CompletedAt: at(0, 14)},
		{ID: "c", Title: "Yesterday", WorkflowState: models.StateDone, CompletedAt: at(-1, 14)},
		{ID: "d", Title: "Canceled", WorkflowState: models.StateCanceled, CompletedAt: at(0, 10)},
		{ID: "e", Title: "Due Monday", WorkflowState: models.StateBacklog, DueDate: at(3, 0)},
		{ID: "f", Title: "Both Monday", WorkflowState: models.StateWaiting, DueDate: at(3, 0), NextCheckIn: at(3, 0)},
		{ID: "g", Title: "Due today", WorkflowState: models.StateBacklog, DueDate: at(0, 0)},
		{ID: "h", Title: "Past horizon", WorkflowState: models.StateBacklog, DueDate: at(8, 0)},
		{ID: "i", Title: "Done but dated", WorkflowState: models.StateDone, DueDate: at(1, 0), CompletedAt: at(-3, 9)},
		{ID: "j", Title: "Hidden", WorkflowState: models.StateDone, CompletedAt: at(0, 11), Hidden: true},
	}
	project := &models.Project{ID: "p", Slug: "home", Tasks: tasks}
	project.BuildTaskTree()
	activity := app.CompletionActivity{DailyCompletions: map[time.Time]int{
		time.Date(2026, 11, 5, 0, 0, 0, 0, time.Local): 1,
		time.Date(2026, 11, 6, 0, 0, 0, 0, time.Local): 99, // stale: today is read off the population
	}}

	canvas := ComputeDayCanvas([]*models.Project{project}, activity, now)

	var done []string
	for _, task := range canvas.Done {
		done = append(done, task.Title)
	}
	if want := []string{"Latest", "Earlier"}; !slices.Equal(done, want) {
		t.Fatalf("Done = %v, want %v: today's Done only, newest first, never canceled or hidden", done, want)
	}
	if canvas.Done[0].ProjectSlug != "home" {
		t.Fatalf("done task names project %q, want home", canvas.Done[0].ProjectSlug)
	}
	if len(canvas.Ahead) != 7 || !canvas.Ahead[0].Date.Equal(time.Date(2026, 11, 7, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("Ahead must be the seven days after today, got %d starting %v", len(canvas.Ahead), canvas.Ahead[0].Date)
	}
	monday := canvas.Ahead[2]
	if monday.Due != 2 || monday.CheckIns != 1 {
		t.Fatalf("Monday = %+v, want two deadlines and one check-in", monday)
	}
	if !canvas.Ahead[0].Empty() {
		t.Fatalf("a terminal task's date counts ahead: %+v", canvas.Ahead[0])
	}
	if len(canvas.Behind) != 7 || canvas.Behind[5].Done != 1 || canvas.Behind[6].Done != 2 {
		t.Fatalf("Behind = %+v, want yesterday 1 and today 2 (the population's count)", canvas.Behind)
	}
	if canvas.WeekDone() != 3 {
		t.Fatalf("WeekDone = %d, want 3", canvas.WeekDone())
	}
}

func TestDayCanvasWithoutHistoryHasNoWeekBehind(t *testing.T) {
	canvas := ComputeDayCanvas(nil, app.CompletionActivity{}, time.Now())
	if canvas.Behind != nil || !canvas.AheadEmpty() || len(canvas.Done) != 0 {
		t.Fatalf("an empty workspace canvas = %+v, want no history and nothing dated", canvas)
	}
}

func TestSparklineScalesToTheBusiestDayAndNeverDrawsOneAsNothing(t *testing.T) {
	canvas := DayCanvas{Behind: []HistoryDay{{Done: 0}, {Done: 1}, {Done: 4}, {Done: 8}}}
	if got, want := string(canvas.Sparkline()), " ▁▄█"; got != want {
		t.Fatalf("Sparkline = %q, want %q", got, want)
	}
}

func TestTickerCountsBetweenValuesOfOneWidthOnly(t *testing.T) {
	before := WorkspacePulse{Inbox: 12, Today: 9, Overdue: 3, Done: 1}
	after := WorkspacePulse{Inbox: 18, Today: 12, Overdue: 4, Done: 2}
	frames := TickerFrames(before, after)
	if frames != 4 {
		t.Fatalf("TickerFrames = %d, want 4: inbox moved six, capped at four in-between values", frames)
	}
	change := PulseChangeBetween(before, after)
	var inbox []int
	for frame := range frames {
		lit := after.Lit(change.TickerAt(before, after, frame, frames))
		if lit.lit.Ticking&PulseToday != 0 || lit.lit.Ticking&PulseOverdue != 0 || lit.lit.Ticking&PulseDone != 0 {
			t.Fatalf("frame %d ticks a count that changed width or moved by one: %b", frame, lit.lit.Ticking)
		}
		inbox = append(inbox, lit.shown(PulseInbox))
	}
	for i, value := range inbox {
		if value <= 12 || value >= 18 || (i > 0 && value < inbox[i-1]) {
			t.Fatalf("inbox ticks %v: each value must lie strictly between 12 and 18 and never go back", inbox)
		}
	}
	if spent := change.TickerAt(before, after, frames, frames); spent.Ticking != 0 {
		t.Fatal("a ticker past its last frame still ticks")
	}
	if TickerFrames(before, WorkspacePulse{Inbox: 13}) != 0 {
		t.Fatal("a count that moved by one has nothing in between")
	}
}

// Work in hand reads its restart line: the resume note first, else the next
// unticked checklist step; Waiting and unstarted work, and tasks with neither
// line, are left out, and the most recently changed state leads.
func TestDayCanvasPicksUpWorkInHand(t *testing.T) {
	now := time.Date(2026, 11, 6, 15, 0, 0, 0, time.Local)
	hour := func(h int) time.Time { return time.Date(2026, 11, 6, h, 0, 0, 0, time.Local) }
	tasks := []*models.Task{
		{ID: "a", Title: "Noted", WorkflowState: models.StateDoing, StateChangedAt: hour(9), ResumeNote: "halfway through",
			Checklist: []models.ChecklistItem{{Text: "ignored"}}},
		{ID: "b", Title: "Stepped", WorkflowState: models.StatePaused, StateChangedAt: hour(11),
			Checklist: []models.ChecklistItem{{Text: "done", Done: true}, {Text: "write the summary"}}},
		{ID: "c", Title: "Bare", WorkflowState: models.StateDoing, StateChangedAt: hour(12)},
		{ID: "d", Title: "Waiting", WorkflowState: models.StateWaiting, StateChangedAt: hour(13), ResumeNote: "their move"},
		{ID: "e", Title: "Hidden", WorkflowState: models.StateDoing, StateChangedAt: hour(14), ResumeNote: "x", Hidden: true},
	}
	project := &models.Project{ID: "p", Slug: "home", Tasks: tasks}
	project.BuildTaskTree()

	canvas := ComputeDayCanvas([]*models.Project{project}, app.CompletionActivity{}, now)

	if len(canvas.PickUp) != 2 {
		t.Fatalf("PickUp = %+v, want Stepped then Noted", canvas.PickUp)
	}
	if got := canvas.PickUp[0]; got.Title != "Stepped" || got.Note != "write the summary" || !got.Step {
		t.Fatalf("first pick-up = %+v, want the paused task's next step", got)
	}
	if got := canvas.PickUp[1]; got.Title != "Noted" || got.Note != "halfway through" || got.Step {
		t.Fatalf("second pick-up = %+v, want the resume note over the checklist", got)
	}
	if lines := canvas.PickUpBlock(80, 1); lines != nil {
		t.Fatalf("a budget with no room for a task drew %v", lines)
	}
}
