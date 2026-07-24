package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// outlinerModePicker is the inline, searchable, categorized mode selector opened
// with P. It replaces the old P0-P9 numeric prefix tree: every catalog mode is
// reachable by name (label or alias) with no memorized numbers. State is
// self-contained; the picker never touches the view stack.
type outlinerModePicker struct {
	query string
	// cursor indexes the selectable modes (headers skipped).
	cursor int
	// scroll is the persistent top-of-window flat-row offset, kept in sync by
	// the viewport so the selected mode stays on screen across renders.
	scroll int
}

// modePickerRow is one displayed line: either a category header or a selectable
// mode. Headers are skipped by cursor movement.
type modePickerRow struct {
	header   string
	mode     OutlinerMode
	isMode   bool
	disabled bool
}

// String names the picker categories.
func (c OutlinerModeCategory) String() string {
	switch c {
	case ModeCategoryWork:
		return "Work"
	case ModeCategoryWorkflow:
		return "Workflow"
	case ModeCategorySmart:
		return "Smart"
	case ModeCategoryLens:
		return "Lenses"
	case ModeCategoryReview:
		return "Review"
	}
	return ""
}

// modeMatchesQuery reports whether a mode matches the fuzzy query against its
// label and aliases. An empty query matches everything.
func modeMatchesQuery(mode OutlinerMode, query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return true
	}
	if strings.Contains(strings.ToLower(mode.Label), q) {
		return true
	}
	for _, alias := range mode.Aliases {
		if strings.Contains(strings.ToLower(alias), q) {
			return true
		}
	}
	return false
}

// modePickerModeUnavailable reports whether a mode cannot yet be entered. Every
// catalog mode — including the Review data strategies (Daily Review, Activity),
// which now render as Outliner modes — is selectable, so nothing is disabled.
// The seam is retained so a future mode can opt out without reintroducing a
// second convention.
func modePickerModeUnavailable(mode OutlinerMode) bool {
	return false
}

// modePickerRows builds the categorized, query-filtered display rows. Category
// headers are emitted only for categories with at least one matching mode.
func modePickerRows(query string) []modePickerRow {
	var rows []modePickerRow
	categories := []OutlinerModeCategory{
		ModeCategoryWork, ModeCategoryWorkflow, ModeCategorySmart, ModeCategoryLens, ModeCategoryReview,
	}
	for _, cat := range categories {
		var modes []OutlinerMode
		for _, mode := range OutlinerModesByCategory(cat) {
			if modeMatchesQuery(mode, query) {
				modes = append(modes, mode)
			}
		}
		if len(modes) == 0 {
			continue
		}
		rows = append(rows, modePickerRow{header: cat.String()})
		for _, mode := range modes {
			rows = append(rows, modePickerRow{
				mode:     mode,
				isMode:   true,
				disabled: modePickerModeUnavailable(mode),
			})
		}
	}
	return rows
}

// selectableModes returns just the mode rows (no headers) for cursor bounds.
func selectableModes(rows []modePickerRow) []modePickerRow {
	out := make([]modePickerRow, 0, len(rows))
	for _, r := range rows {
		if r.isMode {
			out = append(out, r)
		}
	}
	return out
}

// openModePicker opens the inline mode picker, resetting query and cursor.
func (m *OutlinerModel) openModePicker() (tea.Model, tea.Cmd) {
	m.modePicker = &outlinerModePicker{}
	return m, nil
}

// handleModePickerKey processes a key while the mode picker is open.
func (m *OutlinerModel) handleModePickerKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	p := m.modePicker
	selectable := selectableModes(modePickerRows(p.query))

	switch msg.String() {
	case "esc":
		m.modePicker = nil
		return m, nil
	case "enter":
		if len(selectable) == 0 || p.cursor >= len(selectable) {
			return m, nil
		}
		chosen := selectable[p.cursor]
		if chosen.disabled {
			m.setStatus(chosen.mode.Label + " mode is not available yet")
			return m, nil
		}
		m.modePicker = nil
		return m.selectOutlinerMode(chosen.mode.ID)
	case "up", "ctrl+k", "shift+tab":
		if p.cursor > 0 {
			p.cursor--
		}
		return m, nil
	case "down", "ctrl+j", "tab":
		if p.cursor < len(selectable)-1 {
			p.cursor++
		}
		return m, nil
	case "backspace":
		if p.query != "" {
			p.query = p.query[:len(p.query)-1]
			p.cursor = 0
			p.scroll = 0
		}
		return m, nil
	default:
		if s := msg.String(); len(s) == 1 && s >= " " {
			p.query += s
			p.cursor = 0
			p.scroll = 0
		}
		return m, nil
	}
}

// modePickerChromeLines is the fixed number of lines the picker spends outside
// the scrollable list: the title, a blank line beneath it, a blank separator
// above the help footer, and the help footer itself.
const modePickerChromeLines = 4

