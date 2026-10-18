package outliner

import (
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/rivo/uniseg"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/textfit"
)

func (m *OutlinerModel) renderTaskTitle(row OutlinerRow, titleStyle lipgloss.Style, rowWidth int, segments taskRowSegments) string {
	availableWidth := segments.titleWidth(rowWidth)
	title := row.Task.Title
	highlight, caseSensitive := m.titleHighlightQuery()
	if lipgloss.Width(title) > availableWidth {
		title = m.compactTitleForRow(title)
	}
	fullTitle := title
	title = textfit.TruncateToVisualWidth(title, availableWidth)
	if highlight == "" {
		return textfit.TruncateToVisualWidth(textfit.RenderTaskTitle(fullTitle, nil, titleStyle), availableWidth)
	}
	// Keep the familiar title opening when it still explains the match. Only
	// move the excerpt when ordinary shortening would hide a query term.
	matcher := fuzzy.NewMatcher(caseSensitive)
	if title != row.Task.Title && matcher.ScoreTerms(title, highlight) == 0 {
		_, positions := matcher.ScoreTermsWithPositions(row.Task.Title, highlight)
		if len(positions) > 0 {
			excerpt, kept := titleMatchExcerpt(row.Task.Title, positions, availableWidth)
			return textfit.RenderTaskTitle(excerpt, kept, titleStyle)
		}
	}
	positions := query.SearchSubstringPositions(title, highlight, caseSensitive)
	// A title the matcher admitted as a subsequence holds no contiguous run to
	// paint, and a title hit earns no cue, so without this the row would explain
	// itself nowhere. The positions are the matcher's own, as Peek's excerpt
	// reads them, and only for a row the projection attributed to its title.
	if len(positions) == 0 && row.MatchSource == MatchSourceTitle {
		_, positions = fuzzy.NewMatcher(caseSensitive).ScoreTermsWithPositions(title, highlight)
	}
	return textfit.TruncateToVisualWidth(textfit.RenderTaskTitle(fullTitle, positions, titleStyle), availableWidth)
}

// compactTitleForRow is the row's display form of a title too wide for it: a
// URL keeps its host and first path segment, and a path under the home
// directory keeps its last segment. It runs before the cut, so a search
// highlight paints the compacted text the row draws. It is the row's alone —
// Peek, the editors, the form, and exports read the stored title whole.
func (m *OutlinerModel) compactTitleForRow(title string) string {
	return compactRowTitle(title, m.homeDir)
}

// compactRowTitle compacts each space-separated word of title that is an
// http(s) URL or an absolute path under home, leaving the rest as written.
// Punctuation wrapped around a word, such as a closing parenthesis or a full
// stop, stays outside the compacted form.
func compactRowTitle(title, home string) string {
	words := strings.Split(title, " ")
	for i, word := range words {
		core := strings.TrimLeft(word, "(<[\"'")
		lead := word[:len(word)-len(core)]
		core = strings.TrimRight(core, ".,;:!?)>]\"'")
		trail := word[len(lead)+len(core):]
		if core == "" {
			continue
		}
		if compact, ok := compactTitleURL(core); ok {
			words[i] = lead + compact + trail
		} else if compact, ok := compactTitleHomePath(core, home); ok {
			words[i] = lead + compact + trail
		}
	}
	return strings.Join(words, " ")
}

// compactTitleURL renders an http(s) URL as its host, without `www.`, then its
// first path segment, then `/…` when anything follows it.
func compactTitleURL(word string) (string, bool) {
	u, err := url.Parse(word)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", false
	}
	compact := strings.TrimPrefix(u.Host, "www.")
	var segments []string
	for _, segment := range strings.Split(u.EscapedPath(), "/") {
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	if len(segments) > 0 {
		compact += "/" + segments[0]
	}
	if len(segments) > 1 || u.RawQuery != "" || u.Fragment != "" {
		compact += "/…"
	}
	return compact, true
}

