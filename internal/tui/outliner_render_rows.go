package tui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

func (m *OutlinerModel) renderOutlinerRows(viewport outlinerViewport) string {
	var colLayout *chrome.ColumnLayout
	if viewport.rowWidth >= chrome.MinColumnAlignWidth {
		colLayout = ComputeOutlinerColumnLayout(m.rows[viewport.startIdx:viewport.endIdx])
		// Augment with status widths from the same row set.
		var visibleTasks []*models.Task
		for _, row := range m.rows[viewport.startIdx:viewport.endIdx] {
			if !row.isTaskRow() || row.task == nil {
				continue
			}
			visibleTasks = append(visibleTasks, row.task)
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
		row := m.rows[i]
		selected := i == m.Cursor

		switch {
		case row.IsSeparator:
			rowBuilder.WriteString(chrome.RenderHorizontalDivider(viewport.rowWidth))
		case row.IsSectionHeader:
			rowBuilder.WriteString(m.renderOutlinerSectionHeaderRow(row, selected, viewport.rowWidth, i))
		case row.IsProjectHeader:
			rowBuilder.WriteString(m.renderOutlinerProjectHeaderRow(row, selected, viewport.rowWidth, i))
		case row.IsChecklistItem:
			rowBuilder.WriteString(m.renderOutlinerChecklistRow(row, selected, viewport.rowWidth, i))
		case row.IsNoteRow:
			rowBuilder.WriteString(m.renderOutlinerNoteRow(row, selected, viewport.rowWidth, i, colLayout))
		case selected && m.isQuickEditRow(row):
			rowBuilder.WriteString(m.renderOutlinerQuickEditRow(row, viewport.rowWidth))
		default:
			rowBuilder.WriteString(m.renderOutlinerTaskRow(row, selected, viewport.rowWidth, viewport.rowWidth, i, colLayout))
		}

		if i < viewport.endIdx-1 {
			rowBuilder.WriteString("\n")
		}
	}

	scrollbar := chrome.RenderScrollbar(len(m.rows), viewport.visibleRows, viewport.startIdx)
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

func (m *OutlinerModel) renderOutlinerSectionHeaderRow(row outlinerRow, selected bool, rowWidth, rowIndex int) string {
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
		Collapsed:   m.isSectionRowCollapsed(row),
		AccentColor: sectionStyle,
		Prefix:      prefix,
		Cursor:      paint,
	})

	if row.ProjectID != "" {
		project := m.resolveProjectForRow(row)
		projectName := row.ProjectID
		if project != nil {
			projectName = project.Name
		}
		content += "  " + theme.ProjectColorStyle(project).Render("["+projectName+"]")
	}
	content = appendManualOrderMarker(content, row, m.sort.Field, rowWidth-lipgloss.Width(gutter))

	if selected {
		return outlinerCursorRow(gutter, content, rowWidth, paint)
	}

	return chrome.ZebraLine(gutter+content, rowWidth, rowIndex)
}

// rowKeepsManualOrder reports whether the section a row belongs to is one the
// sort never touches. Pinned, Today and Next carry an order the user built by
// hand, and Review is ordered by the projection that promoted it.
func rowKeepsManualOrder(row outlinerRow) bool {
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
func appendManualOrderMarker(content string, row outlinerRow, field query.SortField, budget int) string {
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
func (m *OutlinerModel) renderOutlinerProjectHeaderRow(row outlinerRow, selected bool, rowWidth, rowIndex int) string {
	project := m.resolveProjectForRow(row)
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
	if m.isSectionRowCollapsed(row) {
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
func appendProjectHeaderPressure(content string, row outlinerRow, budget int) string {
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
func (m *OutlinerModel) renderOutlinerChecklistRow(row outlinerRow, selected bool, rowWidth, rowIndex int) string {
	text := ""
	if row.task != nil && row.ChecklistIndex < len(row.task.Checklist) {
		text = strings.ReplaceAll(row.task.Checklist[row.ChecklistIndex].Text, "\n", " ")
	}
	box := "[ ] "
	if row.ChecklistDone {
		box = "[x] "
		text = theme.MutedStyle.Render(text)
	}
	paint := m.cursorPaintFor(selected)
	prefixBudget := max(min(rowWidth/3, 12), 4)
	gutter := paint.bar(lipgloss.NewStyle(), theme.RenderCursorPrefix()) +
		gutterPiece(paint.gutterBg(), theme.MutedStyle, compactOutlinerTreePrefix(row.prefix, prefixBudget))
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
	row outlinerRow,
	selected bool,
	rowWidth, rowIndex int,
	colLayout *chrome.ColumnLayout,
) string {
	entry := m.searchNoteMatches[row.NoteRow]
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
func ComputeOutlinerColumnLayout(rows []outlinerRow) *chrome.ColumnLayout {
	if len(rows) == 0 {
		return nil
	}
	cl := &chrome.ColumnLayout{}
	hasRows := false
	for _, row := range rows {
		if !row.isTaskRow() || row.task == nil {
			continue
		}
		hasRows = true
		task := row.task

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
