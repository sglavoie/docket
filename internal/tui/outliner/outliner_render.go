package outliner

import (
	"fmt"
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
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

	// Rebuild the mode's flat row list and clamp the cursor.
	m.RebuildRows()

	return tea.NewView(m.renderFullWidth())
}

func (m *OutlinerModel) renderOutlinerRows(viewport outlinerViewport) string {
	var colLayout *chrome.ColumnLayout
	if viewport.rowWidth >= chrome.MinColumnAlignWidth {
		colLayout = ComputeOutlinerColumnLayout(m.Rows[viewport.startIdx:viewport.endIdx])
		// Augment with status widths from the same row set.
		var visibleTasks []*models.Task
		for _, row := range m.Rows[viewport.startIdx:viewport.endIdx] {
			if !row.IsTaskRow() || row.Task == nil {
				continue
			}
			visibleTasks = append(visibleTasks, row.Task)
		}
		if badge := chrome.ComputeBadgeWidths(visibleTasks, chrome.BadgeWidthOpts{
			IncludeStatus: true,
		}); badge != nil {
			if colLayout == nil {
				colLayout = badge
			} else {
				colLayout.HasStatus = badge.HasStatus
				colLayout.StatusWidth = badge.StatusWidth
			}
		}
	}

	var rowBuilder strings.Builder
	for i := viewport.startIdx; i < viewport.endIdx; i++ {
		row := m.Rows[i]
		selected := i == m.Cursor

		switch {
		case row.IsSeparator:
			rowBuilder.WriteString(chrome.RenderHorizontalDivider(viewport.rowWidth))
		case row.IsSectionHeader:
			rowBuilder.WriteString(m.RenderOutlinerSectionHeaderRow(row, selected, viewport.rowWidth, i))
		case row.IsProjectHeader:
			rowBuilder.WriteString(m.RenderOutlinerProjectHeaderRow(row, selected, viewport.rowWidth, i))
		case row.IsChecklistItem:
			rowBuilder.WriteString(m.RenderOutlinerChecklistRow(row, selected, viewport.rowWidth, i))
		case row.IsNoteRow:
			rowBuilder.WriteString(m.renderOutlinerNoteRow(row, selected, viewport.rowWidth, i, colLayout))
		case selected && m.isQuickEditRow(row):
			rowBuilder.WriteString(m.RenderOutlinerQuickEditRow(row, viewport.rowWidth))
		default:
			rowBuilder.WriteString(m.RenderOutlinerTaskRow(row, selected, viewport.rowWidth, viewport.rowWidth, i, colLayout))
		}

		if i < viewport.endIdx-1 {
			rowBuilder.WriteString("\n")
		}
	}

	scrollbar := chrome.RenderScrollbar(len(m.Rows), viewport.VisibleRows, viewport.startIdx)
	return chrome.AttachScrollbarAtWidth(rowBuilder.String(), scrollbar, viewport.rowWidth)
}

// cursorPaint decides how one row wears the cursor. One call answers both
// whether a row is the cursor and whether it is mid-flash, so no renderer reads
// m.CursorFlash on its own.
type cursorPaint struct {
	marked bool
	flash  bool
}

func (m *OutlinerModel) cursorPaintFor(selected bool) cursorPaint {
	return cursorPaint{marked: selected, flash: selected && m.CursorFlash}
}

// bar renders the row's leftmost column, the whole of the resting mark. It is
// inverse video rather than a background merged into the glyph's own style
// because one column has to carry the mark alone, so it takes the loudest
// treatment available and gives up the muted gray it would otherwise wear. Every
// column it can cover is structural margin: a blank, or a fold arrow that stays
// legible reversed.
func (p cursorPaint) bar(style lipgloss.Style, text string) string {
	switch {
	case !p.marked:
		return style.Render(text)
	case p.flash:
		return theme.CursorFlashBarStyle.Render(text)
	default:
		return theme.CursorBarStyle.Render(text)
	}
}

// gutterBg names the background the gutter columns after the bar carry. At rest
// there is none — the connectors, the fold arrow and the multi-select box stay
// ordinary, because widening the mark is what made it compete with the row. A
// flash spends exactly that: it floods the whole gutter, which reads as more of
// the same mark rather than a different one.
func (p cursorPaint) gutterBg() color.Color {
	if !p.flash {
		return nil
	}
	return theme.ColorFlashBg
}

