package help

import (
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// NewHelpViewModel creates a new help view model.
func NewHelpViewModel(items []operation.HelpItem, viewName string) *HelpViewModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Search keybindings..."
	searchInput.CharLimit = 50
	searchInput.SetWidth(30)
	theme.ApplyThemedTextInput(&searchInput)

	tabSearchInput := textinput.New()
	tabSearchInput.Placeholder = "Search tabs..."
	tabSearchInput.CharLimit = 30
	tabSearchInput.SetWidth(20)
	theme.ApplyThemedTextInput(&tabSearchInput)

	m := &HelpViewModel{
		items:          items,
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
	m.categories = m.buildSectionCategories()
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

// cycleTab advances a tab index by delta with wrapping.
// delta is typically +1 (right) or -1 (left).
//
// It lives here rather than in listsurface because the help viewer is its only
// caller: nothing on a list surface cycles a tab.
func cycleTab(current *int, count, delta int) {
	if count <= 1 {
		return
	}
	*current = ((*current+delta)%count + count) % count
}

// cycleCategoryTab moves the active tab by delta and refreshes the filtered
// content beneath it.
func (m *HelpViewModel) cycleCategoryTab(delta int) {
	cycleTab(&m.activeTab, len(m.categories)+1, delta)
	m.filterItems()
	m.viewport.SetContent(m.renderContent())
	m.viewport.GotoTop()
}

func (m *HelpViewModel) innerWidth() int {
	return max(m.width-2, 0)
}

// SetSize sets the view dimensions.
func (m *HelpViewModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	viewportWidth := m.innerWidth()
	if viewportWidth < 1 {
		viewportWidth = 1
	}

	statusBarHeight := lipgloss.Height(m.renderHelpFooter())
	// Counts can wrap the tabs onto extra rows, and themed input labels can
	// wrap too. Measure the rendered header, excluding its trailing newline,
	// rather than assuming either search field always occupies two rows.
	headerHeight := lipgloss.Height(m.renderHelpHeader(m.renderHelpTabBar())) - 1
	viewportHeight := max(height-headerHeight-statusBarHeight, 1)

	if !m.ready {
		m.viewport = viewport.New(viewport.WithWidth(viewportWidth), viewport.WithHeight(viewportHeight))
		// Update owns every scrolling key; the viewport's own bindings would
		// scroll a second time and answer keys the footer never names.
		m.viewport.KeyMap = viewport.KeyMap{}
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
		// Search edits and category changes can change the header height without
		// a terminal resize. Reflow after the new query and counts are installed.
		defer func() {
			if m.ready {
				m.SetSize(m.width, m.height)
			}
		}()
		if msg.String() == "tab" && !m.tabSearchMode && m.otherCategoryMatches > 0 {
			m.pendingG = false
			m.activeTab = 0
			m.filterItems()
			m.viewport.SetContent(m.renderContent())
			m.viewport.GotoTop()
			return m, nil
		}
		if m.searchMode {
			return m.updateSearchMode(msg)
		}

		if m.tabSearchMode {
			return m.updateTabSearchMode(msg)
		}

		if msg.String() != "g" {
			m.pendingG = false
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
		case "left", "h":
			m.cycleCategoryTab(-1)
			return m, nil
		case "right", "l":
			m.cycleCategoryTab(1)
			return m, nil
		case "up", "k":
			m.viewport.ScrollUp(1)
			return m, nil
		case "down", "j":
			m.viewport.ScrollDown(1)
			return m, nil
		case "g":
			// Cursor-top is the gg chord everywhere else, so a lone g only arms it.
			if m.pendingG {
				m.pendingG = false
				m.viewport.GotoTop()
			} else {
				m.pendingG = true
			}
			return m, nil
		case "home":
			m.viewport.GotoTop()
			return m, nil
		case "G", "end":
			m.viewport.GotoBottom()
			return m, nil
		case "pgup", "ctrl+u":
			m.viewport.HalfPageUp()
			return m, nil
		case "pgdown", "ctrl+d":
			m.viewport.HalfPageDown()
			return m, nil
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
		m.cycleCategoryTab(-1)
		return m, nil
	case "shift+right":
		m.cycleCategoryTab(1)
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
