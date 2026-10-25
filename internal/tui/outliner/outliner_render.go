package outliner

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// View implements tea.Model. It renders the full outliner tree with connector
// prefixes computed on each render pass from the project's task hierarchy.
func (m *OutlinerModel) View() tea.View {
	if m.Project == nil {
		// Spinner/loading states handled by crossProjectView.
		if m.Population.SpinnerVisible || m.Population.Loading {
			return tea.NewView(m.crossProjectView())
		}
		m.CrossProjectRebuildRows()
		return tea.NewView(m.renderFullWidth())
	}

	// Mutations explicitly rebuild; cursor motion and repaint-only events reuse
	// the projection, just as the cross-project render path does.
	m.SyncJumpProjection()
	if m.OpenAtTask != nil || !m.Population.memoized(m.singleProjectRowsFingerprint(), len(m.Focus.Stack), len(m.Project.Tasks)) {
		m.RebuildRows()
	}

	return tea.NewView(m.renderFullWidth())
}

func (m *OutlinerModel) singleProjectRowsFingerprint() string {
	return fmt.Sprintf("single=%p|%s", m.Project, m.crossProjectRowsFingerprint())
}

func (m *OutlinerModel) renderOutlinerRows(viewport outlinerViewport) string {
	colLayout := m.computeRowLayout(m.Rows, viewport.rowWidth, &m.hiddenLayout)

	var rowBuilder strings.Builder
	if viewport.pinned {
		rowBuilder.WriteString(m.renderPinnedContextRow(viewport))
		if viewport.startIdx < viewport.endIdx {
			rowBuilder.WriteString("\n")
		}
	}
	for i := viewport.startIdx; i < viewport.endIdx; i++ {
		row := m.Rows[i]
		selected := i == m.Cursor

		switch {
		case row.IsSeparator:
			rowBuilder.WriteString(chrome.RenderHorizontalDivider(viewport.rowWidth, chrome.WithDividerColor(theme.ColorBorder)))
		case row.IsSectionHeader:
			rowBuilder.WriteString(m.RenderOutlinerSectionHeaderRow(row, selected, viewport.rowWidth, i))
		case row.IsProjectHeader:
			rowBuilder.WriteString(m.RenderOutlinerProjectHeaderRow(row, selected, viewport.rowWidth, i))
		case row.IsChecklistItem:
			rowBuilder.WriteString(m.RenderOutlinerChecklistRow(row, selected, viewport.rowWidth, i))
		case row.IsNoteRow:
			rowBuilder.WriteString(m.renderOutlinerNoteRow(row, selected, viewport.rowWidth, i, colLayout))
		case selected && m.isQuickEditRow(row):
			rowBuilder.WriteString(m.RenderOutlinerQuickEditRow(row, viewport.rowWidth, colLayout))
		default:
			rowBuilder.WriteString(m.RenderOutlinerTaskRow(row, selected, viewport.rowWidth, i, colLayout))
		}

		if i < viewport.endIdx-1 {
			rowBuilder.WriteString("\n")
		}
	}

	scrollbar := chrome.RenderScrollbar(len(m.Rows), viewport.VisibleRows, viewport.startIdx)
	return chrome.AttachScrollbarAtWidth(rowBuilder.String(), scrollbar, viewport.rowWidth)
}

// renderOutlinerTreeContent renders the full-width Outliner tree constrained to
// the given width, with whatever view-local modal is open composited over it.
func (m *OutlinerModel) RenderOutlinerTreeContent(width int) string {
	return m.composeOutlinerModal(m.renderOutlinerHostContent(width), width)
}

// composeOutlinerModal puts the one open view-local modal over the view it was
// opened from. The host is drawn first and dimmed rather than dropped, so a menu
// or a picker reads as standing on the work it acts on; it is drawn through the
// ordinary path, so ComputeOutlinerViewport stays the one window authority and
// the window a modal stands over is the window its close redraws.
//
// The relation picker is checked first for the same reason it was the first
// branch before: it can only be opened from the list, and a menu cannot be open
// underneath it. Both are keyed on the drawn block rather than on the sub-state's
// own gate, because either returns nothing for a width or a resolution it cannot
// draw, and compositing an empty popup would dim the view over a modal nobody
// can see.
func (m *OutlinerModel) composeOutlinerModal(host string, width int) string {
	frameHeight := m.outlinerHostFrameHeight(width)
	if picker := m.renderRelationPicker(width); picker != "" {
		// The picker is centered and bottom-anchored, the way every shell-level
		// picker already sits, and it is fitted to what the host's header rows leave
		// so the title it stands over is never covered.
		picker = chrome.FitOverlayPopupHeight(picker, m.outlinerModalBudget(width))
		return chrome.OverlayPopupDimmed(host, picker, width, frameHeight)
	}
	if menu, startCol := m.renderOutlinerMenuDropdown(width, m.outlinerModalBudget(width)); menu != "" {
		return chrome.OverlayPopupDimmedAt(host, menu, width, frameHeight, m.outlinerHostHeaderRows(), startCol)
	}
	return host
}

// outlinerHostHeaderRows is what a view-local modal never covers: the menu bar
// row when one is drawn, the operational title, which carries the workspace
// pulse, and the context row when some axis is live. A dropdown anchors
// immediately below them, which is also the row a centered picker is fitted
// against.
func (m *OutlinerModel) outlinerHostHeaderRows() int {
	return m.menuBarRows() + 1 + m.contextRowRows()
}

