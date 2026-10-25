package valuepicker

import (
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// The `@` picker's rows are presets, so no row shows what the task holds now.
// That is the gap a reflexive Enter falls into, and DateFieldsNowLine closes it
// by naming the three stored values the picker can replace, in the order its
// sections stand. One task names its own values; a selection names a value its
// targets share and `mixed` where they disagree, so the line never claims one
// target's value for all of them.

const (
	dateNowLineMixed = "mixed"
	dateNowLineNone  = "none"
)

// DateFieldsNowLine describes the due date, the pending deferral, and the
// check-in schedule the targets hold, as one line. A lapsed deferral is history
// rather than something the picker would replace, so it reads as none. Targets
// holding none of the three get no line at all.
func DateFieldsNowLine(refs []taskref.TaskProjectRef, now time.Time) string {
	tasks := make([]*models.Task, 0, len(refs))
	for _, ref := range refs {
		if ref.Task != nil {
			tasks = append(tasks, ref.Task)
		}
	}
	if len(tasks) == 0 {
		return ""
	}
	due := commonDateFieldText(tasks, func(task *models.Task) dateNowValue {
		return dateNowInstant(task.DueDate, now)
	})
	deferral := commonDateFieldText(tasks, func(task *models.Task) dateNowValue {
		if !task.IsDeferredAt(now) {
			return dateNowInstant(nil, now)
		}
		return dateNowInstant(task.DeferredUntil, now)
	})
	checkIn := commonDateFieldText(tasks, checkInNowValue)
	if due == dateNowLineNone && deferral == dateNowLineNone && checkIn == dateNowLineNone {
		// Nothing stored is nothing to replace, and the disabled Clear rows already
		// say `no date set`, so the line would only cost a list row.
		return ""
	}
	return strings.Join([]string{
		"Now: due " + due,
		"defer " + deferral,
		"check-in " + checkIn,
	}, " · ")
}

// dateNowValue is one field's stored value as an identity and as the words the
// line prints for it. Agreement is judged on the identity, so two different
// years printed alike are still a disagreement.
type dateNowValue struct {
	key, text string
}

// commonDateFieldText is the text every task agrees on, else mixed.
func commonDateFieldText(tasks []*models.Task, value func(*models.Task) dateNowValue) string {
	first := value(tasks[0])
	for _, task := range tasks[1:] {
		if value(task).key != first.key {
			return dateNowLineMixed
		}
	}
	return first.text
}

func dateNowInstant(value *time.Time, now time.Time) dateNowValue {
	if value == nil {
		return dateNowValue{text: dateNowLineNone}
	}
	return dateNowValue{
		key:  value.UTC().Format(time.RFC3339Nano),
		text: OperationDateSecondaryText(*value, now),
	}
}

func checkInNowValue(task *models.Task) dateNowValue {
	if task.NextCheckIn == nil {
		return dateNowValue{text: dateNowLineNone}
	}
	text := task.NextCheckIn.Format("Mon Jan 2")
	if task.CheckInRepeatDays > models.CheckInRepeatOff {
		text += " " + chrome.FormatCheckInRepeat(task.CheckInRepeatDays)
	}
	return dateNowValue{
		key:  task.NextCheckIn.UTC().Format(time.RFC3339Nano) + "/" + strconv.Itoa(task.CheckInRepeatDays),
		text: text,
	}
}
