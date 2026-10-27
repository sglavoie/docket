package operation

func CoreInputOperationDescriptors() []OperationDescriptor {
	core := func(
		id OperationID,
		label string,
		category OperationCategory,
		cardinality TargetCardinality,
		risk OperationRisk,
		aliases ...string,
	) OperationDescriptor {
		return OperationDescriptor{
			ID:                   id,
			Label:                label,
			Aliases:              aliases,
			Category:             category,
			TargetCardinality:    cardinality,
			RequiredCapabilities: NewCapabilitySet(CapabilitySimplifiedInput),
			Applicability:        CoreOutlinerApplicability,
			Risk:                 risk,
		}
	}
	// Single-task operations must not inherit a visual selection or a project
	// default when the cursor does not identify one task.
	withTaskTarget := func(descriptor OperationDescriptor) OperationDescriptor {
		descriptor.TargetSource = OperationTargetCurrentTask
		descriptor.Applicability = func(ctx CommandContext) bool {
			return CoreOutlinerApplicability(ctx) && !ctx.Selection.Visual
		}
		return descriptor
	}
	// Peek and its four scrolling keys read the focused entity rather than a task:
	// the pane draws a task's detail and a project header's alike, so the target
	// source has to name whichever row the cursor is on. Navigation is what makes
	// that safe — the resolver refuses a non-task entity to a task-category
	// descriptor, and these five mutate nothing in either kind.
	withEntityTarget := func(descriptor OperationDescriptor) OperationDescriptor {
		descriptor.TargetSource = OperationTargetCurrentEntity
		return descriptor
	}
	peek := withEntityTarget(core(
		OperationCorePeek,
		"Toggle Peek",
		OperationCategoryNavigation,
		TargetCardinalityOne,
		OperationRiskReadOnly,
		"peek task",
		"peek project",
		"peek checklist item",
		"task preview",
		"project preview",
	))
	peek.Applicability = func(ctx CommandContext) bool {
		if !CoreOutlinerApplicability(ctx) {
			return false
		}
		// An open dock closes from wherever the cursor is. The selection
		// exclusion applies to opening: entering a selection closes the
		// dock, so a visual selection never reaches the first branch.
		if ctx.Peek.Visible {
			return true
		}
		return !ctx.Selection.Visual &&
			CommandContextNamesPeekTarget(ctx)
	}
	// The four scrolling keys are Peek's own while a Peek is on screen: a pane
	// that scrolls beside a list has to say which of the two a key moves, and the
	// pane the reader is looking at is the honest answer. They name an entity
	// target for the same reason Peek does — the document belongs to the row under
	// the cursor — and every one of them is applicable on the visible document alone.
	// Ctrl+D and Ctrl+U share their keys with the list's own half page, so
	// applicability is what hands the press to the pane; were it narrowed to a
	// window with somewhere to go, the key at the end of a document would fall
	// through to the sibling binding and jump the cursor instead. Whether a
	// movement is worth offering is a separate question, and PeekContext's two
	// direction fields are where the footer asks it.
	visiblePeek := func(ctx CommandContext) bool {
		return CoreOutlinerApplicability(ctx) && ctx.Peek.Visible
	}
	peekSize := core(OperationCorePeekSize, "Expand / Compact Peek", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "resize preview")
	peekSize.Applicability = visiblePeek
	peekHalfPageDown := withEntityTarget(core(
		OperationCorePeekHalfPageDown,
		"Scroll Peek Down",
		OperationCategoryNavigation,
		TargetCardinalityOne,
		OperationRiskReadOnly,
		"half page peek down",
		"more task detail",
	))
	peekHalfPageDown.Applicability = visiblePeek
	peekHalfPageUp := withEntityTarget(core(
		OperationCorePeekHalfPageUp,
		"Scroll Peek Up",
		OperationCategoryNavigation,
		TargetCardinalityOne,
		OperationRiskReadOnly,
		"half page peek up",
		"previous task detail",
	))
	peekHalfPageUp.Applicability = visiblePeek
	peekPageDown := withEntityTarget(core(
		OperationCorePeekPageDown,
		"Page Peek Down",
		OperationCategoryNavigation,
		TargetCardinalityOne,
		OperationRiskReadOnly,
		"full page peek down",
		"next peek page",
	))
	peekPageDown.Applicability = visiblePeek
	peekPageUp := withEntityTarget(core(
		OperationCorePeekPageUp,
		"Page Peek Up",
		OperationCategoryNavigation,
		TargetCardinalityOne,
		OperationRiskReadOnly,
		"full page peek up",
		"previous peek page",
	))
	peekPageUp.Applicability = visiblePeek
	focusSubtree := withTaskTarget(core(
		OperationCoreFocusSubtree,
		"Focus Subtree",
		OperationCategoryNavigation,
		TargetCardinalityOne,
		OperationRiskReadOnly,
		"focus task",
		"focus descendants",
	))
	focusSubtree.Applicability = func(ctx CommandContext) bool {
		return CoreOutlinerApplicability(ctx) &&
			!ctx.Selection.Visual &&
			!ctx.Entity.HasChecklistCursor &&
			CurrentTaskIdentity(ctx).Valid()
	}
	focusParent := withTaskTarget(core(
		OperationCoreFocusParent,
		"Focus Parent",
		OperationCategoryNavigation,
		TargetCardinalityOne,
		OperationRiskReadOnly,
		"focus parent",
		"widen focus",
	))
	focusParent.Applicability = focusSubtree.Applicability
	gotoParentTask := withTaskTarget(core(
		OperationCoreGotoParentTask,
		"Go to Parent Task",
		OperationCategoryNavigation,
		TargetCardinalityOne,
		OperationRiskReadOnly,
		"go parent task",
		"jump to parent task",
	))
	gotoParentTask.Applicability = func(ctx CommandContext) bool {
		return CoreOutlinerApplicability(ctx) &&
			!ctx.Selection.Visual && CurrentTaskIdentity(ctx).Valid()
	}
	gotoTaskProject := withTaskTarget(core(
		OperationCoreGotoTaskProject,
		"Go to Task's Project",
		OperationCategoryNavigation,
		TargetCardinalityOne,
		OperationRiskReadOnly,
		"open task project",
		"scope to task project",
	))
	gotoTaskProject.Applicability = func(ctx CommandContext) bool {
		return CoreOutlinerApplicability(ctx) &&
			!ctx.Selection.Visual &&
			!ctx.Entity.HasChecklistCursor &&
			CurrentTaskIdentity(ctx).Valid()
	}
	// The relationship picker takes the same gates as the jump above it: one task,
	// no visual selection, and not a checklist row. It is read-only in the same
	// sense too — a hop moves the view and writes no reference.
	gotoRelation := withTaskTarget(core(
		OperationCoreGotoRelation,
		"Go to Related Task",
		OperationCategoryNavigation,
		TargetCardinalityOne,
		OperationRiskReadOnly,
		"related tasks",
		"dependencies and dependents",
		"walk references",
	))
	gotoRelation.Applicability = gotoTaskProject.Applicability
	zoomSection := core(
		OperationCoreZoomSection,
		"Zoom to Section",
		OperationCategoryNavigation,
		TargetCardinalityNone,
		OperationRiskReadOnly,
		"zoom section",
		"section only",
		"narrow to section",
	)
	zoomSection.Applicability = func(ctx CommandContext) bool {
		return CoreOutlinerApplicability(ctx) &&
			!ctx.Selection.Visual &&
			!ctx.Entity.HasChecklistCursor &&
			ctx.Entity.HasSectionHeaderCursor
	}
	activateGroup := core(
		OperationCoreActivateGroup,
		"Open Project",
		OperationCategoryNavigation,
		TargetCardinalityNone,
		OperationRiskReadOnly,
		"focus project",
		"scope to project",
	)
	activateGroup.Applicability = openProjectApplicability
	// The notes tier's rows name nothing the entity grammar resolves, so a note
	// row reads as an empty entity and would otherwise land on Open Project. It
	// is the fourth Enter act rather than a branch inside one of the other three,
	// for the same reason those three are three: it carries its own target and
	// its own vocabulary, and the footer owes the row an honest label.
	openNote := core(
		OperationCoreOpenNote,
		"Open Note",
		OperationCategoryNote,
		TargetCardinalityNone,
		OperationRiskReadOnly,
		"open note",
		"note in results",
	)
	openNote.Applicability = func(ctx CommandContext) bool {
		return CoreOutlinerApplicability(ctx) &&
			!ctx.Selection.Visual &&
			ctx.Entity.HasNoteCursor
	}
	// The outline editor restructures a whole project tree, so it names no task
	// target and refuses a visual selection: the buffer's scope is the project the
	// cursor sits in, and a selection would claim a narrower one it cannot honor.
	outlineEditor := core(
		OperationCoreOutlineEditor,
		"Outline Edit Project",
		OperationCategoryProject,
		TargetCardinalityNone,
		OperationRiskMutation,
		"bulk restructure",
		"outline buffer",
	)
	outlineEditor.Applicability = func(ctx CommandContext) bool {
		return CoreOutlinerApplicability(ctx) && !ctx.Selection.Visual
	}
	descriptors := []OperationDescriptor{
		core(OperationCoreCapture, "Capture", OperationCategoryTask, TargetCardinalityNone, OperationRiskMutation, "add task", "inbox capture"),
		withTaskTarget(core(OperationCoreEdit, "Edit Task", OperationCategoryTask, TargetCardinalityOne, OperationRiskMutation, "edit task line", "rename task", "change title")),
		withTaskTarget(core(OperationCoreEditDescription, "Edit Description", OperationCategoryTask, TargetCardinalityOne, OperationRiskMutation, "change description", "edit body")),
		withTaskTarget(core(OperationCoreExternalEditor, "Open in External Editor", OperationCategoryTask, TargetCardinalityOne, OperationRiskMutation, "edit task externally", "external editor")),
		outlineEditor,
		core(OperationCoreBacklog, "Move to Backlog", OperationCategoryTask, TargetCardinalityOneOrMore, OperationRiskMutation, "backlog"),
		core(OperationCoreToday, "Toggle Today Plan", OperationCategoryTask, TargetCardinalityOneOrMore, OperationRiskMutation, "today"),
		core(OperationCoreNext, "Toggle Next Plan", OperationCategoryTask, TargetCardinalityOneOrMore, OperationRiskMutation, "next"),
		core(OperationCoreActions, "Task Actions", OperationCategoryTask, TargetCardinalityOneOrMore, OperationRiskReadOnly, "action sheet"),
		core(OperationCoreCursorDown, "Move Cursor Down", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(OperationCoreCursorUp, "Move Cursor Up", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(OperationCoreCursorTop, "Go to First Row", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(OperationCoreCursorBottom, "Go to Last Row", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(OperationCoreCursorPageUp, "Half Page Up", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(OperationCoreCursorPageDown, "Half Page Down", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(OperationCoreScrollCursorTop, "Scroll Cursor to Top of Screen", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "scroll top", "row to top"),
		core(OperationCoreScrollCursorMiddle, "Scroll Cursor to Middle of Screen", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "scroll middle", "row to middle"),
		core(OperationCoreScrollCursorBottom, "Scroll Cursor to Bottom of Screen", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "scroll bottom", "row to bottom"),
		core(OperationCoreNextHeader, "Next Section", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "jump to next section", "next group"),
		core(OperationCorePreviousHeader, "Previous Section", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "jump to previous section", "previous group"),
		core(OperationCoreJumpBackward, "Jump to Older Task Position", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(OperationCoreJumpForward, "Jump to Newer Task Position", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(OperationCoreFocusLatest, "Focus Latest Activity", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "latest activity", "last touched"),
		core(OperationCoreCollapseParent, "Collapse or Go to Parent", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(OperationCoreExpand, "Expand", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		// The three fold axes are three toggles, not six directional operations:
		// each owns one map, and the aliases stay direction-free because a toggle
		// must not advertise a way it does not always go.
		core(OperationCoreToggleSections, "Toggle All Sections", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "fold sections"),
		core(OperationCoreToggleProjectGroup, "Toggle All Project Groups", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "fold project groups"),
		core(OperationCoreToggleSubtrees, "Toggle All Subtrees", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "fold subtrees"),
		// The first operation in the grammar with no key at all. Opening every axis
		// at once is what the three toggles cannot spell, since each reverses and
		// each owns one axis; it is also rare enough that spending a letter on it
		// would cost more than it returns. The menu bar is what makes a keyless
		// operation reachable, and this is the category that makes the bar more than
		// a second way to press a key someone already knows.
		core(OperationCoreClearFolds, "Clear All Folds", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "unfold everything", "open all folds"),
		core(OperationCoreReorderDown, "Reorder Down", OperationCategoryNavigation, TargetCardinalityOne, OperationRiskMutation),
		core(OperationCoreReorderUp, "Reorder Up", OperationCategoryNavigation, TargetCardinalityOne, OperationRiskMutation),
		core(OperationCoreReorderBottom, "Reorder to Bottom", OperationCategoryNavigation, TargetCardinalityOne, OperationRiskMutation),
		core(OperationCoreReorderTop, "Reorder to Top", OperationCategoryNavigation, TargetCardinalityOne, OperationRiskMutation),
		focusSubtree,
		focusParent,
		gotoParentTask,
		gotoTaskProject,
		gotoRelation,
		zoomSection,
		activateGroup,
		openNote,
		peek,
		peekSize,
		peekHalfPageDown,
		peekHalfPageUp,
		peekPageDown,
		peekPageUp,
		core(OperationCoreScopeSearch, "Search Scoped Tasks", OperationCategoryFilter, TargetCardinalityNone, OperationRiskReadOnly, "scope-wide search"),
		core(OperationCoreSelectProject, "Select Project", OperationCategoryProject, TargetCardinalityNone, OperationRiskReadOnly, "projects", "switch project"),
		core(OperationCoreSelectPinned, "Select Pinned Project", OperationCategoryProject, TargetCardinalityNone, OperationRiskReadOnly, "pinned projects", "switch pinned project"),
		core(OperationCoreMultiSelect, "Toggle Task Selection", OperationCategoryTask, TargetCardinalityNone, OperationRiskReadOnly, "visual selection"),
		core(OperationCoreRangeSelect, "Select Task Range", OperationCategoryTask, TargetCardinalityNone, OperationRiskReadOnly, "contiguous selection", "range selection"),
		core(OperationCoreSelectAll, "Select All Visible Tasks", OperationCategoryTask, TargetCardinalityNone, OperationRiskReadOnly, "select visible"),
		// A lens is a saved projection over the mode, query, filter, and sort axes,
		// so the key that opens the list groups with the facets that decide the same
		// axes rather than with a surface of its own.
		core(
			OperationCoreNamedLenses,
			"Named Lenses",
			OperationCategoryFilter,
			TargetCardinalityNone,
			OperationRiskReadOnly,
			"lenses",
			"saved views",
			"save current view",
		),
		core(OperationCoreSyncNow, "Sync Now", OperationCategorySystem, TargetCardinalityNone, OperationRiskMutation, "exchange", "publish", "sync"),
		// The browser reads the same directory the status strip counts, so the key
		// that opens it is the strip's line made actionable rather than a surface of
		// its own. The overlay owns diff, merge editing, and resolution.
		core(
			OperationCoreConflicts,
			"Merge Conflicts",
			OperationCategorySystem,
			TargetCardinalityNone,
			OperationRiskReadOnly,
			"conflicts",
			"merge conflicts",
			"unresolved conflicts",
		),
		// The readout expands what the operational title already names, so it is
		// registered once here and its key, its `?` entry, and its footer label are
		// all projections of this one descriptor. The peel digits inside it are the
		// gated keymap of a modal sub-state, not a second grammar: they exist only
		// while the stack is open and the footer says so for as long as they do.
		core(
			OperationCoreQueryReadout,
			"Explain Current View",
			OperationCategoryFilter,
			TargetCardinalityNone,
			OperationRiskReadOnly,
			"narrowing",
			"what is filtered",
			"peel filters",
		),
		core(OperationCoreNotes, "Notes", OperationCategoryNote, TargetCardinalityNone, OperationRiskReadOnly, "scratch notes"),
		core(OperationCoreToggleTheme, "Toggle Theme", OperationCategorySystem, TargetCardinalityNone, OperationRiskMutation, "dark theme", "light theme"),
		core(OperationCoreToggleMenuBar, "Toggle Menu Bar", OperationCategorySystem, TargetCardinalityNone, OperationRiskMutation, "menu bar", "show menus"),
		// Opening is a separate act from showing: the toggle decides whether the
		// band is drawn at all, this one drops a menu out of it. They are two
		// descriptors so `?` can name both honestly rather than one label having to
		// mean "show, or open, depending on what is already true".
		core(OperationCoreOpenMenuBar, "Open Menu", OperationCategorySystem, TargetCardinalityNone, OperationRiskMutation, "open menu bar", "browse menus"),
		// The second operation reached only from the menu bar. It is target-free
		// like Clear All Folds, which is what lets it stay keyless: it acts on the
		// session's chrome, never on a task or a project. `--no-advisories` is the
		// other way in, so nothing here spends a root key on it.
		core(OperationCoreToggleAdvisories, "Toggle Advisories", OperationCategorySystem, TargetCardinalityNone, OperationRiskMutation, "conflict count", "hide warnings"),
		// The third, on the same terms: chrome, target-free, keyless. `--no-mouse`
		// is its launcher twin, and a key spent on it would be the one key a user
		// who turned the mouse off by accident cannot click their way back to.
		core(OperationCoreToggleMouse, "Toggle Mouse", OperationCategorySystem, TargetCardinalityNone, OperationRiskMutation, "pointer", "trackpad", "scrolling", "clicks"),
		core(OperationCoreLastMessage, "Recent Messages", OperationCategorySystem, TargetCardinalityNone, OperationRiskReadOnly, "feedback", "last error", "last message"),
		core(OperationCoreClearSearchHistory, "Clear Search History", OperationCategorySystem, TargetCardinalityNone, OperationRiskMutation, "forget queries", "recall"),
		core(OperationCoreSetUpCheckIns, "Set Up Check-Ins", OperationCategorySystem, TargetCardinalityNone, OperationRiskReadOnly, "schedule active work", "check-in setup"),
		core(OperationCoreUndo, "Undo", OperationCategorySystem, TargetCardinalityNone, OperationRiskMutation),
		core(OperationCoreRedo, "Redo", OperationCategorySystem, TargetCardinalityNone, OperationRiskMutation),
		core(OperationCoreBack, "Back", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(OperationCoreHelp, "Help", OperationCategorySystem, TargetCardinalityNone, OperationRiskReadOnly, "keyboard shortcuts"),
		core(OperationCoreQuit, "Quit", OperationCategorySystem, TargetCardinalityNone, OperationRiskReadOnly, "exit"),
		core(OperationCoreNextWorkMode, "Next Work Mode", OperationCategoryMode, TargetCardinalityNone, OperationRiskReadOnly),
		core(OperationCorePreviousWorkMode, "Previous Work Mode", OperationCategoryMode, TargetCardinalityNone, OperationRiskReadOnly),
		core(OperationCoreGotoAll, "Go to All", OperationCategoryMode, TargetCardinalityNone, OperationRiskReadOnly, "all mode"),
		core(OperationCoreGotoInbox, "Go to Inbox", OperationCategoryMode, TargetCardinalityNone, OperationRiskReadOnly, "inbox mode"),
		core(OperationCoreGotoToday, "Go to Today", OperationCategoryMode, TargetCardinalityNone, OperationRiskReadOnly, "today mode"),
		core(OperationCoreGotoNext, "Go to Next", OperationCategoryMode, TargetCardinalityNone, OperationRiskReadOnly, "next mode"),
		core(OperationCoreGotoBacklog, "Go to Backlog", OperationCategoryMode, TargetCardinalityNone, OperationRiskReadOnly, "backlog mode"),
		core(OperationCoreGotoDue, "Go to Due", OperationCategoryMode, TargetCardinalityNone, OperationRiskReadOnly, "due mode"),
		core(OperationCoreGotoRecent, "Go to Recent", OperationCategoryMode, TargetCardinalityNone, OperationRiskReadOnly, "recent mode", "recent activity"),
		// The digit that names no mode returns the view to the one a lens-free
		// launch produces. It is a Mode operation because that is the axis it is
		// spelled beside — 0 leads the seven — and read-only because every axis it
		// drops is a view axis: it writes no task, no project, and not even the
		// machine-local pointer naming which lens to open with.
		core(
			OperationCoreResetToLaunchView,
			"Reset to Launch View",
			OperationCategoryMode,
			TargetCardinalityNone,
			OperationRiskReadOnly,
			"reset view",
			"launch view",
			"clear narrowing",
		),
	}
	// A cursor that positively names a project belongs to the project grammar, so
	// a core operation that consumes a task must not bind there. Without this the
	// s chord would still pend on a project header, because its three planning
	// children stayed applicable and kept the whole subtree alive.
	for i := range descriptors {
		if !CoreInputOperationRequiresTask(descriptors[i].ID) {
			continue
		}
		inner := descriptors[i].Applicability
		descriptors[i].Applicability = func(ctx CommandContext) bool {
			return inner(ctx) && (ctx.Selection.Visual || !CommandContextNamesNonTaskEntity(ctx))
		}
	}
	return descriptors
}
