package outliner

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

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
	label := theme.ProjectLabel(project, slug)
	if global {
		label = theme.MutedStyle.Render("[" + slug + "]")
	}
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

// renderOutlinerNoteRow draws a project note or a ranked note search result. It
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
