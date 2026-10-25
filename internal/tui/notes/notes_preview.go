package notes

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/fuzzy"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/route"
	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// notesPreviewKey is the complete identity of one rendered, anchored preview.
// Paging is manual, so an offset may only survive while every input that
// produced the lines and the position they opened at is unchanged: the note the
// cursor is on, the collection it was drawn from, the query that anchored it
// under the case mode the visible rows were built with, the exact bytes it was
// loaded from, and the pane width the Markdown was wrapped to. Anything
// narrower would leave a plausible offset pointing into lines that no longer
// exist, or at a region the current query never matched.
type notesPreviewKey struct {
	identity NoteRowIdentity
	scope    route.NotesScope
	// query is the active draft or the accepted query, whichever the visible rows
	// were derived from, and caseSensitive is the case mode that produced them —
	// including the relaxed retry, so a fallback anchors on the rows it returned.
	query         string
	caseSensitive bool
	revision      string
	width         int
}

// matches names every part of the identity in one place, one comparison per
// field, rather than leaning on struct equality. A field added above without a
// line here is a part of the identity a stale cache would be allowed to ignore,
// which is exactly the failure this key exists to prevent, so the enumeration is
// the point.
func (k notesPreviewKey) matches(other notesPreviewKey) bool {
	return k.identity == other.identity &&
		k.scope == other.scope &&
		k.query == other.query &&
		k.caseSensitive == other.caseSensitive &&
		k.revision == other.revision &&
		k.width == other.width
}

// notesPreviewState is the Notes preview pane's line cache and scroll position.
// It is session-only and deliberately holds no note pointer: the rows behind it
// are rebuilt by every reload, and the key is what proves the cache still
// describes the note under the cursor.
type notesPreviewState struct {
	key notesPreviewKey
	// ready distinguishes a synced empty body from a cache that was never built,
	// so the zero key of a freshly invalidated state cannot pass for a match.
	ready bool
	// lines are the note body rendered through textfit.RenderMarkdown at key.width and
	// split into display lines. Rendering is cached because the cursor moves far
	// more often than the note under it changes.
	lines  []string
	offset int
	// pageSize is the pane's row budget, recorded by the render because only the
	// render knows how many rows the header left. The key handler would otherwise
	// have to guess at a height it never sees.
	pageSize int
}

// invalidate drops the cache so the next frame re-renders and re-anchors from
// scratch.
func (s *notesPreviewState) invalidate() {
	*s = notesPreviewState{}
}

// sync rebuilds the cache whenever any part of the key moved, and re-anchors
// with it: an offset taken from the previous note's length would open the new
// one part-way down for no reason the user can see. While the key holds, the
// offset is the reader's, so ordinary re-renders leave manual paging alone.
//
// bodyMatched is the visible row's own answer to whether the query reached the
// body. A title-only hit, and every state with no query at all, opens at the
// top, because there is no body region the reader was sent here to see.
func (s *notesPreviewState) sync(key notesPreviewKey, body string, bodyMatched bool) {
	if s.ready && s.key.matches(key) {
		return
	}
	*s = notesPreviewState{key: key, ready: true}
	if strings.TrimSpace(body) == "" {
		return
	}
	rendered := textfit.RenderMarkdown(body, max(key.width, 1))
	if rendered == "" {
		return
	}
	s.lines = strings.Split(rendered, "\n")
	if bodyMatched {
		s.offset = notesPreviewAnchor(s.lines, key.query, key.caseSensitive)
	}
}

// notesPreviewAnchor answers the rendered line a body match should open at: the
// earliest line the query is locatable on, less one line of context so the hit
// is not flush against the top of the pane. Search runs over the raw body while
// the pane shows wrapped, styled Markdown, so a hit is located again here
// against the rendered lines; a subsequence that only exists across a wrap point
// is unlocatable and deterministically yields the top rather than a guess.
func notesPreviewAnchor(lines []string, query string, caseSensitive bool) int {
	terms := strings.Fields(query)
	if len(terms) == 0 || len(lines) == 0 {
		return 0
	}
	plain := make([]string, len(lines))
	for i, line := range lines {
		plain[i] = ansi.Strip(line)
	}
	// The contiguous pass is exhausted across every term before the fuzzy one, so
	// a query whose first term appears literally far down the note is never
	// overruled by a later term's incidental subsequence near the top.
	best := earliestNotesPreviewLine(plain, terms, caseSensitive, false)
	if best < 0 {
		best = earliestNotesPreviewLine(plain, terms, caseSensitive, true)
	}
	if best < 0 {
		return 0
	}
	return max(best-1, 0)
}

