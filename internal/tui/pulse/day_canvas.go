package pulse

import (
	"cmp"
	"slices"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/util"
)

// DayCanvas is what Today mode draws in the room its rows leave: the work
// already finished today, the week ahead, and the week behind. It is a reading
// of the population the Outliner already holds, computed at one instant, and
// it decides nothing: no row, no count, and no planning reads it back.
type DayCanvas struct {
	// Done is today's finished work, most recent first.
	Done []DoneTask
	// Ahead is the seven local days after today, each with the open work owed
	// on it: deadlines and check-ins, the two dates a day can bring.
	Ahead []HorizonDay
	// Behind is the seven local days ending today, oldest first, each with its
	// completion count; it is empty when the workspace has no completion
	// history at all.
	Behind []HistoryDay
	// PickUp is the work in hand that says how to resume it: Doing and Paused
	// tasks carrying a resume note or an unticked checklist step, the most
	// recently touched state first.
	PickUp []PickUpTask
}

// PickUpTask is one task in hand and the line that says where to restart:
// its resume note when it has one, else its first unticked checklist item.
type PickUpTask struct {
	Title       string
	ProjectSlug string
	State       models.WorkflowState
	Note        string
	// Step reports that Note is the next checklist item rather than a resume
	// note the user wrote.
	Step      bool
	ChangedAt time.Time
}

// DoneTask is one task finished today.
type DoneTask struct {
	Title       string
	ProjectSlug string
	CompletedAt time.Time
}

// HorizonDay is one day ahead and the open work dated to it.
type HorizonDay struct {
	Date     time.Time
	Due      int
	CheckIns int
	// Items names that work, deadlines before check-ins and each run by title,
	// so a day the week browse selects can say what it holds and not only how
	// much.
	Items []HorizonItem
}

// HorizonItem is one piece of open work dated to a day ahead.
type HorizonItem struct {
	Title       string
	ProjectSlug string
	ProjectID   string
	TaskID      string
	// CheckIn reports that the day is owed a check-in rather than a deadline.
	CheckIn bool
}

// Empty reports whether the day holds nothing dated.
func (d HorizonDay) Empty() bool { return d.Due == 0 && d.CheckIns == 0 }

// HistoryDay is one day behind and how much was finished on it.
type HistoryDay struct {
	Date time.Time
	Done int
}

// dayCanvasHorizon is how many days ahead and behind the canvas reads.
const dayCanvasHorizon = 7

