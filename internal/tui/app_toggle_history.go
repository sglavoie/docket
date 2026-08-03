package tui

import "time"

type toggleUndoKind uint8

const (
	toggleUndoNone toggleUndoKind = iota
	toggleUndoToday
	toggleUndoQueue
)

func (m *AppModel) clearToggleRedoHistory() {
	m.lastTodayRedoAt = time.Time{}
	m.lastQueueRedoAt = time.Time{}
	m.lastToggleRedoKind = toggleUndoNone
}

func (m *AppModel) clearToggleHistory() {
	m.lastTodayToggleAt = time.Time{}
	m.lastQueueToggleAt = time.Time{}
	m.lastToggleUndoKind = toggleUndoNone
	m.clearToggleRedoHistory()
}

func (m *AppModel) toggleUndoTimestamp(kind toggleUndoKind) time.Time {
	switch kind {
	case toggleUndoToday:
		return m.lastTodayToggleAt
	case toggleUndoQueue:
		return m.lastQueueToggleAt
	default:
		return time.Time{}
	}
}

func (m *AppModel) toggleRedoTimestamp(kind toggleUndoKind) time.Time {
	switch kind {
	case toggleUndoToday:
		return m.lastTodayRedoAt
	case toggleUndoQueue:
		return m.lastQueueRedoAt
	default:
		return time.Time{}
	}
}
