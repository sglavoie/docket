package tui

import "github.com/sglavoie/tt/internal/tui/presentation"

// Stable core operation IDs let the final grammar remain independent of dormant
// legacy command adapters and their retired shortcuts or prefix chords.
const (
	operationCoreCapture            OperationID = "tui.core.capture"
	operationCoreEdit               OperationID = "tui.core.edit"
	operationCoreEditDescription    OperationID = "tui.core.edit-description"
	operationCoreExternalEditor     OperationID = "tui.core.external-editor"
	operationCoreOutlineEditor      OperationID = "tui.core.outline-editor"
	operationCoreBacklog            OperationID = "tui.core.backlog"
	operationCoreToday              OperationID = "tui.core.today"
	operationCoreNext               OperationID = "tui.core.next"
	operationCoreActions            OperationID = "tui.core.actions"
	operationCoreCursorDown         OperationID = "tui.core.cursor-down"
	operationCoreCursorUp           OperationID = "tui.core.cursor-up"
	operationCoreCursorTop          OperationID = "tui.core.cursor-top"
	operationCoreCursorBottom       OperationID = "tui.core.cursor-bottom"
	operationCoreCursorPageUp       OperationID = "tui.core.cursor-page-up"
	operationCoreCursorPageDown     OperationID = "tui.core.cursor-page-down"
	operationCoreScrollCursorTop    OperationID = "tui.core.scroll-cursor-top"
	operationCoreScrollCursorMiddle OperationID = "tui.core.scroll-cursor-middle"
	operationCoreScrollCursorBottom OperationID = "tui.core.scroll-cursor-bottom"
	operationCoreNextHeader         OperationID = "tui.core.next-header"
	operationCorePreviousHeader     OperationID = "tui.core.previous-header"
	operationCoreJumpBackward       OperationID = "tui.core.jump-backward"
	operationCoreJumpForward        OperationID = "tui.core.jump-forward"
	operationCoreFocusLatest        OperationID = "tui.core.focus-latest"
	operationCoreGotoTaskProject    OperationID = "tui.core.goto-task-project"
	operationCoreSelectPinned       OperationID = "tui.core.select-pinned-project"
	operationCoreCollapseParent     OperationID = "tui.core.collapse-parent"
	operationCoreExpand             OperationID = "tui.core.expand"
	operationCoreToggleSections     OperationID = "tui.core.toggle-sections"
	operationCoreToggleProjectGroup OperationID = "tui.core.toggle-project-groups"
	operationCoreToggleSubtrees     OperationID = "tui.core.toggle-subtrees"
	operationCoreClearFolds         OperationID = "tui.core.clear-folds"
	operationCoreReorderDown        OperationID = "tui.core.reorder-down"
	operationCoreReorderUp          OperationID = "tui.core.reorder-up"
	operationCoreReorderBottom      OperationID = "tui.core.reorder-bottom"
	operationCoreReorderTop         OperationID = "tui.core.reorder-top"
	operationCoreFocusSubtree       OperationID = "tui.core.focus-subtree"
	operationCoreFocusParent        OperationID = "tui.core.focus-parent"
	operationCoreActivateGroup      OperationID = "tui.core.activate-group"
	operationCoreZoomSection        OperationID = "tui.core.zoom-section"
	operationCoreOpenNote           OperationID = "tui.core.open-note"
	operationCorePeek               OperationID = "tui.core.peek"
	operationCoreScopeSearch        OperationID = "tui.core.scope-search"
	operationCoreSelectProject      OperationID = "tui.core.select-project"
	operationCoreMultiSelect        OperationID = "tui.core.multi-select"
	operationCoreRangeSelect        OperationID = "tui.core.range-select"
	operationCoreSelectAll          OperationID = "tui.core.select-all"
	operationCoreNamedLenses        OperationID = "tui.core.named-lenses"
	operationCoreConflicts          OperationID = "tui.core.conflicts"
	operationCoreQueryReadout       OperationID = "tui.core.query-readout"
	operationCoreNotes              OperationID = "tui.core.notes"
	operationCoreToggleTheme        OperationID = "tui.core.toggle-theme"
	operationCoreToggleMenuBar      OperationID = "tui.core.toggle-menu-bar"
	operationCoreOpenMenuBar        OperationID = "tui.core.open-menu-bar"
	operationCoreToggleAdvisories   OperationID = "tui.core.toggle-advisories"
	operationCoreUndo               OperationID = "tui.core.undo"
	operationCoreRedo               OperationID = "tui.core.redo"
	operationCoreBack               OperationID = "tui.core.back"
	operationCoreHelp               OperationID = "tui.core.help"
	operationCoreQuit               OperationID = "tui.core.quit"
	operationCoreNextWorkMode       OperationID = "tui.core.next-work-mode"
	operationCorePreviousWorkMode   OperationID = "tui.core.previous-work-mode"
	operationCoreGotoAll            OperationID = "tui.core.goto-all"
	operationCoreGotoInbox          OperationID = "tui.core.goto-inbox"
	operationCoreGotoToday          OperationID = "tui.core.goto-today"
	operationCoreGotoNext           OperationID = "tui.core.goto-next"
	operationCoreGotoBacklog        OperationID = "tui.core.goto-backlog"
	operationCoreGotoDue            OperationID = "tui.core.goto-due"
	operationCoreGotoRecent         OperationID = "tui.core.goto-recent"
	operationCoreResetToLaunchView  OperationID = "tui.core.reset-to-launch-view"
)

