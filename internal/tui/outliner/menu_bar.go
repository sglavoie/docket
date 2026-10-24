package outliner

import (
	"fmt"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
)

// The menu bar is a third projection of the one operation grammar, beside the
// `?` help and the footer hints. A menu entry names a registered operation and
// supplies presentation; it never re-implements an act and never spells a key,
// so a rebinding or a retirement moves the menu without anyone editing it.
//
// This file holds the specification and its projection alone. Nothing here
// renders and nothing here is bound to a key.

// menuEntrySpec is one row of a menu: a command reference plus presentation.
// The zero OperationID is a separator, the rule between two groups; it names no
// operation, so it carries neither label, key hint, nor state.
type menuEntrySpec struct {
	KeepUnavailable bool
	OperationID     operation.OperationID
	// Label is optional shorter wording for the bar. Empty means the
	// descriptor's own Label, which is the wording `?` help already teaches.
	Label string
	// Group introduces a heading only when at least one action in it applies.
	Group string
	// Checked is the checkbox or radio state read off live model state. Nil is
	// an entry with no state to show, which is most of them.
	Checked *bool
}

// menuSpec is one menu of the bar: a stable identity, its title, and its
// ordered entries.
type menuSpec struct {
	ID      string
	Title   string
	Entries []menuEntrySpec
}

// resolvedMenuEntry is one spec row projected against the live registry and the
// live input strategy. Key is always the strategy's own spelling for the
// operation, never a string the spec supplied. An empty OperationID is a rule
// or, when Label is present, a non-selectable group heading.
type ResolvedMenuEntry struct {
	DisabledReason string
	OperationID    operation.OperationID
	Label          string
	Key            string
	Checked        *bool
}

// resolvedMenu is one menu ready to be drawn. Every menu here has at least one
// surviving entry: a menu whose entries all fell away is omitted, so the bar
// never shows a label that opens nothing.
type ResolvedMenu struct {
	ID      string
	Title   string
	Entries []ResolvedMenuEntry
}

// The five menu identities. The bar's order is the order outlinerMenuSpecs
// returns them in; these names are what a test or a later renderer addresses a
// menu by, so neither has to match on a title.
const (
	MenuIDTask = "task"
	MenuIDView = "view"
	MenuIDGo   = "go"
	MenuIDFind = "find"
	MenuIDHelp = "help"
)

// menuSeparator is the rule between two groups of entries.
func menuSeparator() menuEntrySpec { return menuEntrySpec{} }

