package outliner

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/keymap"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/route"
	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// relationPickerInnerWidth is the overlay width the picker claims out of the
// terminal. It is the named lens picker's budget, so the two searchable pickers
// sit at the same width on the same terminal.
func relationPickerInnerWidth(width int) int {
	return min(max(width*4/5, 46), max(width-6, 24))
}

// renderRelationPicker draws the box while the picker is open, or the empty
// string when it is not. The caller composites it over a dimmed host: the rows
// underneath answer none of its keys, and the dimming is what says so — the view
// stays visible as the thing the picker is standing on rather than being
// replaced by it.
func (m *OutlinerModel) renderRelationPicker(width int) string {
	if !m.RelationPickerOpen() {
		return ""
	}
	state := &m.RelationPicker
	innerWidth := relationPickerInnerWidth(width)
	contentWidth := max(innerWidth-4, 12)

	header := make([]string, 0, 2)
	if state.Notice != "" {
		header = append(header, theme.ErrorStatusStyle.Render(
			textfit.TruncateToVisualWidth(state.Notice, contentWidth),
		))
	}
	if state.Searching || state.Query != "" {
		header = append(header, state.SearchInput.View(state.Query, state.Searching, contentWidth))
	}

	guidance := chrome.RenderGuidanceText(m.relationPickerGuidance(), contentWidth)
	listHeight := min(max(len(state.Surface.Rows()), 4), 14)
	if budget := m.outlinerModalBudget(width); budget > 0 {
		// Halo, frame, title, the header block's separating row, and the footer
		// consume eight rows plus the measured footer and header lines. A short
		// terminal drops list rows rather than the search line or the footer. The
		// budget is the compositor's, so the box is built at the height it is drawn
		// at and FitOverlayPopupHeight never has to take the middle out of it.
		listHeight = min(listHeight, max(budget-8-lipgloss.Height(guidance)-len(header), 1))
	}
	content := listsurface.RenderCompactListSurface(state.Surface, contentWidth, listHeight)
	m.modalFrame.pickerListTop = chrome.OverlayBoxContentTop
	if len(header) > 0 {
		content = strings.Join(header, "\n") + "\n\n" + content
		m.modalFrame.pickerListTop += len(header) + 1
	}

	title := lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true).Render(
		textfit.TruncateToVisualWidth("Relationships · "+state.SubjectTitle, contentWidth),
	)
	return chrome.RenderOverlayBox(
		content,
		innerWidth,
		title,
		guidance,
		lipgloss.NewStyle().Foreground(theme.ColorPrimary),
	)
}

// relationPickerGuidance is the picker's own footer while it owns the input. It
// takes no footer rank of its own for the same reason the menu bar's does not:
// this is a gated keymap, and the ranked projection underneath describes keys
// the stack is currently refusing.
func (m *OutlinerModel) relationPickerGuidance() chrome.GuidanceSpec {
	if !m.RelationPickerOpen() {
		return chrome.GuidanceSpec{}
	}
	state := &m.RelationPicker
	searching := state.Searching
	navigationKeys := "j/k or ↑/↓"
	if searching {
		navigationKeys = "↑/↓"
	}
	enterLabel := "go to task"
	escapeLabel := "close"
	if searching {
		enterLabel = "apply search"
		escapeLabel = "cancel search"
		if state.Query != "" {
			escapeLabel = "clear search"
		}
	}
	km := &keymap.KeyMap{Entries: []keymap.Entry{
		keymap.Bind("up", "Move selection", "Navigation",
			keymap.WithFooter(10, "navigate", false), keymap.WithFooterKey(navigationKeys)),
		keymap.Bind("enter", "Go to the related task", "Navigation",
			keymap.WithFooter(1, enterLabel, true), keymap.WithFooterKey("Enter")),
		keymap.Bind("/", "Search the listed relationships", "Filters",
			keymap.WithGate(func() bool { return !searching }),
			keymap.WithFooter(20, "search", false)),
		keymap.Bind("esc", "Close the relationship picker", "Navigation",
			keymap.WithFooter(2, escapeLabel, true), keymap.WithFooterKey("Esc")),
	}}
	spec, _ := listsurface.ListSurfaceGuidanceSpec(state.Surface.GuidanceState(searching), km, nil)
	return spec
}

// PointerTarget is the relationship the picker's cursor names while it stands,
// the one Outliner answer the shell delivers as a pressed key; every other
// pointer key reaches the root grammar synchronously and names nothing here.
func (m *OutlinerModel) PointerTarget() string {
	if m == nil || !m.RelationPickerOpen() {
		return ""
	}
	return "relation " + m.RelationPicker.Surface.PointerTarget()
}

// relationPickerPointer is the list surfaces' shared answer inside the picker:
// a click seats the cursor, a double-click is Enter on the row, and the wheel
// steps the cursor. The picker has no Space, and while its search field is
// being edited Enter is the field's own, so a click there only seats. An event
// outside the popup does nothing, and so does a click on a popup fitting cut.
func (m *OutlinerModel) relationPickerPointer(event route.PointerEvent) tea.Cmd {
	state := &m.RelationPicker
	placement := m.modalFrame.picker
	if state.Surface == nil || !placement.Contains(event.X, event.Y) || (!placement.Whole && !event.IsWheel()) {
		return nil
	}
	key := state.Surface.AnswerPointer(
		event.Offset(placement.Col+chrome.OverlayBoxContentLeft, placement.Row+m.modalFrame.pickerListTop),
		false,
	)
	if state.Searching && key == "enter" {
		return nil
	}
	return route.PressKey(key)
}
