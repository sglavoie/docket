package keymap

// FilterDeps holds action closures for the "F" filter prefix group.
type FilterDeps struct {
	// Direct F-prefix actions
	FilterByTags     Action // F# — open tag selector
	TogglePriority   Action // Fp — toggle hi-priority filter
	ToggleHideDimmed Action // Fh — toggle hide-dimmed filter
	ClearFilters     Action // Fx — clear all filters

	// TypeKey returns an Action for a Ft sub-key (e, s, t, b, i, 0).
	TypeKey func(key string) Action
	// StatusKey returns an Action for a Fs sub-key (t, s, r, S, b, d, x, 0).
	StatusKey func(key string) Action
	// DueDateKey returns an Action for a Fd sub-key (o, t, w, n, 0).
	// Nil disables the due-date sub-prefix.
	DueDateKey func(key string) Action
	// AttrDirectKey returns an Action for direct Fa keys (V, 0).
	// Deprecated: kept for backward compatibility; use OpenAttrPicker instead.
	AttrDirectKey func(key string) Action
	// AttrSubKey returns an Action for an attribute sub-prefix leaf key.
	// Deprecated: kept for backward compatibility; use OpenAttrPicker instead.
	AttrSubKey func(prefix, key string) Action
	// OpenAttrPicker opens the attribute filter picker overlay (replaces Fa sub-prefixes).
	OpenAttrPicker Action
	// ScoreKey returns an Action for a FS sub-key (1-9, c, 0).
	ScoreKey func(key string) Action
	// StageFilterKey returns an Action for a F. sub-key (0, 1-9, a-z).
	// Nil disables the stage-filter sub-prefix.
	StageFilterKey func(key string) Action
	// StageFilterNames is the ordered stage list for F. children.
	StageFilterNames []string
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

	// --- Fs: status filter ---
	fsChildren := []Entry{
		Bind("t", "Filter: Todo", cat, deps.StatusKey("t")),
		Bind("s", "Filter: In Progress", cat, deps.StatusKey("s")),
		Bind("r", "Filter: Review", cat, deps.StatusKey("r")),
		Bind("S", "Filter: Standby", cat, deps.StatusKey("S")),
		Bind("b", "Filter: Blocked", cat, deps.StatusKey("b")),
		Bind("d", "Filter: Done", cat, deps.StatusKey("d")),
		Bind("x", "Filter: Canceled", cat, deps.StatusKey("x")),
		Bind("0", "Clear status filter", cat, deps.StatusKey("0")),
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
		Bind("c", "Score ≥ custom threshold", cat, deps.ScoreKey("c")),
		Bind("0", "Clear score filter", cat, deps.ScoreKey("0")),
	}

	// --- F. : stage filter (optional, dynamic) ---
	var fDotEntry *Entry
	if deps.StageFilterKey != nil {
		fDotChildren := []Entry{
			Bind("0", "Clear stage filter", cat, deps.StageFilterKey("0")),
		}
		for i, stage := range deps.StageFilterNames {
			key := stageKeyForIndex(i)
			if key == "" {
				break
			}
			name := stage
			fDotChildren = append(fDotChildren,
				Bind(key, "Filter stage: "+name, cat, deps.StageFilterKey(key)),
			)
		}
		e := Prefix(".", "Filter by stage", cat, fDotChildren)
		fDotEntry = &e
	}

	// --- Assemble F children ---
	fChildren := []Entry{
		Prefix("t", "Filter by type (sub-popup)", cat, ftChildren),
		Prefix("s", "Filter by status (sub-popup)", cat, fsChildren),
	}
	if fdEntry != nil {
		fChildren = append(fChildren, *fdEntry)
	}
	fChildren = append(fChildren,
		Bind("a", "Filter by attribute (picker)", cat, deps.OpenAttrPicker),
		Prefix("S", "Score threshold filter", cat, fsScoreChildren),
	)
	if fDotEntry != nil {
		fChildren = append(fChildren, *fDotEntry)
	}
	fChildren = append(fChildren,
		Bind("#", "Filter by tags", cat, deps.FilterByTags),
		Bind("p", "Toggle priority filter", cat, deps.TogglePriority),
		Bind("h", "Toggle hide-dimmed filter", cat, deps.ToggleHideDimmed),
		Bind("x", "Clear all filters", cat, deps.ClearFilters),
	)

	return []Entry{Prefix("F", "Filter prefix (type/status/tags/attr/score)", cat, fChildren)}
}
