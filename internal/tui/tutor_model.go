package tui

import (
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
)

// TutorTopic represents a single topic in the tutor view.
type TutorTopic struct {
	Title    string
	Category string
	Content  string
}

// TutorModel implements the tutor view with a two-pane topic browser.
type TutorModel struct {
	topics         []TutorTopic
	filteredTopics []TutorTopic
	selectedIndex  int
	activePane     int // 0 = list, 1 = content
	viewport       viewport.Model
	searchInput    textinput.Model
	searchMode     bool
	searchQuery    string
	activeTab      int
	categories     []string
	listOffset     int
	width          int
	height         int
	ready          bool
}

// NewTutorModel creates a new tutor view.
func NewTutorModel() *TutorModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Search topics..."
	searchInput.CharLimit = 50
	searchInput.SetWidth(25)
	applyThemedTextInput(&searchInput)

	m := &TutorModel{
		topics:      tutorTopics(),
		searchInput: searchInput,
	}
	m.filteredTopics = m.topics
	m.categories = m.buildCategories()
	return m
}

func (m *TutorModel) buildCategories() []string {
	return []string{"Concepts", "CLI", "TUI"}
}