// earliestNotesPreviewLine reports the lowest line index any term is found on,
// or -1 when this pass locates none of them.
func earliestNotesPreviewLine(plain, terms []string, caseSensitive, allowFuzzy bool) int {
	matcher := fuzzy.NewMatcher(caseSensitive)
	best := -1
	for _, term := range terms {
		needle := term
		if !caseSensitive {
			needle = strings.ToLower(term)
		}
		for index, line := range plain {
			if best >= 0 && index >= best {
				break
			}
			var hit bool
			if allowFuzzy {
				// The matcher already owns the case mode, so the raw line is the
				// honest haystack for a subsequence test.
				hit = matcher.Score(line, term) > 0
			} else {
				haystack := line
				if !caseSensitive {
					haystack = strings.ToLower(line)
				}
				hit = strings.Contains(haystack, needle)
			}
			if hit {
				best = index
				break
			}
		}
	}
	return best
}

// notesPreviewRevision names the bytes the cache was built from. RawRevision is
// the canonical file revision and is preferred; a note the workspace holds
// without one still has to re-render when its body changes, so the body itself
// is the in-memory fallback, digested rather than retained.
func notesPreviewRevision(note *models.Note) string {
	if revision := strings.TrimSpace(note.RawRevision); revision != "" {
		return revision
	}
	digest := fnv.New64a()
	_, _ = digest.Write([]byte(note.Body))
	return fmt.Sprintf("body:%d:%016x", len(note.Body), digest.Sum64())
}

// notesPreviewGeometry answers the shared two-pane split for a Notes width. A
// preview width of zero is the narrow terminal, which renders the list alone.
// Height is irrelevant to the split and is passed only to satisfy the shared
// config's normalization.
func (m *NotesViewModel) notesPreviewGeometry(width int) (int, int) {
	return listsurface.ListSurfacePreviewGeometry(listsurface.ListSurfaceRenderConfig{
		Width:  max(width, 1),
		Height: 1,
	})
}

// notesPreviewRow is the note the pane previews, or nil when the surface has
// nothing to show beside the list. A picker owns the whole view while it is
// open, and a snapshot error means no row is trustworthy.
func (m *NotesViewModel) notesPreviewRow() *LocatedNoteRow {
	if m.LocationPicker != nil || m.snapshotError != nil {
		return nil
	}
	row := m.CurrentRow()
	if row == nil || row.Note == nil {
		return nil
	}
	return row
}

// notesPreviewCacheKey assembles the whole anchor identity in one place, so the
// render pass cannot spell a key the paging keys or a test would spell
// differently. width is the resolved pane width, which only the render knows.
func (m *NotesViewModel) notesPreviewCacheKey(row *LocatedNoteRow, width int) notesPreviewKey {
	return notesPreviewKey{
		identity:      row.Identity(),
		scope:         m.Scope,
		query:         strings.TrimSpace(m.activeSearchQuery()),
		caseSensitive: m.effectiveSearchCaseSensitive(),
		revision:      notesPreviewRevision(row.Note),
		width:         width,
	}
}

// effectiveSearchCaseSensitive reports the case mode that produced the rows on
// screen, not the one the query's spelling asks for: a relaxed retry is what the
// reader is looking at, so the anchor has to agree with it or it would search
// for something the visible result set never matched.
func (m *NotesViewModel) effectiveSearchCaseSensitive() bool {
	query := strings.TrimSpace(m.activeSearchQuery())
	if query == "" || m.searchCaseRelaxed {
		return false
	}
	return fuzzy.SmartCase(query).CaseSensitive()
}

// notesPreviewBodyMatched reports whether the query reached this row's body, as
// the visible result set decided it. A title-only hit reports false and opens at
// the top.
func (m *NotesViewModel) notesPreviewBodyMatched(row *LocatedNoteRow) bool {
	return m.SearchMatches[row.Identity()].BodyMatched
}

// notesPreviewVisible answers whether the pane is on screen at all: the shared
// geometry leaves room for it and the cursor is on a note it can draw. It is the
// pane-existence authority alone; what may be paged is notesPreviewPageable's
// question, since a drawn pane is not necessarily a scrollable one.
func (m *NotesViewModel) notesPreviewVisible() bool {
	if _, previewWidth := m.notesPreviewGeometry(m.Width()); previewWidth <= 0 {
		return false
	}
	return m.notesPreviewRow() != nil
}

