package keymap

// FilterDeps holds action closures for the "F" filter prefix group.
type FilterDeps struct {
	// Direct F-prefix actions
	FilterByTags      Action // F# — open tag selector
	TogglePriority    Action // Fp — toggle hi-priority filter
	ToggleHideDimmed  Action // Fh — toggle hide-dimmed filter
	ToggleGitHubIssue Action // Fi — toggle GitHub issues filter
	ToggleParked      Action // Fk — toggle parked tasks filter
	ClearFilters      Action // Fx — clear all filters

	// TypeKey returns an Action for a Ft sub-key (e, s, t, b, i, 0).
	TypeKey func(key string) Action
	// StateKey returns an Action for a Fs sub-key (i, r, n, s, w, p, v, m, d,
	// x, 0) selecting one of the ten canonical workflow states. This is the sole
	// state/status filter sub-prefix; the legacy TaskStatus Fs cycle was retired.
	// Nil disables the workflow-state sub-prefix.
	StateKey func(key string) Action
	// DueDateKey returns an Action for a Fd sub-key (o, t, w, n, 0).
	// Nil disables the due-date sub-prefix.
	DueDateKey func(key string) Action
	// FilterByProject opens the project filter selector (FP). Nil disables the
	// project quick filter — only views that maintain a project-scoped filter
	// (e.g. the Planner Backlog) set it.
	FilterByProject Action
	// OpenAttrPicker opens the attribute filter picker overlay (replaces Fa sub-prefixes).
	OpenAttrPicker Action
	// ScoreKey returns an Action for a FS sub-key (1-9, c, 0).
	ScoreKey func(key string) Action
	// ScoreCustomInput opens the inline input for setting a new custom score
	// threshold. Nil disables the FS -> C sub-key — views that cannot accept
	// score input must leave this nil so the keymap, ? help, and popup never
	// advertise an action the view does not handle.
	ScoreCustomInput Action
}

// FilterGroup returns entries for the "F" prefix and all its sub-prefixes.
func FilterGroup(deps FilterDeps) []Entry {
	cat := "Filters"

	// --- Ft: type filter ---
	ftChildren := []Entry{
		Bind("e", "Filter: Epic", cat, deps.TypeKey("e")),
		Bind("s", "Filter: Story", cat, deps.TypeKey("s")),
		Bind("t", "Filter: Task", cat, deps.TypeKey("t")),
		Bind("b", "Filter: Bug", cat, deps.TypeKey("b")),
		Bind("i", "Filter: Idea", cat, deps.TypeKey("i")),
		Bind("0", "Clear type filter", cat, deps.TypeKey("0")),
	}

	// --- Fs: workflow-state filter (ten canonical states; replaces the retired
	// legacy TaskStatus status cycle) ---
	var fsEntry *Entry
	if deps.StateKey != nil {
		fsChildren := []Entry{
			Bind("i", "Filter state: Inbox", cat, deps.StateKey("i")),
			Bind("r", "Filter state: Ready", cat, deps.StateKey("r")),
			Bind("n", "Filter state: Next", cat, deps.StateKey("n")),
			Bind("s", "Filter state: In Progress", cat, deps.StateKey("s")),
			Bind("w", "Filter state: Waiting", cat, deps.StateKey("w")),
			Bind("p", "Filter state: Paused", cat, deps.StateKey("p")),
			Bind("v", "Filter state: Review", cat, deps.StateKey("v")),
			Bind("m", "Filter state: Someday", cat, deps.StateKey("m")),
			Bind("d", "Filter state: Done", cat, deps.StateKey("d")),
			Bind("x", "Filter state: Canceled", cat, deps.StateKey("x")),
			Bind("0", "Clear state filter", cat, deps.StateKey("0")),
		}
		e := Prefix("s", "Filter by workflow state (sub-popup)", cat, fsChildren)
		fsEntry = &e
	}

	// --- Fd: due date filter (optional) ---
	var fdEntry *Entry
	if deps.DueDateKey != nil {
		fdChildren := []Entry{
			Bind("o", "Filter: Overdue", cat, deps.DueDateKey("o")),
			Bind("t", "Filter: Due today", cat, deps.DueDateKey("t")),
			Bind("w", "Filter: Due this week", cat, deps.DueDateKey("w")),
			Bind("n", "Filter: No due date", cat, deps.DueDateKey("n")),
			Bind("0", "Clear due date filter", cat, deps.DueDateKey("0")),
		}
		e := Prefix("d", "Filter by due date (sub-popup)", cat, fdChildren)
		fdEntry = &e
	}

	// --- FS: score threshold filter ---
	fsScoreChildren := []Entry{
		Bind("1", "Score ≥ 30", cat, deps.ScoreKey("1")),
		Bind("2", "Score ≥ 60", cat, deps.ScoreKey("2")),
		Bind("3", "Score ≥ 90", cat, deps.ScoreKey("3")),
		Bind("4", "Score ≥ 120", cat, deps.ScoreKey("4")),
		Bind("5", "Score ≥ 150", cat, deps.ScoreKey("5")),
		Bind("6", "Score ≥ 180", cat, deps.ScoreKey("6")),
		Bind("7", "Score ≥ 210", cat, deps.ScoreKey("7")),
		Bind("8", "Score ≥ 240", cat, deps.ScoreKey("8")),
		Bind("9", "Score ≥ 270", cat, deps.ScoreKey("9")),
		Bind("0", "Clear score filter", cat, deps.ScoreKey("0")),
	}
	if deps.ScoreCustomInput != nil {
		fsScoreChildren = append(fsScoreChildren,
			Bind("C", "Set custom score threshold (inline input)", cat, deps.ScoreCustomInput),
		)
	}

	// --- Assemble F children ---
	fChildren := []Entry{
		Prefix("t", "Filter by type (sub-popup)", cat, ftChildren),
	}
	if fsEntry != nil {
		fChildren = append(fChildren, *fsEntry)
	}
	if fdEntry != nil {
		fChildren = append(fChildren, *fdEntry)
	}
	fChildren = append(fChildren,
		Bind("a", "Filter by attribute (picker)", cat, deps.OpenAttrPicker),
		Prefix("S", "Score threshold filter", cat, fsScoreChildren),
	)
	if deps.FilterByProject != nil {
		fChildren = append(fChildren,
			Bind("P", "Filter by project (picker)", cat, deps.FilterByProject),
		)
	}
	fChildren = append(fChildren,
		Bind("#", "Filter by tags", cat, deps.FilterByTags),
		Bind("p", "Toggle priority filter", cat, deps.TogglePriority),
		Bind("h", "Toggle hide-dimmed filter", cat, deps.ToggleHideDimmed),
		Bind("i", "Toggle GitHub issues filter", cat, deps.ToggleGitHubIssue),
		Bind("k", "Toggle parked filter", cat, deps.ToggleParked),
		Bind("x", "Clear all filters", cat, deps.ClearFilters),
	)

	return []Entry{Prefix("F", "Filter prefix (type/status/tags/parked/issue/attr/score)", cat, fChildren)}
}