// trail styles the columns past the row's content, dim enough to read as the
// cursor's reach rather than compete with the bar for the eye.
func (p cursorPaint) trail() lipgloss.Style {
	if p.flash {
		return theme.CursorFlashTrailStyle
	}
	return theme.CursorTrailStyle
}

// gutterPiece renders one leading structural glyph in its own style, carrying
// the cursor background when bg is non-nil. The background belongs on each piece
// rather than on the assembled gutter: a style applied over already-styled text
// ends at that text's own reset, which would leave the rest of the block bare.
func gutterPiece(bg color.Color, style lipgloss.Style, text string) string {
	if bg != nil {
		style = style.Background(bg)
	}
	return style.Render(text)
}

// gutterSpace paints one of the gutter's structural blank columns, which carry
// no color of their own to preserve.
func gutterSpace(bg color.Color) string {
	return gutterPiece(bg, lipgloss.NewStyle(), " ")
}

// outlinerCursorRow marks the cursor without repainting it. The gutter arrives
// already painted — its first column bright, the rest of it ordinary — and the
// columns past the content take a dim fill, so every color the row earned
// survives under the cursor and no drawn column is covered. The result is always
// exactly rowWidth wide, which is what attachScrollbarAtWidth expects.
// Truncation only ever removes from the end, so clamping the content to the
// columns the gutter leaves cuts where a clamp of the whole line would.
func outlinerCursorRow(gutter, content string, rowWidth int, paint cursorPaint) string {
	gutterWidth := lipgloss.Width(gutter)
	if rowWidth <= gutterWidth {
		return textfit.TruncateToVisualWidth(gutter, rowWidth)
	}
	content = textfit.TruncateToVisualWidth(content, rowWidth-gutterWidth)
	fill := rowWidth - gutterWidth - lipgloss.Width(content)
	if fill <= 0 {
		return gutter + content
	}
	return gutter + content + paint.trail().Render(strings.Repeat(" ", fill))
}

// sectionHeaderOpts configures the section header renderer below.
type sectionHeaderOpts struct {
	Name        string
	Count       int
	Collapsed   bool
	AccentColor lipgloss.Style // applied to the name portion
	Width       int            // if > 0, trailing ─ fill is added
	Prefix      string         // inserted between indicator and name (e.g. "@ ")
	Cursor      cursorPaint    // how this header wears the cursor; zero on an ordinary row
}

// renderSectionHeader renders a standardized collapsible section header:
//
//	▾ [prefix]Name (count) ─────
//
// It returns the leading fold gutter and the rest separately, so a caller
// marking the cursor row can paint the gutter without repainting the name. This
// is the one header whose first column is a glyph rather than a blank, so the
// cursor bar reverses the fold arrow instead of a space.
// AccentColor is applied to the name; indicator and count use MutedStyle.
// When Width > 0 a trailing horizontal rule fills the remaining line width.
// Callers are responsible for applying selection/zebra highlights on the
// returned strings.
func renderSectionHeader(opts sectionHeaderOpts) (gutter, content string) {
	indicator := theme.IconExpanded
	if opts.Collapsed {
		indicator = theme.IconCollapsed
	}
	gutter = opts.Cursor.bar(theme.MutedStyle, indicator) + gutterSpace(opts.Cursor.gutterBg())
	namePart := opts.AccentColor.Bold(true).Render(opts.Prefix + opts.Name)
	content = namePart + " " + theme.MutedStyle.Render(fmt.Sprintf("(%d)", opts.Count))

	if opts.Width > 0 {
		visible := lipgloss.Width(gutter) + lipgloss.Width(content)
		if opts.Width > visible+2 {
			content += " " + theme.MutedStyle.Render(strings.Repeat("─", opts.Width-visible-1))
		}
	}
	return gutter, content
}

