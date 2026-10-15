package outliner

import (
	"fmt"
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
	"github.com/sglavoie/tt/internal/util"
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

// cursorPaint decides how one row wears the cursor. One call answers whether a
// row is the cursor, whether it is mid-flash, and what background the whole row
// carries, so no renderer reads m.CursorFlash on its own and no renderer decides
// a row background beside this.
type cursorPaint struct {
	marked bool
	flash  bool
	// rowBg is the band the marked row wears under everything it drew, from the
	// column after the bar to the end of the line. Nil is a row that wears none,
	// which is every row the cursor is not on and every row kind that did not ask
	// for one — the band is a task-row affordance, so a header, a checklist item
	// and a note row keep the mark they already had.
	rowBg color.Color
}

func (m *OutlinerModel) cursorPaintFor(selected bool) cursorPaint {
	return cursorPaint{marked: selected, flash: selected && m.CursorFlash}
}

// overTaskRow is the opt-in: a marked task row carries a background for its whole
// width, because a task row is the one row kind that runs the full width of the
// terminal with the cursor somewhere in the middle of it. A flash takes the
// flash's own dim rather than the resting band, so the pulse widens and brightens
// one mark instead of introducing a second.
func (p cursorPaint) overTaskRow() cursorPaint {
	switch {
	case !p.marked:
		return p
	case p.flash:
		p.rowBg = theme.ColorFlashDimBg
	default:
		p.rowBg = theme.ColorCursorRowBg
	}
	return p
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

// gutterBg names the background the gutter columns after the bar carry. On a row
// wearing a band it is that band, because the band is the row and a gutter left
// out of it would be a hole between the mark and the work. On a row wearing none
// there is nothing at rest — the connectors and the fold arrow stay ordinary. A flash spends exactly that: it floods the whole gutter,
// which reads as more of the same mark rather than a different one.
func (p cursorPaint) gutterBg() color.Color {
	if p.flash {
		return theme.ColorFlashBg
	}
	return p.rowBg
}

// trail styles the columns past the row's content. A banded row continues its
// band, so the row is one unbroken block the eye can follow to the edge; a row
// with no band keeps the dim fill, which reads as the cursor's reach rather than
// competing with the bar for the eye.
func (p cursorPaint) trail() lipgloss.Style {
	if p.rowBg != nil {
		return lipgloss.NewStyle().Background(p.rowBg)
	}
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
// already painted — its first column bright, the rest of it carrying the row's
// band or nothing — and the columns past the content continue whichever of the
// two the row wears, so every color the row earned survives under the cursor and
// no drawn column is covered. The result is always exactly rowWidth wide, which
// is what attachScrollbarAtWidth expects. Truncation only ever removes from the
// end, so clamping the content to the columns the gutter leaves cuts where a
// clamp of the whole line would.
//
// The band goes under the content after that clamp and never before it: painting
// first would leave the sequences this seam added inside the run the clamp then
// cuts, and the band is the one thing on the row that has to reach its last
// column.
func outlinerCursorRow(gutter, content string, rowWidth int, paint cursorPaint) string {
	gutterWidth := lipgloss.Width(gutter)
	if rowWidth <= gutterWidth {
		return textfit.TruncateToVisualWidth(gutter, rowWidth)
	}
	content = textfit.PaintBackground(
		textfit.TruncateToVisualWidth(content, rowWidth-gutterWidth), paint.rowBg)
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
	Lifted      int // members drawn in a promoted lane above; zero says nothing
	Collapsed   bool
	AccentColor lipgloss.Style // applied to the name portion
	Width       int            // if > 0, trailing ─ fill is added
	Prefix      string         // inserted between indicator and name (e.g. "◷ ")
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
	if opts.Lifted > 0 {
		content += " " + theme.MutedStyle.Render(fmt.Sprintf("+%d above", opts.Lifted))
	}

	if opts.Width > 0 {
		visible := lipgloss.Width(gutter) + lipgloss.Width(content)
		if opts.Width > visible+2 {
			content += " " + theme.MutedStyle.Render(strings.Repeat("─", opts.Width-visible-1))
		}
	}
	return gutter, content
}

// sectionHeaderLabel is the name a section header spells and the mark that
// leads it, read by the header row and by the pinned context row alike so the
// two cannot name one lane differently.
func sectionHeaderLabel(row OutlinerRow) (name, prefix string) {
	name = row.SectionName
	if name == "" {
		name = models.StateGroupBacklog.Label()
	}
	if row.IsComputedHeader {
		prefix = "◷ "
	}
	return name, prefix
}

// renderPinnedContextRow draws the one line that stands in for a lane header
// the window scrolled past: the lane's own name and count, then the project the
// window's first row belongs to, read off that row's group. It is context and
// not a row, so it is muted, wears no zebra stripe, and is never the cursor's;
// the trailing rule is what still sets it apart where a profile carries no
// color. The line is exactly rowWidth.
func (m *OutlinerModel) renderPinnedContextRow(viewport outlinerViewport) string {
	header := m.Rows[viewport.pinnedHeader]
	name, prefix := sectionHeaderLabel(header)
	if projectID := m.Rows[viewport.startIdx].Group.ProjectID; projectID != "" {
		slug, _ := m.projectSlugForRow(OutlinerRow{ProjectID: projectID})
		name += " › [" + slug + "]"
	}
	gutter, content := renderSectionHeader(sectionHeaderOpts{
		Name:        name,
		Count:       header.SectionCount,
		AccentColor: theme.MutedStyle.Padding(0, 1),
		Width:       viewport.rowWidth,
		Prefix:      prefix,
	})
	line := textfit.TruncateToVisualWidth(gutter+content, viewport.rowWidth)
	return line + strings.Repeat(" ", max(viewport.rowWidth-lipgloss.Width(line), 0))
}

func (m *OutlinerModel) RenderOutlinerSectionHeaderRow(row OutlinerRow, selected bool, rowWidth, rowIndex int) string {
	displayName, prefix := sectionHeaderLabel(row)

	sectionStyle := theme.GetWorkflowStateGroupStyle(models.StateGroupBacklog)
	if row.StateGroup.IsValid() {
		sectionStyle = theme.GetWorkflowStateGroupStyle(row.StateGroup)
	}
	if row.PromotedTag == reviewLaneKey {
		sectionStyle = theme.ViewModeReviewStyle
	}

	paint := m.cursorPaintFor(selected)
	gutter, content := renderSectionHeader(sectionHeaderOpts{
		Name:        displayName,
		Count:       row.SectionCount,
		Lifted:      row.SectionLiftedCount,
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

	if selected {
		return outlinerCursorRow(gutter, content, rowWidth, paint)
	}

	return chrome.ZebraLine(gutter+content, rowWidth, rowIndex)
}

// projectSlugForRow is the name a project header spells for the row's
// project, and the project it resolved: the slug, else the name, else the bare
// ID of a project the runtime no longer resolves.
func (m *OutlinerModel) projectSlugForRow(row OutlinerRow) (string, *models.Project) {
	project := m.ResolveProjectForRow(row)
	if project == nil {
		return row.ProjectID, nil
	}
	if project.Slug != "" {
		return project.Slug, project
	}
	return project.Name, project
}

// renderOutlinerProjectHeaderRow renders a synthetic project sub-header row
// that visually clusters adjacent tasks from the same project in cross-project
// modes. In All these rows sit beneath shared state-group headers. The header
// reuses the persisted project color and slug
// text so the inline `[slug]` badges on individual task rows can be suppressed.
// They use the same fold indicator as built-in sections, while Enter retains
// its project scope behavior.
func (m *OutlinerModel) RenderOutlinerProjectHeaderRow(row OutlinerRow, selected bool, rowWidth, rowIndex int) string {
	projectSlug, project := m.projectSlugForRow(row)
	badge := theme.ProjectColorStyle(project).
		Bold(true).
		Render("[" + projectSlug + "]")
	// The focused-project header owns no fold, so it shows no glyph at all: a
	// blank of the same single column keeps its badge on the column every row
	// below it uses, and nothing invites an h/l that would fold the whole
	// outline. It also carries no group size, which row.SectionCount already
	// leaves at zero.
	indicator := " "
	switch {
	case row.IsFocusedProjectHeader:
	case m.IsSectionRowCollapsed(row):
		indicator = theme.IconCollapsed
	default:
		indicator = theme.IconExpanded
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

	line := chrome.ZebraLine(textfit.TruncateToVisualWidth(gutter+content, rowWidth), rowWidth, rowIndex)
	if selected {
		line = outlinerCursorRow(gutter, content, rowWidth, paint)
	}

	// A header row says almost nothing about the project it names — a slug, a
	// count, and what is urgent inside. Peek is where the rest of it is readable,
	// in the dock below the list, on the same key and under the same cursor rule
	// as a task's.
	return line
}

// appendProjectHeaderPressure names the overdue and Today-planned work a
// project group holds, so folding the group does not hide what is urgent in it.
// The suffix degrades before the row is truncated — today drops first, then
// overdue — because a clipped count reads as a different number. Each chip
// carries a word so it cannot be mistaken for the parenthesized group size beside
// it. The Today count says "today" in the Plan vocabulary the rest of the chrome
// speaks, never its capture sigil, and no clock, since Due mode heads its own
// sections with ◷ and a clock there read as a deadline.
// budget is the columns the content region has, which is the row width less the
// gutter the cursor highlight owns.
//
// Both counts wear the same emphasis the rows' own deadline chips wear, because
// a folded group hides exactly the rows that would have said it. They keep two
// colors while doing so: OverdueCount is a deadline and takes the due family's
// accent, TodayCount is durable Today membership and takes the Today group's, so
// matching weight never turns a plan into a deadline.
func appendProjectHeaderPressure(content string, row OutlinerRow, budget int) string {
	var parts []string
	if row.OverdueCount > 0 {
		parts = append(parts, theme.ProjectPressureOverdueStyle.Render(fmt.Sprintf("! overdue %d", row.OverdueCount)))
	}
	if row.TodayCount > 0 {
		parts = append(parts, theme.ProjectPressureTodayStyle.Render(fmt.Sprintf("today %d", row.TodayCount)))
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
		gutterPiece(paint.gutterBg(), m.treeConnectorStyle(row), compactOutlinerTreePrefix(row.Prefix, prefixBudget))
	content := theme.MutedStyle.Render(box) + text

	if selected {
		return outlinerCursorRow(gutter, content, rowWidth, paint)
	}
	return chrome.ZebraLine(textfit.TruncateToVisualWidth(gutter+content, rowWidth), rowWidth, rowIndex)
}

// noteRowGlyph marks a note in a mixed result list. It is a literal here rather
// than an icons registry key for the same reason the computed section header's
// `◷` is: it names a row kind this renderer draws, not an entity the rest of
// the product has to spell the same way.
const noteRowGlyph = "≡"

// noteExcerptSeparator stands between a note's title and the body excerpt that
// explains it, and noteExcerptMinWidth is the narrowest excerpt worth drawing:
// fewer columns than that read as noise rather than as the matched words.
const (
	noteExcerptSeparator = "  "
	noteExcerptMinWidth  = 8
)

// noteGlobalLabel is the owner a workspace-global note names in the label
// column, where a project note names its project.
const noteGlobalLabel = "global"

// noteRowSlug is the bare text a note row's label spells, bounded by the cap
// ranked task rows use so the render and computeRowLayout measure one string:
// the project's slug, or global for a workspace note. The project is nil for a
// global note and for one whose owner no longer resolves.
func (m *OutlinerModel) noteRowSlug(row notes.LocatedNoteRow, rowWidth int) (string, *models.Project, bool) {
	if row.Location.Scope == app.NoteScopeGlobal {
		return noteGlobalLabel, nil, true
	}
	projectRow := OutlinerRow{ProjectID: row.Location.ProjectID}
	slug := m.rankedProjectSlug(projectRow, rowWidth)
	_, project := m.projectSlugForRow(projectRow)
	return slug, project, false
}

// noteProjectLabel leads a note's title with the label a ranked task row leads
// its own with, under the same gate and padded to the same column: the project
// in its bold color, or a muted global for a workspace note.
func (m *OutlinerModel) noteProjectLabel(row notes.LocatedNoteRow, rowWidth int, colLayout *chrome.ColumnLayout) string {
	if !m.showsRankedProjectLabel() {
		return ""
	}
	slug, project, global := m.noteRowSlug(row, rowWidth)
	if slug == "" {
		return ""
	}
	style := theme.ProjectColorStyle(project).Bold(true)
	if global {
		style = theme.MutedStyle
	}
	label := style.Render("[" + slug + "]")
	if colLayout != nil {
		label += strings.Repeat(" ", max(colLayout.ProjectWidth-lipgloss.Width(label), 0))
	}
	return label + " "
}

// noteBodyExcerpt says why a note matched when its title does not: the body is
// re-scored under the projection's own query and case mode, exactly as Peek's
// search excerpt is, and a body the scorer places no position in says nothing.
// The excerpt is fitted to width through searchMatchExcerptWithin, so a narrow
// row spends its columns on the matched words or on nothing at all.
func (m *OutlinerModel) noteBodyExcerpt(entry outlinerSearchNoteMatch, width int) string {
	if !entry.match.BodyMatched || entry.row.Note == nil || m.searchTextQuery() == "" {
		return ""
	}
	body := entry.row.Note.Body
	_, positions := fuzzy.NewMatcher(m.searchCaseSensitive()).ScoreTermsWithPositions(body, m.searchTextQuery())
	if len(positions) == 0 {
		return ""
	}
	return searchMatchExcerptWithin(body, positions, width)
}

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
	gutter := paint.bar(lipgloss.NewStyle(), theme.RenderCursorPrefix())

	glyph := theme.MutedStyle.Render(noteRowGlyph)
	if colLayout != nil && colLayout.HasStatus {
		glyph = theme.PadBadge(glyph, colLayout.StatusWidth)
	}
	priorityColumn := ""
	if colLayout != nil && colLayout.HasPriority {
		priorityColumn = strings.Repeat(" ", colLayout.PriorityWidth+1)
	}
	label := m.noteProjectLabel(entry.row, rowWidth, colLayout)
	cue := taskMatchSourceCue(row)

	// The space after the glyph is the one the task row spends after its state
	// badge, so both blocks start their content at the same column.
	titleWidth := func() int {
		return max(rowWidth-lipgloss.Width(gutter)-lipgloss.Width(glyph)-1-
			lipgloss.Width(priorityColumn)-lipgloss.Width(label)-lipgloss.Width(cue), 1)
	}
	// The note row sheds what the task row sheds, in the same order and under the
	// same title target: the match cue first, because it explains a result whose
	// title has become unreadable, and the project label second. Both are dropped
	// rather than truncated — the whole line is clamped to rowWidth at the end, and
	// a cue cut mid-word would read as a different word.
	titleTarget := min(8, max(rowWidth/4, 4))
	if titleWidth() < titleTarget {
		cue = ""
	}
	if titleWidth() < titleTarget {
		label = ""
	}
	title := ""
	if entry.row.Note != nil {
		title = textfit.TruncateToVisualWidth(entry.row.Note.Title, titleWidth())
	}
	// A body hit is explained in whatever width the title left over, so the
	// excerpt is the first thing a narrow row loses and never costs the title a
	// column. Room that cannot hold the matched words draws no excerpt rather
	// than leading context that explains nothing.
	excerpt := ""
	if room := titleWidth() - lipgloss.Width(title) - lipgloss.Width(noteExcerptSeparator); room >= noteExcerptMinWidth {
		if text := m.noteBodyExcerpt(entry, room); text != "" {
			excerpt = noteExcerptSeparator + text
		}
	}
	content := glyph + " " + priorityColumn + label +
		textfit.HighlightText(title, entry.match.TitlePositions, theme.NormalStyle) + excerpt + cue

	if selected {
		return outlinerCursorRow(gutter, content, rowWidth, paint)
	}
	return chrome.ZebraLine(textfit.TruncateToVisualWidth(gutter+content, rowWidth), rowWidth, rowIndex)
}

// ComputeOutlinerRowLayout is the one measurement of a projection's shared row
// geometry: the fold column and the badge columns the leading segments align in.
// It reads the whole
// row set it is handed rather than the slice a window happens to show, which is
// what keeps the titles where they are while the list scrolls — a measurement
// taken from the window would move every row's title each time a wider badge
// scrolled into view.
//
// It is the seam every task-row consumer takes: the Outliner's own render, the
// inline editor that has to open in the column the title was drawn in, and the
// Hub's preview painter, which is a standalone render of the same rows. Badge
// alignment keeps its width floor, because padding a badge is a wide-terminal
// nicety that a cramped row is better off without.
func (m *OutlinerModel) ComputeOutlinerRowLayout(rows []OutlinerRow, rowWidth int) *chrome.ColumnLayout {
	return m.computeRowLayout(rows, rowWidth, nil)
}

// computeRowLayout measures the drawn rows together with what the projection's
// folds hide. The Outliner's own render hands it the model's accumulator, so a
// fold opening or closing moves no title; every other caller measures the rows
// it holds and nothing else. A projection drawing no task row still measures
// nothing, since there is no row for a hidden one to align with.
func (m *OutlinerModel) computeRowLayout(rows []OutlinerRow, rowWidth int, hidden *hiddenRowLayout) *chrome.ColumnLayout {
	if rowWidth < chrome.MinColumnAlignWidth {
		return nil
	}
	tasks := make([]*models.Task, 0, len(rows))
	hasFold := false
	var slugs map[*models.Task]string
	for _, row := range rows {
		if !row.IsTaskRow() || row.Task == nil {
			continue
		}
		tasks = append(tasks, row.Task)
		hasFold = hasFold || rowDrawsFold(row)
		if m.rowLeadsWithProjectLabel(row) {
			if slugs == nil {
				slugs = make(map[*models.Task]string, len(rows))
			}
			slugs[row.Task] = m.rankedProjectSlug(row, rowWidth)
		}
	}
	noteWidth := 0
	if m.showsRankedProjectLabel() {
		noteWidth = m.noteLabelWidth(rows, rowWidth)
	}
	if len(tasks) == 0 {
		if noteWidth == 0 {
			return nil
		}
		return &chrome.ColumnLayout{ProjectWidth: noteWidth}
	}
	if hidden != nil {
		tasks = append(tasks, hidden.tasks...)
		hasFold = hasFold || hidden.hasFold
	}
	opts := chrome.BadgeWidthOpts{IncludeStatus: true}
	if slugs != nil {
		opts.ProjectNameFor = func(task *models.Task) string { return slugs[task] }
		opts.MaxProjectLen = rankedProjectLabelMaxSlug(rowWidth)
	}
	layout := chrome.ComputeBadgeWidths(tasks, opts)
	if layout != nil {
		layout.HasFold = hasFold
		layout.ProjectWidth = max(layout.ProjectWidth, noteWidth)
	}
	return layout
}

// noteLabelWidth is the widest label the notes tier draws, folded into the
// task rows' project column so a note title starts where a task title does. A
// result set holding only notes still measures it, which is why the layout is
// not nil there.
func (m *OutlinerModel) noteLabelWidth(rows []OutlinerRow, rowWidth int) int {
	width := 0
	for _, row := range rows {
		if !row.IsNoteRow {
			continue
		}
		entry, ok := m.Search.noteMatches[row.NoteRow]
		if !ok {
			continue
		}
		if slug, _, _ := m.noteRowSlug(entry.row, rowWidth); slug != "" {
			width = max(width, lipgloss.Width("["+slug+"]"))
		}
	}
	return width
}

type outlinerViewport struct {
	VisibleRows int
	startIdx    int
	endIdx      int
	rowWidth    int
	// pinned reports that the list's first screen row is the pinned context
	// row rather than a row of the window, and pinnedHeader is the section
	// header it stands in for. VisibleRows still counts that screen row, so
	// startIdx and endIdx then span one row fewer.
	pinned       bool
	pinnedHeader int
	// topRow is the screen row the window's first row is drawn on, below the
	// header rows and any search dropdown, so a cursor row can be named on
	// screen without a second window computation.
	topRow int
}

// drawnRows is the screen rows the window fills: its own rows and the pinned
// context row above them.
func (v outlinerViewport) drawnRows() int {
	drawn := v.endIdx - v.startIdx
	if v.pinned {
		drawn++
	}
	return drawn
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
	return max(m.Height()-m.StatusStripHeight(width), 1)
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
	lines := m.statusStripSignals(width)
	// The expanded stack changes this return value rather than painting outside
	// it, so statusStripHeight reserves the rows it needs from the list.
	if readout := m.QueryReadoutBody(width); readout != "" {
		lines = append(lines, readout)
	}
	return strings.Join(lines, "\n")
}

// statusStripSignals is the strip without the readout: the three one-line
// signals that stand above it. They are separated out because the readout has
// to budget its own viewport against them, and it cannot ask statusStripHeight
// for that — the height it would answer with is the one the readout is still
// deciding.
func (m *OutlinerModel) statusStripSignals(width int) []string {
	width = max(width, 1)
	lines := make([]string, 0, 4)
	if indicator := m.AdvisoryLine(m.conflictIndicator()); indicator != "" {
		lines = append(lines, theme.PreviewWarningStyle.Width(width).Render(indicator))
	}
	if indicator := m.AdvisoryLine(m.syncIndicator()); indicator != "" {
		lines = append(lines, theme.MutedStyle.Width(width).Render(indicator))
	}
	if m.Focus.Notice != "" {
		lines = append(lines, theme.PreviewWarningStyle.Width(width).Render(m.Focus.Notice))
	}
	if m.StatusMessage != "" {
		lines = append(lines, theme.MutedStyle.Width(width).Render(m.StatusMessage))
	}
	if m.HasContentRows() {
		if hint := m.withheldSearchHint(); hint != "" {
			lines = append(lines, theme.MutedStyle.Width(width).Render("Also matches: "+hint+"."))
		}
	}
	return lines
}

// statusStripSignalRows is what those signals cost once wrapped, which is not
// their count: each is rendered at the strip's width and a long one takes more
// than a row.
func (m *OutlinerModel) statusStripSignalRows(width int) int {
	signals := m.statusStripSignals(width)
	if len(signals) == 0 {
		return 0
	}
	return lipgloss.Height(strings.Join(signals, "\n"))
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
// useful half of the segment, so the match count sheds before it does. A refused
// sigil wins over the count and the case badge, because a query that cannot run
// explains an empty list better than how it was matched.
func (m *OutlinerModel) buildOutlinerSearchIndicator(width int) string {
	return m.searchIndicator(width, true)
}

// searchIndicator is that chrome with or without the match count. The context
// row draws the counted form in full and the uncounted one compact, so a row
// too narrow for every entry gives up the count before it drops the query.
func (m *OutlinerModel) searchIndicator(width int, counted bool) string {
	if m.IsSearchMode() && m.Search.historyPicker != nil {
		picker := m.Search.historyPicker
		picker.input.SetWidth(max(width-10, 1))
		return theme.MutedStyle.Render("History: ") + picker.input.View()
	}
	var indicator string
	switch {
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
	if count := m.searchMatchCountLabel(); counted && count != "" {
		indicator = appendSearchBadge(indicator, " · "+count, width)
	}
	if m.Search.caseRelaxed {
		indicator = appendSearchBadge(indicator, " (case ignored)", width)
	}
	if m.IsSearchMode() && !m.Search.Autocomplete.Active() && m.Search.history.notice != "" {
		indicator = appendSearchBadge(indicator, " · "+m.Search.history.notice, width)
	}
	return indicator
}

// searchMatchCountLabel spells how many task and note rows the projection
// draws for the held query, the same in the typing and the applied form. It is
// empty with no projection standing, since a query installed without one has
// drawn nothing of its own to count, and over an empty query, which matched
// nothing and narrowed nothing.
func (m *OutlinerModel) searchMatchCountLabel() string {
	if !m.HasSearchProjection() || strings.TrimSpace(m.SearchQuery()) == "" {
		return ""
	}
	return util.CountPlural(m.Search.drawn, "match", "matches")
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

// effectiveSort is the ordering the rows on screen actually follow. A
// fixed-order mode owns its ordering, so a sort recorded there is not on screen
// and must not be advertised. Under a search projection the mode underneath is
// irrelevant — the sort is live over the ranked rows — so zeroing it there would
// silently hide a live axis.
//
// It is one definition because the context row and the readout both state the
// sort, and a view where the row and the readout disagreed about what the rows
// are ordered by would be worse than either saying nothing.
func (m *OutlinerModel) effectiveSort() (query.SortField, bool) {
	if m.ActiveOutlinerModeEntry().FixedOrder() && !m.ShowsRankedList() {
		return query.SortByDefault, false
	}
	return m.Sort.Field, m.Sort.Ascending
}

// ViewContextEntry is one live axis of interaction context, described once so
// that the compact context row and the expanded readout state the same thing
// without either reading the other's rendered text. Row and RowCompact are the
// two spellings the row may draw, already styled; Label and Value are the plain
// pair the readout lists, whole.
//
// ReadOnly marks an axis the readout reports but no peel digit removes, because
// it narrows nothing: a selection, an ordering, a Focus, and a section zoom are
// all things the view is doing rather than clauses over the population. Those
// four are exactly what the readout adds to its peelable stack, which is why
// they are described here and not beside it.
type ViewContextEntry struct {
	Label      string
	Value      string
	Row        string
	RowCompact string
	ReadOnly   bool
}

// BuildViewContextEntries describes every axis of interaction context that is
// live right now, in the order the context row draws them: the selection, the
// lens naming the context the segments after it describe, the query, the
// filters, the ordering, and last the two narrowings a single Esc leaves. The
// row drops from the right, so that order is also the order it gives them up
// in, and the zoom goes before a narrowing the user cannot leave with one Esc.
//
// Scope and mode are deliberately absent: they are identity rather than
// context, they stay in the title above, and the readout already peels them
// from its own stack.
//
// The set is a question about state alone. Width chooses between a full and a
// compact search badge; it never adds or removes an entry, which is what lets
// contextRowRows answer the height question without one.
func (m *OutlinerModel) BuildViewContextEntries(width int) []ViewContextEntry {
	entries := make([]ViewContextEntry, 0, 7)
	if m.MultiSelectMode {
		entries = append(entries, m.selectionContextEntry())
	}
	if lens := m.ActiveLensLabel(); lens != "" {
		entries = append(entries, ViewContextEntry{
			Label: "lens",
			Value: lens,
			Row:   theme.MutedStyle.Render(lens),
		})
	}
	if search := m.buildOutlinerSearchIndicator(width); search != "" {
		// The plain half is read off the same live state the indicator is built
		// from rather than stripped back out of it, so the readout never depends on
		// how the row happened to style or badge the query.
		label, value := "query", m.SearchQuery()
		if m.HasActiveLens() && m.Lens.Query != "" {
			label, value = "lens query", m.Lens.Query
		}
		entries = append(entries, ViewContextEntry{
			Label:      label,
			Value:      value,
			Row:        search,
			RowCompact: m.searchIndicator(width, false),
		})
	}
	if parts := m.ActiveFilterParts(); len(parts) > 0 {
		entries = append(entries, ViewContextEntry{
			Label:      "filters",
			Value:      strings.Join(parts, " "),
			Row:        query.RenderFilterBadge(parts),
			RowCompact: pulse.OperationalTitleBadge(theme.FilterBadgeStyle, "["+util.Count(len(parts), "filter")+"]"),
		})
	}
	if field, ascending := m.effectiveSort(); field != query.SortByDefault {
		direction := "↓"
		if ascending {
			direction = "↑"
		}
		value := fmt.Sprintf("%s %s", field, direction)
		entries = append(entries, ViewContextEntry{
			Label:    "sort",
			Value:    value,
			Row:      theme.MutedStyle.Render("Sort: " + value),
			ReadOnly: true,
		})
	}
	if focus := m.outlinerFocusContext(); focus != "" {
		entries = append(entries, ViewContextEntry{
			Label:    "focus",
			Value:    strings.TrimPrefix(focus, outlinerFocusContextPrefix),
			Row:      theme.MutedStyle.Render(focus),
			ReadOnly: true,
		})
	}
	if m.Focus.Zoom.Active() && m.Focus.Zoom.Name != "" {
		entries = append(entries, ViewContextEntry{
			Label:    "section",
			Value:    m.Focus.Zoom.Name,
			Row:      theme.MutedStyle.Render("Section: " + m.Focus.Zoom.Name),
			ReadOnly: true,
		})
	}
	return entries
}

// selectionContextEntry states the whole selection, hidden members included. It
// is the only persistent statement of a live selection: the status channel says
// nothing about one, so whatever this entry sheds is shed everywhere.
//
// That is why the hidden count is not what the compact form gives up. A count
// that silently covered rows off screen is the one thing a bulk gesture must not
// be surprised by, so both spellings carry it whenever it is nonzero and the
// compact one instead drops the word the number already implies.
//
// `Q` keeps the whole breakdown on the same terms, and on those terms only: with
// nothing hidden the visible half is the total restated, and a readout row
// costing two wrapped lines to say a number twice is a row the document cannot
// spend. The breakdown appears exactly where there is a difference to read.
func (m *OutlinerModel) selectionContextEntry() ViewContextEntry {
	counts := m.SelectionCounts()
	value := fmt.Sprintf("%d selected", counts.Total)
	full := fmt.Sprintf("Multi-select: %d selected", counts.Total)
	compact := fmt.Sprintf("Multi-select: %d", counts.Total)
	if counts.Hidden > 0 {
		value = fmt.Sprintf("%d total · %d visible · %d hidden", counts.Total, counts.Visible, counts.Hidden)
		hidden := fmt.Sprintf(" (%d hidden)", counts.Hidden)
		full += hidden
		compact += hidden
	}
	return ViewContextEntry{
		Label:      "selection",
		Value:      value,
		Row:        theme.SelectionCountStyle.Render(full),
		RowCompact: theme.SelectionCountStyle.Render(compact),
		ReadOnly:   true,
	}
}

// BuildOutlinerContextRow draws the optional second header row, or nothing at
// all. It exists only while some axis is live: a row standing over a default
// view would spend an outline row every frame to say that nothing is narrowed,
// which is the row the readout already answers on demand.
func (m *OutlinerModel) BuildOutlinerContextRow(width int) string {
	width = max(width, 1)
	entries := m.BuildViewContextEntries(width)
	if len(entries) == 0 {
		return ""
	}
	segments := make([]pulse.ContextRowSegment, 0, len(entries))
	query := -1
	for index, entry := range entries {
		if entry.Label == "query" && entry.RowCompact != entry.Row {
			query = index
		}
		segments = append(segments, pulse.ContextRowSegment{Full: entry.Row, Compact: entry.RowCompact})
	}
	row := pulse.RenderContextRow(segments, width, m.queryReadoutDetailsHint())
	if query < 0 || strings.Contains(row, segments[query].Full) {
		return row
	}
	// The match count is the first thing the row gives up: the search entry
	// steps down to its uncounted form before any other entry compacts or drops,
	// so a count never costs the row an axis. Q still states it.
	segments[query].Full = segments[query].Compact
	return pulse.RenderContextRow(segments, width, m.queryReadoutDetailsHint())
}

// contextRowRows is the row the header spends on context, counted in one place
// so every path that measures the header charges it exactly once. It reads the
// model's own width because entry presence is a question about state and the
// count does not move with the terminal.
func (m *OutlinerModel) contextRowRows() int {
	if len(m.BuildViewContextEntries(max(m.Width(), 1))) == 0 {
		return 0
	}
	return 1
}

// buildOutlinerTitleLine assembles the Outliner's one-row operational title. It
// carries identity and passive state alone — scope, mode, Today pace, and the
// workspace pulse — because interaction context moved to the row below, where a
// value has room to keep its label.
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
	spec := pulse.OperationalTitleSpec{
		Scope: theme.TitleStyle.Render(m.outlinerScopeLabel()),
		Mode:  chrome.OperationalTitleModeSegment(m.ActiveOutlinerModeEntry()),
		Pace:  pace,
		Pulse: &workspacePulse,
	}
	// A wide title spends its blank middle on the active mode's Tab neighbors,
	// so the reader sees where Tab and Shift+Tab go. It is offered only where the whole title still
	// fits unshortened; anywhere narrower the lone active badge stands.
	strip := spec
	strip.Mode = chrome.OperationalTitleModeStrip(m.ActiveOutlinerMode())
	if pulse.OperationalTitleFitsWhole(strip, width) {
		spec = strip
	}
	return pulse.RenderOperationalTitle(spec, max(width, 1))
}

// writeOutlinerHeader is the one place the header's rows are written, so both
// projections draw the same one or two lines and contextRowRows charges for the
// second exactly once.
func (m *OutlinerModel) writeOutlinerHeader(b *strings.Builder, width int) {
	b.WriteString(m.BuildOutlinerTitleLine(width))
	b.WriteString("\n")
	if row := m.BuildOutlinerContextRow(width); row != "" {
		b.WriteString(row)
		b.WriteString("\n")
	}
}

func (m *OutlinerModel) renderOutlinerEmptyState(width int) string {
	var b strings.Builder
	// The empty state carries the bar for the same reason it carries the
	// suggestion pane: a view with no rows is exactly where a user reaches for the
	// map of what they can do next. It is a host like any other: a menu opened
	// over it composites above this block rather than replacing it.
	if m.MenuBarVisible() {
		b.WriteString(m.renderOutlinerMenuBar(width) + "\n")
	}
	m.writeOutlinerHeader(&b, width)
	// The strip and the suggestion pane are charged here exactly as the populated
	// path charges them, because they are what the list block is measured after:
	// a reservation that ignored them would dock Peek at a different screen row
	// the moment a status line or a suggestion arrived.
	extraFooter := m.StatusStripHeight(width)
	// A query that matches nothing yet is exactly when suggestions are worth the
	// most, so the empty state carries the pane too.
	if dropdown := m.renderSearchAutocompleteDropdown(width); dropdown != "" {
		b.WriteString(dropdown + "\n")
		extraFooter += lipgloss.Height(dropdown)
	}

	// The header and the explanation are this path's list block: they stand where
	// the rows stand, so they are written into their own builder and measured the
	// way the rows are measured.
	//
	// A focused projection keeps its project header when it holds no content, in
	// the same place and drawn by the same renderer as when it does: the header is
	// context for the projection, and the explanation below belongs under it. It
	// takes the full width — the scrollbar column the row path subtracts is never
	// reserved here — and it is one line, because the detail an empty project is
	// exactly the place to open is in the dock below rather than hanging off the
	// row.
	var body strings.Builder
	if header, ok := m.focusedProjectHeaderRow(); ok {
		body.WriteString(m.RenderOutlinerProjectHeaderRow(header, m.Cursor == 0, width, 0))
		body.WriteString("\n")
	}
	// The explanation is wrapped here rather than by the terminal, so the block
	// measured below is the block drawn: a sentence the terminal wrapped would
	// count as one row and push the dock down by the rows it spilled onto.
	explanation := textfit.WrapText(m.OutlinerEmptyMessage(), max(width, 1))
	for i, line := range explanation {
		if i > 0 {
			body.WriteString("\n")
		}
		body.WriteString(theme.MutedStyle.Render(line))
	}
	b.WriteString(body.String())

	// The dock sits at the screen row the reservation paid for, on this path for
	// the same reason as on the populated one: the block above it is padded out to
	// the whole list window, so an empty projection docks Peek where a full one
	// docks it instead of hanging it off the explanation. It is drawn whole — a
	// dock this path shortened would be a different height and a different
	// position per projection, which is the moving target docking retired. This
	// path has no rows to window, so it reads the one reservation without
	// computing a window against it; the bound is taken once rather than per
	// padded row, since measuring it now measures the status strip too.
	if dock := m.PeekDockLines(); len(dock) > 0 {
		listRows := m.outlinerListRows(extraFooter)
		for drawn := lipgloss.Height(body.String()); drawn < listRows; drawn++ {
			b.WriteString("\n")
		}
		for _, line := range dock {
			b.WriteString("\n")
			b.WriteString(line)
		}
	}
	return b.String()
}

// focusedProjectHeaderRow names the focused-project header the current
// projection leads with, if it has one. It reads the rows rather than resolving
// the owner again so that the empty state cannot draw a header the row pipeline
// declined to build.
func (m *OutlinerModel) focusedProjectHeaderRow() (OutlinerRow, bool) {
	if len(m.Rows) > 0 && m.Rows[0].IsFocusedProjectHeader {
		return m.Rows[0], true
	}
	return OutlinerRow{}, false
}

// hasContentRows reports whether the projection holds anything the user could
// act on beyond the focused-project header. It is what separates "this project
// is empty" from "there are rows": the header is always present in a focused
// projection, so a bare length test would claim content that is not there and
// suppress the explanation the empty state owes the user.
func (m *OutlinerModel) HasContentRows() bool {
	for _, row := range m.Rows {
		if !row.IsFocusedProjectHeader {
			return true
		}
	}
	return false
}

// withheldSearchHint names the finished and deferred tasks a ranked list
// kept out, with the sigil that brings each population in. The ranked list
// ignores the mode, so adding that sigil is exactly what shows them; a narrowed
// mode would still hide them for its own reasons, so it is never promised there.
func (m *OutlinerModel) withheldSearchHint() string {
	if !m.ShowsRankedList() || !m.Search.withheld.any() {
		return ""
	}
	var parts []string
	if n := m.Search.withheld.completed; n > 0 {
		parts = append(parts, fmt.Sprintf("%s (add ?completed)", util.Count(n, "finished task")))
	}
	if n := m.Search.withheld.deferred; n > 0 {
		parts = append(parts, fmt.Sprintf("%s (add ?deferred)", util.Count(n, "deferred task")))
	}
	return strings.Join(parts, " and ")
}

func (m *OutlinerModel) OutlinerEmptyMessage() string {
	switch m.ClassifyEmptyCause() {
	case outlinerEmptyNoProjects:
		return "No projects are available. Press P to open Projects."
	case outlinerEmptyProjectFiltered:
		return "No projects match project filters. Press Fx to clear project filters."
	case outlinerEmptySearch:
		if hint := m.withheldSearchHint(); hint != "" {
			verb := "does"
			if m.Search.withheld.completed+m.Search.withheld.deferred > 1 {
				verb = "do"
			}
			return fmt.Sprintf("No open tasks match search %q, but %s %s. Press %s.", m.SearchQuery(), hint, verb, m.searchExitClause())
		}
		if hint := m.Search.eliminationHint; hint != "" {
			return fmt.Sprintf("No tasks match search %q. %s Press %s.", m.SearchQuery(), hint, m.searchExitClause())
		}
		return fmt.Sprintf("No tasks match search %q. Press %s.", m.SearchQuery(), m.searchExitClause())
	case OutlinerEmptySearchFiltered:
		// The second exit is spelled out here rather than taken from the shared
		// clause: beside "clear filters" a bare "clear" would read as clearing them.
		exit := "Esc to clear the search"
		if m.HasSuspendedLens() {
			exit = "Esc to restore the lens"
		}
		if m.ShowsRankedList() && m.Search.hiddenFilterHint != "" {
			return fmt.Sprintf("%s Press Q to inspect individual filters, fx to clear filters or %s.", m.Search.hiddenFilterHint, exit)
		}
		return fmt.Sprintf(
			"No tasks match search %q under the active filters. Press fx to clear filters or %s.",
			m.SearchQuery(), exit)
	case outlinerEmptySearchInvalid:
		return fmt.Sprintf("%s. Press %s.", m.Search.Gate.GuidanceText(), m.searchExitClause())
	case outlinerEmptyLens:
		// The three sentences below name the act the cascade's top step performs,
		// in the verb the footer entry beside them uses, so the prose and the hint
		// are one answer read twice.
		return "No tasks match this lens. Press Esc to leave the lens."
	case outlinerEmptyFiltered:
		return listsurface.EmptyStateFiltered
	case outlinerEmptyFocus:
		return "No tasks are visible in this Focus. Press Esc to leave Focus."
	case OutlinerEmptySection:
		return "No tasks are left in this section. Press Esc to unzoom."
	case outlinerEmptyMode:
		mode := m.ActiveOutlinerModeEntry()
		if mode.ID == chrome.ModeInbox {
			return "Inbox is empty. Press a to capture or Tab to change modes."
		}
		if step, ok := m.emptyModeNextStep(mode.ID); ok {
			return fmt.Sprintf("%s is empty. Press %s, or Tab to change modes.", mode.Label, step)
		}
		return fmt.Sprintf("%s is empty. Press Tab or Shift+Tab to change modes.", mode.Label)
	default:
		return listsurface.EmptyStateNoTasks
	}
}

// emptyModeNextStep names the act that fills an empty planning or deadline
// mode, spelled with the key the live strategy binds, so a rebinding moves the
// sentence with the grammar. A mode without one, or a strategy that no longer
// binds the act, falls back to the plain mode-change sentence.
func (m *OutlinerModel) emptyModeNextStep(mode chrome.OutlinerModeID) (string, bool) {
	var (
		operationID operation.OperationID
		act         string
	)
	switch mode {
	case chrome.ModeToday:
		operationID, act = operation.OperationCoreToday, "on a task to plan it for today"
	case chrome.ModeNext:
		operationID, act = operation.OperationCoreNext, "on a task to add it to Next"
	case chrome.ModeDue:
		operationID, act = operation.OperationTaskSetDueDate, "on a task to give it a deadline"
	default:
		return "", false
	}
	if m == nil || m.OperationRegistry == nil {
		return "", false
	}
	key := m.OperationRegistry.InputShortcutForOperation(operation.InputStrategySimplified, operationID)
	if key == "" {
		return "", false
	}
	return key + " " + act, true
}

// outlinerListRows is the height of the block between the header chrome and the
// dock, and it is one definition because both projections have to measure
// against it: the populated path fills it with rows and pads the remainder, and
// the empty path fills it with its header and its explanation and pads the same
// way. That is what puts the dock on one screen row whichever path drew the
// frame. It is pure — the dock's cost is the frame's answer and the rest is the
// frame — so nothing here reads the cursor or the document.
func (m *OutlinerModel) outlinerListRows(extraFooterLines int) int {
	// Clamp to at least one row: at pathologically small heights (e.g. the
	// dimmed project-cycle/pinned preview backgrounds sized to a couple of
	// lines) contentHeight()-extraFooterLines can go non-positive, which would
	// drive a negative window slice.
	//
	// The shell's footer is charged here, once, from what it measured rather than
	// from what this view can guess about it: a footer carrying feedback beside
	// its controls, or controls wrapped at minimum width, is taller than the two
	// rows Height() was sized against, and the difference is the list's to pay.
	// Nothing below re-measures it.
	geometry := m.peekDockGeometry()
	reserved := m.ShellFrame.ExcessRows()
	if geometry.Displayed() {
		// While the dock is open that charge has a floor, and the floor is what
		// keeps the dock on one screen row. The dock's own guidance is the one
		// footer content this view's cursor decides — `Ctrl+D more` is advertised
		// over a document that can move and withheld over one that cannot — so at
		// minimum width the footer wraps for one row and not for the next, and a
		// list following that difference would slide the dock under the cursor.
		// One row is paid whether or not this row's document pages; a footer that
		// genuinely costs more than that is still charged in full. The dock's own
		// availability arithmetic charges the same reservation, so the two cannot
		// disagree about what the footer costs.
		reserved = m.peekDockFooterReservation()
	}
	rows := max(m.ContentHeight()-extraFooterLines-reserved, 1)
	// The dock's height answers to the frame alone, so the list loses the same
	// rows whatever the cursor is resting on: opening Peek moves the list once,
	// and nothing the cursor does afterwards moves it again.
	return max(rows-geometry.TotalRows(), 1)
}

// SetShellFrameMetrics records the footer budget the shell measured for this
// frame. It is deliberately not SetSize: the scroll window survives it, so a
// message arriving and expiring costs the list a row without re-anchoring what
// it is showing.
func (m *OutlinerModel) SetShellFrameMetrics(metrics chrome.ShellFrameMetrics) {
	m.ShellFrame = metrics
}

func (m *OutlinerModel) ComputeOutlinerViewport(width int, extraFooterLines int) outlinerViewport {
	visibleRows := m.outlinerListRows(extraFooterLines)

	window := m.pinnedContextWindow(visibleRows)
	window.VisibleRows = visibleRows
	window.rowWidth = width
	if len(m.Rows) > visibleRows {
		window.rowWidth = width - 1
	}

	m.LastViewport = window
	return m.LastViewport
}

// pinnedContextWindow decides the row window and whether its first screen row
// is spent on the pinned context row. A window that opens inside a lane whose
// header scrolled above it gives that row up and windows one row fewer, through
// the same PersistentIndexWindow call over the smaller height, so the cursor is
// kept visible beneath the pinned row by the rule that keeps it visible
// anywhere. Both heights are tried against the stored offset rather than one
// against the other's result, so a frame that changes nothing writes the offset
// it found.
//
// The remaining case is the smaller window opening exactly on a header: nothing
// is left to pin, so the row is given back to the window, which then starts on
// that header. At the end of the list that window would come up a row short, and
// there the full window stands instead, unpinned, since a blank row would hide a
// row to name a lane one line above it.
func (m *OutlinerModel) pinnedContextWindow(visibleRows int) outlinerViewport {
	window, offset := m.pinnedContextWindowAt(m.ViewScrollOffset, visibleRows)
	m.ViewScrollOffset = offset
	return window
}

// pinnedContextWindowAt is that decision over a given offset, returning the
// offset the frame settles on instead of writing it. It is what lets the scroll
// chord ask which window an offset would draw before it stores one: whether the
// first screen row is pinned is a property of the destination, not of the frame
// the key was pressed on.
func (m *OutlinerModel) pinnedContextWindowAt(offset, visibleRows int) (outlinerViewport, int) {
	full := offset
	startIdx, endIdx := chrome.PersistentIndexWindow(m.Cursor, len(m.Rows), visibleRows, &full)
	if _, scrolledPast := m.sectionHeaderAbove(startIdx); !scrolledPast || visibleRows < 2 {
		return outlinerViewport{startIdx: startIdx, endIdx: endIdx}, full
	}
	fullWindow := outlinerViewport{startIdx: startIdx, endIdx: endIdx}
	reduced := offset
	startIdx, endIdx = chrome.PersistentIndexWindow(m.Cursor, len(m.Rows), visibleRows-1, &reduced)
	header, scrolledPast := m.sectionHeaderAbove(startIdx)
	if !scrolledPast && startIdx+visibleRows > len(m.Rows) {
		return fullWindow, full
	}
	if !scrolledPast {
		return outlinerViewport{startIdx: startIdx, endIdx: startIdx + visibleRows}, reduced
	}
	return outlinerViewport{startIdx: startIdx, endIdx: endIdx, pinned: true, pinnedHeader: header}, reduced
}

// sectionHeaderAbove answers whether the row at index sits in a lane whose
// header is above it, and where that header is. It reads the row's own group
// and the index the projection recorded, never the rows between them. Either
// single-project form answers no, as does a row carrying no section — the
// ranked list, a separator, a projection that emits no lanes — and an index the
// rows no longer bear out is not trusted.
func (m *OutlinerModel) sectionHeaderAbove(index int) (int, bool) {
	if index <= 0 || index >= len(m.Rows) || !m.groupsRowsByProject() {
		return 0, false
	}
	key := m.Rows[index].Group.SectionKey
	if key == "" {
		return 0, false
	}
	header, ok := m.sectionHeaderRows[key]
	if !ok || header >= index {
		return 0, false
	}
	if row := m.Rows[header]; !row.IsSectionHeader || row.Group.SectionKey != key {
		return 0, false
	}
	return header, true
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
