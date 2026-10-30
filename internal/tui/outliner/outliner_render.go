package outliner

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/route"
	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// View implements tea.Model. It renders the full outliner tree with connector
// prefixes computed on each render pass from the project's task hierarchy.
func (m *OutlinerModel) View() tea.View {
	// Each render path records what it drew for the pointer; a path that draws
	// no list leaves nothing behind for a click to hit.
	m.pointerFrame = outlinerPointerFrame{}
	m.modalFrame = outlinerModalFrame{}
	if m.Project == nil {
		// Spinner/loading states handled by crossProjectView.
		if m.Population.SpinnerVisible || m.Population.Loading {
			return tea.NewView(m.crossProjectView())
		}
		m.CrossProjectRebuildRows()
		return m.recordReadoutFrame(tea.NewView(m.renderFullWidth()))
	}

	// Mutations explicitly rebuild; cursor motion and repaint-only events reuse
	// the projection, just as the cross-project render path does.
	m.SyncJumpProjection()
	if m.OpenAtTask != nil || !m.Population.memoized(m.singleProjectRowsFingerprint(), len(m.Focus.Stack), len(m.Project.Tasks)) {
		m.RebuildRows()
	}

	return m.recordReadoutFrame(tea.NewView(m.renderFullWidth()))
}

// recordReadoutFrame notes where the shell will draw the open Q readout: the
// last rows of the strip, which stands directly on the footer. The strip body
// is pure, so the readout measured here is the one the shell draws this frame.
func (m *OutlinerModel) recordReadoutFrame(view tea.View) tea.View {
	if readout := m.QueryReadoutBody(m.Width()); readout != "" {
		rows := lipgloss.Height(readout)
		m.pointerFrame.readoutTop = m.Height() - m.ShellFrame.ExcessRows() - rows
		m.pointerFrame.readoutRows = rows
	}
	return view
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
		fitted := chrome.FitOverlayPopupHeight(picker, m.outlinerModalBudget(width))
		composed, placement := chrome.OverlayPopupDimmedPlaced(host, fitted, width, frameHeight)
		placement.Whole = placement.Whole && fitted == picker
		m.modalFrame.picker = placement
		return composed
	}
	if menu, startCol := m.renderOutlinerMenuDropdown(width, m.outlinerModalBudget(width)); menu != "" {
		composed, placement := chrome.OverlayPopupDimmedAt(host, menu, width, frameHeight, m.outlinerHostHeaderRows(), startCol)
		m.modalFrame.dropdown = placement
		return composed
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

// The pointer's hit map is CursorScreenRow read backwards: it answers "which row
// did the last render draw on this screen row" from the window that render
// recorded, and never computes one. The rows a click can reach are the window's
// own and the pinned context row above them, which stands for the header it
// names; the title, the context row, the status strip, the Peek dock, the
// scrollbar column and the padding under a short projection answer nothing. An
// empty projection draws no window, so its render records the one row it may
// draw — the focused project header — in pointerFrame instead, and a frame that
// drew neither (the loading state) records nothing a click could hit.

// outlinerPointerFrame is what the last render drew, for the hit map alone.
type outlinerPointerFrame struct {
	// rows reports that LastViewport describes this frame's list block.
	rows bool
	// emptyHeader reports that the empty state drew the focused project header,
	// on screen row emptyHeaderRow.
	emptyHeader    bool
	emptyHeaderRow int
	// dockTop and dockRows are the screen rows the Peek dock was drawn on, its
	// divider included; dockRows is zero when no dock was drawn.
	dockTop, dockRows int
	// readoutTop and readoutRows are where the open Q readout sits at the
	// bottom of the strip the shell pins under this view.
	readoutTop, readoutRows int
}

// SetPointerKeyDispatch injects the shell's root key path, the one route a
// double-click and a right-click take to the operation their key reaches. A nil
// dispatch is inert, so a model without a shell still seats the cursor.
func (m *OutlinerModel) SetPointerKeyDispatch(dispatch func(key string) tea.Cmd) {
	m.pointerKey = dispatch
}

// outlinerPointerHit is one resolved click: the row it seats the cursor on, and
// whether it landed on that row's fold glyph.
type outlinerPointerHit struct {
	row  int
	fold bool
}

// pointerHitAt resolves a cell of this view to the row the last render drew
// there, reporting false for every cell that names no row.
func (m *OutlinerModel) pointerHitAt(x, y int) (outlinerPointerHit, bool) {
	frame := m.pointerFrame
	if frame.emptyHeader {
		if y != frame.emptyHeaderRow || x < 0 || x >= m.Width() {
			return outlinerPointerHit{}, false
		}
		if _, ok := m.focusedProjectHeaderRow(); !ok {
			return outlinerPointerHit{}, false
		}
		return outlinerPointerHit{row: 0}, true
	}
	viewport := m.LastViewport
	if !frame.rows || viewport.VisibleRows <= 0 || x < 0 || x >= viewport.rowWidth ||
		viewport.endIdx > len(m.Rows) {
		return outlinerPointerHit{}, false
	}
	offset := y - viewport.topRow
	if viewport.pinned {
		// The context row is not a row and draws no fold of its own to toggle: it
		// stands in for its header, so a click on it is a click on that header's
		// name.
		if offset == 0 {
			if viewport.pinnedHeader < 0 || viewport.pinnedHeader >= len(m.Rows) {
				return outlinerPointerHit{}, false
			}
			return outlinerPointerHit{row: viewport.pinnedHeader}, true
		}
		offset--
	}
	index := viewport.startIdx + offset
	if offset < 0 || index >= viewport.endIdx || !m.Rows[index].IsNavigable() {
		return outlinerPointerHit{}, false
	}
	return outlinerPointerHit{row: index, fold: m.pointerHitsFold(m.Rows[index], x, viewport.rowWidth)}, true
}

// pointerHitsFold reports whether column x of a drawn row is that row's fold
// glyph or the blank beside it: the two columns renderSectionHeader,
// RenderOutlinerProjectHeaderRow and renderTaskFoldIndicator spend on it. A row
// that draws no glyph — the focused project header, a checklist item, a note, a
// task with nothing to fold — has no fold column to hit, so a click there is a
// plain click.
func (m *OutlinerModel) pointerHitsFold(row OutlinerRow, x, rowWidth int) bool {
	var start int
	switch {
	case !row.IsCollapsibleHeader() && !row.IsTaskRow():
		return false
	case row.IsSectionHeader:
		start = 0
	case row.IsProjectHeader:
		start = 1
	case !rowDrawsFold(row):
		return false
	case row.LeadsMergedGroup:
		start = 1
	default:
		start = 1 + lipgloss.Width(m.outlinerTreePrefix(row, rowWidth))
	}
	return x >= start && x < start+2
}

// pointerFoldKey is the key a click on the row's fold glyph re-expresses: h
// where the glyph shows the fold open, l where it shows it closed. It reads the
// state the glyph was drawn from, so the click does what the arrow says, and h
// is never chosen where it would walk to a parent instead of folding.
func (m *OutlinerModel) pointerFoldKey(row OutlinerRow) string {
	var expanded bool
	switch {
	case row.IsHeaderRow():
		expanded = !m.IsSectionRowCollapsed(row)
	case row.HasVisibleChildren:
		expanded = m.IsExpanded(m.rowIdentity(row))
	default:
		expanded = m.IsChecklistExpanded(m.rowIdentity(row))
	}
	if expanded {
		return "h"
	}
	return "l"
}

// HandlePointer answers a click on the Outliner. A click is j/k motion onto the
// row under it, with every consequence that motion has: an active range
// stretches from its anchor to the new cursor, a frozen selection stays as it
// was, a pending relocation capture is voided, and no jump frame is pushed. A
// click on a fold glyph then toggles that fold with h or l, a double-click then
// presses Enter, and a right-click then presses Space, each through the shell's
// key path so the operation, its refusals and its undo are the key's own. A
// double-click is always preceded by its single click, so the cursor has
// already landed when Enter runs; on a fold glyph every press is a toggle, since
// the arrow is what was aimed at. A wheel step scrolls what it is over.
func (m *OutlinerModel) HandlePointer(event route.PointerEvent) tea.Cmd {
	if m == nil {
		return nil
	}
	if cmd, owned := m.handleModalPointer(event); owned {
		return cmd
	}
	if event.IsWheel() {
		m.handlePointerWheel(event)
		return nil
	}
	if m.SimplifiedInlineControlOwnsInput() {
		return nil
	}
	if event.Button != route.PointerButtonLeft && event.Button != route.PointerButtonRight {
		return nil
	}
	hit, ok := m.pointerHitAt(event.X, event.Y)
	if !ok {
		return nil
	}
	previous := m.Cursor
	m.Cursor = hit.row
	m.UpdateRangeSelection()
	m.AdoptNavigatedCursor(previous)

	key := ""
	switch {
	case event.Button == route.PointerButtonRight:
		key = "space"
	case hit.fold:
		key = m.pointerFoldKey(m.Rows[hit.row])
	case event.IsDoubleClick():
		key = "enter"
	}
	if key == "" || m.pointerKey == nil {
		return nil
	}
	return m.pointerKey(key)
}

// recordDock notes the dock's screen rows: its first line, divider included, and
// how many it drew.
func (f *outlinerPointerFrame) recordDock(top, rows int) {
	f.dockTop, f.dockRows = top, rows
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
	if m.PeekFullscreen {
		lines := m.PeekDockLines()
		m.pointerFrame.recordDock(0, len(lines))
		return strings.Join(lines, "\n")
	}
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
	m.pointerFrame = outlinerPointerFrame{rows: true}
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
	m.pointerFrame.recordDock(strings.Count(b.String(), "\n")+1, len(dock))
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