func (m *OutlinerModel) RenderOutlinerSectionHeaderRow(row OutlinerRow, selected bool, rowWidth, rowIndex int) string {
	displayName := row.SectionName
	if displayName == "" {
		displayName = models.StateGroupBacklog.Label()
	}

	sectionStyle := theme.GetWorkflowStateGroupStyle(models.StateGroupBacklog)
	if row.StateGroup.IsValid() {
		sectionStyle = theme.GetWorkflowStateGroupStyle(row.StateGroup)
	}
	if row.PromotedTag == reviewLaneKey {
		sectionStyle = theme.ViewModeReviewStyle
	}
	prefix := "@ "
	if row.IsComputedHeader {
		prefix = "◷ "
	}

	paint := m.cursorPaintFor(selected)
	gutter, content := renderSectionHeader(sectionHeaderOpts{
		Name:        displayName,
		Count:       row.SectionCount,
		Collapsed:   m.IsSectionRowCollapsed(row),
		AccentColor: sectionStyle,
		Prefix:      prefix,
		Cursor:      paint,
	})

	if row.ProjectID != "" {
		project := m.ResolveProjectForRow(row)
		projectName := row.ProjectID
		if project != nil {
			projectName = project.Name
		}
		content += "  " + theme.ProjectColorStyle(project).Render("["+projectName+"]")
	}
	content = appendManualOrderMarker(content, row, m.Sort.Field, rowWidth-lipgloss.Width(gutter))

	if selected {
		return outlinerCursorRow(gutter, content, rowWidth, paint)
	}

	return chrome.ZebraLine(gutter+content, rowWidth, rowIndex)
}

// rowKeepsManualOrder reports whether the section a row belongs to is one the
// sort never touches. Pinned, Today and Next carry an order the user built by
// hand, and Review is ordered by the projection that promoted it.
func rowKeepsManualOrder(row OutlinerRow) bool {
	return row.IsPinnedSection ||
		row.IsTodaySection ||
		row.IsNextSection ||
		row.PromotedTag == reviewLaneKey
}

// appendManualOrderMarker names an exempt section while a sort is active, so a
// lane that did not move says why instead of reading as a broken key. It is
// dropped rather than allowed to overflow, exactly as the pressure chips are.
// budget is the columns the content region has, which is the row width less the
// gutter the cursor highlight owns.
func appendManualOrderMarker(content string, row OutlinerRow, field query.SortField, budget int) string {
	if field == query.SortByDefault || !rowKeepsManualOrder(row) {
		return content
	}
	suffix := theme.MutedStyle.Render("  · manual")
	if lipgloss.Width(content)+lipgloss.Width(suffix) > budget {
		return content
	}
	return content + suffix
}

// renderOutlinerProjectHeaderRow renders a synthetic project sub-header row
// that visually clusters adjacent tasks from the same project in cross-project
// modes. In All these rows sit beneath shared state-group headers. The header
// reuses the persisted project color and slug
// text so the inline `[slug]` badges on individual task rows can be suppressed.
// They use the same fold indicator as built-in sections, while Enter retains
// its project scope behavior.
func (m *OutlinerModel) RenderOutlinerProjectHeaderRow(row OutlinerRow, selected bool, rowWidth, rowIndex int) string {
	project := m.ResolveProjectForRow(row)
	projectSlug := row.ProjectID
	if project != nil {
		projectSlug = project.Slug
		if projectSlug == "" {
			projectSlug = project.Name
		}
	}
	badge := theme.ProjectColorStyle(project).
		Bold(true).
		Render("[" + projectSlug + "]")
	indicator := theme.IconExpanded
	if m.IsSectionRowCollapsed(row) {
		indicator = theme.IconCollapsed
	}
	paint := m.cursorPaintFor(selected)
	bg := paint.gutterBg()
	gutter := paint.bar(lipgloss.NewStyle(), " ") +
		gutterPiece(bg, theme.MutedStyle, indicator) +
		gutterSpace(bg)
	// The marker leads the badge so a pinned project's `*` sits at the same
	// column as a pinned task's one row below. It goes on before the pressure
	// ladder measures, so its two columns are inside that budget and today or
	// overdue sheds first; the marker itself is never dropped. Unlike the task
	// row's it is not reserved — a header has no ColumnLayout, so an unpinned
	// header pays nothing for it.
	content := badge
	if project != nil && project.Pinned {
		content = listsurface.PinnedMarker() + badge
	}
	if row.SectionCount > 0 {
		content += " " + theme.MutedStyle.Render(fmt.Sprintf("(%d)", row.SectionCount))
	}
	content = appendProjectHeaderPressure(content, row, rowWidth-lipgloss.Width(gutter))

	if selected {
		return outlinerCursorRow(gutter, content, rowWidth, paint)
	}

	return chrome.ZebraLine(textfit.TruncateToVisualWidth(gutter+content, rowWidth), rowWidth, rowIndex)
}

