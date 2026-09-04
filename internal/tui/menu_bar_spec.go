package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
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
	OperationID operation.OperationID
	// Label is optional shorter wording for the bar. Empty means the
	// descriptor's own Label, which is the wording `?` help already teaches.
	Label string
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
// operation, never a string the spec supplied.
type resolvedMenuEntry struct {
	OperationID operation.OperationID
	Label       string
	Key         string
	Checked     *bool
}

// resolvedMenu is one menu ready to be drawn. Every menu here has at least one
// surviving entry: a menu whose entries all fell away is omitted, so the bar
// never shows a label that opens nothing.
type resolvedMenu struct {
	ID      string
	Title   string
	Entries []resolvedMenuEntry
}

// The five menu identities. The bar's order is the order outlinerMenuSpecs
// returns them in; these names are what a test or a later renderer addresses a
// menu by, so neither has to match on a title.
const (
	menuIDTask = "task"
	menuIDView = "view"
	menuIDGo   = "go"
	menuIDFind = "find"
	menuIDHelp = "help"
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
func outlinerMenuSpecs(activeMode chrome.OutlinerModeID, menuBarVisible, advisoriesVisible bool) []menuSpec {
	return []menuSpec{
		{
			ID:    menuIDTask,
			Title: "Task",
			Entries: []menuEntrySpec{
				{OperationID: operation.OperationCoreCapture},
				{OperationID: operationTaskNewChild, Label: "Capture Child"},
				{OperationID: operation.OperationCoreEdit, Label: "Edit"},
				{OperationID: operation.OperationCoreEditDescription, Label: "Description"},
				{OperationID: operation.OperationCoreExternalEditor, Label: "External Editor"},
				{OperationID: operation.OperationCoreOutlineEditor},
				menuSeparator(),
				{OperationID: operationTaskAttachments},
				{OperationID: operationTaskComments},
				{OperationID: operation.OperationTaskReferences},
				menuSeparator(),
				{OperationID: operationTaskCopyTitle},
				{OperationID: operationTaskCopyDetails},
				{OperationID: operationTaskCopyTitleDescription},
			},
		},
		{
			ID:      menuIDView,
			Title:   "View",
			Entries: viewMenuEntries(activeMode, menuBarVisible, advisoriesVisible),
		},
		{
			ID:    menuIDGo,
			Title: "Go",
			Entries: []menuEntrySpec{
				{OperationID: operation.OperationCoreSelectProject},
				{OperationID: operation.OperationCoreSelectPinned},
				{OperationID: operation.OperationCoreGotoTaskProject, Label: "Project of Task"},
				menuSeparator(),
				{OperationID: operation.OperationCoreNotes},
				{OperationID: operation.OperationCoreNamedLenses},
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
			ID:    menuIDFind,
			Title: "Find",
			Entries: []menuEntrySpec{
				{OperationID: operation.OperationCoreScopeSearch, Label: "Search"},
				{OperationID: operationFilterPriority},
				{OperationID: operationFilterStates},
				{OperationID: operationFilterTags},
				{OperationID: operationFilterContexts},
				{OperationID: operationFilterReview},
				{OperationID: operationFilterDue},
				{OperationID: operationFilterCompletionAge},
				{OperationID: operationFilterHierarchy},
				{OperationID: operationFilterDependency},
				{OperationID: operationFilterStale},
				{OperationID: operationFilterPinned},
				{OperationID: operationFilterClear},
				{OperationID: operationFilterProjectTags},
				{OperationID: operationFilterShowParked},
				{OperationID: operationFilterOnlyParked},
				{OperationID: operationFilterIncludeArchived},
				{OperationID: operationFilterOnlyArchived},
				{OperationID: operationFilterProjectClear},
				menuSeparator(),
				{OperationID: operation.OperationCoreResetToLaunchView},
			},
		},
		{
			ID:    menuIDHelp,
			Title: "Help",
			Entries: []menuEntrySpec{
				{OperationID: operation.OperationCoreHelp, Label: "Key Reference"},
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
		menuEntrySpec{OperationID: operationSortDueDate},
		menuEntrySpec{OperationID: operationSortPriority},
		menuEntrySpec{OperationID: operationSortCreatedAt},
		menuEntrySpec{OperationID: operationSortUpdatedAt},
		menuEntrySpec{OperationID: operationSortTitle},
		menuEntrySpec{OperationID: operationSortClear},
		menuEntrySpec{OperationID: operationSortRanked},
		menuSeparator(),
		// Clear All Folds is the bar's whole justification in one row: it is bound
		// to nothing, so this menu is the only place it exists. A keyless operation
		// may not be an entity operation — see menu_bar_test.go — and this one is
		// target-free, acting on the view's fold state alone.
		menuEntrySpec{OperationID: operation.OperationCoreClearFolds},
		menuEntrySpec{OperationID: operation.OperationCorePeek, Label: "Peek"},
		menuEntrySpec{OperationID: operation.OperationCoreQueryReadout},
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
// catalog and the jump operations are already paired by gotoModeOperations, so
// the menu reads that pairing rather than declaring a second one.
func gotoOperationForMode(mode chrome.OutlinerModeID) operation.OperationID {
	for id, target := range gotoModeOperations {
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
//   - An entry whose descriptor does not apply to ctx is removed, not greyed,
//     which is what makes the menu honest about the current cursor.
//   - A key hint is always DisplayKeysForOperation, the same helper `?` reads.
//   - A menu with no surviving entry is omitted, so the bar never shows a label
//     that opens nothing.
//
// Nothing here is cached: the caller rebuilds on every render, which is what
// keeps a checkmark or a key hint from going stale.
func resolveMenuEntries(
	registry *operation.OperationRegistry,
	strategy operation.InputStrategy,
	ctx operation.CommandContext,
	specs []menuSpec,
) ([]resolvedMenu, error) {
	if registry == nil {
		return nil, fmt.Errorf("operation registry is required")
	}
	menus := make([]resolvedMenu, 0, len(specs))
	for _, spec := range specs {
		entries := make([]resolvedMenuEntry, 0, len(spec.Entries))
		for _, entry := range spec.Entries {
			if entry.OperationID == "" {
				entries = append(entries, resolvedMenuEntry{})
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
			if !descriptor.Applicable(ctx) {
				continue
			}
			label := entry.Label
			if label == "" {
				label = descriptor.Label
			}
			entries = append(entries, resolvedMenuEntry{
				OperationID: entry.OperationID,
				Label:       label,
				Key:         operation.DisplayKeysForOperation(strategy, entry.OperationID),
				Checked:     entry.Checked,
			})
		}
		entries = collapseMenuSeparators(entries)
		if len(entries) == 0 {
			continue
		}
		menus = append(menus, resolvedMenu{ID: spec.ID, Title: spec.Title, Entries: entries})
	}
	return menus, nil
}

// collapseMenuSeparators drops the rules that removal stranded. Entries leave a
// menu whenever the cursor cannot do them, so a group can empty out from under
// its own separator; a menu must not open on a rule, end on one, or show two in
// a row, and a menu left holding nothing but rules holds nothing at all.
func collapseMenuSeparators(entries []resolvedMenuEntry) []resolvedMenuEntry {
	collapsed := make([]resolvedMenuEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.OperationID != "" {
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

// validateMenuBarSpecs resolves the bar once against the catalog the app just
// registered, so a menu naming an operation that was renamed or retired fails
// at startup rather than opening a menu with a hole in it. The launch view is
// All with no cursor and a hidden bar, which is the state this resolution
// describes; a machine whose file says otherwise changes what the rows show,
// never which rows exist.
func (m *AppModel) validateMenuBarSpecs() error {
	strategy, ok := m.operationRegistry.InputStrategy(operation.InputStrategySimplified)
	if !ok {
		return fmt.Errorf("simplified input strategy is required")
	}
	ctx := PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	ctx.Capabilities.Add(operation.CapabilitySimplifiedInput)
	menus, err := resolveMenuEntries(
		m.operationRegistry,
		strategy,
		ctx,
		outlinerMenuSpecs(chrome.ModeAll, false, true),
	)
	if err != nil {
		return err
	}
	seen := make(map[string]bool, len(menus))
	for _, menu := range menus {
		if menu.ID == "" || menu.Title == "" {
			return fmt.Errorf("menu %q declares no identity or title", menu.Title)
		}
		if seen[menu.ID] {
			return fmt.Errorf("menu ID %s is declared twice", menu.ID)
		}
		seen[menu.ID] = true
		for _, entry := range menu.Entries {
			if entry.OperationID == "" {
				// A separator names no operation, so it can carry none of the three
				// things an operation supplies.
				if entry.Label != "" || entry.Key != "" || entry.Checked != nil {
					return fmt.Errorf("menu %s separator carries entry content", menu.ID)
				}
				continue
			}
			if entry.Label == "" {
				return fmt.Errorf("menu %s entry %s resolved without a label", menu.ID, entry.OperationID)
			}
		}
	}
	return nil
}