// compactTitleHomePath renders an absolute path under home as `~/…/<last>`,
// or as `~/<last>` when the last segment sits directly in home.
func compactTitleHomePath(word, home string) (string, bool) {
	if home == "" {
		return "", false
	}
	sep := string(filepath.Separator)
	if word == home || word == home+sep {
		return "~", true
	}
	rest, ok := strings.CutPrefix(word, home+sep)
	if !ok {
		return "", false
	}
	var segments []string
	for _, segment := range strings.Split(rest, sep) {
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	switch len(segments) {
	case 0:
		return "~", true
	case 1:
		return "~" + sep + segments[0], true
	default:
		return "~" + sep + "…" + sep + segments[len(segments)-1], true
	}
}

// titleHighlightQuery names the free text the visible rows were narrowed by,
// with the case mode they were actually produced under. Ordinary search and a
// lens narrowing are the same act to a title: whichever one owns the current
// projection is what a highlight must read, so the two can never disagree.
func (m *OutlinerModel) titleHighlightQuery() (string, bool) {
	if query := m.searchTextQuery(); query != "" {
		return query, m.searchCaseSensitive()
	}
	if !m.HasActiveLens() {
		return "", false
	}
	return m.LensTextQuery(), m.LensCaseSensitive()
}

func outlinerNonTitleWidth(cursorPrefix, prefixStr, foldIndicator, statusIcon, prioritySegment, projectLabel, pinMarker, indicators, relationMarker, matchCue string) int {
	return lipgloss.Width(cursorPrefix) +
		lipgloss.Width(prefixStr) + lipgloss.Width(foldIndicator) +
		lipgloss.Width(statusIcon) + 1 +
		lipgloss.Width(prioritySegment) + lipgloss.Width(projectLabel) + lipgloss.Width(pinMarker) +
		lipgloss.Width(indicators) +
		lipgloss.Width(relationMarker) + lipgloss.Width(matchCue)
}

// PeekDocumentBodyForRow is one row's complete detail document, wrapped to the
// dock's own indent and width. It is the whole of it rather than the window on
// screen: the dock reaches the window through Lines, and this is the seam for
// naming a row's detail directly — what Peek can be scrolled to, independent of
// where the cursor is and of what the frame currently shows.
func (m *OutlinerModel) PeekDocumentBodyForRow(row OutlinerRow) []string {
	if m == nil || !rowSupportsPeek(row) {
		return nil
	}
	// The body is the whole document, so the geometry stamped onto it changes
	// nothing here; the frame's own is passed so this seam names no second window.
	return m.inlinePeekDocumentForRow(row, peekDockIndent, m.peekDockWidth(), m.peekDockGeometry()).Body
}

// projectAbbreviations keeps ordinary prefixes unless they collide. Colliding
// names retain progressively more of their suffix, using display cells rather
// than bytes. The loaded population, not the visible rows, determines the names,
// so folding and scrolling cannot change a label.
func projectAbbreviations(names []string, width int) map[string]string {
	names = slices.Clone(names)
	slices.Sort(names)
	names = slices.Compact(names)
	labels := make(map[string]string, len(names))
	counts := make(map[string]int, len(names))
	used := make(map[string]bool, len(names))
	for _, name := range names {
		counts[textfit.TruncateToVisualWidth(name, width)]++
	}
	for _, name := range names {
		label := textfit.TruncateToVisualWidth(name, width)
		if counts[label] == 1 || lipgloss.Width(name) <= width {
			labels[name], used[label] = label, true
		}
	}
	for _, name := range names {
		if _, ok := labels[name]; ok {
			continue
		}
		for head := width / 2; head >= 0; head-- {
			candidate := abbreviateProjectMiddle(name, width, head)
			unique := !used[candidate]
			for _, other := range names {
				if other != name && abbreviateProjectMiddle(other, width, head) == candidate {
					unique = false
					break
				}
			}
			if unique {
				labels[name], used[candidate] = candidate, true
				break
			}
		}
		if _, ok := labels[name]; ok {
			continue
		}
		// Extremely similar names may differ only in their middle. A stable ordinal
		// still keeps them distinct when neither end fits that distinction.
		for ordinal := 1; ; ordinal++ {
			suffix := fmt.Sprintf("~%d", ordinal)
			candidate := textfit.TruncateIntentional(name, max(width-len(suffix), 0), "") + suffix
			if !used[candidate] {
				labels[name], used[candidate] = candidate, true
				break
			}
		}
	}
	return labels
}

func abbreviateProjectMiddle(name string, width, head int) string {
	if lipgloss.Width(name) <= width {
		return name
	}
	tail := max(width-head-1, 0)
	end := lipgloss.Width(name)
	start := max(end-tail, 0)
	suffix := ansi.Cut(name, start, end)
	for lipgloss.Width(suffix) > tail {
		start++
		suffix = ansi.Cut(name, start, end)
	}
	return textfit.TruncateIntentional(name, head, "") + "…" + suffix
}

type projectLabelCache struct {
	width  int
	names  []string
	labels map[string]string
	// A scoped preview can repeatedly render one project outside the population.
	standaloneName  string
	standaloneLabel string
}

func projectLabelName(project *models.Project) string {
	if project == nil {
		return ""
	}
	if project.Slug != "" {
		return project.Slug
	}
	return project.Name
}

func (m *OutlinerModel) abbreviatedProjectSlug(slug string, width int) string {
	// Compare the population without allocating or sorting for every task row.
	// Names, rather than project pointers, also detect an in-place rename.
	cache := &m.projectLabels
	valid := cache.width == width && len(cache.names) == len(m.Population.Projects)
	if valid {
		for i, project := range m.Population.Projects {
			if cache.names[i] != projectLabelName(project) {
				valid = false
				break
			}
		}
	}
	if !valid || cache.labels == nil {
		names := make([]string, len(m.Population.Projects))
		for i, project := range m.Population.Projects {
			names[i] = projectLabelName(project)
		}
		*cache = projectLabelCache{width: width, names: names, labels: projectAbbreviations(names, width)}
	}
	if label, ok := cache.labels[slug]; ok {
		return label
	}
	// Standalone previews may name an unloaded project. It must not change
	// labels for the loaded population on the following render.
	if cache.standaloneName != slug {
		cache.standaloneName = slug
		cache.standaloneLabel = projectAbbreviations(append(slices.Clone(cache.names), slug), width)[slug]
	}
	return cache.standaloneLabel
}

// titleMatchExcerpt keeps the matching part of a long title, with explicit
// elision and original rune positions. Boundaries are whole graphemes, so a
// match inside an emoji or combining sequence never detaches its other runes.
func titleMatchExcerpt(title string, positions []int, width int) (string, []int) {
	if len(positions) == 0 || width <= 0 {
		return "", nil
	}
	var clusters []string
	cells, runes := []int{0}, []int{0}
	g := uniseg.NewGraphemes(title)
	for g.Next() {
		cluster := g.Str()
		clusters = append(clusters, cluster)
		cells = append(cells, cells[len(cells)-1]+lipgloss.Width(cluster))
		runes = append(runes, runes[len(runes)-1]+utf8.RuneCountInString(cluster))
	}
	first, last := 0, len(clusters)
	for first+1 < len(runes) && runes[first+1] <= positions[0] {
		first++
	}
	for last > first+1 && runes[last-1] > positions[len(positions)-1] {
		last--
	}
	cost := func(start, end int) int {
		n := cells[end] - cells[start]
		if start > 0 {
			n++
		}
		if end < len(clusters) {
			n++
		}
		return n
	}
	start, end := first, last
	for end > start+1 && cost(start, end) > width {
		end--
	}
	if cost(start, end) > width {
		// At an exceptionally small title budget even the elision markers may
		// not fit. Show the matched grapheme if possible, still within budget.
		part := textfit.TruncateToVisualWidth(clusters[start], width)
		return part, nil
	}
	for start > 0 && cells[first]-cells[start-1] <= 8 && cost(start-1, end) <= width {
		start--
	}
	for end < len(clusters) && cost(start, end+1) <= width {
		end++
	}
	prefix := ""
	if start > 0 {
		prefix = "…"
	}
	excerpt := prefix + strings.Join(clusters[start:end], "")
	if end < len(clusters) {
		excerpt += "…"
	}
	var kept []int
	for _, position := range positions {
		if position >= runes[start] && position < runes[end] {
			kept = append(kept, position-runes[start]+utf8.RuneCountInString(prefix))
		}
	}
	return excerpt, kept
}