// appendProjectHeaderPressure names the overdue and Today-planned work a
// project group holds, so folding the group does not hide what is urgent in it.
// The suffix degrades before the row is truncated — today drops first, then
// overdue — because a clipped count reads as a different number. Each chip
// carries a glyph so it cannot be mistaken for the bare group size beside it.
// budget is the columns the content region has, which is the row width less the
// gutter the cursor highlight owns.
func appendProjectHeaderPressure(content string, row OutlinerRow, budget int) string {
	var parts []string
	if row.OverdueCount > 0 {
		parts = append(parts, theme.PreviewWarningStyle.Render(fmt.Sprintf("! overdue %d", row.OverdueCount)))
	}
	if row.TodayCount > 0 {
		parts = append(parts, theme.MutedStyle.Render(fmt.Sprintf("◷ today %d", row.TodayCount)))
	}
	for ; len(parts) > 0; parts = parts[:len(parts)-1] {
		suffix := "  " + strings.Join(parts, theme.MutedStyle.Render(" · "))
		if lipgloss.Width(content)+lipgloss.Width(suffix) <= budget {
			return content + suffix
		}
	}
	return content
}

// renderOutlinerChecklistRow renders one expanded checklist item beneath its
// owner task. It reuses the overlay's `[ ]`/`[x]` box and muted done text so the
// same item reads the same way wherever it is shown.
func (m *OutlinerModel) RenderOutlinerChecklistRow(row OutlinerRow, selected bool, rowWidth, rowIndex int) string {
	text := ""
	if row.Task != nil && row.ChecklistIndex < len(row.Task.Checklist) {
		text = strings.ReplaceAll(row.Task.Checklist[row.ChecklistIndex].Text, "\n", " ")
	}
	box := "[ ] "
	if row.ChecklistDone {
		box = "[x] "
		text = theme.MutedStyle.Render(text)
	}
	paint := m.cursorPaintFor(selected)
	prefixBudget := max(min(rowWidth/3, 12), 4)
	gutter := paint.bar(lipgloss.NewStyle(), theme.RenderCursorPrefix()) +
		gutterPiece(paint.gutterBg(), theme.MutedStyle, compactOutlinerTreePrefix(row.Prefix, prefixBudget))
	content := theme.MutedStyle.Render(box) + text

	if selected {
		return outlinerCursorRow(gutter, content, rowWidth, paint)
	}
	return chrome.ZebraLine(textfit.TruncateToVisualWidth(gutter+content, rowWidth), rowWidth, rowIndex)
}

// noteRowGlyph marks a note in a mixed result list. It is a literal here rather
// than an icons registry key for the same reason the section header's `@` is: it
// names a row kind this renderer draws, not an entity the rest of the product
// has to spell the same way.
const noteRowGlyph = "≡"

// renderOutlinerNoteRow draws one row of the search projection's notes tier. It
// keeps the task row's rhythm — the same cursor bar, the same badge columns, the
// same muted match cue in the same place — without borrowing task furniture: a
// note has no state, priority, or due date, so those columns carry the note
// glyph and blanks rather than a plausible-looking badge. Holding the columns
// open is what keeps the notes block's titles on the column the task matches
// above it use.
func (m *OutlinerModel) renderOutlinerNoteRow(
	row OutlinerRow,
	selected bool,
	rowWidth, rowIndex int,
	colLayout *chrome.ColumnLayout,
) string {
	entry := m.Search.noteMatches[row.NoteRow]
	paint := m.cursorPaintFor(selected)
	bg := paint.gutterBg()
	gutter := paint.bar(lipgloss.NewStyle(), theme.RenderCursorPrefix())
	if m.MultiSelectMode {
		// A note is not selectable, so the checkbox column is held open rather than
		// drawn: an empty box beside the task boxes would advertise a membership
		// every selection operation refuses.
		gutter += gutterPiece(bg, lipgloss.NewStyle(), "   ") + gutterSpace(bg)
	}

	glyph := theme.MutedStyle.Render(noteRowGlyph)
	if colLayout != nil && colLayout.HasStatus {
		glyph = theme.PadBadge(glyph, colLayout.StatusWidth)
	}
	priorityColumn := ""
	if colLayout != nil && colLayout.HasPriority {
		priorityColumn = strings.Repeat(" ", colLayout.PriorityWidth+1)
	}
	owner := ""
	if entry.row.OwnerLabel != "" {
		owner = "  " + theme.MutedStyle.Render("["+entry.row.OwnerLabel+"]")
	}
	cue := taskMatchSourceCue(row)

	// The space after the glyph is the one the task row spends after its state
	// badge, so both blocks start their content at the same column.
	titleWidth := func() int {
		return max(rowWidth-lipgloss.Width(gutter)-lipgloss.Width(glyph)-1-
			lipgloss.Width(priorityColumn)-lipgloss.Width(owner)-lipgloss.Width(cue), 1)
	}
	// The note row sheds the same two things in the same order the task row does,
	// under the same title target: the match cue first, because it explains a
	// result whose title has become unreadable, and the owner label second. Both
	// are dropped rather than truncated — the whole line is clamped to rowWidth at
	// the end, and a cue cut mid-word would read as a different word.
	titleTarget := min(8, max(rowWidth/4, 4))
	if titleWidth() < titleTarget {
		cue = ""
	}
	if titleWidth() < titleTarget {
		owner = ""
	}
	title := ""
	if entry.row.Note != nil {
		title = textfit.TruncateToVisualWidth(entry.row.Note.Title, titleWidth())
	}
	content := glyph + " " + priorityColumn +
		textfit.HighlightText(title, entry.match.TitlePositions, theme.NormalStyle) + owner + cue

	if selected {
		return outlinerCursorRow(gutter, content, rowWidth, paint)
	}
	return chrome.ZebraLine(textfit.TruncateToVisualWidth(gutter+content, rowWidth), rowWidth, rowIndex)
}

