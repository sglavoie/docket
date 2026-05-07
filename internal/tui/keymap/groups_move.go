package keymap

// MoveDeps holds action closures for the "m" section-move prefix group.
type MoveDeps struct {
	// MoveToSection returns an Action that moves the task to the given section.
	// The closure is responsible for choosing bulk vs. single based on multi-select state.
	MoveToSection func(section string) Action
	// MoveToTop moves the current task to the top of its section.
	MoveToTop Action
	// MoveToBottom moves the current task to the bottom of its section.
	MoveToBottom Action
	// CanMoveSection gates section-move children. Nil means always active.
	CanMoveSection func() bool
}

// MoveGroup returns entries for the "m" prefix: section moves and position moves.
func MoveGroup(deps MoveDeps) []Entry {
	cat := "Move (m prefix)"
	var sectionGate Gate
	if deps.CanMoveSection != nil {
		sectionGate = deps.CanMoveSection
	}

	sectionBind := func(key, desc, section string) Entry {
		e := Bind(key, desc, cat, deps.MoveToSection(section))
		if sectionGate != nil {
			e = Bind(key, desc, cat, deps.MoveToSection(section), WithGate(sectionGate))
		}
		return e
	}

	children := []Entry{
		sectionBind("a", "Send task(s) to Active", ""),
		sectionBind("n", "Send task(s) to Next", "Next"),
		sectionBind("i", "Send task(s) to Inbox", "Inbox"),
		sectionBind("w", "Send task(s) to Waiting", "Waiting"),
		sectionBind("r", "Send task(s) to Review", "Review"),
		sectionBind("s", "Send task(s) to Someday", "Someday"),
		sectionBind("S", "Send task(s) to Standby", "Standby"),
		Bind("K", "Move task to top of section", cat, deps.MoveToTop),
		Bind("J", "Move task to bottom of section", cat, deps.MoveToBottom),
	}
	return []Entry{Prefix("m", "Move", cat, children)}
}
