package tui

type toggleUndoKind uint8

const (
	toggleUndoNone toggleUndoKind = iota
	toggleUndoDailyPlan
	toggleUndoQueue
)

func (m *AppModel) clearToggleRedoHistory() {
	m.lastDailyPlanToggleRedo = nil
	m.lastQueueToggleRedo = nil
	m.lastToggleRedoKind = toggleUndoNone
}

func (m *AppModel) nextUndoKind(excluding toggleUndoKind) toggleUndoKind {
	switch excluding {
	case toggleUndoNone:
		if m.lastDailyPlanToggle != nil {
			return toggleUndoDailyPlan
		}
		if m.lastQueueToggle != nil {
			return toggleUndoQueue
		}
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
		if m.lastDailyPlanToggleRedo != nil {
			return toggleUndoDailyPlan
		}
		if m.lastQueueToggleRedo != nil {
			return toggleUndoQueue
		}
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