// ComputeOutlinerColumnLayout computes a ColumnLayout by scanning a slice of
// visible outliner rows. Non-task rows (section headers, project sub-headers,
// separators) are skipped via isTaskRow(). Returns nil when no task rows are
// present.
func ComputeOutlinerColumnLayout(rows []OutlinerRow) *chrome.ColumnLayout {
	if len(rows) == 0 {
		return nil
	}
	cl := &chrome.ColumnLayout{}
	hasRows := false
	for _, row := range rows {
		if !row.IsTaskRow() || row.Task == nil {
			continue
		}
		hasRows = true
		task := row.Task

		// Priority (pre-title in outliner)
		badge := chrome.RenderPriorityBadge(task.Priority)
		if w := lipgloss.Width(badge); w > 0 {
			cl.HasPriority = true
			if w > cl.PriorityWidth {
				cl.PriorityWidth = w
			}
		}
	}
	if !hasRows {
		return nil
	}
	return cl
}

type outlinerViewport struct {
	VisibleRows int
	startIdx    int
	endIdx      int
	rowWidth    int
}

// renderOutlinerTreeContent renders the full-width Outliner tree constrained to
// the given width.
func (m *OutlinerModel) RenderOutlinerTreeContent(width int) string {
	// An open menu hangs off the label it was opened from, so it is drawn before
	// anything else on the frame — and it is the whole frame. The branch is keyed
	// on the drawn block rather than on the menuBar sub-state's gate, because the
	// dropdown returns
	// nothing for a width or a resolution it cannot draw, and blanking the view
	// over a menu nobody can see would be the worse failure.
	if picker := m.renderRelationPicker(width); picker != "" {
		return picker
	}
	if menu := m.renderOutlinerMenuDropdown(width); menu != "" {
		return m.renderOutlinerMenuFrame(width, menu)
	}
	if len(m.Rows) == 0 {
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
		extraFooter += m.quickEditExtraLineCount()
		if spec, ok := m.ContextualGuidance(); ok {
			extraFooter += max(lipgloss.Height(chrome.RenderGuidance(spec, width))-2, 0)
		}
	}

	viewport := m.ComputeOutlinerViewport(width, extraFooter)
	b.WriteString(m.renderOutlinerRows(viewport))

	return b.String()
}