func coreOutlinerApplicability(ctx CommandContext) bool {
	return ctx.View == presentation.ViewCrossProjectOutliner
}

func coreInputOperationDescriptors() []OperationDescriptor {
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
			Applicability:        coreOutlinerApplicability,
			Risk:                 risk,
		}
	}
	// Single-task operations must not inherit a visual selection or a project
	// default when the cursor does not identify one task.
	withTaskTarget := func(descriptor OperationDescriptor) OperationDescriptor {
		descriptor.TargetSource = OperationTargetCurrentTask
		descriptor.Applicability = func(ctx CommandContext) bool {
			return coreOutlinerApplicability(ctx) && !ctx.Selection.Visual
		}
		return descriptor
	}
	peek := withTaskTarget(core(
		operationCorePeek,
		"Toggle Task Peek",
		OperationCategoryNavigation,
		TargetCardinalityOne,
		OperationRiskReadOnly,
		"peek task",
		"task preview",
	))
	peek.Applicability = func(ctx CommandContext) bool {
		return coreOutlinerApplicability(ctx) &&
			!ctx.Selection.Visual &&
			!ctx.Entity.HasChecklistCursor &&
			currentTaskIdentity(ctx).Valid()
	}
	focusSubtree := withTaskTarget(core(
		operationCoreFocusSubtree,
		"Focus Subtree",
		OperationCategoryNavigation,
		TargetCardinalityOne,
		OperationRiskReadOnly,
		"focus task",
		"focus descendants",
	))
	focusSubtree.Applicability = func(ctx CommandContext) bool {
		return coreOutlinerApplicability(ctx) &&
			!ctx.Selection.Visual &&
			!ctx.Entity.HasChecklistCursor &&
			currentTaskIdentity(ctx).Valid()
	}
	focusParent := withTaskTarget(core(
		operationCoreFocusParent,
		"Focus Parent",
		OperationCategoryNavigation,
		TargetCardinalityOne,
		OperationRiskReadOnly,
		"focus parent",
		"widen focus",
	))
	focusParent.Applicability = focusSubtree.Applicability
	gotoTaskProject := withTaskTarget(core(
		operationCoreGotoTaskProject,
		"Go to Task's Project",
		OperationCategoryNavigation,
		TargetCardinalityOne,
		OperationRiskReadOnly,
		"open task project",
		"scope to task project",
	))
	gotoTaskProject.Applicability = func(ctx CommandContext) bool {
		return coreOutlinerApplicability(ctx) &&
			!ctx.Selection.Visual &&
			!ctx.Entity.HasChecklistCursor &&
			currentTaskIdentity(ctx).Valid()
	}
	zoomSection := core(
		operationCoreZoomSection,
		"Zoom to Section",
		OperationCategoryNavigation,
		TargetCardinalityNone,
		OperationRiskReadOnly,
		"zoom section",
		"section only",
		"narrow to section",
	)
	zoomSection.Applicability = func(ctx CommandContext) bool {
		return coreOutlinerApplicability(ctx) &&
			!ctx.Selection.Visual &&
			!ctx.Entity.HasChecklistCursor &&
			ctx.Entity.HasSectionHeaderCursor
	}
	activateGroup := core(
		operationCoreActivateGroup,
		"Open Project",
		OperationCategoryNavigation,
		TargetCardinalityNone,
		OperationRiskReadOnly,
		"focus project",
		"scope to project",
	)
	activateGroup.Applicability = func(ctx CommandContext) bool {
		return coreOutlinerApplicability(ctx) &&
			!ctx.Selection.Visual &&
			!ctx.Entity.HasChecklistCursor &&
			!ctx.Entity.HasSectionHeaderCursor &&
			!ctx.Entity.HasNoteCursor &&
			!currentTaskIdentity(ctx).Valid()
	}
	// The notes tier's rows name nothing the entity grammar resolves, so a note
	// row reads as an empty entity and would otherwise land on Open Project. It
	// is the fourth Enter act rather than a branch inside one of the other three,
	// for the same reason those three are three: it carries its own target and
	// its own vocabulary, and the footer owes the row an honest label.
	openNote := core(
		operationCoreOpenNote,
		"Open Note",
		OperationCategoryNote,
		TargetCardinalityNone,
		OperationRiskReadOnly,
		"open note",
		"note in results",
	)
	openNote.Applicability = func(ctx CommandContext) bool {
		return coreOutlinerApplicability(ctx) &&
			!ctx.Selection.Visual &&
			ctx.Entity.HasNoteCursor
	}
	// The outline editor restructures a whole project tree, so it names no task
	// target and refuses a visual selection: the buffer's scope is the project the
	// cursor sits in, and a selection would claim a narrower one it cannot honor.
	outlineEditor := core(
		operationCoreOutlineEditor,
		"Outline Edit Project",
		OperationCategoryProject,
		TargetCardinalityNone,
		OperationRiskMutation,
		"bulk restructure",
		"outline buffer",
	)
	outlineEditor.Applicability = func(ctx CommandContext) bool {
		return coreOutlinerApplicability(ctx) && !ctx.Selection.Visual
	}
	descriptors := []OperationDescriptor{
		core(operationCoreCapture, "Capture", OperationCategoryTask, TargetCardinalityNone, OperationRiskMutation, "add task", "inbox capture"),
		withTaskTarget(core(operationCoreEdit, "Edit Task", OperationCategoryTask, TargetCardinalityOne, OperationRiskMutation, "edit task line", "rename task", "change title")),
		withTaskTarget(core(operationCoreEditDescription, "Edit Description", OperationCategoryTask, TargetCardinalityOne, OperationRiskMutation, "change description", "edit body")),
		withTaskTarget(core(operationCoreExternalEditor, "Open in External Editor", OperationCategoryTask, TargetCardinalityOne, OperationRiskMutation, "edit task externally", "external editor")),
		outlineEditor,
		core(operationCoreBacklog, "Move to Backlog", OperationCategoryTask, TargetCardinalityOneOrMore, OperationRiskMutation, "backlog"),
		core(operationCoreToday, "Toggle Today", OperationCategoryTask, TargetCardinalityOneOrMore, OperationRiskMutation, "today"),
		core(operationCoreNext, "Toggle Next", OperationCategoryTask, TargetCardinalityOneOrMore, OperationRiskMutation, "next"),
		core(operationCoreActions, "Task Actions", OperationCategoryTask, TargetCardinalityOneOrMore, OperationRiskReadOnly, "action sheet"),
		core(operationCoreCursorDown, "Move Cursor Down", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreCursorUp, "Move Cursor Up", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreCursorTop, "Go to First Row", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreCursorBottom, "Go to Last Row", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreCursorPageUp, "Half Page Up", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreCursorPageDown, "Half Page Down", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreScrollCursorTop, "Scroll Cursor to Top of Screen", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "scroll top", "row to top"),
		core(operationCoreScrollCursorMiddle, "Scroll Cursor to Middle of Screen", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "scroll middle", "row to middle"),
		core(operationCoreScrollCursorBottom, "Scroll Cursor to Bottom of Screen", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "scroll bottom", "row to bottom"),
		core(operationCoreNextHeader, "Next Section", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "jump to next section", "next group"),
		core(operationCorePreviousHeader, "Previous Section", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "jump to previous section", "previous group"),
		core(operationCoreJumpBackward, "Jump to Older Task Position", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreJumpForward, "Jump to Newer Task Position", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreFocusLatest, "Focus Latest Activity", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "latest activity", "last touched"),
		core(operationCoreCollapseParent, "Collapse or Go to Parent", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreExpand, "Expand", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		// The three fold axes are three toggles, not six directional operations:
		// each owns one map, and the aliases stay direction-free because a toggle
		// must not advertise a way it does not always go.
		core(operationCoreToggleSections, "Toggle All Sections", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "fold sections"),
		core(operationCoreToggleProjectGroup, "Toggle All Project Groups", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "fold project groups"),
		core(operationCoreToggleSubtrees, "Toggle All Subtrees", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "fold subtrees"),
		// The first operation in the grammar with no key at all. Opening every axis
		// at once is what the three toggles cannot spell, since each reverses and
		// each owns one axis; it is also rare enough that spending a letter on it
		// would cost more than it returns. The menu bar is what makes a keyless
		// operation reachable, and this is the category that makes the bar more than
		// a second way to press a key someone already knows.
		core(operationCoreClearFolds, "Clear All Folds", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "unfold everything", "open all folds"),
		core(operationCoreReorderDown, "Reorder Down", OperationCategoryNavigation, TargetCardinalityOne, OperationRiskMutation),
		core(operationCoreReorderUp, "Reorder Up", OperationCategoryNavigation, TargetCardinalityOne, OperationRiskMutation),
		core(operationCoreReorderBottom, "Reorder to Bottom", OperationCategoryNavigation, TargetCardinalityOne, OperationRiskMutation),
		core(operationCoreReorderTop, "Reorder to Top", OperationCategoryNavigation, TargetCardinalityOne, OperationRiskMutation),
		focusSubtree,
		focusParent,
		gotoTaskProject,
		zoomSection,
		activateGroup,
		openNote,
		peek,
		core(operationCoreScopeSearch, "Search Scoped Tasks", OperationCategoryFilter, TargetCardinalityNone, OperationRiskReadOnly, "scope-wide search"),
		core(operationCoreSelectProject, "Select Project", OperationCategoryProject, TargetCardinalityNone, OperationRiskReadOnly, "projects", "switch project"),
		core(operationCoreSelectPinned, "Select Pinned Project", OperationCategoryProject, TargetCardinalityNone, OperationRiskReadOnly, "pinned projects", "switch pinned project"),
		core(operationCoreMultiSelect, "Toggle Task Selection", OperationCategoryTask, TargetCardinalityNone, OperationRiskReadOnly, "visual selection"),
		core(operationCoreRangeSelect, "Select Task Range", OperationCategoryTask, TargetCardinalityNone, OperationRiskReadOnly, "contiguous selection", "range selection"),
		core(operationCoreSelectAll, "Select All Visible Tasks", OperationCategoryTask, TargetCardinalityNone, OperationRiskReadOnly, "select visible"),
		// A lens is a saved projection over the mode, query, filter, and sort axes,
		// so the key that opens the list groups with the facets that decide the same
		// axes rather than with a surface of its own.
		core(
			operationCoreNamedLenses,
			"Named Lenses",
			OperationCategoryFilter,
			TargetCardinalityNone,
			OperationRiskReadOnly,
			"lenses",
			"saved views",
			"save current view",
		),
		// The browser reads the same directory the status strip counts, so the key
		// that opens it is the strip's line made actionable rather than a surface of
		// its own. It stays read-only: resolving is `tt conflict resolve`.
		core(
			operationCoreConflicts,
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
			operationCoreQueryReadout,
			"Query Readout",
			OperationCategoryFilter,
			TargetCardinalityNone,
			OperationRiskReadOnly,
			"narrowing",
			"what is filtered",
			"peel filters",
		),
		core(operationCoreNotes, "Notes", OperationCategoryNote, TargetCardinalityNone, OperationRiskReadOnly, "scratch notes"),
		core(operationCoreToggleTheme, "Toggle Theme", OperationCategorySystem, TargetCardinalityNone, OperationRiskMutation, "dark theme", "light theme"),
		core(operationCoreToggleMenuBar, "Toggle Menu Bar", OperationCategorySystem, TargetCardinalityNone, OperationRiskMutation, "menu bar", "show menus"),
		// Opening is a separate act from showing: the toggle decides whether the
		// band is drawn at all, this one drops a menu out of it. They are two
		// descriptors so `?` can name both honestly rather than one label having to
		// mean "show, or open, depending on what is already true".
		core(operationCoreOpenMenuBar, "Open Menu", OperationCategorySystem, TargetCardinalityNone, OperationRiskMutation, "open menu bar", "browse menus"),
		// The second operation reached only from the menu bar. It is target-free
		// like Clear All Folds, which is what lets it stay keyless: it acts on the
		// session's chrome, never on a task or a project. `--no-advisories` is the
		// other way in, so nothing here spends a root key on it.
		core(operationCoreToggleAdvisories, "Toggle Advisories", OperationCategorySystem, TargetCardinalityNone, OperationRiskMutation, "conflict count", "hide warnings"),
		core(operationCoreUndo, "Undo", OperationCategorySystem, TargetCardinalityNone, OperationRiskMutation),
		core(operationCoreRedo, "Redo", OperationCategorySystem, TargetCardinalityNone, OperationRiskMutation),
		core(operationCoreBack, "Back", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreHelp, "Help", OperationCategorySystem, TargetCardinalityNone, OperationRiskReadOnly, "keyboard shortcuts"),
		core(operationCoreQuit, "Quit", OperationCategorySystem, TargetCardinalityNone, OperationRiskReadOnly, "exit"),
		core(operationCoreNextWorkMode, "Next Work Mode", OperationCategoryMode, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCorePreviousWorkMode, "Previous Work Mode", OperationCategoryMode, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreGotoAll, "Go to All", OperationCategoryMode, TargetCardinalityNone, OperationRiskReadOnly, "all mode"),
		core(operationCoreGotoInbox, "Go to Inbox", OperationCategoryMode, TargetCardinalityNone, OperationRiskReadOnly, "inbox mode"),
		core(operationCoreGotoToday, "Go to Today", OperationCategoryMode, TargetCardinalityNone, OperationRiskReadOnly, "today mode"),
		core(operationCoreGotoNext, "Go to Next", OperationCategoryMode, TargetCardinalityNone, OperationRiskReadOnly, "next mode"),
		core(operationCoreGotoBacklog, "Go to Backlog", OperationCategoryMode, TargetCardinalityNone, OperationRiskReadOnly, "backlog mode"),
		core(operationCoreGotoDue, "Go to Due", OperationCategoryMode, TargetCardinalityNone, OperationRiskReadOnly, "due mode"),
		core(operationCoreGotoRecent, "Go to Recent", OperationCategoryMode, TargetCardinalityNone, OperationRiskReadOnly, "recent mode", "recent activity"),
		// The digit that names no mode returns the view to the one a lens-free
		// launch produces. It is a Mode operation because that is the axis it is
		// spelled beside — 0 leads the seven — and read-only because every axis it
		// drops is a view axis: it writes no task, no project, and not even the
		// machine-local pointer naming which lens to open with.
		core(
			operationCoreResetToLaunchView,
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
		if !coreInputOperationRequiresTask(descriptors[i].ID) {
			continue
		}
		inner := descriptors[i].Applicability
		descriptors[i].Applicability = func(ctx CommandContext) bool {
			return inner(ctx) && (ctx.Selection.Visual || !commandContextNamesNonTaskEntity(ctx))
		}
	}
	return descriptors
}
