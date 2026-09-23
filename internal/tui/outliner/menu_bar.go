package outliner

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/theme"
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
type ResolvedMenuEntry struct {
	OperationID operation.OperationID
	Label       string
	Key         string
	Checked     *bool
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
				{OperationID: operation.OperationCoreCapture},
				{OperationID: operation.OperationTaskNewChild, Label: "Capture Child"},
				{OperationID: operation.OperationChecklistAddItem},
				{OperationID: operation.OperationCoreEdit, Label: "Edit"},
				{OperationID: operation.OperationCoreEditDescription, Label: "Description"},
				{OperationID: operation.OperationCoreExternalEditor, Label: "External Editor"},
				{OperationID: operation.OperationCoreOutlineEditor},
				menuSeparator(),
				{OperationID: operation.OperationTaskAttachments},
				{OperationID: operation.OperationTaskComments},
				{OperationID: operation.OperationTaskReferences},
				{OperationID: operation.OperationTaskSetResumeNote, Label: "Resume Note"},
				{OperationID: operation.OperationTaskToggleReview},
				{OperationID: operation.OperationCoreSetUpCheckIns},
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
//   - An entry whose descriptor does not apply to ctx is removed, not greyed,
//     which is what makes the menu honest about the current cursor.
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
		for _, entry := range spec.Entries {
			if entry.OperationID == "" {
				entries = append(entries, ResolvedMenuEntry{})
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
			entries = append(entries, ResolvedMenuEntry{
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

// menuBarState is the open menu's whole sub-state: the dropdown gate and where
// the highlight sits inside it. It is a named field on OutlinerModel rather
// than three promoted ones because the cluster is genuinely separable — the
// bar's own input and render are the only readers, and every other caller
// reaches it through the one close below.
//
// It holds no menus. The bar and its entries are resolved off the live catalog
// every frame, so the two indices are the only thing that could go stale, and
// both are re-resolved against that resolution before they are read. That is
// why the clamp and the walk take a count or an entry set as an argument rather
// than reaching for a model this type deliberately cannot see: the state is the
// cursor, never the population it points into.
type menuBarState struct {
	// open is the dropdown gate. The bar itself is machine-local visibility that
	// `M` writes and this sub-state never touches — closing a menu must not hide
	// the band it dropped out of.
	Open bool
	// menuIndex is which bar label is open and itemIndex where the highlight sits
	// inside it. Both are stored positions into a resolution that may have dropped
	// rows out from under them, never trusted identities.
	MenuIndex int
	ItemIndex int
}

// openFirstMenu drops the first menu out of the bar. It always opens on the
// leftmost menu rather than on the last one used: the bar is read left to right
// and an open menu that remembered where it was would put the highlight
// somewhere the user did not leave it, one whole session ago.
func (s *menuBarState) openFirstMenu() {
	if s == nil {
		return
	}
	s.Open = true
	s.MenuIndex = 0
	s.ItemIndex = 0
}

// close puts the menu away without asking what it was showing. It leaves both
// indices alone because the next open resets them anyway, and a close that also
// cleared them would make the two writes look like separate facts.
func (s *menuBarState) close() {
	if s == nil {
		return
	}
	s.Open = false
}

// clampMenu pulls the stored menu index back inside a resolution that may hold
// fewer menus than it did when the index was written. A menu leaves the bar
// whenever the drop ladder sheds its label or the catalog stops offering it, so
// the index is clamped every time it is read rather than on the frame the menu
// left.
func (s *menuBarState) clampMenu(count int) {
	if s == nil || count <= 0 {
		return
	}
	s.MenuIndex = min(max(s.MenuIndex, 0), count-1)
}

// cycleMenu moves along the bar and starts the new menu at its top. It wraps,
// unlike the vertical walk: the bar is a ring of five labels a user crosses
// repeatedly, and stopping at Help would make the leftmost menu three presses
// away from the rightmost.
func (s *menuBarState) cycleMenu(delta, count int) {
	if s == nil || count <= 0 {
		return
	}
	s.MenuIndex = ((s.MenuIndex+delta)%count + count) % count
	s.ItemIndex = 0
}

// selectItem writes the highlight back. Both the keymap's walk and the render's
// re-resolution land here, which is what stops the row a user sees and the entry
// Enter runs from ever disagreeing.
func (s *menuBarState) selectItem(index int) {
	if s == nil {
		return
	}
	s.ItemIndex = index
}

// An open menu is a gated sub-state of the Outliner, built on the same template
// the `Q` readout uses: a flag on the model, a first branch in handleKeyMsg, a
// claim in simplifiedInlineControlOwnsInput, and a gated footer keymap that
// takes no footer rank. It mints no PresentationID, opens no overlay, claims no
// InputStrategy, and takes no manifest entry.
//
// It diverges from the readout in exactly one place, deliberately: an unlisted
// key here does not vanish into a close. The bar is a map of the grammar, and a
// user reading it who recognizes the key beside a row should be able to press
// that key rather than the row. So an unrecognized key closes the menu and is
// re-emitted, and the root grammar runs it on the next update — the menu is
// non-modal, which is the same promise hunk's handleMenuShortcut keeps by
// returning "notMine".

// MenuBarActivateMsg asks the shell to run the operation a menu entry names.
// The menu raises it rather than executing the operation itself for the same
// reason the readout raises its save request: a view model holds no executor,
// and the one invocation pipeline lives above it.
type MenuBarActivateMsg struct{ OperationID operation.OperationID }

func (msg MenuBarActivateMsg) String() string { return "menu bar activate" }

// openMenuBar drops the first menu out of the bar. It is the model's one-line
// way into menuBarState, kept as a named method because the operation handler
// asks the Outliner to open a menu rather than reaching into its sub-state.
func (m *OutlinerModel) OpenMenuBar() {
	if m == nil {
		return
	}
	m.MenuBar.openFirstMenu()
}

// closeMenuBar puts the menu away without asking what it was showing. Every
// reset that replaces the whole projection calls it, so a dropdown resolved
// against one population cannot survive into another — and this one line is
// where that spread meets the sub-state, which is what keeps the six
// projection-replacing callers out of the menu's fields entirely.
func (m *OutlinerModel) closeMenuBar() {
	if m == nil {
		return
	}
	m.MenuBar.close()
}

// openMenuBarMenus is the resolution both the dropdown and this keymap read.
// Navigation walks the menus that survived filtering rather than the specs, so
// `l` never stops on a label the bar is not drawing, and the highlight the
// render paints is indexed into the same slice Enter dispatches from.
//
// A resolution that came back empty is a bar with nothing to open, which closes
// the sub-state rather than leaving a gate open over no menus.
func (m *OutlinerModel) OpenMenuBarMenus() ([]ResolvedMenu, bool) {
	if m == nil || !m.MenuBar.Open {
		return nil, false
	}
	menus := m.resolvedMenuBar()
	if len(menus) == 0 {
		m.closeMenuBar()
		return nil, false
	}
	m.MenuBar.clampMenu(len(menus))
	return menus, true
}

// resolveMenuBarItem maps a stored item index onto a real entry. The stored
// index survives a resolution that dropped rows out from under it — an entry
// leaves whenever the cursor cannot do it — so it is re-resolved every time it
// is read rather than trusted: it walks forward off a separator or off the end,
// then backward, and lands on the nearest thing Enter can actually run. Both
// the render and the keymap resolve through here, which is what stops the
// highlight and Enter from ever naming different rows.
//
// It returns -1 only for a menu holding no runnable entry at all, which
// collapseMenuSeparators has already made impossible for a resolved menu.
func resolveMenuBarItem(entries []ResolvedMenuEntry, index int) int {
	if len(entries) == 0 {
		return -1
	}
	index = min(max(index, 0), len(entries)-1)
	for i := index; i < len(entries); i++ {
		if entries[i].OperationID != "" {
			return i
		}
	}
	for i := index - 1; i >= 0; i-- {
		if entries[i].OperationID != "" {
			return i
		}
	}
	return -1
}

// stepMenuBarItem moves the highlight by one entry, skipping the separators a
// rule is drawn for. It does not wrap: a menu is a short list with two ends, and
// wrapping from the last row to the first is how a user overshoots without
// noticing.
func StepMenuBarItem(entries []ResolvedMenuEntry, index, delta int) int {
	current := resolveMenuBarItem(entries, index)
	if current < 0 {
		return index
	}
	for next := current + delta; next >= 0 && next < len(entries); next += delta {
		if entries[next].OperationID != "" {
			return next
		}
	}
	return current
}

// handleMenuBarKey is the open menu's gated keymap. It runs before every other
// Outliner branch while a menu is open, and it is the only place a keystroke
// moves the two stored indices — the render's own write is a re-resolution of
// what this one left, never a second navigation.
func (m *OutlinerModel) handleMenuBarKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	menus, ok := m.OpenMenuBarMenus()
	if !ok {
		return m, nil
	}
	entries := menus[m.MenuBar.MenuIndex].Entries
	key := msg.String()
	// The key that opened the menu puts it away again, which is the one thing the
	// readout gets for free from its close-on-anything fall-through and this
	// keymap has to say out loud: re-emitting Ctrl+M through the non-modal arm
	// below would close the menu and immediately reopen it.
	if slices.Contains(m.menuBarOpenKeys(), operation.NormalizeOperationKey(key)) {
		m.closeMenuBar()
		return m, nil
	}
	switch key {
	case "esc":
		m.closeMenuBar()
		return m, nil
	case "h", "left":
		m.cycleMenuBarMenu(-1, len(menus))
		return m, nil
	case "l", "right":
		m.cycleMenuBarMenu(1, len(menus))
		return m, nil
	case "k", "up":
		m.MenuBar.selectItem(StepMenuBarItem(entries, m.MenuBar.ItemIndex, -1))
		return m, nil
	case "j", "down":
		m.MenuBar.selectItem(StepMenuBarItem(entries, m.MenuBar.ItemIndex, 1))
		return m, nil
	case "enter":
		index := resolveMenuBarItem(entries, m.MenuBar.ItemIndex)
		m.closeMenuBar()
		if index < 0 {
			return m, nil
		}
		id := entries[index].OperationID
		return m, func() tea.Msg { return MenuBarActivateMsg{OperationID: id} }
	}
	// The non-modal arm. The menu closes first and the key is re-emitted, so the
	// next update finds simplifiedInlineControlOwnsInput false and the root
	// grammar runs the accelerator exactly as it would have with no menu open.
	// Re-emitting rather than dispatching here is what keeps this model free of
	// an executor it has no business holding.
	m.closeMenuBar()
	return m, func() tea.Msg { return msg }
}

// cycleMenuBarMenu moves along the bar. The wrap and the reset to the new
// menu's top are menuBarState's, so the keymap here names the direction and the
// sub-state keeps the two indices in step.
func (m *OutlinerModel) cycleMenuBarMenu(delta, count int) {
	m.MenuBar.cycleMenu(delta, count)
}

// menuBarOpenKeys is the grammar's own spelling of the key that opens a menu,
// read back out of the strategy rather than repeated here, so a rebinding moves
// the close arm above and the footer hint below together.
func (m *OutlinerModel) menuBarOpenKeys() []string {
	if m == nil || m.OperationRegistry == nil {
		return nil
	}
	strategy, ok := m.OperationRegistry.InputStrategy(operation.InputStrategySimplified)
	if !ok {
		return nil
	}
	keys := make([]string, 0, 1)
	for _, binding := range strategy.Bindings {
		if binding.OperationID != operation.OperationCoreOpenMenuBar {
			continue
		}
		for _, key := range binding.Keys {
			keys = append(keys, operation.NormalizeOperationKey(key))
		}
	}
	return keys
}

// menuBarGuidance is the footer while a menu is open. It is the gated keymap the
// sub-state supplies, which is why none of these keys carries a footer rank of
// its own and the ranked projection underneath is untouched.
func (m *OutlinerModel) menuBarGuidance() chrome.GuidanceSpec {
	closeKey := "Esc"
	if m.OperationRegistry != nil {
		if key := m.OperationRegistry.InputShortcutForOperation(
			operation.InputStrategySimplified, operation.OperationCoreOpenMenuBar); key != "" {
			closeKey = key + "/Esc"
		}
	}
	return chrome.GuidanceSpec{
		Hints: append(
			chrome.RequiredGuidanceHints("←/→ menu", "↑/↓ entry", "Enter run", closeKey+" close"),
			chrome.OptionalGuidanceHints(10, "any other key runs its own act")...),
		Rows: 2,
	}
}

// The bar is a view-local gated sub-state of the Outliner, not a presentation:
// it mints no PresentationID, opens no overlay, claims no InputStrategy, and
// takes no manifest entry, exactly as `gc` does. It costs one row of the list
// while it is shown, and contentHeight is the single place that subtracts it.

// menuBarLabelPadding is the horizontal padding around one bar label. It is the
// tab bar's own spelling of the same measurement, so a menu label and a tab
// label occupy the same shape.
const menuBarLabelPadding = 1

// menuLabelBox is one drawn menu label: its wording, the column its rendered
// cell starts at, and the width that cell occupies. The columns are what a
// dropdown is padded to, which is why the layout is a value rather than a
// string — RenderTabBar returns only a string and so can never say where a
// label landed.
type menuLabelBox struct {
	Title    string
	StartCol int
	Width    int
}

// menuBarMeasureStyle is the plain style the layout measures a label with, the
// same one RenderColoredTabBar sizes a tab by. Padding is the only thing that
// decides a cell's width, so this style and the drawing ones must agree on it or
// the measured layout and the painted row would drift apart.
func menuBarMeasureStyle() lipgloss.Style {
	return lipgloss.NewStyle().Padding(0, menuBarLabelPadding)
}

// menuBarGap is the blank run between two cells and after the last one. It
// carries the theme background for the same reason a tab filler does: the bar is
// one band, and an unpainted gap would cut it into words.
func menuBarGap(width int) string {
	if width <= 0 {
		return ""
	}
	s := lipgloss.NewStyle().Width(width)
	if theme.ThemeBgColor != "" {
		s = s.Background(lipgloss.Color(theme.ThemeBgColor))
	}
	return s.Render("")
}

// menuBarLayout places the menus left to right and reports where each one
// landed. When the labels do not all fit it drops whole menus from the right —
// Help first, then Find, then Go — rather than truncating one: a dropped menu
// is still reachable by the accelerators its entries carry, while a truncated
// label is a lie about what the bar offers.
//
// The first menu always survives. At a width no single label fits, the row is
// clipped by the same full-width fit every other Outliner line goes through,
// which is the honest failure for a terminal that narrow.
func menuBarLayout(menus []ResolvedMenu, width int) []menuLabelBox {
	if len(menus) == 0 || width <= 0 {
		return nil
	}
	style := menuBarMeasureStyle()
	boxes := make([]menuLabelBox, 0, len(menus))
	col := 0
	for _, menu := range menus {
		cell := lipgloss.Width(style.Render(menu.Title))
		boxes = append(boxes, menuLabelBox{Title: menu.Title, StartCol: col, Width: cell})
		col += cell
	}
	for len(boxes) > 1 && col > width {
		col -= boxes[len(boxes)-1].Width
		boxes = boxes[:len(boxes)-1]
	}
	return boxes
}

// renderMenuBarRow draws the bar from its layout. It writes each label at the
// column the layout assigned it, so the row a user sees and the offsets a
// dropdown is padded to come from one computation rather than two.
//
// openIdx names the menu whose dropdown is open, or -1 when none is. The open
// label is the only thing on this row that ever changes appearance, and
// deciding that here keeps the highlight beside the placement the dropdown
// under it is padded to.
func renderMenuBarRow(menus []ResolvedMenu, width int, openIdx int) string {
	boxes := menuBarLayout(menus, width)
	open := chrome.ActiveTabStyle(menuBarLabelPadding, false)
	closed := chrome.InactiveTabStyle(menuBarLabelPadding, false)

	var b strings.Builder
	col := 0
	for i, box := range boxes {
		b.WriteString(menuBarGap(box.StartCol - col))
		if i == openIdx {
			b.WriteString(open.Render(box.Title))
		} else {
			b.WriteString(closed.Render(box.Title))
		}
		col = box.StartCol + box.Width
	}
	// The row is exactly m.width like every other Outliner line, so the bar reads
	// as a band rather than as a ragged string of words.
	b.WriteString(menuBarGap(width - col))
	return b.String()
}

// MenuBarPinned reads the machine-local flag `M` writes. RuntimeState.Machine
// is nil-safe and returns defaults, so an Outliner built without a runtime — the
// dimmed preview backgrounds among them — simply draws no bar.
func (m *OutlinerModel) MenuBarPinned() bool {
	return m.Runtime.Machine().MenuBarVisible
}

// MenuBarVisible is whether this frame draws the band. An open menu hangs off a
// bar label, so opening one out of a hidden bar reveals it for exactly as long
// as the menu is open: the reveal is the sub-state's, not a write to the pinned
// flag, so closing the menu leaves visibility where the user last put it rather
// than where a dropdown passed through.
func (m *OutlinerModel) MenuBarVisible() bool {
	return m.MenuBarPinned() || m.MenuBar.Open
}

// menuBarRows is what the bar costs the row budget. It is deliberately a
// constant one row rather than a measurement of the drawn bar: contentHeight is
// called from key handlers that must not build a menu resolution, and a bar that
// resolved to no menus at all still owes its row or the reservation and the
// paint would disagree.
func (m *OutlinerModel) menuBarRows() int {
	if m.MenuBarVisible() {
		return 1
	}
	return 0
}

// resolvedMenuBar projects the specs against the live catalog for this render.
// Nothing is cached: a menu whose every entry fell away is omitted, so which
// labels the bar draws follows the cursor the same way the entries do.
func (m *OutlinerModel) resolvedMenuBar() []ResolvedMenu {
	if m.OperationRegistry == nil {
		return nil
	}
	strategy, ok := m.OperationRegistry.InputStrategy(operation.InputStrategySimplified)
	if !ok {
		return nil
	}
	menus, err := ResolveMenuEntries(
		m.OperationRegistry,
		strategy,
		m.OperationCommandContext(),
		// The Menu Bar row is checked from the pinned flag its toggle writes, not
		// from what this frame draws: a bar revealed only for the open menu is not
		// a bar the user asked to keep, and a tick that said otherwise would make
		// the next Enter on that row read as "hide" while it shows.
		OutlinerMenuSpecs(m.PrimaryMode, m.MenuBarPinned(), !m.AdvisoriesSuppressed()),
	)
	if err != nil {
		// Startup already refused a spec naming an operation the registry does not
		// hold, so there is nothing here a render could report that validation did
		// not. Drawing an empty band beats painting an error into the chrome.
		return nil
	}
	return menus
}

// renderOutlinerMenuBar is the one line the Outliner writes above its title when
// the bar is shown.
func (m *OutlinerModel) renderOutlinerMenuBar(width int) string {
	return renderMenuBarRow(m.resolvedMenuBar(), width, m.openMenuBarIndex())
}

// The dropdown is a popup composited over the view that opened it, anchored to
// the column the layout gave the open label. This file returns the block and the
// column alone; the compositing, the dimming, and the rows the block may claim
// are the render's, so a menu and every other popup in the app agree about
// geometry by calling one seam rather than by resembling each other.

const (
	// menuBarDropdownCheckColumn is the width of the checkmark gutter. Every
	// entry in a menu that has any state to show reserves it, so the labels stay
	// on one column instead of the checked rows stepping right.
	menuBarDropdownCheckColumn = 2
	// menuBarDropdownMinRows is the shortest list worth drawing. Below this the
	// dropdown says less than the bar above it already does.
	menuBarDropdownMinRows = 3
	// menuBarDropdownListRows is what the frame costs around its rows: two border
	// lines and the two halo lines renderOverlayBox adds outside them.
	menuBarDropdownListRows = 4
)

// openMenuBarIndex is which bar label is drawn open, or -1 when none is. It is
// read by the bar row and by the dropdown from the one resolution, so the
// highlight and the block under it can never name different menus.
func (m *OutlinerModel) openMenuBarIndex() int {
	if m == nil || !m.MenuBar.Open {
		return -1
	}
	return m.MenuBar.MenuIndex
}

// renderOutlinerMenuDropdown draws the open menu and the column it hangs at, or
// the empty string when none is open. It re-resolves the stored item index
// against the entries this frame actually holds and writes the resolved index
// back, which is what stops the highlight a user sees and the entry Enter runs
// from disagreeing after a row left the menu underneath them.
//
// maxBoxRows is the compositor's, not this function's: the block is sized to
// what the host frame leaves under its header rather than to a budget measured
// twice.
func (m *OutlinerModel) renderOutlinerMenuDropdown(width, maxBoxRows int) (string, int) {
	menus, ok := m.OpenMenuBarMenus()
	if !ok || width <= 0 {
		return "", 0
	}
	entries := menus[m.MenuBar.MenuIndex].Entries
	cursor := resolveMenuBarItem(entries, m.MenuBar.ItemIndex)
	if cursor < 0 {
		return "", 0
	}
	m.MenuBar.selectItem(cursor)

	innerWidth := menuBarDropdownWidth(entries, width)
	// The box's own rows come off the budget before the list is sized.
	maxBoxRows = max(maxBoxRows, menuBarDropdownMinRows+menuBarDropdownListRows)
	listHeight := max(min(len(entries), maxBoxRows-menuBarDropdownListRows), 1)

	surface, err := newMenuBarDropdownSurface(entries, innerWidth-4)
	if err != nil {
		// The surface refuses only a malformed item set, which is this function's
		// own construction rather than anything a user did. Drawing no dropdown is
		// the honest failure: the bar above still says which menu is open.
		return "", 0
	}
	surface.SetCursorToID(menuBarDropdownItemID(cursor))
	content := listsurface.RenderCompactListSurface(surface, innerWidth-4, listHeight)
	box := chrome.RenderOverlayBox(content, innerWidth, "", "",
		lipgloss.NewStyle().Foreground(theme.ColorPrimary))
	return box, menuBarDropdownStartCol(menus, width, m.MenuBar.MenuIndex, box)
}

// menuBarDropdownStartCol is the column the block is composited at: the open
// label's own StartCol. The compositor clamps the right edge itself, so this
// reports the anchor and nothing about the frame.
//
// A menu the drop ladder shed at a narrow width has no drawn label to align to,
// so its dropdown starts at the left margin instead of under a column nothing is
// painted at.
func menuBarDropdownStartCol(menus []ResolvedMenu, width, openIdx int, box string) int {
	startCol := 0
	if boxes := menuBarLayout(menus, width); openIdx < len(boxes) {
		startCol = boxes[openIdx].StartCol
	}
	return min(max(startCol, 0), max(width-chrome.PopupWidth(box), 0))
}

// menuBarDropdownWidth sizes the frame to its longest entry rather than to a
// fraction of the screen. A dropdown is anchored under one label and reads as
// belonging to it, so a box that claimed three fifths of the terminal would
// read as a modal that happens to be left-aligned.
func menuBarDropdownWidth(entries []ResolvedMenuEntry, width int) int {
	marker := menuBarDropdownMarkerWidth(entries)
	longest := 0
	for _, entry := range entries {
		if entry.OperationID == "" {
			continue
		}
		// The gap is LayoutListText's own, charged here because the labels are
		// padded out to it: the widest row is the column every key hint is then
		// flushed to, so a menu wide enough for that row truncates nothing.
		row := marker + lipgloss.Width(entry.Label)
		if entry.Key != "" {
			row += lipgloss.Width(listsurface.ListTextGap) + lipgloss.Width(entry.Key)
		}
		longest = max(longest, row)
	}
	// The four columns are the frame's: one border and one padding cell each side.
	return min(max(longest+4, 20), max(width-6, 24))
}

// menuBarDropdownMarkerWidth is the chrome charged to every row before its text:
// the cursor gutter the list renderer prepends, plus the checkmark column when
// this menu has any state to show. It is measured once for the whole menu rather
// than per row, because a column that appeared only on the checked rows would
// step every label beside it out of line.
func menuBarDropdownMarkerWidth(entries []ResolvedMenuEntry) int {
	if menuBarDropdownChecks(entries) {
		return 2 + menuBarDropdownCheckColumn
	}
	return 2
}

// menuBarDropdownChecks reports whether this menu has any state to read back.
// One entry carrying a checkbox or a radio is enough: the gutter is a column of
// the menu, not a property of a row.
func menuBarDropdownChecks(entries []ResolvedMenuEntry) bool {
	for _, entry := range entries {
		if entry.Checked != nil {
			return true
		}
	}
	return false
}

// newMenuBarDropdownSurface builds the list the frame holds. It is rebuilt every
// frame from the live resolution for the same reason resolvedMenuBar is: a
// cached surface would keep offering a row the cursor stopped being able to run.
//
// The menu owns its own navigation, so this surface never receives a cursor key:
// separators are ordinary rows here because nothing in it can land on one.
func newMenuBarDropdownSurface(entries []ResolvedMenuEntry, contentWidth int) (*listsurface.ListSurfaceModel, error) {
	checked := menuBarDropdownChecks(entries)
	// A rule starts where the labels start and ends where they end, so it wears
	// the same marker they do and is measured against what the marker leaves.
	blank := ""
	if checked {
		blank = strings.Repeat(" ", menuBarDropdownCheckColumn)
	}
	textWidth := max(contentWidth-menuBarDropdownMarkerWidth(entries), 1)
	items := make([]listsurface.ListItem, 0, len(entries))
	for index, entry := range entries {
		item := listsurface.ListItem{ID: menuBarDropdownItemID(index), PrimaryMarker: blank}
		if entry.OperationID == "" {
			item.PrimaryText = strings.Repeat("─", textWidth)
			items = append(items, item)
			continue
		}
		item.PrimaryText = padMenuBarDropdownLabel(entry.Label, entry.Key, textWidth)
		item.SecondaryText = entry.Key
		if entry.Checked != nil && *entry.Checked {
			item.PrimaryMarker = "✓ "
		}
		items = append(items, item)
	}
	return listsurface.NewListSurfaceModel(items, nil, listsurface.ListSurfaceConfig{SelectionMode: listsurface.ListSelectionSingle})
}

// padMenuBarDropdownLabel pads a label out so its key hint lands flush against
// the menu's right edge, the way a desktop menu reads: the eye scans one column
// of labels and one column of keys instead of a ragged pair. The padding stops
// short of LayoutListText's own gap, which the layout adds back after it.
//
// A row too wide to hold both at this width is left unpadded, so the layout
// truncates exactly as it did before. Alignment is a courtesy the menu extends
// when it has the columns to spend; a key pushed off the edge to buy it would
// cost the one thing the row is there to say.
func padMenuBarDropdownLabel(label, key string, textWidth int) string {
	if key == "" {
		return label
	}
	pad := textWidth - lipgloss.Width(label) - lipgloss.Width(listsurface.ListTextGap) - lipgloss.Width(key)
	if pad <= 0 {
		return label
	}
	return label + strings.Repeat(" ", pad)
}

// menuBarDropdownItemID names one row by its position in the resolution, which
// is the only identity a menu entry has: two rows can name the same operation
// with different wording, and a separator names none.
func menuBarDropdownItemID(index int) listsurface.ListItemID {
	return listsurface.ListItemID("menu-entry-" + strconv.Itoa(index))
}