// notesPreviewPageable is the paging predicate, and it is deliberately narrower
// than notesPreviewVisible: a pane can be on screen and still have nothing a
// page could move. A parse-error note draws a fixed warning instead of a body —
// the render zeroes its offset and leaves it no page size — so Ctrl+F/B is a
// no-op there and must not be offered or allowed to write paging state. It stays
// a body-content test rather than a page-count test for the reason
// notesPreviewVisible gives: the pane's height is settled during the render this
// gate is part of, so a count here would be one frame stale.
func (m *NotesViewModel) notesPreviewPageable() bool {
	if !m.notesPreviewVisible() {
		return false
	}
	row := m.notesPreviewRow()
	return row != nil && row.Note.ParseError == ""
}

// pageNotesPreview moves the pane a full page, vim's Ctrl+F and Ctrl+B. Only the
// lower bound is applied here: the upper one needs the pane's row capacity,
// which the render pass owns, so an offset past the tail is corrected there
// rather than guessed at from a stale page size.
func (m *NotesViewModel) pageNotesPreview(direction int) {
	if direction == 0 || !m.notesPreviewPageable() {
		return
	}
	m.preview.offset = max(m.preview.offset+direction*max(m.preview.pageSize, 1), 0)
}

// notesPreviewHeader supplies the shared preview renderer's header. A note whose
// frontmatter failed to parse reports only a reliable Filename, so it names the
// file and leaves the title, owner-independent metadata, and body to the warning
// the body hook draws.
func notesPreviewHeader(row *LocatedNoteRow) listsurface.ListPreview {
	note := row.Note
	if note.ParseError != "" {
		return listsurface.ListPreview{Title: note.Filename, Summary: row.OwnerLabel}
	}
	title := strings.TrimSpace(note.Title)
	if title == "" {
		title = note.Filename
	}
	preview := listsurface.ListPreview{Title: title, Summary: row.OwnerLabel}
	if !note.ModifiedAt.IsZero() {
		preview.Fields = []listsurface.ListPreviewField{{
			Label: "Modified",
			Value: formatNotesPreviewTime(note.ModifiedAt),
		}}
	}
	return preview
}

func formatNotesPreviewTime(t time.Time) string {
	return t.Format("2006-01-02 15:04") + " (" + storage.FormatRelativeTime(t) + ")"
}

// renderNotesPreviewBody fills the rows the preview header left with the
// selected note's Markdown. It is the RenderListSurfacePreview body hook, so
// height is the pane's remaining capacity and this is the only place that knows
// it: the scroll offset is clamped here rather than where Ctrl+F moved it, the
// same arrangement the Project Hub preview and the list pane's own viewport
// sizing use.
func (m *NotesViewModel) renderNotesPreviewBody(width, height int) []string {
	if width <= 0 || height <= 0 {
		return nil
	}
	row := m.notesPreviewRow()
	if row == nil {
		return nil
	}
	note := row.Note
	key := m.notesPreviewCacheKey(row, width)
	if note.ParseError != "" {
		// Syncing against an empty body keeps the anchor honest and the offset at
		// zero, so returning to a parseable note cannot inherit this note's scroll.
		m.preview.sync(key, "", false)
		// The warning is not a scrollable body, so the render — the one authority
		// over what the offset may be, exactly as it is for the tail clamp below —
		// puts the pane back at the top and leaves no page size for a key press to
		// spend.
		m.preview.offset, m.preview.pageSize = 0, 0
		lines := []string{theme.WarningIconStyle.Render(
			textfit.TruncateToVisualWidth("! Note could not be parsed.", width),
		)}
		for _, line := range textfit.WrapText(note.ParseError, max(width, 1)) {
			lines = append(lines, theme.MutedStyle.Render(textfit.TruncateToVisualWidth(line, width)))
		}
		return lines[:min(len(lines), height)]
	}

	m.preview.sync(key, note.Body, m.notesPreviewBodyMatched(row))
	if len(m.preview.lines) == 0 {
		return []string{theme.MutedStyle.Render(textfit.TruncateToVisualWidth("No body.", width))}
	}

	start, count, above, below := listsurface.ListPreviewScrollWindow(len(m.preview.lines), m.preview.offset, height)
	m.preview.offset = start
	// A page is the row budget less both continuation rows, not the rows this
	// frame happened to show. The visible count changes by one as continuations
	// come and go, and paging by it would leave Ctrl+B a row short of where Ctrl+F
	// started. Spending both rows always costs a row of overlap instead, which
	// reads as context rather than as drift.
	m.preview.pageSize = max(height-2, 1)
	lines := make([]string, 0, height)
	if above {
		lines = append(lines, listsurface.RenderListContinuation("↑", start, width))
	}
	for index := start; index < start+count; index++ {
		lines = append(lines, textfit.TruncateToVisualWidth(m.preview.lines[index], width))
	}
	if below {
		lines = append(lines, listsurface.RenderListContinuation("↓", len(m.preview.lines)-start-count, width))
	}
	return lines
}
