package outliner

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

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
	count := header.SectionCount
	if !header.IsSectionHeader {
		slug, _ := m.projectSlugForRow(header)
		name, prefix, count = "["+slug+"]", "", 0
	}
	if projectID := m.Rows[viewport.startIdx].Group.ProjectID; projectID != "" {
		if header.IsSectionHeader {
			slug, _ := m.projectSlugForRow(OutlinerRow{ProjectID: projectID})
			name += " › [" + slug + "]"
		}
	}
	gutter, content := renderSectionHeader(sectionHeaderOpts{
		Name:        name,
		Count:       count,
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
		AccentColor: theme.StructuralSectionStyle(sectionStyle),
		Prefix:      prefix,
		Cursor:      paint,
	})

	if row.ProjectID != "" {
		project := m.ResolveProjectForRow(row)
		projectName := row.ProjectID
		if project != nil {
			projectName = project.Name
		}
		content += "  " + theme.ProjectLabel(project, projectName)
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
	badge := theme.ProjectLabel(project, projectSlug)
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
	content := m.projectHeaderPinMarker(project) + badge
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

// projectHeaderPinMarker reserves the same columns on grouped project rows
// when any project in scope is pinned, keeping their labels aligned.
func (m *OutlinerModel) projectHeaderPinMarker(project *models.Project) string {
	if project != nil && project.Pinned {
		return listsurface.PinnedMarker()
	}
	if !m.groupsRowsByProject() {
		return ""
	}
	for _, candidate := range m.ScopeProjects() {
		if candidate != nil && candidate.Pinned {
			return strings.Repeat(" ", lipgloss.Width(listsurface.PinnedMarker()))
		}
	}
	return ""
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
	gutter := paint.bar(lipgloss.NewStyle(), theme.RenderCursorPrefix()) +
		gutterPiece(paint.gutterBg(), m.treeConnectorStyle(row), m.outlinerTreePrefix(row, rowWidth))
	content := theme.MutedStyle.Render(box) + text

	if selected {
		return outlinerCursorRow(gutter, content, rowWidth, paint)
	}
	return chrome.ZebraLine(textfit.TruncateToVisualWidth(gutter+content, rowWidth), rowWidth, rowIndex)
}