// modePickerListHeight returns how many list lines (mode rows plus continuation
// indicators) the picker may render before it would exceed the Outliner height.
func (m *OutlinerModel) modePickerListHeight() int {
	h := m.height - modePickerChromeLines
	if h < 1 {
		return 1
	}
	return h
}

// modePickerFlatCursor maps the selectable-mode cursor index to its flat row
// index in rows (category headers included).
func modePickerFlatCursor(rows []modePickerRow, cursor int) int {
	modeIdx := -1
	for i, row := range rows {
		if !row.isMode {
			continue
		}
		modeIdx++
		if modeIdx == cursor {
			return i
		}
	}
	return 0
}

// modePickerWindow computes the visible [start,end) flat-row slice of the picker
// list, scrolled to keep the selected mode on screen, plus whether continuation
// indicators are needed above/below. It budgets the list against the Outliner
// height so the picker never exceeds its content region and every category stays
// reachable by scrolling the cursor.
func (m *OutlinerModel) modePickerWindow(rows []modePickerRow) (start, end int, above, below bool) {
	p := m.modePicker
	total := len(rows)
	listBudget := m.modePickerListHeight()

	if total <= listBudget {
		p.scroll = 0
		return 0, total, false, false
	}

	flatCursor := modePickerFlatCursor(rows, p.cursor)

	// Too small to fit a row plus both continuation markers: show just the
	// selected row so the cursor stays visible and nothing overflows.
	if listBudget < 3 {
		p.scroll = flatCursor
		return flatCursor, flatCursor + 1, false, false
	}

	// Reserve one line for each continuation marker so the window plus markers
	// never exceeds the list budget.
	windowRows := listBudget - 2
	start, end = persistentIndexWindow(flatCursor, total, windowRows, &p.scroll)
	return start, end, start > 0, end < total
}

// renderModePicker draws the full inline picker panel constrained to width and to
// the current Outliner height.
func (m *OutlinerModel) renderModePicker(width int) string {
	p := m.modePicker
	rows := modePickerRows(p.query)
	current := m.activeOutlinerMode()

	var b strings.Builder
	title := TitleStyle.Render("Switch mode")
	queryText := MutedStyle.Render("  filter: ") + p.query + "▏"
	b.WriteString(title + queryText)
	b.WriteString("\n\n")

	if len(selectableModes(rows)) == 0 {
		b.WriteString(MutedStyle.Render("No modes match \"" + p.query + "\""))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("type to filter · esc cancel"))
		return b.String()
	}

	start, end, above, below := m.modePickerWindow(rows)

	if above {
		b.WriteString(MutedStyle.Render("  ↑ more above"))
		b.WriteString("\n")
	}

	// modeIdx counts selectable modes from the top of the full list so the
	// selection marker stays correct even when the window starts mid-list.
	modeIdx := -1
	for i := 0; i < start; i++ {
		if rows[i].isMode {
			modeIdx++
		}
	}
	for i := start; i < end; i++ {
		row := rows[i]
		if !row.isMode {
			b.WriteString(MutedStyle.Render(row.header))
			b.WriteString("\n")
			continue
		}
		modeIdx++
		line := m.renderModePickerLine(row, modeIdx == p.cursor, row.mode.ID == current)
		b.WriteString(line)
		b.WriteString("\n")
	}

	if below {
		b.WriteString(MutedStyle.Render("  ↓ more below"))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(HelpStyle.Render("↑/↓ move · enter select · type to filter · esc cancel"))
	return b.String()
}

func (m *OutlinerModel) renderModePickerLine(row modePickerRow, selected, isCurrent bool) string {
	marker := "  "
	if selected {
		marker = "▸ "
	}

	label := row.mode.Label
	if isCurrent {
		label += " ●"
	}

	suffix := ""
	if alias := primaryModeAlias(row.mode); alias != "" {
		suffix = "  " + MutedStyle.Render(alias)
	}
	if row.disabled {
		suffix += "  " + MutedStyle.Render("(soon)")
	}

	styledLabel := label
	switch {
	case row.disabled:
		styledLabel = MutedStyle.Render(label)
	case selected:
		styledLabel = FilterItemSelectedStyle.Render(label)
	default:
		styledLabel = GetViewModeStyleForSection(row.mode.Label).Render(label)
	}

	return "  " + marker + styledLabel + suffix
}

// primaryModeAlias returns a short alias worth surfacing next to a mode label,
// skipping the alias that merely duplicates the label.
func primaryModeAlias(mode OutlinerMode) string {
	for _, alias := range mode.Aliases {
		if !strings.EqualFold(alias, mode.Label) && !strings.EqualFold(alias, string(mode.ID)) {
			return alias
		}
	}
	return ""
}

// modePickerActive reports whether the inline picker is open.
func (m *OutlinerModel) modePickerActive() bool { return m.modePicker != nil }
