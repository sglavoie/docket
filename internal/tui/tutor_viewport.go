package tui

import "charm.land/bubbles/v2/viewport"

func (m *TutorModel) GetHelpItems() []HelpItem {
	return helpItemsFromKeyMap(m.buildKeyMap())
}

func (m *TutorModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	vpWidth := m.contentWidth() - 1 // 1 char padding in split pane
	tabBarHeight := 2
	searchHeight := 2
	footerHeight := 2
	vpHeight := height - tabBarHeight - searchHeight - footerHeight - 2 // -2: 1 for spacing, 1 for focus bar

	if vpWidth < 30 {
		vpWidth = 30
	}
	if vpHeight < 5 {
		vpHeight = 5
	}

	if !m.ready {
		m.viewport = viewport.New(viewport.WithWidth(vpWidth), viewport.WithHeight(vpHeight))
		m.viewport.YPosition = 0
		m.ready = true
	} else {
		m.viewport.SetWidth(vpWidth)
		m.viewport.SetHeight(vpHeight)
	}

	m.viewport.SetContent(m.renderTopicContent())
}
