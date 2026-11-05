package pulse

import (
	"fmt"
	"time"
)

// WeekStart is the Monday that opens the local week holding t, at midnight.
func WeekStart(t time.Time) time.Time {
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	offset := (int(day.Weekday()) + 6) % 7 // Monday is 0
	return day.AddDate(0, 0, -offset)
}

// OpensNewWeek reports whether a greeting at now is the first of its local
// week, given the day the machine last greeted (YYYY-MM-DD, empty for never).
// An unreadable day counts as never, so a damaged value costs one extra recap
// rather than a lost one.
func OpensNewWeek(greetedOn string, now time.Time) bool {
	if greetedOn == "" {
		return true
	}
	last, err := time.ParseInLocation(time.DateOnly, greetedOn, now.Location())
	if err != nil {
		return true
	}
	return last.Before(WeekStart(now))
}

// WeeklyRecap is the line the first greeting of a week adds: how much was
// Done in the previous Monday-to-Sunday week, and on which day most of it
// was, when more than one day carried any. daily is the completion history by
// local day, as app.CompletionActivity holds it; a day is matched by its
// calendar date, so the keys' clock and location do not matter. A week with
// nothing Done says nothing, since the recap marks progress and is not a
// verdict on a quiet week.
func WeeklyRecap(daily map[time.Time]int, now time.Time) string {
	thisWeek := WeekStart(now)
	lastWeek := thisWeek.AddDate(0, 0, -7)
	perWeekday := [7]int{}
	total := 0
	for day, count := range daily {
		local := day.In(now.Location())
		date := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, now.Location())
		if date.Before(lastWeek) || !date.Before(thisWeek) || count <= 0 {
			continue
		}
		perWeekday[int(date.Sub(lastWeek).Hours()/24+0.5)%7] += count
		total += count
	}
	if total == 0 {
		return ""
	}
	busiest, activeDays := 0, 0
	for offset, count := range perWeekday {
		if count > 0 {
			activeDays++
		}
		if count > perWeekday[busiest] {
			busiest = offset
		}
	}
	recap := fmt.Sprintf("last week %d done", total)
	if activeDays > 1 {
		recap += ", most on " + lastWeek.AddDate(0, 0, busiest).Weekday().String()
	}
	return recap
}
