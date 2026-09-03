package chrome

import "github.com/sglavoie/tt/internal/models"

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