// ComputeDayCanvas reads the canvas off projects at now. Hidden work is left
// out of every part, as it is out of every count beside it; terminal work is
// read only for what finished today, and deferred work still counts on its
// day ahead, since a deferral that ends before the date owes it all the same.
func ComputeDayCanvas(projects []*models.Project, activity app.CompletionActivity, now time.Time) DayCanvas {
	today := util.StartOfDay(now)
	// Days are stepped from a noon anchor, as the activity window steps them,
	// so a daylight-saving change never lands two dates on one day.
	anchor := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
	var canvas DayCanvas
	index := make(map[time.Time]int, dayCanvasHorizon)
	for offset := 1; offset <= dayCanvasHorizon; offset++ {
		day := util.StartOfDay(anchor.AddDate(0, 0, offset))
		index[day] = len(canvas.Ahead)
		canvas.Ahead = append(canvas.Ahead, HorizonDay{Date: day})
	}
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil || models.TaskEffectivelyHidden(project, task) {
				continue
			}
			if task.IsTerminalState() {
				if task.WorkflowState == models.StateDone && task.CompletedAt != nil &&
					util.StartOfDay(task.CompletedAt.In(now.Location())).Equal(today) {
					canvas.Done = append(canvas.Done, DoneTask{
						Title: task.Title, ProjectSlug: projectLabel(project), CompletedAt: *task.CompletedAt,
					})
				}
				continue
			}
			if pick, ok := pickUpFor(project, task); ok {
				canvas.PickUp = append(canvas.PickUp, pick)
			}
			item := HorizonItem{Title: task.Title, ProjectSlug: projectLabel(project), ProjectID: project.ID, TaskID: task.ID}
			if task.DueDate != nil {
				if at, ok := index[util.StartOfDay(task.DueDate.In(now.Location()))]; ok {
					canvas.Ahead[at].Due++
					canvas.Ahead[at].Items = append(canvas.Ahead[at].Items, item)
				}
			}
			if task.NextCheckIn != nil {
				if at, ok := index[util.StartOfDay(task.NextCheckIn.In(now.Location()))]; ok {
					canvas.Ahead[at].CheckIns++
					item.CheckIn = true
					canvas.Ahead[at].Items = append(canvas.Ahead[at].Items, item)
				}
			}
		}
	}
	for at := range canvas.Ahead {
		slices.SortStableFunc(canvas.Ahead[at].Items, func(a, b HorizonItem) int {
			if a.CheckIn != b.CheckIn {
				if a.CheckIn {
					return 1
				}
				return -1
			}
			return cmp.Compare(a.Title, b.Title)
		})
	}
	slices.SortStableFunc(canvas.PickUp, func(a, b PickUpTask) int {
		return cmp.Or(b.ChangedAt.Compare(a.ChangedAt), cmp.Compare(a.Title, b.Title))
	})
	slices.SortStableFunc(canvas.Done, func(a, b DoneTask) int {
		return cmp.Or(b.CompletedAt.Compare(a.CompletedAt), cmp.Compare(a.Title, b.Title))
	})
	if activity.HasHistory() {
		for offset := dayCanvasHorizon - 1; offset >= 0; offset-- {
			day := util.StartOfDay(anchor.AddDate(0, 0, -offset))
			canvas.Behind = append(canvas.Behind, HistoryDay{Date: day, Done: activity.DailyCompletions[day]})
		}
		// Today's own count is read off the population rather than the
		// activity snapshot, which the shell refreshes on its own schedule: the
		// bar for today and the list beside it then never disagree.
		canvas.Behind[len(canvas.Behind)-1].Done = len(canvas.Done)
	}
	return canvas
}

// pickUpFor reads the restart line off a task in hand. Only Doing and Paused
// are in hand: Waiting is someone else's move, and the rest are not started.
// The resume note wins because it is the user's own words about where they
// stopped; the checklist's next item is the fallback the task already carries.
func pickUpFor(project *models.Project, task *models.Task) (PickUpTask, bool) {
	state := task.State()
	if state != models.StateDoing && state != models.StatePaused {
		return PickUpTask{}, false
	}
	pick := PickUpTask{Title: task.Title, ProjectSlug: projectLabel(project), State: state, ChangedAt: task.StateChangedAt}
	if note := task.ResumeNote; note != "" {
		pick.Note = note
		return pick, true
	}
	for _, item := range task.Checklist {
		if !item.Done {
			pick.Note, pick.Step = item.Text, true
			return pick, true
		}
	}
	return PickUpTask{}, false
}

// AheadEmpty reports whether nothing is dated in the week ahead.
func (c DayCanvas) AheadEmpty() bool {
	for _, day := range c.Ahead {
		if !day.Empty() {
			return false
		}
	}
	return true
}

// WeekDone is the completions across Behind.
func (c DayCanvas) WeekDone() int {
	total := 0
	for _, day := range c.Behind {
		total += day.Done
	}
	return total
}

// sparkLevels are the eighth-block bars a history day is drawn with, from a
// day with nothing finished to the busiest day of the week.
var sparkLevels = []rune(" ▁▂▃▄▅▆▇█")

// Sparkline draws Behind as one bar per day scaled to the week's busiest, so
// the shape of the week reads at a glance. A day with any completion draws at
// least the lowest bar, so one done task is never drawn as nothing.
func (c DayCanvas) Sparkline() []rune {
	peak := 0
	for _, day := range c.Behind {
		peak = max(peak, day.Done)
	}
	bars := make([]rune, len(c.Behind))
	for i, day := range c.Behind {
		level := 0
		if peak > 0 && day.Done > 0 {
			level = max(1, (day.Done*(len(sparkLevels)-1)+peak-1)/peak)
		}
		bars[i] = sparkLevels[level]
	}
	return bars
}

// projectLabel is the name the canvas brackets a project by: its alias when it
// has one, else its slug, as the rows above it do.
func projectLabel(project *models.Project) string {
	label, _ := models.ProjectDisplayLabel(project)
	return label
}
