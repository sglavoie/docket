package tui

import (
	"strings"
	"unicode/utf8"
)

// outlinerSearchHistoryLimit bounds the ring. It is the same size as the undo
// ledger for the same reason: far more than a working session reaches back for,
// and small enough that the oldest entries are never worth walking to.
const outlinerSearchHistoryLimit = 50

// outlinerSearchHistory is the session-only ring of submitted slash queries.
//
// entries is oldest first, so the newest query is the last element and Up walks
// backward from the end. index is the recall position and index == len(entries)
// means no walk is in progress, which makes the zero value inert: a
// struct-literal test model recalls nothing rather than panicking. draft holds
// the text the user had typed when the walk started, so stepping back down past
// the newest entry returns what they were writing rather than an empty input.
type outlinerSearchHistory struct {
	entries []string
	index   int
	draft   string
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
}

// older steps one entry back and reports the query to install. The first step of
// a walk sets the caller's current text aside as the draft. The oldest entry is
// a floor: pressing Up there reports nothing rather than wrapping, because a
// wrap would silently hand back the newest query as if it were older.
func (h *outlinerSearchHistory) older(current string) (string, bool) {
	if h == nil || len(h.entries) == 0 {
		return "", false
	}
	if h.index >= len(h.entries) {
		h.draft = current
		h.index = len(h.entries)
	}
	if h.index == 0 {
		return "", false
	}
	h.index--
	return h.entries[h.index], true
}

// newer steps one entry forward, ending on the draft the walk set aside. Down
// outside a walk reports nothing, so it never clears an input the user typed.
func (h *outlinerSearchHistory) newer() (string, bool) {
	if h == nil || h.index >= len(h.entries) {
		return "", false
	}
	h.index++
	if h.index == len(h.entries) {
		draft := h.draft
		h.draft = ""
		return draft, true
	}
	return h.entries[h.index], true
}

// recordSearchHistoryEntry retains a query the user submitted with Enter.
func (m *OutlinerModel) recordSearchHistoryEntry(query string) {
	if m == nil {
		return
	}
	m.searchHistory.record(query)
}

// recallSearchHistory claims Up and Down inside the slash input, and only there.
// It is called from the key cascade below the suggestion pane's own claim, so an
// open pane keeps both keys for its cursor and one Esc gives them back to the
// walk. Any other key, an exhausted walk, and a Down outside a walk all return
// false and fall through to the ordinary input handling, which ignores them.
func (m *OutlinerModel) recallSearchHistory(key string) bool {
	if m == nil || !m.IsSearchMode() {
		return false
	}
	var (
		value string
		ok    bool
	)
	switch key {
	case "up":
		value, ok = m.searchHistory.older(m.SearchInputValue())
	case "down":
		value, ok = m.searchHistory.newer()
	default:
		return false
	}
	if !ok {
		return false
	}
	m.applyRecalledSearchQuery(value)
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
	m.searchAutocomplete.clear()
	rebuildOutlinerPreservingCursor(m)
}

// hasSearchHistory reports whether anything is there to recall, which is what
// decides if the search footer advertises the walk.
func (m *OutlinerModel) hasSearchHistory() bool {
	return m != nil && len(m.searchHistory.entries) > 0
}
