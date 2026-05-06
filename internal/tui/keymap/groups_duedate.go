package keymap

// DueDateDeps holds action closures for the "d" due-date prefix group.
type DueDateDeps struct {
	// DueToday sets due date to today.
	DueToday Action
	// DueTomorrow sets due date to tomorrow.
	DueTomorrow Action
	// DueInAWeek sets due date 7 days from now.
	DueInAWeek Action
	// DueInAMonth sets due date 30 days from now.
	DueInAMonth Action
	// ClearDueDate removes the due date.
	ClearDueDate Action
	// OpenDatePicker opens the interactive date picker dialog.
	OpenDatePicker Action
}

// DueDateGroup returns entries for the "d" prefix: quick due-date setting.
func DueDateGroup(deps DueDateDeps) []Entry {
	cat := "Due Date"
	children := []Entry{
		Bind("t", "Due today", cat, deps.DueToday),
		Bind("m", "Due tomorrow", cat, deps.DueTomorrow),
		Bind("w", "Due in a week", cat, deps.DueInAWeek),
		Bind("n", "Due in a month", cat, deps.DueInAMonth),
		Bind("0", "Clear due date", cat, deps.ClearDueDate),
		Bind("d", "Open date picker", cat, deps.OpenDatePicker),
	}
	return []Entry{Prefix("d", "Due date", cat, children)}
}
