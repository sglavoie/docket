package chrome

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
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
