package chrome

import (
	"strconv"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/util"
)

// dueDisplayDateOnly is the calendar-day layout for a due date without a time.
const dueDisplayDateOnly = "2006-01-02"

// dueDisplayWithTime is the layout for a due date that carries an explicit
// hh:mm clock time.
const dueDisplayWithTime = "2006-01-02 15:04"

// DueGrammarValue renders a task's due date as the value that follows the @
// sigil in a grammar surface (outline dump, previews). It appends the hh:mm
// clock only when the due date carries an explicit time, so a date-only due
// date round-trips unchanged and a timed one preserves its time.
func DueGrammarValue(task *models.Task) string {
	if task.DueDate == nil {
		return ""
	}
	if task.DueHasTime() {
		return task.DueDate.Format(dueDisplayWithTime)
	}
	return task.DueDate.Format(dueDisplayDateOnly)
}

// FormatDueClock renders just the hh:mm clock of a timed due date, for compact
// row and Peek surfaces that already carry the date elsewhere.
func FormatDueClock(task *models.Task) string {
	if task.DueDate == nil || !task.DueHasTime() {
		return ""
	}
	return task.DueDate.Format("15:04")
}

// FormatDeferDate renders a deferral against now. A midnight-valued deferral is
// a whole day and keeps the bare date it has always had; any other instant
// carries the clock, because the row has to say when the task comes back. The
// value is resolved into now's zone first, so the midnight and same-day tests
// and the rendered text all read against one clock.
func FormatDeferDate(deferredUntil, now time.Time) string {
	value := deferredUntil.In(now.Location())
	offYear := value.Year() != now.Year()
	switch {
	case value.Equal(util.StartOfDay(value)):
		if offYear {
			return value.Format("Jan 2 '06")
		}
		return value.Format("Jan 2")
	case util.StartOfDay(value).Equal(util.StartOfDay(now)):
		return value.Format("15:04")
	case offYear:
		return value.Format("Jan 2 '06 15:04")
	default:
		return value.Format("Jan 2 15:04")
	}
}

// FormatWeekdayDate is the one weekday spelling of a resolved date: "Mon Jan 2",
// with the year when it is not now's and the hh:mm clock when the value is
// timed. Timed is the caller's fact, never read off the clock, because a
// deadline timed at midnight is indistinguishable from a bare day in a
// time.Time.
func FormatWeekdayDate(value, now time.Time, timed bool) string {
	layout := "Mon Jan 2"
	if value.Year() != now.Year() {
		layout += " 2006"
	}
	if timed {
		layout += " 15:04"
	}
	return value.Format(layout)
}

// DayGloss names how far a calendar day is from now for a reader rather than a
// parser: "in 6 days" or "2 days ago". Peek appends it to the weekday date,
// which already names the day itself.
func DayGloss(day, now time.Time) string {
	day = day.In(now.Location())
	switch days := util.DaysBetween(now, day); {
	case days == 0:
		return "today"
	case days == 1:
		return "tomorrow"
	case days == -1:
		return "yesterday"
	case days > 1:
		return "in " + util.Count(days, "day")
	default:
		return util.Count(-days, "day") + " ago"
	}
}

// FormatCheckInRepeat is the one spelling of a check-in cadence: "every 7d", or
// "off" when the schedule does not repeat. "every" keeps a cadence from reading
// as a row's lateness `+Nd`, and the compact day count matches the stored
// `[check-in ... every Nd]` form.
func FormatCheckInRepeat(days int) string {
	if days <= models.CheckInRepeatOff {
		return "off"
	}
	return "every " + strconv.Itoa(days) + "d"
}
