package tui

import (
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
)

func coreOutlinerApplicability(ctx operation.CommandContext) bool {
	return ctx.View == presentation.ViewCrossProjectOutliner
}

func coreInputOperationDescriptors() []operation.OperationDescriptor {
	core := func(
		id operation.OperationID,
		label string,
		category operation.OperationCategory,
		cardinality operation.TargetCardinality,
		risk operation.OperationRisk,
		aliases ...string,
	) operation.OperationDescriptor {
		return operation.OperationDescriptor{
			ID:                   id,
			Label:                label,
			Aliases:              aliases,
			Category:             category,
			TargetCardinality:    cardinality,
			RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilitySimplifiedInput),
			Applicability:        coreOutlinerApplicability,
			Risk:                 risk,
		}
	}
	// Single-task operations must not inherit a visual selection or a project
	// default when the cursor does not identify one task.
	withTaskTarget := func(descriptor operation.OperationDescriptor) operation.OperationDescriptor {
		descriptor.TargetSource = operation.OperationTargetCurrentTask
		descriptor.Applicability = func(ctx operation.CommandContext) bool {
			return coreOutlinerApplicability(ctx) && !ctx.Selection.Visual
		}
		return descriptor
	}
	peek := withTaskTarget(core(
		operation.OperationCorePeek,
		"Toggle Task Peek",
		operation.OperationCategoryNavigation,
		operation.TargetCardinalityOne,
		operation.OperationRiskReadOnly,
		"peek task",
		"task preview",
	))
	peek.Applicability = func(ctx operation.CommandContext) bool {
		return coreOutlinerApplicability(ctx) &&
			!ctx.Selection.Visual &&
			!ctx.Entity.HasChecklistCursor &&
			operation.CurrentTaskIdentity(ctx).Valid()
	}
	focusSubtree := withTaskTarget(core(
		operation.OperationCoreFocusSubtree,
		"Focus Subtree",
		operation.OperationCategoryNavigation,
		operation.TargetCardinalityOne,
		operation.OperationRiskReadOnly,
		"focus task",
		"focus descendants",
	))
	focusSubtree.Applicability = func(ctx operation.CommandContext) bool {
		return coreOutlinerApplicability(ctx) &&
			!ctx.Selection.Visual &&
			!ctx.Entity.HasChecklistCursor &&
			operation.CurrentTaskIdentity(ctx).Valid()
	}
	focusParent := withTaskTarget(core(
		operation.OperationCoreFocusParent,
		"Focus Parent",
		operation.OperationCategoryNavigation,
		operation.TargetCardinalityOne,
		operation.OperationRiskReadOnly,
		"focus parent",
		"widen focus",
	))
	focusParent.Applicability = focusSubtree.Applicability
	gotoTaskProject := withTaskTarget(core(
		operation.OperationCoreGotoTaskProject,
		"Go to Task's Project",
		operation.OperationCategoryNavigation,
		operation.TargetCardinalityOne,
		operation.OperationRiskReadOnly,
		"open task project",
		"scope to task project",
	))
	gotoTaskProject.Applicability = func(ctx operation.CommandContext) bool {
		return coreOutlinerApplicability(ctx) &&
			!ctx.Selection.Visual &&
			!ctx.Entity.HasChecklistCursor &&
			operation.CurrentTaskIdentity(ctx).Valid()
	}
	zoomSection := core(
		operation.OperationCoreZoomSection,
		"Zoom to Section",
		operation.OperationCategoryNavigation,
		operation.TargetCardinalityNone,
		operation.OperationRiskReadOnly,
		"zoom section",
		"section only",
		"narrow to section",
	)
	zoomSection.Applicability = func(ctx operation.CommandContext) bool {
		return coreOutlinerApplicability(ctx) &&
			!ctx.Selection.Visual &&
			!ctx.Entity.HasChecklistCursor &&
			ctx.Entity.HasSectionHeaderCursor
	}
	activateGroup := core(
		operation.OperationCoreActivateGroup,
		"Open Project",
		operation.OperationCategoryNavigation,
		operation.TargetCardinalityNone,
		operation.OperationRiskReadOnly,
		"focus project",
		"scope to project",
	)
	activateGroup.Applicability = func(ctx operation.CommandContext) bool {
		return coreOutlinerApplicability(ctx) &&
			!ctx.Selection.Visual &&
			!ctx.Entity.HasChecklistCursor &&
			!ctx.Entity.HasSectionHeaderCursor &&
			!ctx.Entity.HasNoteCursor &&
			!operation.CurrentTaskIdentity(ctx).Valid()
	}
	// The notes tier's rows name nothing the entity grammar resolves, so a note
	// row reads as an empty entity and would otherwise land on Open Project. It
	// is the fourth Enter act rather than a branch inside one of the other three,
	// for the same reason those three are three: it carries its own target and
	// its own vocabulary, and the footer owes the row an honest label.
	openNote := core(
		operation.OperationCoreOpenNote,
		"Open Note",
		operation.OperationCategoryNote,
		operation.TargetCardinalityNone,
		operation.OperationRiskReadOnly,
		"open note",
		"note in results",
	)
	openNote.Applicability = func(ctx operation.CommandContext) bool {
		return coreOutlinerApplicability(ctx) &&
			!ctx.Selection.Visual &&
			ctx.Entity.HasNoteCursor
	}
	// The outline editor restructures a whole project tree, so it names no task
	// target and refuses a visual selection: the buffer's scope is the project the
	// cursor sits in, and a selection would claim a narrower one it cannot honor.
	outlineEditor := core(
		operation.OperationCoreOutlineEditor,
		"Outline Edit Project",
		operation.OperationCategoryProject,
		operation.TargetCardinalityNone,
		operation.OperationRiskMutation,
		"bulk restructure",
		"outline buffer",
	)
	outlineEditor.Applicability = func(ctx operation.CommandContext) bool {
		return coreOutlinerApplicability(ctx) && !ctx.Selection.Visual
	}
	descriptors := []operation.OperationDescriptor{
		core(operation.OperationCoreCapture, "Capture", operation.OperationCategoryTask, operation.TargetCardinalityNone, operation.OperationRiskMutation, "add task", "inbox capture"),
		withTaskTarget(core(operation.OperationCoreEdit, "Edit Task", operation.OperationCategoryTask, operation.TargetCardinalityOne, operation.OperationRiskMutation, "edit task line", "rename task", "change title")),
		withTaskTarget(core(operation.OperationCoreEditDescription, "Edit Description", operation.OperationCategoryTask, operation.TargetCardinalityOne, operation.OperationRiskMutation, "change description", "edit body")),
		withTaskTarget(core(operation.OperationCoreExternalEditor, "Open in External Editor", operation.OperationCategoryTask, operation.TargetCardinalityOne, operation.OperationRiskMutation, "edit task externally", "external editor")),
		outlineEditor,
		core(operation.OperationCoreBacklog, "Move to Backlog", operation.OperationCategoryTask, operation.TargetCardinalityOneOrMore, operation.OperationRiskMutation, "backlog"),
		core(operation.OperationCoreToday, "Toggle Today", operation.OperationCategoryTask, operation.TargetCardinalityOneOrMore, operation.OperationRiskMutation, "today"),
		core(operation.OperationCoreNext, "Toggle Next", operation.OperationCategoryTask, operation.TargetCardinalityOneOrMore, operation.OperationRiskMutation, "next"),
		core(operation.OperationCoreActions, "Task Actions", operation.OperationCategoryTask, operation.TargetCardinalityOneOrMore, operation.OperationRiskReadOnly, "action sheet"),
		core(operation.OperationCoreCursorDown, "Move Cursor Down", operation.OperationCategoryNavigation, operation.TargetCardinalityNone, operation.OperationRiskReadOnly),
		core(operation.OperationCoreCursorUp, "Move Cursor Up", operation.OperationCategoryNavigation, operation.TargetCardinalityNone, operation.OperationRiskReadOnly),
		core(operation.OperationCoreCursorTop, "Go to First Row", operation.OperationCategoryNavigation, operation.TargetCardinalityNone, operation.OperationRiskReadOnly),
		core(operation.OperationCoreCursorBottom, "Go to Last Row", operation.OperationCategoryNavigation, operation.TargetCardinalityNone, operation.OperationRiskReadOnly),
		core(operation.OperationCoreCursorPageUp, "Half Page Up", operation.OperationCategoryNavigation, operation.TargetCardinalityNone, operation.OperationRiskReadOnly),
		core(operation.OperationCoreCursorPageDown, "Half Page Down", operation.OperationCategoryNavigation, operation.TargetCardinalityNone, operation.OperationRiskReadOnly),
		core(operation.OperationCoreScrollCursorTop, "Scroll Cursor to Top of Screen", operation.OperationCategoryNavigation, operation.TargetCardinalityNone, operation.OperationRiskReadOnly, "scroll top", "row to top"),
		core(operation.OperationCoreScrollCursorMiddle, "Scroll Cursor to Middle of Screen", operation.OperationCategoryNavigation, operation.TargetCardinalityNone, operation.OperationRiskReadOnly, "scroll middle", "row to middle"),
		core(operation.OperationCoreScrollCursorBottom, "Scroll Cursor to Bottom of Screen", operation.OperationCategoryNavigation, operation.TargetCardinalityNone, operation.OperationRiskReadOnly, "scroll bottom", "row to bottom"),
		core(operation.OperationCoreNextHeader, "Next Section", operation.OperationCategoryNavigation, operation.TargetCardinalityNone, operation.OperationRiskReadOnly, "jump to next section", "next group"),
		core(operation.OperationCorePreviousHeader, "Previous Section", operation.OperationCategoryNavigation, operation.TargetCardinalityNone, operation.OperationRiskReadOnly, "jump to previous section", "previous group"),
		core(operation.OperationCoreJumpBackward, "Jump to Older Task Position", operation.OperationCategoryNavigation, operation.TargetCardinalityNone, operation.OperationRiskReadOnly),
		core(operation.OperationCoreJumpForward, "Jump to Newer Task Position", operation.OperationCategoryNavigation, operation.TargetCardinalityNone, operation.OperationRiskReadOnly),
		core(operation.OperationCoreFocusLatest, "Focus Latest Activity", operation.OperationCategoryNavigation, operation.TargetCardinalityNone, operation.OperationRiskReadOnly, "latest activity", "last touched"),
		core(operation.OperationCoreCollapseParent, "Collapse or Go to Parent", operation.OperationCategoryNavigation, operation.TargetCardinalityNone, operation.OperationRiskReadOnly),
		core(operation.OperationCoreExpand, "Expand", operation.OperationCategoryNavigation, operation.TargetCardinalityNone, operation.OperationRiskReadOnly),
		// The three fold axes are three toggles, not six directional operations:
		// each owns one map, and the aliases stay direction-free because a toggle
		// must not advertise a way it does not always go.
		core(operation.OperationCoreToggleSections, "Toggle All Sections", operation.OperationCategoryNavigation, operation.TargetCardinalityNone, operation.OperationRiskReadOnly, "fold sections"),
		core(operation.OperationCoreToggleProjectGroup, "Toggle All Project Groups", operation.OperationCategoryNavigation, operation.TargetCardinalityNone, operation.OperationRiskReadOnly, "fold project groups"),
		core(operation.OperationCoreToggleSubtrees, "Toggle All Subtrees", operation.OperationCategoryNavigation, operation.TargetCardinalityNone, operation.OperationRiskReadOnly, "fold subtrees"),
		// The first operation in the grammar with no key at all. Opening every axis
		// at once is what the three toggles cannot spell, since each reverses and
		// each owns one axis; it is also rare enough that spending a letter on it
		// would cost more than it returns. The menu bar is what makes a keyless
		// operation reachable, and this is the category that makes the bar more than
		// a second way to press a key someone already knows.
		core(operation.OperationCoreClearFolds, "Clear All Folds", operation.OperationCategoryNavigation, operation.TargetCardinalityNone, operation.OperationRiskReadOnly, "unfold everything", "open all folds"),
		core(operation.OperationCoreReorderDown, "Reorder Down", operation.OperationCategoryNavigation, operation.TargetCardinalityOne, operation.OperationRiskMutation),
		core(operation.OperationCoreReorderUp, "Reorder Up", operation.OperationCategoryNavigation, operation.TargetCardinalityOne, operation.OperationRiskMutation),
		core(operation.OperationCoreReorderBottom, "Reorder to Bottom", operation.OperationCategoryNavigation, operation.TargetCardinalityOne, operation.OperationRiskMutation),
		core(operation.OperationCoreReorderTop, "Reorder to Top", operation.OperationCategoryNavigation, operation.TargetCardinalityOne, operation.OperationRiskMutation),
		focusSubtree,
		focusParent,
		gotoTaskProject,
		zoomSection,
		activateGroup,
		openNote,
		peek,
		core(operation.OperationCoreScopeSearch, "Search Scoped Tasks", operation.OperationCategoryFilter, operation.TargetCardinalityNone, operation.OperationRiskReadOnly, "scope-wide search"),
		core(operation.OperationCoreSelectProject, "Select Project", operation.OperationCategoryProject, operation.TargetCardinalityNone, operation.OperationRiskReadOnly, "projects", "switch project"),
		core(operation.OperationCoreSelectPinned, "Select Pinned Project", operation.OperationCategoryProject, operation.TargetCardinalityNone, operation.OperationRiskReadOnly, "pinned projects", "switch pinned project"),
		core(operation.OperationCoreMultiSelect, "Toggle Task Selection", operation.OperationCategoryTask, operation.TargetCardinalityNone, operation.OperationRiskReadOnly, "visual selection"),
		core(operation.OperationCoreRangeSelect, "Select Task Range", operation.OperationCategoryTask, operation.TargetCardinalityNone, operation.OperationRiskReadOnly, "contiguous selection", "range selection"),
		core(operation.OperationCoreSelectAll, "Select All Visible Tasks", operation.OperationCategoryTask, operation.TargetCardinalityNone, operation.OperationRiskReadOnly, "select visible"),
		// A lens is a saved projection over the mode, query, filter, and sort axes,
		// so the key that opens the list groups with the facets that decide the same
		// axes rather than with a surface of its own.
		core(
			operation.OperationCoreNamedLenses,
			"Named Lenses",
			operation.OperationCategoryFilter,
			operation.TargetCardinalityNone,
			operation.OperationRiskReadOnly,
			"lenses",
			"saved views",
			"save current view",
		),
		// The browser reads the same directory the status strip counts, so the key
		// that opens it is the strip's line made actionable rather than a surface of
		// its own. It stays read-only: resolving is `tt conflict resolve`.
		core(
			operation.OperationCoreConflicts,
			"Merge Conflicts",
			operation.OperationCategorySystem,
			operation.TargetCardinalityNone,
			operation.OperationRiskReadOnly,
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
			operation.OperationCoreQueryReadout,
			"Query Readout",
			operation.OperationCategoryFilter,
			operation.TargetCardinalityNone,
			operation.OperationRiskReadOnly,
			"narrowing",
			"what is filtered",
			"peel filters",
		),
		core(operation.OperationCoreNotes, "Notes", operation.OperationCategoryNote, operation.TargetCardinalityNone, operation.OperationRiskReadOnly, "scratch notes"),
		core(operation.OperationCoreToggleTheme, "Toggle Theme", operation.OperationCategorySystem, operation.TargetCardinalityNone, operation.OperationRiskMutation, "dark theme", "light theme"),
		core(operation.OperationCoreToggleMenuBar, "Toggle Menu Bar", operation.OperationCategorySystem, operation.TargetCardinalityNone, operation.OperationRiskMutation, "menu bar", "show menus"),
		// Opening is a separate act from showing: the toggle decides whether the
		// band is drawn at all, this one drops a menu out of it. They are two
		// descriptors so `?` can name both honestly rather than one label having to
		// mean "show, or open, depending on what is already true".
		core(operation.OperationCoreOpenMenuBar, "Open Menu", operation.OperationCategorySystem, operation.TargetCardinalityNone, operation.OperationRiskMutation, "open menu bar", "browse menus"),
		// The second operation reached only from the menu bar. It is target-free
		// like Clear All Folds, which is what lets it stay keyless: it acts on the
		// session's chrome, never on a task or a project. `--no-advisories` is the
		// other way in, so nothing here spends a root key on it.
		core(operation.OperationCoreToggleAdvisories, "Toggle Advisories", operation.OperationCategorySystem, operation.TargetCardinalityNone, operation.OperationRiskMutation, "conflict count", "hide warnings"),
		core(operation.OperationCoreUndo, "Undo", operation.OperationCategorySystem, operation.TargetCardinalityNone, operation.OperationRiskMutation),
		core(operation.OperationCoreRedo, "Redo", operation.OperationCategorySystem, operation.TargetCardinalityNone, operation.OperationRiskMutation),
		core(operation.OperationCoreBack, "Back", operation.OperationCategoryNavigation, operation.TargetCardinalityNone, operation.OperationRiskReadOnly),
		core(operation.OperationCoreHelp, "Help", operation.OperationCategorySystem, operation.TargetCardinalityNone, operation.OperationRiskReadOnly, "keyboard shortcuts"),
		core(operation.OperationCoreQuit, "Quit", operation.OperationCategorySystem, operation.TargetCardinalityNone, operation.OperationRiskReadOnly, "exit"),
		core(operation.OperationCoreNextWorkMode, "Next Work Mode", operation.OperationCategoryMode, operation.TargetCardinalityNone, operation.OperationRiskReadOnly),
		core(operation.OperationCorePreviousWorkMode, "Previous Work Mode", operation.OperationCategoryMode, operation.TargetCardinalityNone, operation.OperationRiskReadOnly),
		core(operation.OperationCoreGotoAll, "Go to All", operation.OperationCategoryMode, operation.TargetCardinalityNone, operation.OperationRiskReadOnly, "all mode"),
		core(operation.OperationCoreGotoInbox, "Go to Inbox", operation.OperationCategoryMode, operation.TargetCardinalityNone, operation.OperationRiskReadOnly, "inbox mode"),
		core(operation.OperationCoreGotoToday, "Go to Today", operation.OperationCategoryMode, operation.TargetCardinalityNone, operation.OperationRiskReadOnly, "today mode"),
		core(operation.OperationCoreGotoNext, "Go to Next", operation.OperationCategoryMode, operation.TargetCardinalityNone, operation.OperationRiskReadOnly, "next mode"),
		core(operation.OperationCoreGotoBacklog, "Go to Backlog", operation.OperationCategoryMode, operation.TargetCardinalityNone, operation.OperationRiskReadOnly, "backlog mode"),
		core(operation.OperationCoreGotoDue, "Go to Due", operation.OperationCategoryMode, operation.TargetCardinalityNone, operation.OperationRiskReadOnly, "due mode"),
		core(operation.OperationCoreGotoRecent, "Go to Recent", operation.OperationCategoryMode, operation.TargetCardinalityNone, operation.OperationRiskReadOnly, "recent mode", "recent activity"),
		// The digit that names no mode returns the view to the one a lens-free
		// launch produces. It is a Mode operation because that is the axis it is
		// spelled beside — 0 leads the seven — and read-only because every axis it
		// drops is a view axis: it writes no task, no project, and not even the
		// machine-local pointer naming which lens to open with.
		core(
			operation.OperationCoreResetToLaunchView,
			"Reset to Launch View",
			operation.OperationCategoryMode,
			operation.TargetCardinalityNone,
			operation.OperationRiskReadOnly,
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
		descriptors[i].Applicability = func(ctx operation.CommandContext) bool {
			return inner(ctx) && (ctx.Selection.Visual || !operation.CommandContextNamesNonTaskEntity(ctx))
		}
	}
	return descriptors
}
