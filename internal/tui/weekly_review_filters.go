package tui

// refreshAndClamp reloads data and clamps cursor after filter changes.
func (m *WeeklyReviewModel) refreshAndClamp() { m.loadData(); m.clampCursor() }