// statusStripBody builds the outstanding-conflict indicator, the focus notice,
// the status message, and the expanded query readout the shell pins above the
// footer. It is pure: the height reservation and the strip itself must measure
// the same string.
//
// The indicator is an ambient advisory, so it is gated by advisoryLine: hiding
// it drops the row it reserved too, because the reservation measures this same
// body.
//
// The indicator leads because it is the one durable line here — it stands until
// the CLI resolves the last bundle, while the lines below it come and go — so a
// transient line arriving never shifts it to another row. The readout sits last
// for the same reason from the other end: it is the tallest thing the strip can
// hold, and growing it must not push the three one-line signals around.
func (m *OutlinerModel) StatusStripBody(width int) string {
	width = max(width, 1)
	lines := make([]string, 0, 4)
	if indicator := m.AdvisoryLine(m.conflictIndicator()); indicator != "" {
		lines = append(lines, theme.PreviewWarningStyle.Width(width).Render(indicator))
	}
	if m.Focus.Notice != "" {
		lines = append(lines, theme.PreviewWarningStyle.Width(width).Render(m.Focus.Notice))
	}
	if m.StatusMessage != "" {
		lines = append(lines, theme.MutedStyle.Width(width).Render(m.StatusMessage))
	}
	// The expanded stack changes this return value rather than painting outside
	// it, so statusStripHeight reserves the rows it needs from the list.
	if readout := m.QueryReadoutBody(width); readout != "" {
		lines = append(lines, readout)
	}
	return strings.Join(lines, "\n")
}

// statusStripHeight is the strip's divider plus its wrapped lines. The Outliner
// reserves it from its own row budget, so the strip takes rows from the list
// rather than pushing the footer off screen.
func (m *OutlinerModel) StatusStripHeight(width int) int {
	body := m.StatusStripBody(width)
	if body == "" {
		return 0
	}
	return 1 + lipgloss.Height(body)
}

// ViewStatusStrip hands the strip to the shell and consumes the one-shot focus
// notice. AppModel.View renders the view before the status bar, so the
// reservation above and this call see the same inputs in the same frame.
func (m *OutlinerModel) ViewStatusStrip(width int) string {
	body := m.StatusStripBody(width)
	m.Focus.clearNotice()
	return body
}

func (m *OutlinerModel) outlinerFocusContext() string {
	if m.Project == nil {
		return m.CrossProjectFocusContext()
	}
	return m.ProjectFocusContext()
}

// buildOutlinerSearchIndicator renders the search chrome, appending a badge only
// when the title row can hold it: at narrow widths the query itself is the more
// useful half of the segment. A refused sigil wins over the case badge, because a
// query that cannot run explains an empty list better than how it was matched.
func (m *OutlinerModel) buildOutlinerSearchIndicator(width int) string {
	var indicator string
	switch {
	case m.EditingLensQuery():
		// The lens badge reads the lens compilation rather than the search one,
		// because those are the rows on screen.
		return m.appendLensQueryBadge(theme.MutedStyle.Render("Lens query: ")+m.SearchInputView(), width)
	case m.HasActiveLens() && m.Lens.Query != "":
		return m.appendLensQueryBadge(theme.MutedStyle.Render(fmt.Sprintf("Lens query: %q", m.Lens.Query)), width)
	case m.IsSearchMode():
		indicator = theme.MutedStyle.Render("Search: ") + m.SearchInputView()
	case m.SearchQuery() != "":
		indicator = theme.MutedStyle.Render(fmt.Sprintf("Search: \"%s\"", m.SearchQuery()))
	default:
		return ""
	}
	if guidance := m.Search.Gate.GuidanceText(); guidance != "" {
		return appendSearchBadge(indicator, " "+guidance, width)
	}
	if !m.Search.caseRelaxed {
		return indicator
	}
	return appendSearchBadge(indicator, " (case ignored)", width)
}

// appendLensQueryBadge gives the lens query the badges ordinary search already
// uses, read off the lens compilation: a refused sigil wins over the case badge
// there for the same reason it does in search.
func (m *OutlinerModel) appendLensQueryBadge(indicator string, width int) string {
	if guidance := m.LensGuidanceText(); guidance != "" {
		return appendSearchBadge(indicator, " "+guidance, width)
	}
	if !m.Lens.NarrowRelaxed {
		return indicator
	}
	return appendSearchBadge(indicator, " (case ignored)", width)
}

func appendSearchBadge(indicator, text string, width int) string {
	badge := theme.MutedStyle.Render(text)
	if lipgloss.Width(indicator)+lipgloss.Width(badge) > width {
		return indicator
	}
	return indicator + badge
}

func (m *OutlinerModel) outlinerScopeLabel() string {
	if m.Project == nil {
		if m.IsProjectScoped() {
			return m.scopedProjectChromeLabel(m.Scope.ProjectID)
		}
		return m.baseScopeChromeLabel()
	}
	return m.Project.Name
}

