package outliner

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/query"
)

// resetToGlobalAll returns the cross-project Outliner to a clean, unscoped
// global All root: it drops any project scope, clears every narrowing and ordering
// axis (search, saved criteria, common filters and manual sort), clears
// focus/selection, activates All, and parks the cursor at the top. It is the state
// Ctrl+Esc restores as the single global reset, so no advertised clean reset can
// leave the root silently narrowed by a stale filter, tag, or sort.
func (m *OutlinerModel) ResetToGlobalAll() {
	m.Confirm.Cancel()
	m.QuickEdit.Exit()
	m.closeMenuBar()
	m.Scope.clearStack()
	m.Scope.clearProject()
	m.dropParkedProjectFocus()
	m.Focus.clear()
	m.Focus.clearCross()
	// Restated rather than left to the selectOutlinerMode(ModeAll) below, which
	// also clears it: the reset names every axis it drops, the way it already
	// restates the focus-stack clears.
	m.clearSectionZoom()
	m.Folds.ResetAll()
	m.ReplaceFilterClauses(query.FilterQueryTaskFields(), nil)
	m.Sort.Reset()
	m.TreeSort.Reset()
	if m.MultiSelectMode {
		m.ExitMultiSelect()
		m.MultiSelectMode = false
	}
	m.clearSearchProjectionWithoutRestore()
	// The reset is replacing the whole projection, so the lens is dropped rather
	// than restored: restoring its baseline would reinstate the narrowing this
	// key exists to clear.
	m.ClearActiveLensWithoutRestore()
	m.Cursor = 0
	m.resetViewScrollOffset()
	m.SelectOutlinerMode(chrome.ModeAll)
	m.Population.Invalidate()
	m.RebuildRows()
	m.ClampCursor()
}

// FoldQuietLanesForLaunch seeds the folds a launch owes: All's Backlog and
// Someday lanes start folded, so the first screen leads with the work already
// in motion and each header still counts what its fold hides. It is a launch act rather than
// part of ResetToGlobalAll, so a reset, a reload, and Esc never reapply it, and
// it reads nothing from disk, so no fold is ever restored.
func (m *OutlinerModel) FoldQuietLanesForLaunch() {
	if m == nil {
		return
	}
	for _, group := range []models.WorkflowStateGroup{models.StateGroupBacklog, models.StateGroupSomeday} {
		m.Folds.setSection(stateGroupCollapsePrefix+string(group), true)
	}
	// The rebuild guard's fingerprint does not cover the fold maps, so the
	// population is invalidated to make the rebuild honor the new key.
	m.Population.Invalidate()
	m.RebuildRows()
	m.ClampCursor()
}

// resetToLaunchView returns the view to the state a lens-free launch produces:
// Active Projects, All, no query, no filters, no sort, no focus, and no lens
// applied. Every axis is taken away through the same call its own key uses, the
// way peelQueryReadoutAxis does, so the reset can never reach a state the
// ordinary grammar cannot.
//
// It reports which axes it found set, so the strip can name what it cleared, and
// whether the population owes a reload — the one thing this model cannot do for
// itself. The axes are read before any of them is written, because the unwinding
// calls restore captured frames that reinstate one another's state.
//
// Unlike resetToGlobalAll this is a reset of the view alone: the machine-local
// active-lens pointer is left untouched, so the lens is put down now and opens
// again at the next launch.
func (m *OutlinerModel) ResetToLaunchView() ([]string, bool) {
	if m == nil {
		return nil, false
	}
	cleared := make([]string, 0, 8)
	note := func(active bool, axis string) bool {
		if active {
			cleared = append(cleared, axis)
		}
		return active
	}
	// A lens a temporary search suspended is still a lens this key clears: it owns
	// no axis right now, but an Esc would put it back, so a reset that stayed
	// silent about it would report nothing to clear while still having work to do.
	note(m.HasActiveLens() || m.HasSuspendedLens(), "lens")
	note(m.HasSearchProjection(), "query")
	note(m.Focus.Active(), "focus")
	note(m.Focus.Zoom.Active(), "zoom")
	note(m.IsProjectScoped(), "scope")
	note(query.CommonFilterFacetCount(&m.Filter.CommonFilterState) > 0, "filters")
	reload := note(query.ProjectFilterFacetCount(&m.ProjectFilter) > 0, "project filters")
	note(m.Sort.Field != query.SortByDefault || m.TreeSort.Field != query.SortByDefault, "sort")
	// The mode is named rather than called "mode", so the status says what the
	// key left: `cleared Recent mode`, not a bare axis word.
	if mode, ok := chrome.OutlinerModeByID(m.PrimaryMode); ok {
		note(m.PrimaryMode != chrome.ModeAll, mode.Label+" mode")
	} else {
		note(m.PrimaryMode != chrome.ModeAll, "mode")
	}
	// The relationship trail is an axis of its own for exactly this reason: a hop
	// can land somewhere the launch view already describes, and every note above
	// would then report nothing to clear while an Esc still had a return to make.
	note(m.HasRelationTrail(), "relationship trail")
	if len(cleared) == 0 {
		return cleared, false
	}

	// The trail goes first and is discarded rather than walked: this key restores
	// the launch view, and returning through a hop would restore a view instead.
	m.ClearRelationTrail()
	// The lens goes first and goes without its baseline: restoring it would
	// reinstate the narrowing this key exists to clear.
	m.forgetActiveLensFrame()
	// The query goes next, so no ranked projection is left scored over the
	// population the axes below are about to replace.
	m.ClearSearch()
	// Focus and zoom unwind in the Esc cascade's own order. Focus is a stack and
	// its key pops one level, so it is popped until it is empty.
	for m.PopFocusLevel() {
	}
	m.ExitSectionZoom()
	// Scope restores the base frame captured at the first push, and that frame
	// carries a mode, a sort, and a zoom of its own, so it lands before the axes
	// below rather than after them.
	m.ClearProjectScope()
	// The two filter families are cleared separately for the same reason fx and
	// Fx are two keys: clearing one must never clear the other.
	m.ReplaceFilterClauses(query.FilterQueryTaskFields(), nil)
	m.ReplaceFilterClauses(FilterOperationFields(operation.OperationFilterProjectClear), nil)
	m.Sort.Reset()
	m.TreeSort.Reset()
	// Selecting the mode is the last write and owns the rebuild, so the rows are
	// built once, over every axis this reset has already put down.
	m.SelectOutlinerMode(chrome.ModeAll)
	// A read already in flight is carrying the position this reset has just
	// discarded. It keeps running — its projects are still the newest reading —
	// but it is re-aimed at the launch view, so landing cannot undo the key.
	m.rebaseReloadTargetAfterRelocation()
	return cleared, reload
}
