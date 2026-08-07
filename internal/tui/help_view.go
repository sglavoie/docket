package tui

import (
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

// NewHelpViewModel creates a new help view model.
func NewHelpViewModel(items []HelpItem, sourceView PresentationID, viewName string) *HelpViewModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Search keybindings..."
	searchInput.CharLimit = 50
	searchInput.SetWidth(30)
	applyThemedTextInput(&searchInput)

	tabSearchInput := textinput.New()
	tabSearchInput.Placeholder = "Search tabs..."
	tabSearchInput.CharLimit = 30
	tabSearchInput.SetWidth(20)
	applyThemedTextInput(&tabSearchInput)

	m := &HelpViewModel{
		items:          items,
		sourceView:     sourceView,
		viewName:       viewName,
		searchMode:     false,
		searchInput:    searchInput,
		searchQuery:    "",
		ready:          false,
		tabSearchInput: tabSearchInput,
	}
	m.filteredItems = make([]filteredHelpItem, len(items))
	for i, item := range items {
		m.filteredItems[i] = filteredHelpItem{HelpItem: item}
	}
	m.categories = m.buildCategories()
	return m
}

// HasActiveSearch returns true if search mode is active or a search filter is applied.
func (m *HelpViewModel) HasActiveSearch() bool {
	return m.searchMode || m.searchQuery != "" || m.tabSearchMode
}

// HasTextInput returns true when help search or tab search is active.
func (m *HelpViewModel) HasTextInput() bool {
	return m.searchMode || m.tabSearchMode
}

// IsFullHeight returns true because the help view renders its own footer.
func (m *HelpViewModel) IsFullHeight() bool {
	return true
}

// ShouldCaptureEsc returns true when help search or tab search is active.
func (m *HelpViewModel) ShouldCaptureEsc() bool {
	return m.HasActiveSearch()
}

// SetSize sets the view dimensions.
func (m *HelpViewModel) innerWidth() int {
	return max(m.width-2, 0)
}

func (m *HelpViewModel) SetSize(width, height int) {
	m.width = width

	viewportWidth := m.innerWidth()
	if viewportWidth < 1 {
		viewportWidth = 1
	}

	searchHeight := 0
	if m.searchMode || m.searchQuery != "" {
		searchHeight = 2
	}
	tabSearchHeight := 0
	if m.tabSearchMode {
		tabSearchHeight = 2
	}
	titleHeight := 1
	statusBarHeight := 3 // blank line + divider + one browse-guidance row
	if m.searchMode || m.tabSearchMode {
		statusBarHeight++ // active input may use the two-row guidance budget
	}
	tabBarHeight := m.tabBarLines()
	viewportHeight := height - titleHeight - tabBarHeight - searchHeight - tabSearchHeight - statusBarHeight
	if viewportHeight < 5 {
		viewportHeight = 5
	}

	if !m.ready {
		m.viewport = viewport.New(viewport.WithWidth(viewportWidth), viewport.WithHeight(viewportHeight))
		m.viewport.YPosition = 0
		m.ready = true
	} else {
		m.viewport.SetWidth(viewportWidth)
		m.viewport.SetHeight(viewportHeight)
	}

	m.viewport.SetContent(m.renderContent())
}

func (m *HelpViewModel) Init() tea.Cmd {
	return nil
}

func (m *HelpViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyPressMsg:
		if m.searchMode {
			return m.updateSearchMode(msg)
		}

		if m.tabSearchMode {
			return m.updateTabSearchMode(msg)
		}

		switch msg.String() {
		case "?":
			return m, func() tea.Msg {
				return CloseHelpMsg{}
			}
		case "/":
			m.searchMode = true
			m.searchInput.Focus()
			return m, textinput.Blink
		case `\`:
			m.tabSearchMode = true
			m.tabSearchInput.SetValue("")
			m.tabSearchInput.Focus()
			return m, textinput.Blink
		case "backspace":
			needsRefresh := false
			if m.searchQuery != "" {
				m.searchQuery = ""
				m.searchInput.SetValue("")
				needsRefresh = true
			}
			if m.activeTab != 0 {
				m.activeTab = 0
				needsRefresh = true
			}
			if needsRefresh {
				m.filterItems()
				m.viewport.SetContent(m.renderContent())
				m.viewport.GotoTop()
			}
			return m, nil
		case "esc":
			if m.searchQuery != "" {
				m.searchQuery = ""
				m.searchInput.SetValue("")
				m.filterItems()
				m.viewport.SetContent(m.renderContent())
				m.viewport.GotoTop()
				return m, nil
			}
		case "shift+left":
			CycleTab(&m.activeTab, len(m.categories)+1, -1)
			m.filterItems()
			m.viewport.SetContent(m.renderContent())
			m.viewport.GotoTop()
			return m, nil
		case "shift+right":
			CycleTab(&m.activeTab, len(m.categories)+1, 1)
			m.filterItems()
			m.viewport.SetContent(m.renderContent())
			m.viewport.GotoTop()
			return m, nil
		case "up", "k":
			m.viewport.ScrollUp(1)
		case "down", "j":
			m.viewport.ScrollDown(1)
		case "g":
			m.viewport.GotoTop()
		case "G":
			m.viewport.GotoBottom()
		case "pgup", "ctrl+u":
			m.viewport.HalfPageUp()
		case "pgdown", "ctrl+d":
			m.viewport.HalfPageDown()
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *HelpViewModel) updateSearchMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searchMode = false
		m.searchInput.Blur()
		return m, nil
	case "enter", "/":
		m.searchMode = false
		m.searchInput.Blur()
		return m, nil
	case `\`:
		m.searchMode = false
		m.searchInput.Blur()
		m.tabSearchMode = true
		m.tabSearchInput.SetValue("")
		m.tabSearchInput.Focus()
		return m, textinput.Blink
	case "shift+left":
		CycleTab(&m.activeTab, len(m.categories)+1, -1)
		m.filterItems()
		m.viewport.SetContent(m.renderContent())
		m.viewport.GotoTop()
		return m, nil
	case "shift+right":
		CycleTab(&m.activeTab, len(m.categories)+1, 1)
		m.filterItems()
		m.viewport.SetContent(m.renderContent())
		m.viewport.GotoTop()
		return m, nil
	default:
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		m.searchQuery = m.searchInput.Value()
		m.filterItems()
		m.viewport.SetContent(m.renderContent())
		m.viewport.GotoTop()
		return m, cmd
	}
}

func (m *HelpViewModel) updateTabSearchMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter", `\`:
		m.tabSearchMode = false
		m.tabSearchInput.Blur()
		m.tabSearchInput.SetValue("")
		return m, nil
	case "/":
		m.tabSearchMode = false
		m.tabSearchInput.Blur()
		m.tabSearchInput.SetValue("")
		m.searchMode = true
		m.searchInput.Focus()
		return m, textinput.Blink
	default:
		var cmd tea.Cmd
		m.tabSearchInput, cmd = m.tabSearchInput.Update(msg)
		m.matchTab(m.tabSearchInput.Value())
		return m, cmd
	}
}
