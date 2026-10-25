package outliner

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/fuzzy"
	"github.com/sglavoie/docket/internal/tui/grammar"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// outlinerSearchHistoryLimit bounds the ring. It is the same size as the undo
// ledger for the same reason: far more than a working session reaches back for,
// and small enough that the oldest entries are never worth walking to.
const outlinerSearchHistoryLimit = config.SearchHistoryLimit

// outlinerSearchHistory is the bounded ring of submitted slash queries.
//
// entries is oldest first, so the newest query is the last element and Up walks
// backward from the end. index is the recall position and index == len(entries)
// means no walk is in progress, which makes the zero value inert: a
// struct-literal test model recalls nothing rather than panicking. draft holds
// the text the user had typed when the walk started, so stepping back down past
// the newest entry returns what they were writing rather than an empty input.
type outlinerSearchHistory struct {
	wired       bool
	persist     func([]string) error
	entries     []string
	index       int
	draft       string
	draftCursor int    // rune position to restore when returning to the draft
	notice      string // local recall feedback, never persisted or added to message history
}

// record appends a submitted query. An earlier identical entry is removed rather
// than left in place, so the ring holds each distinct query once and a query
// reused all afternoon stays one press away instead of filling the walk with
// copies of itself. Submitting also ends any walk in progress: the query now in
// the input is the newest one, so the next Up starts from it.
func (h *outlinerSearchHistory) record(query string) {
	if h == nil {
		return
	}
	h.resetRecall()
	query = strings.TrimSpace(query)
	if query == "" {
		return
	}
	for i, entry := range h.entries {
		if entry == query {
			h.entries = append(h.entries[:i], h.entries[i+1:]...)
			break
		}
	}
	h.entries = append(h.entries, query)
	if len(h.entries) > outlinerSearchHistoryLimit {
		h.entries = append([]string(nil), h.entries[len(h.entries)-outlinerSearchHistoryLimit:]...)
	}
	h.index = len(h.entries)
}

// resetRecall ends a walk without touching the entries. Opening the input and
// typing into it both land here, so a walk lasts exactly as long as the user
// keeps pressing Up and Down.
func (h *outlinerSearchHistory) resetRecall() {
	if h == nil {
		return
	}
	h.index = len(h.entries)
	h.draft = ""
	h.draftCursor = 0
	h.notice = ""
}

// older steps one entry back and reports the query to install. The first step of
// a walk sets the caller's current text aside as both draft and smart-case prefix.
// Only entries beginning with that prefix are visited in either direction. The oldest entry is
// a floor: pressing Up there reports nothing rather than wrapping, because a
// wrap would silently hand back the newest query as if it were older.
func (h *outlinerSearchHistory) older(current string, cursor int) (string, bool) {
	if h == nil {
		return "", false
	}
	if len(h.entries) == 0 {
		h.notice = "No search history yet"
		return "", false
	}
	if h.index >= len(h.entries) {
		h.draft = current
		h.draftCursor = cursor
		h.index = len(h.entries)
	}
	for i := h.index - 1; i >= 0; i-- {
		if h.matchesPrefix(h.entries[i]) {
			h.index = i
			h.notice = h.positionLabel()
			return h.entries[i], true
		}
	}
	h.notice = "No history matches this prefix"
	if h.index < len(h.entries) {
		h.notice = "Oldest matching query"
	}
	return "", false
}

func (h *outlinerSearchHistory) positionLabel() string {
	position, total := 0, 0
	for i := len(h.entries) - 1; i >= 0; i-- {
		if h.matchesPrefix(h.entries[i]) {
			total++
			if i == h.index {
				position = total
			}
		}
	}
	return fmt.Sprintf("History %d of %d", position, total)
}

func (h *outlinerSearchHistory) matchesPrefix(entry string) bool {
	if fuzzy.CaseSensitiveQuery(h.draft) {
		return strings.HasPrefix(entry, h.draft)
	}
	return strings.HasPrefix(strings.ToLower(entry), strings.ToLower(h.draft))
}

// newer steps one entry forward, ending on the draft the walk set aside. Down
// outside a walk reports nothing, so it never clears an input the user typed.
func (h *outlinerSearchHistory) newer() (string, bool) {
	if h == nil || h.index >= len(h.entries) {
		return "", false
	}
	for i := h.index + 1; i < len(h.entries); i++ {
		if h.matchesPrefix(h.entries[i]) {
			h.index = i
			h.notice = h.positionLabel()
			return h.entries[i], true
		}
	}
	draft := h.draft
	h.resetRecall()
	return draft, true
}

// recordSearchHistoryEntry retains a query the user submitted with Enter.
func (m *OutlinerModel) recordSearchHistoryEntry(query string) error {
	if m == nil {
		return nil
	}
	m.Search.history.record(query)
	if m.Search.history.persist != nil && strings.TrimSpace(query) != "" {
		return m.Search.history.persist(slices.Clone(m.Search.history.entries))
	}
	return nil
}

// ConfigureSearchHistory loads once, even if the shell rewires this view later.
// A nil persistence seam keeps standalone views usable without a runtime.
func (m *OutlinerModel) ConfigureSearchHistory(entries []string, persist func([]string) error) {
	h := &m.Search.history
	if h.wired {
		return
	}
	for _, query := range entries {
		h.record(query)
	}
	h.resetRecall()
	h.persist, h.wired = persist, true
}

// ClearSearchHistory publishes the clear before forgetting local entries. A
// failed write leaves recall intact, and neither path changes the active query.
func (m *OutlinerModel) ClearSearchHistory() error {
	h := &m.Search.history
	if h.persist != nil {
		if err := h.persist(nil); err != nil {
			return err
		}
	}
	h.entries = nil
	h.resetRecall()
	return nil
}