func (m *OutlinerModel) buildOutlinerFilterContext() string {
	sortField := m.Sort.Field
	sortAscending := m.Sort.Ascending
	// A fixed-order mode owns its ordering, so a sort recorded there is not on
	// screen and must not be advertised. Under a search projection the mode
	// underneath is irrelevant — the sort is live over the ranked rows — so
	// zeroing the segment there would silently hide it.
	if m.ActiveOutlinerModeEntry().FixedOrder() && !m.ShowsRankedList() {
		sortField = query.SortByDefault
		sortAscending = false
	}
	suffix := buildTitleFilterSuffix(&TitleParams{
		FilterParts:   m.ActiveFilterParts(),
		SortField:     sortField,
		SortAscending: sortAscending,
	}, 0)
	return strings.TrimPrefix(suffix, "  ")
}

func (m *OutlinerModel) BuildOutlinerInteractionContext(width int) []string {
	context := make([]string, 0, 5)
	if m.MultiSelectMode {
		context = append(context, theme.SelectionCountStyle.Render(fmt.Sprintf("MULTI-SELECT: %d selected", m.SelectedCount())))
	}
	// The lens names the whole context the segments after it describe, so it is
	// read first and its marker states whether they still match what was saved.
	if lens := m.ActiveLensLabel(); lens != "" {
		context = append(context, theme.MutedStyle.Render(lens))
	}
	if search := m.buildOutlinerSearchIndicator(width); search != "" {
		context = append(context, search)
	}
	if filter := m.buildOutlinerFilterContext(); filter != "" {
		context = append(context, filter)
	}
	if focus := m.outlinerFocusContext(); focus != "" {
		context = append(context, theme.MutedStyle.Render(focus))
	}
	// Last, so that the one joined Context region loses the zoom before it loses
	// a narrowing the user cannot leave with a single Esc.
	if m.Focus.Zoom.Active() && m.Focus.Zoom.Name != "" {
		context = append(context, theme.MutedStyle.Render("Section: "+m.Focus.Zoom.Name))
	}
	return context
}

// buildOutlinerTitleLine assembles the Outliner's one-row operational title.
func (m *OutlinerModel) BuildOutlinerTitleLine(width int) string {
	// Read once into a local: the spec takes the pulse by pointer, and the reader
	// is the promoted accessor rather than a field, so there is no address to
	// take without naming the value first.
	workspacePulse := m.CurrentWorkspacePulse()
	var pace *pulse.TodayPaceReadout
	if m.PrimaryMode == chrome.ModeToday && m.CompletionActivity.HasHistory() {
		pace = &pulse.TodayPaceReadout{
			OpenCount:    workspacePulse.Today,
			MedianPerDay: m.CompletionActivity.MedianPerDay(),
		}
	}
	return pulse.RenderOperationalTitle(pulse.OperationalTitleSpec{
		Scope:   theme.TitleStyle.Render(m.outlinerScopeLabel()),
		Mode:    chrome.OperationalTitleModeSegment(m.ActiveOutlinerModeEntry()),
		Context: m.BuildOutlinerInteractionContext(max(width, 1)),
		Pace:    pace,
		Pulse:   &workspacePulse,
	}, max(width, 1))
}

func (m *OutlinerModel) writeOutlinerHeader(b *strings.Builder, width int) {
	b.WriteString(m.BuildOutlinerTitleLine(width))
	b.WriteString("\n")
}

// renderOutlinerMenuFrame is the whole frame while a menu is open: the bar row
// and the block hanging off the open label, and nothing under them. The list is
// not shrunk to make room, it is not drawn at all, so opening and closing a menu
// moves no row on screen — the rows behind a dropdown were unreadable anyway,
// and a list that shifted twice per menu press was the cost of pretending
// otherwise.
//
// Nothing here calls computeOutlinerViewport, deliberately. It is the single
// window authority and the only writer of ViewScrollOffset, so a frame that
// draws no rows must leave the window the last one drew exactly as it was: that
// is the window the close redraws, and the window a key re-emitted through the
// non-modal arm pages against.
func (m *OutlinerModel) renderOutlinerMenuFrame(width int, menu string) string {
	var b strings.Builder
	b.WriteString(m.renderOutlinerMenuBar(width) + "\n")
	b.WriteString(menu)
	return b.String()
}

