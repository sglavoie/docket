package keymap

// StatusDeps holds action closures for the "," workflow-state prefix group.
type StatusDeps struct {
	// ChangeState returns an Action that transitions the task to the given
	// canonical WorkflowState string (one of the ten states).
	ChangeState func(state string) Action
	// HasRecurrence reports whether the current task has a recurrence rule.
	HasRecurrence func() bool
	// SkipRecurrence is the action for ,k (skip one recurrence).
	SkipRecurrence Action
	// DeferChildren are the pre-built entries for the ,D sub-prefix.
	DeferChildren []Entry
}

// StatusGroup returns entries for the "," prefix: workflow-state transitions and
// optional recurrence operations. The keys map to canonical WorkflowState values
// (b→waiting, o→inbox/reopen, S→paused, m→someday).
func StatusGroup(deps StatusDeps) []Entry {
	cat := "Status"
	children := []Entry{
		Bind("s", "Start (in progress)", cat, deps.ChangeState("in_progress")),
		Bind("d", "Mark done", cat, deps.ChangeState("done")),
		Bind("b", "Waiting (blocked)", cat, deps.ChangeState("waiting")),
		Bind("o", "Reopen (inbox)", cat, deps.ChangeState("inbox")),
		Bind("x", "Cancel task", cat, deps.ChangeState("canceled")),
		Bind("r", "Send to review", cat, deps.ChangeState("review")),
		Bind("S", "Pause task", cat, deps.ChangeState("paused")),
		Bind("m", "Set to someday", cat, deps.ChangeState("someday")),
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
