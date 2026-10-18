package outliner

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// inlinePeekFields collects `Label: value` pairs into one dense line. Peek has
// three such lines — what the task is, what it still owes, and what it has
// already spent — and they share this so a label reads the same way on all
// three: muted label, keyed value, a middot between fields. An empty value is
// no field, which is how each line omits what the task does not carry. Nothing
// here measures: the line is handed to the ordinary wrapping layout like every
// other section.
type inlinePeekFields struct {
	parts []string
}

func (f *inlinePeekFields) add(label, value string) {
	if value == "" {
		return
	}
	f.parts = append(f.parts, theme.HelpStyle.Render(label+": ")+theme.HelpKeyStyle.Render(value))
}

// flag is a field that is its own value. Pinned and Review are booleans a task
// either wears or does not, so a label beside one would say the word twice.
func (f *inlinePeekFields) flag(word string) {
	f.parts = append(f.parts, theme.HelpKeyStyle.Render(word))
}

func (f inlinePeekFields) line() string {
	if len(f.parts) == 0 {
		return ""
	}
	return strings.Join(f.parts, theme.HelpStyle.Render(" · "))
}

// inlinePeekStatusLine spells what the row's badges abbreviate and what it drops
// entirely: the planning collection the task belongs to, the lifecycle state in
// words rather than as a glyph, the assigned priority as a number, and the two
// independent boolean axes. The status is never omitted — every task is in one —
// so a task Peek always has at least this line to show.
//
// Plan leads and status follows it, which is the order the workflow chord and the
// capture route both read in, and the two sit adjacent so the pair is read as one
// answer: a task planned for Today is in Backlog, and the line says so rather
// than leaving a reader to know it. They are two axes and not two values of one,
// so an absent plan drops its field and changes nothing about the status beside
// it.
func inlinePeekStatusLine(task *models.Task, planning string, now time.Time) string {
	if task == nil {
		return ""
	}
	var fields inlinePeekFields
	fields.add(chrome.PlanFieldLabel, planning)
	fields.add(chrome.StatusFieldLabel, inlinePeekStatusValue(task, now))
	// Keep the reason to act in the first summary, before optional metadata.
	// Dates and repeat details remain complete in the schedule section.
	if task.IsOverdueAt(now) {
		fields.parts = append(fields.parts, theme.ProjectPressureOverdueStyle.Render("Overdue"))
	}
	if task.CheckInIsDueAt(now) && models.TaskNeedsAttention(task, now) {
		fields.flag("Check-in owed")
	}
	// The same gate theme.PriorityIcon takes, so a task the row draws no priority
	// badge for names no priority here either.
	if models.ValidatePriority(task.Priority, false) == nil {
		fields.add("Priority", strconv.Itoa(task.Priority)+" "+strings.ToLower(models.PriorityName(task.Priority)))
	}
	if task.Pinned {
		fields.flag("Pinned")
	}
	if task.Review {
		fields.flag("Review")
	}
	return fields.line()
}

func inlinePeekStatusLines(task *models.Task, planning string, now time.Time, indent string, width int) []string {
	return inlinePeekStyledLines("", inlinePeekStatusLine(task, planning, now), indent, width)
}

// inlinePeekStatusValue names the lifecycle state and, for the two states whose
// age is a question about the work rather than about the record, how long it has
// held it. StateChangedAt is what it reads, so commenting on a blocked task does
// not reset how long it has been blocked.
func inlinePeekStatusValue(task *models.Task, now time.Time) string {
	state := chrome.PlanStatusValue(task.State().String())
	if !task.IsInState(models.StateDoing) && !task.IsInState(models.StateWaiting) {
		return state
	}
	age := query.FormatRecencyAge(task.StateChangedAt, now)
	if age == "" {
		return state
	}
	return state + " " + age
}

