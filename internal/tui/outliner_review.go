package tui

import (
	"fmt"
	"sort"
	"strings"
)

// reviewCollapseKeyPrefix marks a section-header CollapseKey as belonging to a
// review mode so the fold routing reads/writes reviewCollapsedSections instead
// of the persisted collapsedSections map.
const reviewCollapseKeyPrefix = "_rv:"

// reviewCollapseKey builds the CollapseKey sentinel for one review section.
// The kind is embedded so Daily Review and Activity headers with the same
// index never share fold state.
func reviewCollapseKey(mode OutlinerModeID, idx int) string {
	return fmt.Sprintf("%s%s:%d", reviewCollapseKeyPrefix, mode, idx)
}

// isReviewCollapseKey reports whether a CollapseKey belongs to a review section.
func isReviewCollapseKey(key string) bool {
	return strings.HasPrefix(key, reviewCollapseKeyPrefix)
}

// buildReviewModeRows dispatches to the active review row builder and settles
// selections/cursor afterward. Shared by the single-project and cross-project
// rebuild paths so both surfaces render review modes identically.
func (m *OutlinerModel) buildReviewModeRows() {
	switch m.primaryMode {
	case ModeDailyReview:
		m.buildDailyReviewRows()
	case ModeActivity:
		m.buildActivityRows()
	default:
		return
	}
	m.pruneInvisibleSelections()
	m.clampCursor()
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
