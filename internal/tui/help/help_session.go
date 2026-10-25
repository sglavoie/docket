package help

import "github.com/sglavoie/docket/internal/tui/operation"

type browseState struct {
	query, category string
	offset          int
}

// Session remembers Help browsing separately for each named surface. It keeps
// no action catalog: every opening receives fresh items from its caller.
// The zero value is ready to use and nothing is written to disk.
type Session struct {
	current *HelpViewModel
	states  map[string]browseState
}

// Open builds a fresh viewer, restoring only its query, category, and scroll.
// Input focus and partially typed chords never survive closing Help.
func (s *Session) Open(items []operation.HelpItem, name string, width, height int) *HelpViewModel {
	if s.states == nil {
		s.states = make(map[string]browseState)
	}
	if previous := s.current; previous != nil {
		state := browseState{query: previous.searchQuery, offset: previous.viewport.YOffset()}
		if previous.activeTab > 0 && previous.activeTab <= len(previous.categories) {
			state.category = previous.categories[previous.activeTab-1]
		}
		s.states[previous.viewName] = state
	}

	state := s.states[name]
	m := NewHelpViewModel(items, name)
	m.searchQuery = state.query
	m.searchInput.SetValue(state.query)
	for i, category := range m.categories {
		if category == state.category {
			m.activeTab = i + 1
			break
		}
	}
	m.filterItems()
	m.SetSize(width, height)
	// The viewport clamps an old offset when items or terminal dimensions change.
	m.viewport.SetYOffset(state.offset)
	s.current = m
	return m
}