// inlinePeekStatusKey identifies the summary by its stored values, the state
// clock included rather than the age rendered from it: the age drifts with every
// tick and would otherwise return Peek's window to the top on the minute.
func inlinePeekStatusKey(task *models.Task, planning string) string {
	return strings.Join([]string{
		"plan:" + planning,
		"status:" + task.State().String(),
		"status-changed:" + task.StateChangedAt.Format(time.RFC3339Nano),
		"priority:" + strconv.Itoa(task.Priority),
		"pinned:" + strconv.FormatBool(task.Pinned),
		"review:" + strconv.FormatBool(task.Review),
		inlinePeekScheduleKey(task),
	}, "|")
}

// inlinePeekChecklistLines keeps the checklist a count. It is the only place the
// tally is stated now that the row carries none, but it stays a tally: the items
// themselves are rows of their own under the task, where they are edited, ticked,
// and promoted, and repeating their text here would spend the window on content
// the outline already holds one keypress away.
func inlinePeekChecklistLines(task *models.Task, indent string, width int) []string {
	if task == nil || len(task.Checklist) == 0 {
		return nil
	}
	done := 0
	for _, item := range task.Checklist {
		if item.Done {
			done++
		}
	}
	entry := fmt.Sprintf("%d/%d done", done, len(task.Checklist))
	return inlinePeekEntryLines("Checklist: ", []string{entry}, indent, width)
}

func inlinePeekChecklistKey(task *models.Task) string {
	states := make([]string, 0, len(task.Checklist))
	for _, item := range task.Checklist {
		states = append(states, strconv.FormatBool(item.Done))
	}
	return strings.Join(states, ",")
}

// inlinePeekScheduleLine names what the task still owes: its deadline, the day a
// pending deferral brings it back, and its check-in with the cadence that
// repeats it. Every field here is a date in front of the task, which is why they
// are one line beneath the status summary rather than mixed in with the instants
// the task has already spent — a reader deciding what to do next reads this line
// and stops.
//
// The row keeps the deadline alone, so the other three are read here.
// Labels stay muted and values take the footer's key color, so the dates read as
// data rather than as prose.
func inlinePeekScheduleLine(task *models.Task, now time.Time) string {
	if task == nil {
		return ""
	}
	var fields inlinePeekFields
	if task.DueDate != nil {
		gloss := chrome.DayGloss(*task.DueDate, now)
		if task.IsOverdueAt(now) {
			gloss += "; overdue"
		}
		fields.add("Due", chrome.FormatWeekdayDate(*task.DueDate, now, task.DueHasTime())+" ("+gloss+")")
	}
	// The pending half of the deferral alone. Its lapsed half is a wake that
	// already happened, so it is history and is named there instead.
	if task.DeferredUntil != nil && task.IsDeferredAt(now) {
		fields.add("Defer", chrome.FormatDeferDate(*task.DeferredUntil, now))
	}
	if task.NextCheckIn != nil {
		// An arrived check-in is what the row calls `review`, so the gloss says
		// so: one fact keeps one name across the row and its document.
		gloss := chrome.DayGloss(*task.NextCheckIn, now)
		if task.CheckInIsDueAt(now) && models.TaskNeedsAttention(task, now) {
			gloss += ", review owed"
		}
		fields.add("Check-in", chrome.FormatWeekdayDate(*task.NextCheckIn, now, false)+" ("+gloss+")")
		// "every" keeps a cadence from reading as the row's lateness `+Nd`.
		repeat := "off"
		if task.CheckInRepeatDays > models.CheckInRepeatOff {
			repeat = fmt.Sprintf("every %dd", task.CheckInRepeatDays)
		}
		fields.add("Repeat", repeat)
	}
	return fields.line()
}

func inlinePeekScheduleLines(task *models.Task, now time.Time, indent string, width int) []string {
	return inlinePeekStyledLines("", inlinePeekScheduleLine(task, now), indent, width)
}

