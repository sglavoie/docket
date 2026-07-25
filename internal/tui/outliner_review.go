package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// reviewModeKind selects which review surface the Outliner renders as a
// computed mode. reviewNone means no review mode is active and the normal
// perspective/smart-focus/work path runs unchanged.
type reviewModeKind uint8

const (
	reviewNone reviewModeKind = iota
	reviewDaily
	reviewActivity
)

// reviewCollapseKeyPrefix marks a section-header CollapseKey as belonging to a
// review mode so the fold routing reads/writes reviewCollapsedSections instead
// of the persisted collapsedSections map.
const reviewCollapseKeyPrefix = "_rv:"

// reviewCollapseKey builds the CollapseKey sentinel for one review section.
// The kind is embedded so Daily Review and Activity headers with the same
// index never share fold state.
func reviewCollapseKey(kind reviewModeKind, idx int) string {
	return fmt.Sprintf("%s%d:%d", reviewCollapseKeyPrefix, kind, idx)
}

// isReviewCollapseKey reports whether a CollapseKey belongs to a review section.
func isReviewCollapseKey(key string) bool {
	return strings.HasPrefix(key, reviewCollapseKeyPrefix)
}

// reviewModeActive reports whether either review computed mode is selected.
func (m *OutlinerModel) reviewModeActive() bool {
	return m.reviewMode != reviewNone
}

// applyReviewMode switches into the given review computed mode, clearing the
// other computed modes so the review row builder owns the row list. Cursor and
// scroll reset to the top like the other mode transitions.
func (m *OutlinerModel) applyReviewMode(kind reviewModeKind) {
	m.smartFocus = nil
	m.crossPriorityMode = false
	m.workNextMode = false
	m.workBacklogMode = false
	m.activeSavedFilter = ""
	m.activeCriteriaFilter = nil
	m.lens = OutlinerLensTree
	m.reviewMode = kind
	m.zoomStack = nil
	m.crossZoomStack = nil
	m.crossForceRebuild = true
	m.cursor = 0
	m.resetViewScrollOffset()
}

// buildReviewModeRows dispatches to the active review row builder and settles
// selections/cursor afterward. Shared by the single-project and cross-project
// rebuild paths so both surfaces render review modes identically.
func (m *OutlinerModel) buildReviewModeRows() {
	switch m.reviewMode {
	case reviewNone:
	case reviewDaily:
		m.buildDailyReviewRows()
	case reviewActivity:
		m.buildActivityRows()
	}
	m.pruneInvisibleSelections()
	m.clampCursor()
}

// reviewCompletionActive reports whether the persisted daily-review completion
// timestamp falls on today, mirroring the standalone Daily Review model's
// construction-time check.
func (m *OutlinerModel) reviewCompletionActive() bool {
	if m.runtime == nil {
		return false
	}
	if t := m.runtime.Planning().GetLastDailyReviewAt(); t != nil && isToday(*t) {
		return true
	}
	return false
}

// reviewGlobalScopeBlockers lists the active narrowings that prevent the global
// review from being marked complete. The global review is a workspace-wide
// checkpoint, so it may only be toggled when the Outliner shows every project
// with no filter, search, scope, or zoom narrowing the visible set.
func (m *OutlinerModel) reviewGlobalScopeBlockers() []string {
	var blockers []string
	if m.isProjectScoped() {
		blockers = append(blockers, "the project scope (Ctrl+X → All Projects)")
	}
	if !m.activeContext.IsAll() {
		blockers = append(blockers, "the active context ("+m.contextScopeLabel()+")")
	}
	if m.SearchQuery() != "" {
		blockers = append(blockers, "the search")
	}
	if m.activePinnedTagFilter() != "" {
		blockers = append(blockers, "the tag filter")
	}
	if hasActiveCommonFilters(&m.filter.CommonFilterState) {
		blockers = append(blockers, "the active filters")
	}
	if m.activeCriteriaFilter != nil {
		blockers = append(blockers, "the saved filter")
	}
	if len(m.filter.Sections) > 0 {
		blockers = append(blockers, "the section filter")
	}
	if m.filter.Actionable {
		blockers = append(blockers, "the actionable filter")
	}
	if len(m.crossZoomStack) > 0 || len(m.zoomStack) > 0 {
		blockers = append(blockers, "the task zoom")
	}
	return blockers
}

// handleReviewCompletionToggle drives the R key. It marks or resets the global
// daily review, but only in Daily Review mode at full workspace scope with no
// narrowing filters — otherwise it explains precisely what must be cleared so a
// narrowed subset can never accidentally complete the workspace-wide review.
func (m *OutlinerModel) handleReviewCompletionToggle() (tea.Model, tea.Cmd) {
	if m.reviewMode != reviewDaily {
		m.setStatus("Toggle review completion from Daily Review mode (P → Daily Review)")
		return m, nil
	}
	if blockers := m.reviewGlobalScopeBlockers(); len(blockers) > 0 {
		m.setStatus("Clear " + strings.Join(blockers, ", ") + " to complete the global review")
		return m, nil
	}
	if m.reviewCompletionActive() {
		return m, func() tea.Msg { return ResetDailyReviewMsg{} }
	}
	return m, func() tea.Msg { return MarkDailyReviewDoneMsg{} }
}

// reviewCollapsedFingerprint produces a stable term for the row-memoization
// guard so folding a review section forces a rebuild.
func reviewCollapsedFingerprint(collapsed map[string]bool) string {
	if len(collapsed) == 0 {
		return ""
	}
	keys := make([]string, 0, len(collapsed))
	for k, v := range collapsed {
		if v {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}
