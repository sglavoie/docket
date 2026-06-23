package keymap

// TriagePrimaryDeps holds action closures for the canonical primary triage
// actions shared by the local Inbox Processor, the Global Inbox Processor, and
// any other triage-style task-list context.
//
// # Triage Vocabulary
//
// The shared triage interaction model splits actions into two layers:
//
//   - Direct primary actions (one canonical key each) — exposed by this group:
//     n (skip), e (edit), D (mark done), T (manage tags), u (undo), ? (help).
//   - Prefix vocabulary (multi-key deeper actions) — exposed by sibling groups:
//     m (route/section, MoveGroup), d (due date, DueDateGroup), ! and 1-5
//     (priority, PriorityGroup), , (status, StatusGroup), z (snooze, SnoozeGroup),
//     . (stage, StageGroup), t (task ops, TaskPrefixGroup), c (linked task,
//     LinkedTaskGroup), b (blocker, BlockerGroup).
//
// Primary triage actions never use chord modifiers (no ctrl+/alt+); the shared
// vocabulary keeps the grammar uniform so the footer, help, command palette,
// and tutor content can describe one model.
type TriagePrimaryDeps struct {
	// Skip moves to the next task without changing the current task.
	Skip Action
	// Edit opens the title/description editor on the current task.
	Edit Action
	// MarkDone marks the current task done and advances.
	MarkDone Action
	// ManageTags opens the tag manager for the current task.
	ManageTags Action
	// Undo reverses the last action on the current task.
	Undo Action
	// ToggleHelp shows or hides the help panel for the current view.
	ToggleHelp Action
}

// TriagePrimaryGroup returns the direct-key primary triage bindings.
// Callers compose it with the prefix vocabulary groups (MoveGroup, DueDateGroup,
// PriorityGroup, etc.) to expose the full triage interaction model.
func TriagePrimaryGroup(deps TriagePrimaryDeps) []Entry {
	navCat := "Navigation"
	opsCat := "Operations"
	return []Entry{
		Bind("n", "Skip task (move to next without processing)", navCat, deps.Skip),
		Bind("e", "Edit task (title/description)", opsCat, deps.Edit),
		Bind("D", "Mark done", opsCat, deps.MarkDone),
		Bind("T", "Manage tags", opsCat, deps.ManageTags),
		Bind("u", "Undo last operation on current task", opsCat, deps.Undo),
		Bind("?", "Toggle help", navCat, deps.ToggleHelp),
	}
}