// HostHeaderRows is outlinerHostHeaderRows for a shell overlay that anchors to
// a row of this view: the rows it may never cover.
func (m *OutlinerModel) HostHeaderRows() int {
	return m.outlinerHostHeaderRows()
}

// CursorScreenRow is the screen row the last render drew the cursor on,
// counted from the view's first row. It reads the window ComputeOutlinerViewport
// recorded rather than computing one, and answers false when no render has
// drawn the cursor: an empty projection, a cursor outside the window, or no
// render at all.
func (m *OutlinerModel) CursorScreenRow() (int, bool) {
	viewport := m.LastViewport
	if viewport.VisibleRows <= 0 || !m.HasContentRows() ||
		m.Cursor < viewport.startIdx || m.Cursor >= viewport.endIdx || viewport.endIdx > len(m.Rows) {
		return 0, false
	}
	row := viewport.topRow + m.Cursor - viewport.startIdx
	if viewport.pinned {
		row++
	}
	return row, true
}

// outlinerHostFrameHeight is the frame a modal composites into: the rows this
// view draws before the shell pins its strip and its footer underneath. A popup
// measured against the terminal instead would put its bottom halo behind the
// status strip, where the shell clips rather than scrolls it.
func (m *OutlinerModel) outlinerHostFrameHeight(width int) int {
	return max(m.Height()-m.StatusStripHeight(width)-m.ShellFrame.ExcessRows(), 1)
}

// outlinerModalBudget is the rows a modal may claim: the frame, less the host
// rows it may not cover. It is the one budget both modals size against, so a box
// is built at the height it is drawn at rather than fitted down afterwards,
// which is the difference between a menu that scrolls its entries and one that
// silently drops the middle of them.
func (m *OutlinerModel) outlinerModalBudget(width int) int {
	return max(m.outlinerHostFrameHeight(width)-m.outlinerHostHeaderRows(), 1)
}

// renderOutlinerHostContent renders the view itself: the bar, the title, the
// suggestion pane, the rows, and the Peek dock, or the empty state when the
// projection holds nothing.
func (m *OutlinerModel) renderOutlinerHostContent(width int) string {
	if !m.HasContentRows() {
		return m.renderOutlinerEmptyState(width)
	}

	var b strings.Builder
	// The bar leads, so the title row and its workspace pulse simply sit one line
	// down rather than sharing a row with chrome. Its cost is already out of the
	// budget contentHeight reports, which is why nothing is measured here.
	if m.MenuBarVisible() {
		b.WriteString(m.renderOutlinerMenuBar(width) + "\n")
	}
	m.writeOutlinerHeader(&b, width)

	// Render the pane once and measure what came back: RenderDropdownWithLimit
	// decides for itself how much of its budget goes to overflow hints, so a
	// predicted height and a drawn one would drift apart.
	dropdown := m.renderSearchAutocompleteDropdown(width)

	extraFooter := m.StatusStripHeight(width)
	if dropdown != "" {
		b.WriteString(dropdown + "\n")
		extraFooter += lipgloss.Height(dropdown)
	}
	if m.QuickEdit.IsActive() {
		// Only the editor's own extra rows are charged here. Its guidance is the
		// shell's footer, already measured once and delivered as the frame budget
		// outlinerListRows subtracts; re-measuring it here would charge the list
		// for the same rows twice.
		extraFooter += m.quickEditExtraLineCount()
	}

	viewport := m.ComputeOutlinerViewport(width, extraFooter)
	m.LastViewport.topRow = strings.Count(b.String(), "\n")
	b.WriteString(m.renderOutlinerRows(viewport))
	m.writePeekDock(&b, viewport)

	return b.String()
}

// writePeekDock puts the dock under the list, at the screen row the viewport
// already paid for. The list block is padded out to its whole window first: a
// projection shorter than the window would otherwise pull the dock up the frame,
// and a dock that sits in a different place per projection is the same moving
// target docking it was meant to retire.
func (m *OutlinerModel) writePeekDock(b *strings.Builder, viewport outlinerViewport) {
	dock := m.PeekDockLines()
	if len(dock) == 0 {
		return
	}
	for drawn := viewport.drawnRows(); drawn < viewport.VisibleRows; drawn++ {
		b.WriteString("\n")
	}
	for _, line := range dock {
		b.WriteString("\n")
		b.WriteString(line)
	}
}

// renderFullWidth renders the outliner tree using the full terminal width.
//
// The content is wrapped and then defensively fitted to the model width. Lip Gloss'
// width wrapper handles ordinary long headers, but some styled/wide-glyph
// boundary cases can still produce a line one cell over budget; the final pass
// keeps the full-width surface inside the terminal.
func (m *OutlinerModel) renderFullWidth() string {
	width := m.Width()
	rendered := lipgloss.NewStyle().Width(width).Render(m.RenderOutlinerTreeContent(width))
	return fitRenderedBlockToWidth(rendered, width)
}

func fitRenderedBlockToWidth(content string, width int) string {
	if width <= 0 {
		return content
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if lipgloss.Width(line) > width {
			line = textfit.TruncateIntentional(line, width, "")
		}
		if pad := width - lipgloss.Width(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}