func (m *OutlinerModel) renderOutlinerEmptyState(width int) string {
	var b strings.Builder
	// The empty state carries the bar for the same reason it carries the
	// suggestion pane: a view with no rows is exactly where a user reaches for the
	// map of what they can do next. An open menu never reaches here — it is its
	// own frame, drawn before the row count is even consulted.
	if m.MenuBarVisible() {
		b.WriteString(m.renderOutlinerMenuBar(width) + "\n")
	}
	b.WriteString(m.BuildOutlinerTitleLine(width))
	b.WriteString("\n")
	// A query that matches nothing yet is exactly when suggestions are worth the
	// most, so the empty state carries the pane too.
	if dropdown := m.renderSearchAutocompleteDropdown(width); dropdown != "" {
		b.WriteString(dropdown + "\n")
	}
	b.WriteString(theme.MutedStyle.Render(m.OutlinerEmptyMessage()))
	return b.String()
}

func (m *OutlinerModel) OutlinerEmptyMessage() string {
	switch m.ClassifyEmptyCause() {
	case outlinerEmptyNoProjects:
		return "No projects are available. Press P to open Projects."
	case outlinerEmptyProjectFiltered:
		return "No projects match project filters. Press Fx to clear project filters."
	case outlinerEmptySearch:
		return fmt.Sprintf("No tasks match search %q. Press Esc to clear.", m.SearchQuery())
	case OutlinerEmptySearchFiltered:
		return fmt.Sprintf(
			"No tasks match search %q under the active filters. Press fx to clear filters or Esc to clear the search.",
			m.SearchQuery())
	case outlinerEmptySearchInvalid:
		return fmt.Sprintf("%s. Press Esc to clear.", m.Search.Gate.GuidanceText())
	case outlinerEmptyLens:
		return "No tasks match this lens. Press Esc to restore the view it replaced."
	case outlinerEmptyFiltered:
		return listsurface.EmptyStateFiltered
	case outlinerEmptyFocus:
		return "No tasks are visible in this Focus. Press Esc to return."
	case OutlinerEmptySection:
		return "No tasks are left in this section. Press Esc to return."
	case outlinerEmptyMode:
		mode := m.ActiveOutlinerModeEntry()
		if mode.ID == chrome.ModeInbox {
			return "Inbox is empty. Press a to capture or Tab to change modes."
		}
		return fmt.Sprintf("%s is empty. Press Tab or Shift+Tab to change modes.", mode.Label)
	default:
		return listsurface.EmptyStateNoTasks
	}
}

func (m *OutlinerModel) ComputeOutlinerViewport(width int, extraFooterLines int) outlinerViewport {
	// Clamp to at least one row: at pathologically small heights (e.g. the
	// dimmed project-cycle/pinned preview backgrounds sized to a couple of
	// lines) contentHeight()-extraFooterLines can go non-positive, which would
	// drive a negative window slice.
	visibleRows := max(m.ContentHeight()-extraFooterLines, 1)

	if m.PeekMode && m.Cursor >= 0 && m.Cursor < len(m.Rows) && m.Rows[m.Cursor].IsTaskRow() {
		// The shell footer may wrap to three lines at minimum width while views
		// retain the standard two-line size budget. Reserve one guard row so a
		// Peek attached to the last visible task is never the line shell clipping
		// removes.
		visibleRows = max(visibleRows-m.inlinePeekLineCount(m.Rows[m.Cursor], width-1)-1, 1)
	}

	startIdx, endIdx := chrome.PersistentIndexWindow(m.Cursor, len(m.Rows), visibleRows, &m.ViewScrollOffset)
	rowWidth := width
	if len(m.Rows) > visibleRows {
		rowWidth = width - 1
	}

	m.LastViewport = outlinerViewport{
		VisibleRows: visibleRows,
		startIdx:    startIdx,
		endIdx:      endIdx,
		rowWidth:    rowWidth,
	}
	return m.LastViewport
}

// visibleRowWindow returns the half-open row range the last render drew,
// clamped to the current row count. computeOutlinerViewport is the single
// window authority and mutates ViewScrollOffset, so a key handler consumes its
// cached result here instead of calling it again.
func (m *OutlinerModel) VisibleRowWindow() (int, int) {
	start, end := m.LastViewport.startIdx, m.LastViewport.endIdx
	if m.LastViewport.VisibleRows <= 0 {
		start = m.ViewScrollOffset
		end = start + m.ContentHeight()
	}
	start = min(max(start, 0), len(m.Rows))
	end = min(max(end, start), len(m.Rows))
	return start, end
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
