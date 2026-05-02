package tui

// innerWidth returns the available width minus padding.
func (m *TutorModel) innerWidth() int {
	w := max(
		// 1 char padding each side
		m.width-2, 40)
	return w
}

// listWidth returns the width of the topic list pane.
func (m *TutorModel) listWidth() int {
	lw := min(max(m.innerWidth()*30/100, 18), 30)
	return lw
}

// contentWidth returns the width of the content pane.
func (m *TutorModel) contentWidth() int {
	return m.innerWidth() - m.listWidth() - 1 // -1 for divider
}
