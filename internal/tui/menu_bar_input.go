package tui

import (
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
)

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

// openMenuBar drops the first menu out of the bar. It always opens on the first
// menu rather than on the last one used: the bar is read left to right and an
// open menu that remembers where it was would put the highlight somewhere the
// user did not leave it, one whole session ago.
func (m *OutlinerModel) openMenuBar() {
	if m == nil {
		return
	}
	m.menuBarOpen = true
	m.menuBarMenuIndex = 0
	m.menuBarItemIndex = 0
}

// closeMenuBar puts the menu away without asking what it was showing. Every
// reset that replaces the whole projection calls it, so a dropdown resolved
// against one population cannot survive into another.
func (m *OutlinerModel) closeMenuBar() {
	if m == nil {
		return
	}
	m.menuBarOpen = false
}

// openMenuBarMenus is the resolution both the dropdown and this keymap read.
// Navigation walks the menus that survived filtering rather than the specs, so
// `l` never stops on a label the bar is not drawing, and the highlight the
// render paints is indexed into the same slice Enter dispatches from.
//
// A resolution that came back empty is a bar with nothing to open, which closes
// the sub-state rather than leaving a gate open over no menus.
func (m *OutlinerModel) openMenuBarMenus() ([]resolvedMenu, bool) {
	if m == nil || !m.menuBarOpen {
		return nil, false
	}
	menus := m.resolvedMenuBar()
	if len(menus) == 0 {
		m.closeMenuBar()
		return nil, false
	}
	m.menuBarMenuIndex = min(max(m.menuBarMenuIndex, 0), len(menus)-1)
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
func resolveMenuBarItem(entries []resolvedMenuEntry, index int) int {
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
func stepMenuBarItem(entries []resolvedMenuEntry, index, delta int) int {
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
// Outliner branch while a menu is open, and it is the only place the two stored
// indices are written.
func (m *OutlinerModel) handleMenuBarKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	menus, ok := m.openMenuBarMenus()
	if !ok {
		return m, nil
	}
	entries := menus[m.menuBarMenuIndex].Entries
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
		m.menuBarItemIndex = stepMenuBarItem(entries, m.menuBarItemIndex, -1)
		return m, nil
	case "j", "down":
		m.menuBarItemIndex = stepMenuBarItem(entries, m.menuBarItemIndex, 1)
		return m, nil
	case "enter":
		index := resolveMenuBarItem(entries, m.menuBarItemIndex)
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

// cycleMenuBarMenu moves along the bar and starts the new menu at its top. It
// wraps, unlike the vertical walk: the bar is a ring of five labels a user
// crosses repeatedly, and stopping at Help would make the leftmost menu three
// presses away from the rightmost.
func (m *OutlinerModel) cycleMenuBarMenu(delta, count int) {
	if count <= 0 {
		return
	}
	m.menuBarMenuIndex = ((m.menuBarMenuIndex+delta)%count + count) % count
	m.menuBarItemIndex = 0
}

// menuBarOpenKeys is the grammar's own spelling of the key that opens a menu,
// read back out of the strategy rather than repeated here, so a rebinding moves
// the close arm above and the footer hint below together.
func (m *OutlinerModel) menuBarOpenKeys() []string {
	if m == nil || m.operationRegistry == nil {
		return nil
	}
	strategy, ok := m.operationRegistry.InputStrategy(operation.InputStrategySimplified)
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
	if m.operationRegistry != nil {
		if key := m.operationRegistry.InputShortcutForOperation(
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
