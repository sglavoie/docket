package tui

import "time"

type toggleUndoKind uint8

const (
	toggleUndoNone toggleUndoKind = iota
	toggleUndoDailyPlan
	toggleUndoQueue
)

func (m *AppModel) clearToggleRedoHistory() {
	m.lastDailyPlanToggleRedo = nil
	m.lastQueueToggleRedo = nil
	m.lastDailyPlanRedoAt = time.Time{}
	m.lastQueueRedoAt = time.Time{}
	m.lastToggleRedoKind = toggleUndoNone
}

func (m *AppModel) clearToggleHistory() {
	m.lastDailyPlanToggle = nil
	m.lastQueueToggle = nil
	m.lastDailyPlanToggleAt = time.Time{}
	m.lastQueueToggleAt = time.Time{}
	m.lastToggleUndoKind = toggleUndoNone
	m.clearToggleRedoHistory()
}

func (m *AppModel) nextUndoKind(excluding toggleUndoKind) toggleUndoKind {
	switch excluding {
	case toggleUndoNone:
		return newerToggleKind(
			toggleUndoDailyPlan, m.lastDailyPlanToggle != nil, m.lastDailyPlanToggleAt,
			toggleUndoQueue, m.lastQueueToggle != nil, m.lastQueueToggleAt,
		)
	case toggleUndoDailyPlan:
		if m.lastQueueToggle != nil {
			return toggleUndoQueue
		}
	case toggleUndoQueue:
		if m.lastDailyPlanToggle != nil {
			return toggleUndoDailyPlan
		}
	}
	return toggleUndoNone
}

func (m *AppModel) nextRedoKind(excluding toggleUndoKind) toggleUndoKind {
	switch excluding {
	case toggleUndoNone:
		return newerToggleKind(
			toggleUndoDailyPlan, m.lastDailyPlanToggleRedo != nil, m.lastDailyPlanRedoAt,
			toggleUndoQueue, m.lastQueueToggleRedo != nil, m.lastQueueRedoAt,
		)
	case toggleUndoDailyPlan:
		if m.lastQueueToggleRedo != nil {
			return toggleUndoQueue
		}
	case toggleUndoQueue:
		if m.lastDailyPlanToggleRedo != nil {
			return toggleUndoDailyPlan
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
	case toggleUndoDailyPlan:
		return m.lastDailyPlanToggleAt
	case toggleUndoQueue:
		return m.lastQueueToggleAt
	default:
		return time.Time{}
	}
}

func (m *AppModel) toggleRedoTimestamp(kind toggleUndoKind) time.Time {
	switch kind {
	case toggleUndoDailyPlan:
		return m.lastDailyPlanRedoAt
	case toggleUndoQueue:
		return m.lastQueueRedoAt
	default:
		return time.Time{}
	}
}
