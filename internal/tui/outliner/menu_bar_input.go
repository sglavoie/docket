package outliner

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
)

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
	// History descriptions are a snapshot read at open, not during rendering.
	undoDescription string
	redoDescription string
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
	m.MenuBar.undoDescription, m.MenuBar.redoDescription = "", ""
	if m.HistoryPreview != nil {
		m.MenuBar.undoDescription, m.MenuBar.redoDescription = m.HistoryPreview()
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
		if index >= 0 && entries[index].DisabledReason != "" {
			return m, nil
		}
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
	for i := range menus {
		entries := menus[i].Entries[:0]
		for _, entry := range menus[i].Entries {
			var description string
			switch entry.OperationID { //nolint:exhaustive // Only undo and redo describe ledger entries.
			case operation.OperationCoreUndo:
				description = m.MenuBar.undoDescription
				if m.HistoryPreview != nil && description == "" {
					entry.DisabledReason = "Nothing to " + strings.ToLower(entry.Label) + "."
				}
			case operation.OperationCoreRedo:
				description = m.MenuBar.redoDescription
				if m.HistoryPreview != nil && description == "" {
					entry.DisabledReason = "Nothing to " + strings.ToLower(entry.Label) + "."
				}
			}
			if description != "" {
				entry.Label += ": " + description
			}
			entries = append(entries, entry)
		}
		menus[i].Entries = collapseMenuSeparators(entries)
	}
	return menus
}