// outlinerMenuSpecs is the hand-curated bar over the Outliner's grammar. Every
// entry names a registered operation identifier rather than a string literal,
// which is what lets the parity test hold the whole bar to the catalog.
//
// activeMode, menuBarVisible, and advisoriesVisible are the live states the View
// menu's rows show. They are passed in rather than read from a model so the
// specification stays pure data: a menu row can describe live state, but it
// cannot reach back into the Outliner to find it.
func OutlinerMenuSpecs(activeMode chrome.OutlinerModeID, menuBarVisible, advisoriesVisible bool) []menuSpec {
	return []menuSpec{
		{
			ID:    MenuIDTask,
			Title: "Task",
			Entries: []menuEntrySpec{
				{OperationID: operation.OperationCoreCapture, KeepUnavailable: true},
				{OperationID: operation.OperationCoreEdit, Label: "Edit", KeepUnavailable: true},
				{OperationID: operation.OperationCoreToday, KeepUnavailable: true, Label: "Today", Group: "Plan"},
				{OperationID: operation.OperationCoreNext, KeepUnavailable: true, Label: "Next", Group: "Plan"},
				{OperationID: operation.OperationTaskSetState, KeepUnavailable: true, Label: "Set Status…", Group: "Status"},
				{OperationID: operation.OperationCoreBacklog, KeepUnavailable: true, Label: "Backlog (clear plan)", Group: "Status"},
				{OperationID: operation.OperationCoreUndo, Group: "Recovery", KeepUnavailable: true},
				{OperationID: operation.OperationCoreRedo, Group: "Recovery", KeepUnavailable: true},
				menuSeparator(),
				{OperationID: operation.OperationTaskNewChild, Label: "Capture Child"},
				{OperationID: operation.OperationChecklistAddItem},
				{OperationID: operation.OperationTaskSetPriority, Label: "Priority…", Group: "Details"},
				{OperationID: operation.OperationTaskSetDueDate, Label: "Due / Defer / Check-in…", Group: "Details"},
				{OperationID: operation.OperationTaskSetTags, Label: "Tags / Context…", Group: "Details"},
				{OperationID: operation.OperationTaskTogglePin, Group: "Details"},
				{OperationID: operation.OperationTaskMoveToProject, Group: "Details"},
				{OperationID: operation.OperationCoreEditDescription, Label: "Description", Group: "Details"},
				{OperationID: operation.OperationCoreExternalEditor, Label: "External Editor", Group: "Details"},
				{OperationID: operation.OperationCoreOutlineEditor, Group: "Details"},
				{OperationID: operation.OperationTaskAttachments, Group: "Details"},
				{OperationID: operation.OperationTaskComments, Group: "Details"},
				{OperationID: operation.OperationTaskReferences, Group: "Details"},
				{OperationID: operation.OperationTaskSetResumeNote, Label: "Resume Note", Group: "Details"},
				{OperationID: operation.OperationTaskToggleReview, Group: "Details"},
				{OperationID: operation.OperationCoreSetUpCheckIns, Group: "Details"},
				menuSeparator(),
				{OperationID: operation.OperationTaskCopyTitle},
				{OperationID: operation.OperationTaskCopyDetails},
				{OperationID: operation.OperationTaskCopyTitleDescription},
			},
		},
		{
			ID:      MenuIDView,
			Title:   "View",
			Entries: viewMenuEntries(activeMode, menuBarVisible, advisoriesVisible),
		},
		{
			ID:    MenuIDGo,
			Title: "Go",
			Entries: []menuEntrySpec{
				{OperationID: operation.OperationCoreSelectProject},
				{OperationID: operation.OperationCoreSelectPinned},
				{OperationID: operation.OperationCoreGotoTaskProject, Label: "Project of Task"},
				{OperationID: operation.OperationCoreGotoRelation, Label: "Related Task"},
				menuSeparator(),
				{OperationID: operation.OperationCoreNotes},
				{OperationID: operation.OperationCoreNamedLenses},
				{OperationID: operation.OperationCoreSyncNow},
				{OperationID: operation.OperationCoreConflicts, Label: "Conflict Browser"},
				menuSeparator(),
				{OperationID: operation.OperationCoreCursorTop, Label: "Top"},
				{OperationID: operation.OperationCoreCursorBottom, Label: "Bottom"},
				{OperationID: operation.OperationCoreJumpBackward, Label: "Jump Back"},
				{OperationID: operation.OperationCoreJumpForward, Label: "Jump Forward"},
				{OperationID: operation.OperationCoreFocusLatest, Label: "Latest Activity"},
			},
		},
		{
			ID:    MenuIDFind,
			Title: "Find",
			Entries: []menuEntrySpec{
				{OperationID: operation.OperationCoreScopeSearch, Label: "Search"},
				{OperationID: operation.OperationFilterPriority},
				{OperationID: operation.OperationFilterStates},
				{OperationID: operation.OperationFilterTags},
				{OperationID: operation.OperationFilterContexts},
				{OperationID: operation.OperationFilterProjects},
				{OperationID: operation.OperationFilterReview},
				{OperationID: operation.OperationFilterDue},
				{OperationID: operation.OperationFilterCompletionAge},
				{OperationID: operation.OperationFilterHierarchy},
				{OperationID: operation.OperationFilterDependency},
				{OperationID: operation.OperationFilterStale},
				{OperationID: operation.OperationFilterPinned},
				{OperationID: operation.OperationFilterHidden, Label: "Show hidden"},
				{OperationID: operation.OperationFilterClear},
				{OperationID: operation.OperationFilterProjectTags},
				{OperationID: operation.OperationFilterShowParked},
				{OperationID: operation.OperationFilterOnlyParked},
				{OperationID: operation.OperationFilterIncludeArchived},
				{OperationID: operation.OperationFilterOnlyArchived},
				{OperationID: operation.OperationFilterProjectClear},
				menuSeparator(),
				{OperationID: operation.OperationCoreResetToLaunchView},
			},
		},
		{
			ID:    MenuIDHelp,
			Title: "Help",
			Entries: []menuEntrySpec{
				{OperationID: operation.OperationCoreHelp, Label: "Key Reference"},
				{OperationID: operation.OperationCoreLastMessage},
			},
		},
	}
}

