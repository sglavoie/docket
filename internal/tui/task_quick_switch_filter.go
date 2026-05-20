package tui

import (
	"sort"

	"github.com/sglavoie/tt/internal/fuzzy"
)

// rebuildQuickSwitchFilter rebuilds the filtered indices based on current filter text.
func (m *AppModel) rebuildQuickSwitchFilter() {
	entries := m.recentTaskVisits
	if m.quickSwitchFilter == "" {
		m.quickSwitchFiltered = make([]int, 0, len(entries))
		m.quickSwitchTitlePositions = make([][]int, 0, len(entries))
		for i, entry := range entries {
			if entry.Task.Status.IsComplete() != m.quickSwitchShowCompleted {
				continue
			}
			m.quickSwitchFiltered = append(m.quickSwitchFiltered, i)
			m.quickSwitchTitlePositions = append(m.quickSwitchTitlePositions, nil)
		}
		return
	}

	type scored struct {
		idx            int
		score          int
		titlePositions []int
	}

	var pinned, unpinned []scored
	for i, entry := range entries {
		if entry.Task.Status.IsComplete() != m.quickSwitchShowCompleted {
			continue
		}
		text := entry.Task.Title + " " + entry.Project.Name
		score := fuzzy.ScoreTerms(text, m.quickSwitchFilter)
		if score <= 0 {
			continue
		}
		_, titlePos := fuzzy.ScoreTermsWithPositions(entry.Task.Title, m.quickSwitchFilter)
		scoredEntry := scored{idx: i, score: score, titlePositions: titlePos}
		if entry.Pinned {
			pinned = append(pinned, scoredEntry)
		} else {
			unpinned = append(unpinned, scoredEntry)
		}
	}

	// Stable sort + idx tiebreak so equal-score entries preserve recency order
	// (entries are already in recency order; sort.Slice is not stable).
	sort.SliceStable(pinned, func(i, j int) bool {
		if pinned[i].score != pinned[j].score {
			return pinned[i].score > pinned[j].score
		}
		return pinned[i].idx < pinned[j].idx
	})
	sort.SliceStable(unpinned, func(i, j int) bool {
		if unpinned[i].score != unpinned[j].score {
			return unpinned[i].score > unpinned[j].score
		}
		return unpinned[i].idx < unpinned[j].idx
	})

	all := append(pinned, unpinned...)
	m.quickSwitchFiltered = make([]int, 0, len(all))
	m.quickSwitchTitlePositions = make([][]int, 0, len(all))
	for _, entry := range all {
		m.quickSwitchFiltered = append(m.quickSwitchFiltered, entry.idx)
		m.quickSwitchTitlePositions = append(m.quickSwitchTitlePositions, entry.titlePositions)
	}
}

// quickSwitchFilteredLen returns the number of visible (filtered) entries.
func (m *AppModel) quickSwitchFilteredLen() int {
	return len(m.quickSwitchFiltered)
}

// quickSwitchClampCursor clamps the cursor to the filtered list bounds.
func (m *AppModel) quickSwitchClampCursor() {
	n := m.quickSwitchFilteredLen()
	if n == 0 {
		m.quickSwitchCursor = 0
	} else if m.quickSwitchCursor >= n {
		m.quickSwitchCursor = n - 1
	}
}
