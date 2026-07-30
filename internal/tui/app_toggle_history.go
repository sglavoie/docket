package tui

import "time"

type toggleUndoKind uint8

const (
	toggleUndoNone toggleUndoKind = iota
	toggleUndoToday
	toggleUndoQueue
)

func (m *AppModel) clearToggleRedoHistory() {
	m.lastTodayToggleRedo = nil
	m.lastQueueToggleRedo = nil
	m.lastTodayRedoAt = time.Time{}
	m.lastQueueRedoAt = time.Time{}
	m.lastToggleRedoKind = toggleUndoNone
}

func (m *AppModel) clearToggleHistory() {
	m.lastTodayToggle = nil
	m.lastQueueToggle = nil
	m.lastTodayToggleAt = time.Time{}
	m.lastQueueToggleAt = time.Time{}
	m.lastToggleUndoKind = toggleUndoNone
	m.clearToggleRedoHistory()
}

func (m *AppModel) nextUndoKind(excluding toggleUndoKind) toggleUndoKind {
	switch excluding {
	case toggleUndoNone:
		return newerToggleKind(
			toggleUndoToday, m.lastTodayToggle != nil, m.lastTodayToggleAt,
			toggleUndoQueue, m.lastQueueToggle != nil, m.lastQueueToggleAt,
		)
	case toggleUndoToday:
		if m.lastQueueToggle != nil {
			return toggleUndoQueue
		}
	case toggleUndoQueue:
		if m.lastTodayToggle != nil {
			return toggleUndoToday
		}
	}
	return toggleUndoNone
}

func (m *AppModel) nextRedoKind(excluding toggleUndoKind) toggleUndoKind {
	switch excluding {
	case toggleUndoNone:
		return newerToggleKind(
			toggleUndoToday, m.lastTodayToggleRedo != nil, m.lastTodayRedoAt,
			toggleUndoQueue, m.lastQueueToggleRedo != nil, m.lastQueueRedoAt,
		)
	case toggleUndoToday:
		if m.lastQueueToggleRedo != nil {
			return toggleUndoQueue
		}
	case toggleUndoQueue:
		if m.lastTodayToggleRedo != nil {
			return toggleUndoToday
		}
	}
	return toggleUndoNone
}

func newerToggleKind(a toggleUndoKind, hasA bool, atA time.Time, b toggleUndoKind, hasB bool, atB time.Time) toggleUndoKind {
	if !hasA && !hasB {
		return toggleUndoNone
	}
	if hasA && !hasB {
		return a
	}
	if !hasA && hasB {
		return b
	}
	if atB.After(atA) {
		return b
	}
	return a
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