// inlinePeekScheduleKey identifies the schedule by its stored instants rather
// than by anything rendered from the clock, so a tick that moves no stored value
// leaves Peek's window where the reader left it.
func inlinePeekScheduleKey(task *models.Task) string {
	parts := []string{"check-in-repeat:" + strconv.Itoa(task.CheckInRepeatDays)}
	if task.DueDate != nil {
		parts = append(parts, "due:"+task.DueDate.Format(time.RFC3339Nano))
	}
	if task.DeferredUntil != nil {
		parts = append(parts, "deferred:"+task.DeferredUntil.Format(time.RFC3339Nano))
	}
	if task.NextCheckIn != nil {
		parts = append(parts, "check-in:"+task.NextCheckIn.Format(time.RFC3339Nano))
	}
	return strings.Join(parts, "|")
}

// inlinePeekHistoryLine names the instants the task has already spent: how long
// ago it was created and last touched (the touch only when it differs from the
// creation to the second), when it was last reviewed, whether it
// recently came back from a deferral, and how many times it has been deferred.
// None of them is owed, so they close the document rather than standing between
// the schedule and the task's own text.
func inlinePeekHistoryLine(task *models.Task, now time.Time) string {
	if task == nil {
		return ""
	}
	var fields inlinePeekFields
	fields.add("Created", inlinePeekPastAge(task.CreatedAt, now))
	// A task never touched since its creation was updated when it was created,
	// so a second instant saying the same thing is noise.
	if !sameStoredSecond(task.CreatedAt, task.UpdatedAt) {
		fields.add("Updated", inlinePeekPastAge(task.UpdatedAt, now))
	}
	if task.LastReviewedAt != nil {
		fields.add("Last review", inlinePeekPastAge(*task.LastReviewedAt, now))
	}
	// The lapsed half of the deferral: a task that came back is work again, and
	// the row carries no deferral at all, so this is where the wake is said.
	if taskWokeRecently(task, now) {
		fields.add("Woke", inlinePeekPastAge(*task.DeferredUntil, now))
	}
	if taskHasRepeatedDeferrals(task) {
		fields.add("Deferred", fmt.Sprintf("×%d", task.DeferCount))
	}
	return fields.line()
}

// sameStoredSecond reports whether two stored instants name the same second,
// which is the precision a creation and its first write share.
func sameStoredSecond(a, b time.Time) bool {
	return a.Truncate(time.Second).Equal(b.Truncate(time.Second))
}

// inlinePeekPastAge spells an instant already spent as how long ago it was, so
// `47d ago` cannot be read as a duration the way the status line's `Doing 9d`
// is. `now` already says when, and an empty age stays empty so the field drops.
func inlinePeekPastAge(at, now time.Time) string {
	age := query.FormatRecencyAge(at, now)
	if age == "" || age == "now" {
		return age
	}
	return age + " ago"
}

func inlinePeekHistoryLines(task *models.Task, now time.Time, indent string, width int) []string {
	return inlinePeekStyledLines("", inlinePeekHistoryLine(task, now), indent, width)
}

// inlinePeekHistoryKey identifies the history by its stored instants rather than
// by the rendered durations, which drift with every tick and would otherwise
// restart Peek's paging on the minute. The deferral instant is keyed here as
// well as in the schedule because the wake is read off it: a section's key must
// identify that section's own content whether or not the other one is drawn.
func inlinePeekHistoryKey(task *models.Task) string {
	parts := []string{
		"created:" + task.CreatedAt.Format(time.RFC3339Nano),
		"updated:" + task.UpdatedAt.Format(time.RFC3339Nano),
		"defer-count:" + strconv.Itoa(task.DeferCount),
	}
	if task.DeferredUntil != nil {
		parts = append(parts, "deferred:"+task.DeferredUntil.Format(time.RFC3339Nano))
	}
	if task.LastReviewedAt != nil {
		parts = append(parts, "last-review:"+task.LastReviewedAt.Format(time.RFC3339Nano))
	}
	return strings.Join(parts, "|")
}
