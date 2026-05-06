package keymap

// LinkedTaskDeps holds action closures for the "c" linked-task prefix group.
type LinkedTaskDeps struct {
	// CreateBlocker opens the linked-task creator with link type = blocker.
	CreateBlocker Action
	// CreateDependent opens the linked-task creator with link type = dependent.
	CreateDependent Action
	// CreateSubtask opens the linked-task creator with link type = subtask.
	CreateSubtask Action
	// CreateFollowUp opens the linked-task creator with link type = follow-up.
	CreateFollowUp Action
	// NavigateLinked opens the linked-task navigator. Nil if not supported.
	NavigateLinked Action
	// CrossProjectLink opens the cross-project linker. Nil if not supported.
	CrossProjectLink Action
	// UnlinkTask opens the unlink selector. Nil if not supported.
	UnlinkTask Action
}

// LinkedTaskGroup returns entries for the "c" prefix: create and navigate linked tasks.
func LinkedTaskGroup(deps LinkedTaskDeps) []Entry {
	cat := "Linked Tasks"
	children := []Entry{
		Bind("b", "Create blocker", cat, deps.CreateBlocker),
		Bind("d", "Create dependent", cat, deps.CreateDependent),
		Bind("s", "Create subtask", cat, deps.CreateSubtask),
		Bind("f", "Create follow-up", cat, deps.CreateFollowUp),
	}
	if deps.NavigateLinked != nil {
		children = append(children, Bind("n", "Navigate linked tasks", cat, deps.NavigateLinked))
	}
	if deps.CrossProjectLink != nil {
		children = append(children, Bind("p", "Cross-project link", cat, deps.CrossProjectLink))
	}
	if deps.UnlinkTask != nil {
		children = append(children, Bind("x", "Unlink linked task", cat, deps.UnlinkTask))
	}
	return []Entry{Prefix("c", "Linked tasks", cat, children)}
}
