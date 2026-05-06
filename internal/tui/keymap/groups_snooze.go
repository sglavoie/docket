package keymap

// SnoozeDeps holds action closures for the "s" snooze prefix group.
type SnoozeDeps struct {
	// SnoozeTomorrow snoozes until tomorrow.
	SnoozeTomorrow Action
	// SnoozeOneWeek snoozes for one week.
	SnoozeOneWeek Action
	// SnoozeOneMonth snoozes for one month.
	SnoozeOneMonth Action
	// SnoozeNextMonday snoozes until next Monday.
	SnoozeNextMonday Action
	// ClearSnooze removes the snooze date.
	ClearSnooze Action
	// OpenDatePicker opens the interactive snooze date picker.
	OpenDatePicker Action
}

// SnoozeGroup returns entries for the "s" prefix: quick snooze scheduling.
func SnoozeGroup(deps SnoozeDeps) []Entry {
	cat := "Snooze"
	children := []Entry{
		Bind("t", "Snooze until tomorrow", cat, deps.SnoozeTomorrow),
		Bind("w", "Snooze 1 week", cat, deps.SnoozeOneWeek),
		Bind("m", "Snooze 1 month", cat, deps.SnoozeOneMonth),
		Bind("n", "Snooze until next Monday", cat, deps.SnoozeNextMonday),
		Bind("0", "Clear snooze", cat, deps.ClearSnooze),
		Bind("d", "Open snooze date picker", cat, deps.OpenDatePicker),
	}
	return []Entry{Prefix("s", "Snooze", cat, children)}
}