// viewMenuEntries builds the View menu. The mode rows are generated from the
// mode catalog rather than listed, so the seven the bar offers are exactly the
// seven Tab cycles through, in the order it cycles them, and their wording is
// the catalog's own.
//
// The bar's own visibility toggle is the last row, checked when the bar is
// drawn. A menu that can hide the surface it is opened from is unusual, but the
// key that shows the bar is the one thing a user who found the bar by accident
// cannot read off the bar itself, so it belongs here rather than only in `?`.
func viewMenuEntries(activeMode chrome.OutlinerModeID, menuBarVisible, advisoriesVisible bool) []menuEntrySpec {
	entries := make([]menuEntrySpec, 0, len(chrome.OutlinerModes)+13)
	for _, mode := range chrome.OutlinerModes {
		checked := mode.ID == activeMode
		entries = append(entries, menuEntrySpec{
			OperationID: gotoOperationForMode(mode.ID),
			Label:       mode.Label,
			Checked:     &checked,
		})
	}
	return append(entries,
		menuSeparator(),
		menuEntrySpec{OperationID: operation.OperationSortDueDate},
		menuEntrySpec{OperationID: operation.OperationSortPriority},
		menuEntrySpec{OperationID: operation.OperationSortCreatedAt},
		menuEntrySpec{OperationID: operation.OperationSortUpdatedAt},
		menuEntrySpec{OperationID: operation.OperationSortTitle},
		menuEntrySpec{OperationID: operation.OperationSortClear},
		menuEntrySpec{OperationID: operation.OperationSortRanked},
		menuSeparator(),
		// Clear All Folds is the bar's whole justification in one row: it is bound
		// to nothing, so this menu is the only place it exists. A keyless operation
		// may not be an entity operation — see menu_bar_test.go — and this one is
		// target-free, acting on the view's fold state alone.
		menuEntrySpec{OperationID: operation.OperationCoreClearFolds},
		menuEntrySpec{OperationID: operation.OperationCorePeek, Label: "Peek"},
		menuEntrySpec{OperationID: operation.OperationCorePeekSize},
		menuEntrySpec{OperationID: operation.OperationCoreQueryReadout},
		menuEntrySpec{OperationID: operation.OperationCoreClearSearchHistory},
		menuSeparator(),
		menuEntrySpec{OperationID: operation.OperationCoreToggleTheme, Label: "Theme"},
		menuEntrySpec{
			OperationID: operation.OperationCoreToggleMenuBar,
			Label:       "Menu Bar",
			Checked:     &menuBarVisible,
		},
		// Advisories are chrome the way the bar and the theme are, and this row is
		// the only way to reach the toggle from inside the app: it is bound to no
		// key, exactly like Clear All Folds above. Checked means advisories are
		// shown, so the box reads the same way Menu Bar's does — ticked is the thing
		// being on screen.
		menuEntrySpec{
			OperationID: operation.OperationCoreToggleAdvisories,
			Label:       "Advisories",
			Checked:     &advisoriesVisible,
		},
	)
}

// gotoOperationForMode names the operation that selects one mode. The mode
// catalog and the jump operations are already paired by operation.GotoModeOperations, so
// the menu reads that pairing rather than declaring a second one.
func gotoOperationForMode(mode chrome.OutlinerModeID) operation.OperationID {
	for id, target := range operation.GotoModeOperations {
		if target == mode {
			return id
		}
	}
	return ""
}

