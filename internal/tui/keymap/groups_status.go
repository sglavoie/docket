package keymap

// StatusDeps holds action closures for the status prefix group.
type StatusDeps struct {
	// ChangeStatus returns an Action that sets the given status string.
	ChangeStatus func(status string) Action
	// HasRecurrence reports whether the current task has a recurrence rule.
	HasRecurrence func() bool
	// SkipRecurrence is the action for ,k (skip one recurrence).
	SkipRecurrence Action
	// DeferChildren are the pre-built entries for the ,D sub-prefix.
	DeferChildren []Entry
}

// StatusGroup returns entries for the "," prefix: status transitions and
// optional recurrence operations.
func StatusGroup(deps StatusDeps) []Entry {
	cat := "Status"
	children := []Entry{
		Bind("s", "Start (in progress)", cat, deps.ChangeStatus("in_progress")),
		Bind("d", "Mark done", cat, deps.ChangeStatus("done")),
		Bind("b", "Block task", cat, deps.ChangeStatus("blocked")),
		Bind("o", "Reopen (todo)", cat, deps.ChangeStatus("todo")),
		Bind("x", "Cancel task", cat, deps.ChangeStatus("canceled")),
		Bind("r", "Send to review", cat, deps.ChangeStatus("review")),
		Bind("S", "Put task on standby", cat, deps.ChangeStatus("standby")),
		Bind("m", "Set to maybe", cat, deps.ChangeStatus("maybe")),
	}

	if deps.SkipRecurrence != nil {
		children = append(children,
			Bind("k", "Skip recurrence", "Recurrence", deps.SkipRecurrence,
				WithGate(deps.HasRecurrence)),
		)
	}
	if len(deps.DeferChildren) > 0 {
		children = append(children,
			Prefix("D", "Defer recurrence", "Recurrence", deps.DeferChildren,
				WithPrefixGate(deps.HasRecurrence)),
		)
	}

	return []Entry{Prefix(",", "Status", cat, children)}
}

// DeferRecurrenceGroup builds the ,D sub-prefix children.
// Separate helper so callers can construct DeferChildren independently.
func DeferRecurrenceGroup(deferDay, defer3, deferWeek, defer2Weeks, deferMonth Action) []Entry {
	cat := "Recurrence"
	return []Entry{
		Bind("t", "Defer recurrence 1 day", cat, deferDay),
		Bind("3", "Defer recurrence 3 days", cat, defer3),
		Bind("w", "Defer recurrence 1 week", cat, deferWeek),
		Bind("W", "Defer recurrence 2 weeks", cat, defer2Weeks),
		Bind("m", "Defer recurrence 1 month", cat, deferMonth),
	}
}
