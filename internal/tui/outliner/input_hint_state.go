package outliner

import (
	"slices"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/query"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// PendingChordHintDecorator shares the render's dispatch context across all
// entries, rather than rebuilding the selection and project scope for each key.
func (m *OutlinerModel) PendingChordHintDecorator(ctx operation.CommandContext) operation.ChordHintDecorator {
	return func(path []string, hint operation.ChordHint) operation.ChordHint {
		if m != nil && m.OperationRegistry != nil {
			binding, _, resolution := m.OperationRegistry.InputBindingForSequence(operation.InputStrategySimplified, path, ctx)
			if resolution == operation.ChordTerminal {
				hint.Dimmed = m.inputBindingIsIneffective(binding)
			}
		}
		return m.DecoratePendingChordHint(path, hint)
	}
}

// inputBindingIsIneffective reads live state solely for hint emphasis. It does
// not gate dispatch: an inert shortcut remains available and keeps its place.
// Value pickers and toggles remain useful even when their current value is set.
func (m *OutlinerModel) inputBindingIsIneffective(binding operation.OperationKeyBinding) bool {
	id := binding.OperationID
	if mode, ok := operation.GotoModeOperations[id]; ok {
		// Recent doubles as refresh, and reselecting a zoomed mode exits its zoom.
		return mode != chrome.ModeRecent && mode == m.ActiveOutlinerMode() && !m.Focus.Zoom.Active()
	}
	switch id {
	case operation.OperationCoreConflicts:
		return m.outstandingConflicts == nil || m.outstandingConflicts() <= 0
	case operation.OperationCoreGotoTaskProject:
		task, project := m.GetTaskRef()
		return task == nil || project == nil || (m.IsProjectScoped() && m.Scope.ProjectID == project.ID &&
			m.ActiveOutlinerMode() == chrome.ModeAll && !m.Focus.Zoom.Active() && !m.HasSearchProjection())
	case operation.OperationCoreGotoRelation:
		task, project := m.GetTaskRef()
		return task == nil || project == nil || len(m.relationCandidatesFor(task, taskref.For(task, project))) == 0
	case operation.OperationCoreCursorTop, operation.OperationCoreCursorUp, operation.OperationCoreCursorPageUp:
		return m.Cursor <= 0 || len(m.Rows) == 0
	case operation.OperationCoreCursorBottom, operation.OperationCoreCursorDown, operation.OperationCoreCursorPageDown:
		return m.Cursor >= len(m.Rows)-1
	case operation.OperationCoreScrollCursorTop, operation.OperationCoreScrollCursorMiddle, operation.OperationCoreScrollCursorBottom:
		return m.CursorScrollOffset(id) == m.ViewScrollOffset
	case operation.OperationFilterClear:
		return query.CommonFilterFacetCount(&m.Filter.CommonFilterState) == 0
	case operation.OperationFilterProjectClear:
		return !query.HasActiveProjectFilters(&m.ProjectFilter)
	case operation.OperationFilterPriority:
		priority, ok := binding.PresetValue.(int)
		return binding.HasPresetValue && ok && priority == m.Filter.Priority
	case operation.OperationSortClear:
		return m.Sort.Field == query.SortByDefault && m.TreeSort.Field == query.SortByDefault
	case operation.OperationTaskSetPriority, operation.OperationTaskSetState,
		operation.OperationCoreBacklog, operation.OperationCoreToday, operation.OperationCoreNext:
		return m.taskBindingIsIneffective(binding)
	default:
		return false
	}
}

// CursorScrollOffset is the offset both the z chord and its hints project. It
// measures against the window that offset would draw, not the one the key was
// pressed on: an anchor that scrolls a lane's header away costs the list its
// first screen row, so the cursor is placed among the rows left beneath the
// pinned one. The full height is tried first and the pinned one only when the
// window authority says that offset pins.
func (m *OutlinerModel) CursorScrollOffset(id operation.OperationID) int {
	if start, end := m.VisibleRowWindow(); start >= end {
		return m.ViewScrollOffset
	}
	visibleRows := m.LastViewport.VisibleRows
	if visibleRows <= 0 {
		visibleRows = m.ContentHeight()
	}
	offset := m.ViewScrollOffset
	for _, height := range []int{visibleRows, visibleRows - 1} {
		if height < 1 {
			break
		}
		window, settled := m.pinnedContextWindowAt(m.cursorAnchorOffset(id, height), visibleRows)
		offset = settled
		if !window.pinned {
			break
		}
	}
	return offset
}

// cursorAnchorOffset seats the cursor in a window of height document rows,
// clamped at either end of the list.
func (m *OutlinerModel) cursorAnchorOffset(id operation.OperationID, height int) int {
	desired := m.Cursor
	switch id {
	case operation.OperationCoreScrollCursorMiddle:
		desired -= (height - 1) / 2
	case operation.OperationCoreScrollCursorBottom:
		desired -= height - 1
	default: // Top is the unshifted cursor position.
	}
	return min(max(desired, 0), max(len(m.Rows)-height, 0))
}

func (m *OutlinerModel) taskBindingIsIneffective(binding operation.OperationKeyBinding) bool {
	identities := m.SelectedTaskIdentities()
	if !m.IsMultiSelectActive() {
		identity, ok := m.cursorTaskIdentity()
		if !ok {
			return true
		}
		task, project := m.TaskAndProjectForIdentity(identity)
		if task == nil || project == nil {
			return true
		}
		return !m.taskBindingChangesTask(binding, task, project)
	}
	changes := false
	for _, identity := range identities {
		task, project := m.TaskAndProjectForIdentity(identity)
		if task == nil || project == nil {
			return true
		}
		changed := m.taskBindingChangesTask(binding, task, project)
		// State transitions validate the entire selection before applying any
		// member. Even one already-set/forbidden state refuses the whole batch.
		if binding.OperationID == operation.OperationTaskSetState && !changed {
			return true
		}
		if binding.OperationID != operation.OperationTaskSetPriority && task.IsTerminalState() &&
			binding.OperationID != operation.OperationTaskSetState {
			return true
		}
		changes = changes || changed
	}
	return !changes
}

func (m *OutlinerModel) taskBindingChangesTask(binding operation.OperationKeyBinding, task *models.Task, project *models.Project) bool {
	switch binding.OperationID {
	case operation.OperationTaskSetPriority:
		priority, ok := binding.PresetValue.(int)
		return !binding.HasPresetValue || !ok || task.Priority != priority
	case operation.OperationTaskSetState:
		state, ok := binding.PresetValue.(models.WorkflowState)
		if !binding.HasPresetValue || !ok {
			return true
		}
		if models.CanTransitionState(task.State(), state) != "" {
			return false
		}
		if state.IsTerminal() {
			for _, child := range project.Tasks {
				if child != nil && child.ParentID != nil && *child.ParentID == task.ID && !child.IsTerminalState() {
					return false
				}
			}
		}
		return true
	case operation.OperationCoreToday, operation.OperationCoreNext:
		return !task.IsTerminalState()
	case operation.OperationCoreBacklog:
		if task.IsTerminalState() {
			return false
		}
		if task.State() != models.StateBacklog {
			return true
		}
		if m.Runtime != nil {
			locator := config.ProjectTaskLocator{ProjectID: project.ID, TaskID: task.ID}
			planning := m.Runtime.Planning()
			return slices.Contains(planning.GetToday(), locator) || slices.Contains(planning.GetNext(), locator)
		}
	default:
		return false
	}
	return false
}