// resolveMenuEntries projects the specs against the live registry and strategy.
// It is the menu bar's whole relationship with the operation grammar:
//
//   - A spec naming an operation the registry does not hold is a startup error,
//     not a silently dropped row.
//   - An unavailable everyday entry stays visible with a reason; specialist
//     entries are omitted. Disabled rows cannot dispatch an operation.
//   - A key hint is always DisplayKeysForOperation, the same helper `?` reads.
//   - A menu with no surviving entry is omitted, so the bar never shows a label
//     that opens nothing.
//
// Nothing here is cached: the caller rebuilds on every render, which is what
// keeps a checkmark or a key hint from going stale.
func ResolveMenuEntries(
	registry *operation.OperationRegistry,
	strategy operation.InputStrategy,
	ctx operation.CommandContext,
	specs []menuSpec,
) ([]ResolvedMenu, error) {
	if registry == nil {
		return nil, fmt.Errorf("operation registry is required")
	}
	menus := make([]ResolvedMenu, 0, len(specs))
	for _, spec := range specs {
		entries := make([]ResolvedMenuEntry, 0, len(spec.Entries))
		group := ""
		for _, entry := range spec.Entries {
			if entry.OperationID == "" {
				entries = append(entries, ResolvedMenuEntry{})
				group = ""
				continue
			}
			descriptor, exists := registry.Descriptor(entry.OperationID)
			if !exists {
				return nil, fmt.Errorf(
					"menu %s names unregistered operation %s",
					spec.ID,
					entry.OperationID,
				)
			}
			reason := ""
			if !descriptor.Applicable(ctx) {
				if !entry.KeepUnavailable {
					continue
				}
				reason = menuUnavailableReason(entry.OperationID, ctx)
			}
			if entry.Group != "" && entry.Group != group {
				entries = append(entries, ResolvedMenuEntry{Label: entry.Group})
			}
			group = entry.Group
			label := entry.Label
			if label == "" {
				label = descriptor.Label
			}
			entries = append(entries, ResolvedMenuEntry{
				OperationID:    entry.OperationID,
				Label:          label,
				Key:            operation.DisplayKeysForOperation(strategy, entry.OperationID),
				Checked:        entry.Checked,
				DisabledReason: reason,
			})
		}
		entries = collapseMenuSeparators(entries)
		if len(entries) == 0 {
			continue
		}
		menus = append(menus, ResolvedMenu{ID: spec.ID, Title: spec.Title, Entries: entries})
	}
	return menus, nil
}

// collapseMenuSeparators drops the rules that removal stranded. Entries leave a
// menu whenever the cursor cannot do them, so a group can empty out from under
// its own separator; a menu must not open on a rule, end on one, or show two in
// a row, and a menu left holding nothing but rules holds nothing at all.
func collapseMenuSeparators(entries []ResolvedMenuEntry) []ResolvedMenuEntry {
	collapsed := make([]ResolvedMenuEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.OperationID != "" || entry.Label != "" {
			collapsed = append(collapsed, entry)
			continue
		}
		if len(collapsed) == 0 || collapsed[len(collapsed)-1].OperationID == "" {
			continue
		}
		collapsed = append(collapsed, entry)
	}
	for len(collapsed) > 0 && collapsed[len(collapsed)-1].OperationID == "" {
		collapsed = collapsed[:len(collapsed)-1]
	}
	return collapsed
}

// Reasons explain presentation availability; execution still belongs to the
// registry. Disabled entries remain focusable so keyboard users can read why.
func menuUnavailableReason(id operation.OperationID, ctx operation.CommandContext) string {
	if ctx.Entity.Archived {
		return "This project is archived. Open an active project to make changes."
	}
	if id == operation.OperationCoreEdit && ctx.Selection.Visual {
		return "Leave multi-select, then choose one task to edit."
	}
	if ctx.Entity.HasChecklistCursor {
		return "Choose the task row to use this action."
	}
	if ctx.Selection.Visual && ctx.Selection.Count() == 0 {
		return "Select at least one task."
	}
	if ctx.Entity.TaskID == "" && ctx.Selection.Count() == 0 {
		return "Choose a task to use this action."
	}
	return "This action is unavailable for the current selection."
}