// recallSearchHistory claims Up and Down inside the slash input, and only there.
// It is called from the key cascade below the suggestion pane's own claim, so an
// open pane keeps both keys for its cursor and one Esc gives them back to the
// walk. Boundary presses are consumed without changing the input; every other
// key falls through to ordinary input handling.
func (m *OutlinerModel) recallSearchHistory(key string) bool {
	if m == nil || !m.IsSearchMode() {
		return false
	}
	var (
		value string
		ok    bool
	)
	draftCursor := m.Search.history.draftCursor
	switch key {
	case "up":
		value, ok = m.Search.history.older(m.SearchInputValue(), m.SearchInputCursor())
	case "down":
		value, ok = m.Search.history.newer()
	default:
		return false
	}
	if !ok {
		// Claim boundary presses too: they must not reset the walk or its notice
		// through the ordinary text-input path.
		return true
	}
	m.applyRecalledSearchQuery(value)
	if key == "down" && m.Search.history.index == len(m.Search.history.entries) {
		m.SetSearchInputCursor(draftCursor)
	}
	return true
}

// applyRecalledSearchQuery installs a recalled query exactly as typing it would:
// the same live query update and the same rebuild the ordinary input path makes,
// with the caret at the end so the next character extends it. The suggestion
// pane is cleared rather than refreshed, because a recalled query replaces the
// token the pane was describing whole, and leaving Up and Down with the pane
// would end the walk on its first step.
func (m *OutlinerModel) applyRecalledSearchQuery(value string) {
	m.SetSearchInputValue(value)
	m.SetSearchInputCursor(utf8.RuneCountInString(value))
	m.BaseSearchableView.SetSearchQuery(value)
	m.Search.Autocomplete.clear()
	m.Search.queued = m.rebuildLiveSearch()
}

// hasSearchHistory reports whether anything is there to recall, which is what
// decides if the search footer advertises the walk.
func (m *OutlinerModel) hasSearchHistory() bool {
	return m != nil && len(m.Search.history.entries) > 0
}

// The filter is separate from the live search input: canceling changes neither
// its text nor its caret, and choosing a query never submits or records it.
type searchHistoryPicker struct {
	input   textinput.Model
	choices grammar.AutocompleteState
}

func (m *OutlinerModel) openSearchHistoryPicker() tea.Cmd {
	p := &searchHistoryPicker{input: textinput.New()}
	p.input.Prompt = ""
	p.input.Placeholder = "Find a saved query…"
	p.input.CharLimit = 4000
	p.input.SetValue(m.SearchInputValue())
	m.Search.Autocomplete.clear()
	m.Search.historyPicker = p
	m.refreshSearchHistoryPicker()
	return p.input.Focus()
}

func (m *OutlinerModel) refreshSearchHistoryPicker() {
	p := m.Search.historyPicker
	terms := strings.Fields(strings.ToLower(p.input.Value()))
	var items []grammar.ACSuggestion
	for i := len(m.Search.history.entries) - 1; i >= 0; i-- {
		entry := m.Search.history.entries[i]
		match := true
		for _, term := range terms {
			if !strings.Contains(strings.ToLower(entry), term) {
				match = false
				break
			}
		}
		if match {
			items = append(items, grammar.ACSuggestion{Display: entry, Value: entry})
		}
	}
	p.choices.UpdateSuggestions(grammar.ACModeTag, 0, items)
}

// Forget the selected query only after its durable history has been saved.
// The picker filter, live search draft and accepted projection remain intact.
func (m *OutlinerModel) forgetSelectedSearchHistory() tea.Cmd {
	p := m.Search.historyPicker
	if !p.choices.IsActive() {
		return nil
	}
	value := p.choices.Selected().Value
	h := &m.Search.history
	entries := slices.DeleteFunc(slices.Clone(h.entries), func(entry string) bool { return entry == value })
	if h.persist != nil {
		if err := h.persist(entries); err != nil {
			return status.ErrorCmd("Search history could not be updated: %v", err)
		}
	}
	h.entries = entries
	h.resetRecall()
	m.refreshSearchHistoryPicker()
	return func() tea.Msg { return status.InfoStatusMsg("Query forgotten from search history") }
}

func (m *OutlinerModel) updateSearchHistoryPicker(msg tea.Msg) tea.Cmd {
	p := m.Search.historyPicker
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "ctrl+x":
			return m.forgetSelectedSearchHistory()
		case "esc", "ctrl+r":
			m.Search.historyPicker = nil
			return nil
		case "enter", "tab":
			if p.choices.IsActive() {
				value := p.choices.Selected().Value
				m.Search.historyPicker = nil
				m.Search.history.resetRecall()
				m.applyRecalledSearchQuery(value)
			}
			return m.takeSearchCommand()
		case "up", "down", "ctrl+k", "ctrl+j", "ctrl+p", "ctrl+n":
			p.choices.HandleKey(key.String(), nil)
			return nil
		}
	}
	before := p.input.Value()
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	if before != p.input.Value() {
		m.refreshSearchHistoryPicker()
	}
	return cmd
}

func (m *OutlinerModel) renderSearchHistoryPicker(width int) string {
	p := m.Search.historyPicker
	if !p.choices.IsActive() {
		label := "No history matches"
		if !m.hasSearchHistory() {
			label = "No search history yet"
		}
		return textfit.TruncateToVisualWidth(theme.MutedStyle.Render(label), max(width, 1))
	}
	rendered := p.choices.RenderDropdownWithLimit(outlinerSearchDropdownIndent, m.searchAutocompleteRowBudget())
	lines := strings.Split(rendered, "\n")
	for i, line := range lines {
		lines[i] = textfit.TruncateToVisualWidth(line, max(width, 1))
	}
	return strings.Join(lines, "\n")
}
